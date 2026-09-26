package phasespec

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
			continue
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

func TestRepoPhaseCatalog_VerdictFromSentinelStageIsKnown(t *testing.T) {
	t.Parallel()
	// Copies specrunner's stage words: specrunner imports phasespec, so importing them back would cycle.
	known := map[string]bool{"": true, "shadow": true, "enforce": true}
	eachTrackedPhaseClassify(t, func(path string, c *ClassifyRules) {
		if c != nil && !known[c.VerdictFromSentinel] {
			t.Errorf("%s declares classify.verdict_from_sentinel=%q — EvaluateClassify FAILs this phase unconditionally at runtime. Use \"\" (off), \"shadow\" or \"enforce\".", path, c.VerdictFromSentinel)
		}
	})
}

// eachTrackedPhaseClassify calls fn with each git-tracked phase.json's classify rules, skipping when the catalog is absent.
func eachTrackedPhaseClassify(t *testing.T, fn func(path string, c *ClassifyRules)) {
	t.Helper()
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
			t.Errorf("%s: unparseable phase.json: %v", path, jerr)
			continue
		}
		checked++
		fn(path, cfg.Classify)
	}
	if checked == 0 {
		t.Skip("no phase.json files found — catalog layout moved?")
	}
}
