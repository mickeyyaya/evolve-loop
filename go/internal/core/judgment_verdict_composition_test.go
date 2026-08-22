package core

// judgment_verdict_composition_test.go — the coupling between "a judgment phase
// can now FAIL a cycle" and "a judgment FAIL teaches the next one".
//
// Two changes have to compose. specrunner's verdict_from_sentinel lets a
// judgment phase's stated verdict become the routed verdict; judgment_lesson.go
// turns such a FAIL into a carryover lesson instead of a silent halt. Enable the
// first on a phase the second does not know about and the loop gets the worst of
// both: a cycle stopped by an objection that leaves no trace, so the next cycle
// re-derives the same falsified premise — exactly the gap judgment_lesson.go was
// written to close.
//
// Bound at DECLARATION, not at enforce, so promoting a TRACKED catalog phase
// from shadow to enforce can never open the gap: by the time anyone flips the
// stage word, this test has already required the teaching side to exist.
//
// Scope limit, stated because docs/architecture/user-defined-phases.md now
// documents the key for user phases: this binds git-TRACKED catalog phases only.
// A runtime-minted, untracked phase that declares the key gets neither this
// guard nor phasespec's stage-word guard, and judgmentTeachingPhases has no
// runtime enforcement (recordJudgmentLesson simply no-ops). That is the residual
// gap; the author docs carry the requirement in prose.

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
		// Deliberately NOT an error. Setting both phases back to "" is the
		// designed CONFIG rollback for this feature; failing the build on it
		// would convert a config action into a code change, against
		// phase_settings_from_config_not_code. Inertness still surfaces — the
		// shadow records simply stop appearing in the run workspaces.
		t.Log("no tracked phase declares classify.verdict_from_sentinel — the judgment-verdict wiring is currently inert (expected only if it was deliberately rolled back)")
	}
}

// trackedPhaseDirsForTest returns the git-tracked phase dirs, or nil when there
// is no usable git context (then the caller binds every dir — the stricter
// fallback).
//
// Builds on repostate.TrackedFiles, the production primitive, rather than
// shelling out to git again: a second hand-rolled implementation drifted from
// phasespec.TrackedPhaseDirs on the definition of "tracked" (any file under the
// dir, versus the phase.json itself), so a dir with a tracked agent.md and an
// untracked phase.json bound in one guard and not the other. Same primitive,
// same predicate, one meaning.
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
