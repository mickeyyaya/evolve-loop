package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

func TestJudgmentTeachingPhases_CoverEveryPhaseThatCanStateItsOwnVerdict(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	dir := filepath.Join(root, ".evolve", "phases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("phase catalog not present at %s: %v", dir, err)
	}
	tracked := trackedPhaseDirsForTest(t, root)

	checked, declaring := 0, 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if tracked != nil && !tracked[e.Name()] {
			t.Logf("skipping .evolve/phases/%s: untracked — runtime/local state, never in a CI checkout", e.Name())
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name(), "phase.json"))
		if rerr != nil {
			continue
		}
		var cfg struct {
			Name     string `json:"name"`
			Classify struct {
				VerdictFromSentinel string `json:"verdict_from_sentinel"`
			} `json:"classify"`
		}
		if jerr := json.Unmarshal(data, &cfg); jerr != nil {
			t.Errorf(".evolve/phases/%s/phase.json: unparseable: %v", e.Name(), jerr)
			continue
		}
		checked++
		stage := cfg.Classify.VerdictFromSentinel
		if stage == "" {
			continue
		}
		declaring++
		name := cfg.Name
		if name == "" {
			name = e.Name()
		}
		if !judgmentTeachingPhases[Phase(name)] {
			t.Errorf("phase %q declares classify.verdict_from_sentinel=%q — its stated FAIL can become the cycle's verdict — but it is not in judgmentTeachingPhases, so that FAIL would teach nothing and the next cycle would re-derive the same objection. Add it to judgmentTeachingPhases (judgment_lesson.go) or drop the key.", name, stage)
		}
	}
	if checked == 0 {
		t.Skip("no phase.json files found — catalog layout moved?")
	}
	if declaring == 0 {
		// Deliberately not an error: setting the key back to "" is a config
		// rollback, and failing the build on it would turn a config action
		// into a code change.
		t.Log("no tracked phase declares classify.verdict_from_sentinel — the judgment-verdict wiring is currently inert (expected only if it was deliberately rolled back)")
	}
}

// trackedPhaseDirsForTest returns the git-tracked phase dirs, or nil when
// there is no usable git context (then the caller binds every dir, the
// stricter fallback). It builds on repostate.TrackedFiles, the production
// primitive, rather than a second hand-rolled "tracked" definition that once
// drifted from it.
func trackedPhaseDirsForTest(t *testing.T, root string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".evolve", "phases"))
	if err != nil {
		return nil
	}
	tracked := map[string]bool{}
	for _, e := range entries {
		files, ferr := repostate.TrackedFiles(root, filepath.Join(".evolve", "phases", e.Name()))
		if ferr != nil {
			return nil
		}
		for _, f := range files {
			if filepath.Base(f) == "phase.json" {
				tracked[e.Name()] = true
			}
		}
	}
	if len(tracked) == 0 {
		return nil
	}
	return tracked
}
