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
// Bound at DECLARATION, not at enforce, so promoting a phase from shadow to
// enforce can never open the gap: by the time anyone flips the stage word, this
// test has already required the teaching side to exist.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		t.Errorf("no tracked phase declares classify.verdict_from_sentinel: the judgment-verdict wiring is inert, and a fix nothing exercises is the defect it was written to cure")
	}
}

// trackedPhaseDirsForTest returns the git-tracked phase dirs, or nil when there
// is no usable git context (then the caller binds every dir — the stricter
// fallback). Mirrors phasespec's TrackedPhaseDirs, which lives in that package's
// test binary and so cannot be imported here.
func trackedPhaseDirsForTest(t *testing.T, root string) map[string]bool {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "ls-files", "--", ".evolve/phases").Output()
	if err != nil {
		return nil
	}
	tracked := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, "/")
		if len(parts) >= 3 && parts[0] == ".evolve" && parts[1] == "phases" {
			tracked[parts[2]] = true
		}
	}
	if len(tracked) == 0 {
		return nil
	}
	return tracked
}
