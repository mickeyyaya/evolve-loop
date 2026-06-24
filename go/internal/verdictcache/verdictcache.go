// Package verdictcache is the ADR-0048 Slice B content-addressed audit-reuse
// store: it binds an audit PASS/WARN verdict to the CONTENT it audited, keyed by
// the worktree tree SHA (git write-tree of the staged changes) — the same
// content identity ship verifies in the pre-commit binding (ADR-0048 Slice C1).
//
// The cache is ADVISORY and SELF-INVALIDATING: a lookup miss costs a full
// tdd/build/audit run, so a lost, stale, or corrupt cache only ever costs time,
// never correctness — the same degradation contract as clihealth. Persistence,
// atomic write, and degrade-to-empty-on-corrupt mirror that package.
//
// NOTE on the ADR (ADR-0048 Slice B): the ADR keys on (tree_sha, inputs_digest).
// inputs_digest is NOT a machine-computed value anywhere in the codebase (it is
// only a parse-only field in agent-emitted phasecoherence Markdown). The shadow
// stage therefore keys on the worktree tree SHA alone — already computed and
// ledger-persisted by recordAuditBinding. inputs_digest (to also pin eval-set /
// goal identity) is a refinement deferred to the enforce stage, where actually
// SKIPPING phases on a match demands the stronger key.
package verdictcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/log"
)

// schemaVersion is the on-disk format version of .evolve/verdict-cache.json.
const schemaVersion = 1

// Entry is one cached audit verdict bound to the content it audited.
type Entry struct {
	TreeSHA        string    `json:"tree_sha"`        // git write-tree of the staged worktree changes — the content key
	Cycle          int       `json:"cycle"`           // the cycle that produced the verdict (provenance, not key)
	Verdict        string    `json:"verdict"`         // PASS or WARN
	ArtifactSHA256 string    `json:"artifact_sha256"` // sha256 of the audit-report.md it bound to
	ArtifactPath   string    `json:"artifact_path"`   // path to that audit-report.md
	CachedAt       time.Time `json:"cached_at"`       // when Put stamped it
}

// fileSchema is the on-disk shape of .evolve/verdict-cache.json.
type fileSchema struct {
	SchemaVersion int              `json:"schema_version"`
	Verdicts      map[string]Entry `json:"verdicts"` // keyed by Entry.TreeSHA
}

// Store reads and writes the verdict cache. now is injectable for tests; nil
// means time.Now.
type Store struct {
	path string
	now  func() time.Time
}

// NewStore returns a Store rooted at <projectRoot>/.evolve/verdict-cache.json.
func NewStore(projectRoot string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{path: filepath.Join(projectRoot, ".evolve", "verdict-cache.json"), now: now}
}

// Load returns all cached verdicts keyed by tree SHA. Missing or corrupt file
// degrades to an empty map with a stderr WARN — the cache must never break a
// cycle (a miss just means a full run).
func (s *Store) Load() (map[string]Entry, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Entry{}, nil
		}
		log.Default().Warnf("[verdictcache] WARN read %s: %v (treating as empty)\n", s.path, err)
		return map[string]Entry{}, nil
	}
	var f fileSchema
	if err := json.Unmarshal(b, &f); err != nil {
		log.Default().Warnf("[verdictcache] WARN corrupt %s: %v (treating as empty)\n", s.path, err)
		return map[string]Entry{}, nil
	}
	if f.Verdicts == nil {
		return map[string]Entry{}, nil
	}
	return f.Verdicts, nil
}

// Put upserts e keyed by e.TreeSHA (read-modify-write + temp+rename). An empty
// TreeSHA is a no-op (a verdict with no content identity cannot be
// content-addressed) — never an error, so a best-effort caller need not branch.
func (s *Store) Put(e Entry) error {
	if e.TreeSHA == "" {
		return nil
	}
	if e.CachedAt.IsZero() {
		e.CachedAt = s.now().UTC()
	}
	verdicts, _ := s.Load()
	verdicts[e.TreeSHA] = e
	return s.write(verdicts)
}

// Lookup returns the cached verdict for treeSHA, if any. An empty treeSHA never
// hits (no content identity to match).
func (s *Store) Lookup(treeSHA string) (Entry, bool) {
	if treeSHA == "" {
		return Entry{}, false
	}
	verdicts, _ := s.Load()
	e, ok := verdicts[treeSHA]
	return e, ok
}

func (s *Store) write(verdicts map[string]Entry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("verdictcache: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(fileSchema{SchemaVersion: schemaVersion, Verdicts: verdicts}, "", "  ")
	if err != nil {
		return fmt.Errorf("verdictcache: marshal: %w", err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", s.path, os.Getpid())
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("verdictcache: write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("verdictcache: rename: %w", err)
	}
	return nil
}
