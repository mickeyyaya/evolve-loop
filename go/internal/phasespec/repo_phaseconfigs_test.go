package phasespec

// repo_phaseconfigs_test.go — authoring-time guard for the repo's tracked
// phase catalog (cycle-263 incident). The cycle-241 declared-semantics
// rejection deliberately FAILs any phase whose classify rules carry
// fail_if_signal (the Stage-3 signal bus does not exist, so the gate is
// inert — silently passing it would let an authoring mistake reach runtime
// undetected). Correct invariant, wrong enforcement boundary: 15 catalog
// phases shipped WITH the inert gate, so the rejection fired mid-cycle on
// first router insertion (adversarial-review in cycle-263 — a perfect PASS
// report recorded as FAIL, cycle dead, ~$ and ~30 min burned). This test
// moves the same invariant to CI: a mis-authored phase config fails the
// BUILD, never a production cycle. Delete this test when the Stage-3 signal
// bus lands and EvaluateClassify actually evaluates the gate.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoPhaseCatalog_NoInertFailIfSignal(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	dir := filepath.Join(root, ".evolve", "phases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("phase catalog not present at %s: %v", dir, err)
	}
	// Bind only git-TRACKED phase dirs: untracked dirs are runtime/local
	// state that can never reach a CI checkout (cd49274beab2 class); nil
	// tracked set = no usable git context = bind all (stricter fallback).
	tracked := TrackedPhaseDirs(t, root)
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if tracked != nil && !tracked[e.Name()] {
			t.Logf("skipping .evolve/phases/%s: phase.json untracked — runtime/local state, not repo config", e.Name())
			continue
		}
		path := filepath.Join(dir, e.Name(), "phase.json")
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // phase dirs without phase.json are someone else's problem
		}
		var cfg struct {
			Classify *ClassifyRules `json:"classify"`
		}
		if jerr := json.Unmarshal(data, &cfg); jerr != nil {
			t.Errorf("%s: unparseable phase.json: %v", path, jerr)
			continue
		}
		checked++
		if cfg.Classify != nil && len(cfg.Classify.FailIfSignal) > 0 {
			t.Errorf("%s declares fail_if_signal — the Stage-3 signal bus does not exist, so EvaluateClassify unconditionally FAILs this phase at runtime (cycle-241 rejection, cycle-263 incident). Remove the gate or land the signal bus first.", path)
		}
	}
	if checked == 0 {
		t.Skip("no phase.json files found — catalog layout moved?")
	}
}

// TestRepoPhaseCatalog_VerdictFromSentinelStageIsKnown catches a typo'd rollout
// stage at AUTHORING time rather than mid-cycle.
//
// EvaluateClassify hard-FAILs an unknown verdict_from_sentinel word on purpose
// (an inert gate must fail loudly, cycle-241) — but discovering that from a
// dead cycle costs a dispatch and an operator's afternoon. This is the same
// belt-and-braces pairing the fail_if_signal guard above already uses: the
// runtime rejection is the floor, this is the tripwire.
func TestRepoPhaseCatalog_VerdictFromSentinelStageIsKnown(t *testing.T) {
	t.Parallel()
	// Mirrors specrunner's stage words. Duplicated as literals rather than
	// imported because phasespec is the LEAF here — specrunner imports it, so
	// importing back would cycle. The catalog test above makes the same trade.
	known := map[string]bool{"": true, "shadow": true, "enforce": true}

	root := filepath.Join("..", "..", "..")
	dir := filepath.Join(root, ".evolve", "phases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("phase catalog not present at %s: %v", dir, err)
	}
	tracked := TrackedPhaseDirs(t, root)
	checked := 0
	for _, e := range entries {
		if !e.IsDir() || (tracked != nil && !tracked[e.Name()]) {
			continue
		}
		path := filepath.Join(dir, e.Name(), "phase.json")
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		var cfg struct {
			Classify *ClassifyRules `json:"classify"`
		}
		if jerr := json.Unmarshal(data, &cfg); jerr != nil {
			continue // the sibling test reports unparseable catalogs
		}
		checked++
		if cfg.Classify != nil && !known[cfg.Classify.VerdictFromSentinel] {
			t.Errorf("%s declares classify.verdict_from_sentinel=%q — EvaluateClassify FAILs this phase unconditionally at runtime. Use \"\" (off), \"shadow\" or \"enforce\".", path, cfg.Classify.VerdictFromSentinel)
		}
	}
	if checked == 0 {
		t.Skip("no phase.json files found — catalog layout moved?")
	}
}
