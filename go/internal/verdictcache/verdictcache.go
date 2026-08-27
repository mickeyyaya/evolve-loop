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

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
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

// ProbeEligible reports whether candidateTreeSHA may be used as a verdict-cache
// key given the worktree's base tree identity. It is the SINGLE source of the
// fresh-base collision guard, shared by the ADR-0048 Slice B pre-loop shadow
// probe (Lookup) and the audit-binding projection (Put), so the two sites can
// never drift apart and a future enforce-stage lookup has one predicate to
// reuse.
//
// A candidate with no content identity is never eligible — there is nothing to
// content-address, matching Lookup/Put's empty-SHA no-ops. A candidate equal to
// a RESOLVED base tree is a fresh/untouched worktree: every sibling lane cut
// from the same base carries that identity, so a verdict stored or matched
// under it collides across unrelated cycles. An unresolvable base ("") leaves
// the candidate eligible — absence of a base identity is not evidence of
// freshness, and the pre-guard behaviour there is deliberately frozen.
func ProbeEligible(baseTreeSHA, candidateTreeSHA string) bool {
	if candidateTreeSHA == "" {
		return false
	}
	return baseTreeSHA == "" || candidateTreeSHA != baseTreeSHA
}

// Reusable reports whether a verdict may let a later run skip work on its
// strength. PASS and WARN ship (WARN by the fluent-audit default); FAIL is a
// rejection and SKIPPED means no audit ran, so neither may be carried forward,
// and an out-of-vocabulary string is never trusted.
//
// This is the SINGLE definition of the cacheable/carry-forward vocabulary.
// Before cycle-1571 it existed three times over: documented on Entry.Verdict,
// enforced nowhere by the store, and hand-written as `verdict == VerdictFAIL`
// at the binding call site — while the RUNG 0 composition snapshot, which needs
// the same rule, consulted no verdict at all and would carry a REJECTED audit
// forward. One predicate, so the three sites cannot drift (§3.5).
func Reusable(verdict string) bool {
	// The canonical constants, not literals: this predicate exists so the
	// vocabulary has ONE definition, and re-typing the words here would leave a
	// copy that can drift from the enum every other reader keys on. cyclestate
	// is a zero-dependency leaf (verified: it does not import this package), so
	// the edge is safe — importing core would not be.
	return verdict == cyclestate.VerdictPASS || verdict == cyclestate.VerdictWARN
}

// Put upserts e keyed by e.TreeSHA (read-modify-write + temp+rename). An empty
// TreeSHA is a no-op (a verdict with no content identity cannot be
// content-addressed) — never an error, so a best-effort caller need not branch.
//
// A non-Reusable verdict is REFUSED: this store is shared, its entries outlive
// the cycle that wrote them, and the enforce-stage lookup would skip real work
// on a rejected tree's strength. The write side fails closed on purpose — the
// same asymmetry the binding call site documents.
func (s *Store) Put(e Entry) error {
	if e.TreeSHA == "" {
		return nil
	}
	if !Reusable(e.Verdict) {
		return fmt.Errorf("verdictcache: refusing to cache verdict %q for tree %s — only PASS/WARN are reusable", e.Verdict, e.TreeSHA)
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
