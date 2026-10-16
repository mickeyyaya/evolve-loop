// Package verdictcache is the content-addressed store that binds a reusable audit verdict to the tree SHA it judged.
// See docs/architecture/packages/internal-verdictcache.md.
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

const schemaVersion = 1

type Entry struct {
	TreeSHA        string    `json:"tree_sha"`
	Cycle          int       `json:"cycle"`
	Verdict        string    `json:"verdict"`
	ArtifactSHA256 string    `json:"artifact_sha256"`
	ArtifactPath   string    `json:"artifact_path"`
	CachedAt       time.Time `json:"cached_at"`
}

type fileSchema struct {
	SchemaVersion int              `json:"schema_version"`
	Verdicts      map[string]Entry `json:"verdicts"`
}

type Store struct {
	path string
	now  func() time.Time
}

func NewStore(projectRoot string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{path: filepath.Join(projectRoot, ".evolve", "verdict-cache.json"), now: now}
}

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

func ProbeEligible(baseTreeSHA, candidateTreeSHA string) bool {
	if candidateTreeSHA == "" {
		return false
	}
	return baseTreeSHA == "" || candidateTreeSHA != baseTreeSHA
}

func Reusable(verdict string) bool {
	return verdict == cyclestate.VerdictPASS || verdict == cyclestate.VerdictWARN
}

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
