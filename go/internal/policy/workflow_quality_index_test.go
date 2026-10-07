package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

func qualityIndexFrom(t *testing.T, raw string) QualityIndexConfig {
	t.Helper()
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return p.WorkflowConfig().QualityIndex
}

func TestQualityIndex_CompiledDefaultIsTheBarOfFourEverywhere(t *testing.T) {
	for _, raw := range []string{`{}`, `{"workflow":{}}`, `{"workflow":{"quality_index":{}}}`} {
		got := qualityIndexFrom(t, raw)
		for _, k := range qualityindex.Keys() {
			if got.Thresholds[k] != qualityindex.DefaultThreshold {
				t.Errorf("%s: threshold %s = %d, want %d", raw, k, got.Thresholds[k], qualityindex.DefaultThreshold)
			}
		}
		if len(got.Warnings) != 0 {
			t.Errorf("%s warned %q", raw, got.Warnings)
		}
	}
}

func TestQualityIndex_ThresholdsResolveThroughTheIndexOwnRule(t *testing.T) {
	got := qualityIndexFrom(t, `{"workflow":{"quality_index":{"thresholds":{"*":3,"security":5,"speed":2}}}}`)
	if got.Thresholds["correctness"] != 3 || got.Thresholds["security"] != 5 {
		t.Errorf("thresholds = %v, want * = 3 with security at 5", got.Thresholds)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "speed") {
		t.Errorf("warnings = %q, want one for the unknown dimension", got.Warnings)
	}
}

func TestQualityIndexThresholdsFor_ReadsTheProjectPolicyAndNeverSwallowsALoadError(t *testing.T) {
	root := t.TempDir()
	if got, err := QualityIndexThresholdsFor(root); err != nil || got["performance"] != qualityindex.DefaultThreshold {
		t.Errorf("no policy file: performance = %d (err %v), want the default and no error", got["performance"], err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"workflow":{"quality_index":{"thresholds":{"performance":2}}}}`)
	if got, err := QualityIndexThresholdsFor(root); err != nil || got["performance"] != 2 || got["security"] != qualityindex.DefaultThreshold {
		t.Errorf("configured policy: %v (err %v), want performance 2 and the rest at the default", got, err)
	}
	write(`{not json`)
	got, err := QualityIndexThresholdsFor(root)
	if err == nil || !strings.Contains(err.Error(), "policy.json") {
		t.Errorf("malformed policy: err = %v, want the load error naming policy.json", err)
	}
	if got != nil {
		t.Errorf("malformed policy: thresholds = %v, want none beside the error: no caller reads a default it was told is wrong", got)
	}
}

func TestQualityIndex_ABlockBuiltInCodeResolvesLikeTheDecodedOne(t *testing.T) {
	built := Policy{Workflow: &WorkflowPolicy{QualityIndex: &QualityIndexPolicy{Thresholds: map[string]int{"concurrency": 5}}}}
	if got := built.WorkflowConfig().QualityIndex; got.Thresholds["concurrency"] != 5 || got.Thresholds["correctness"] != qualityindex.DefaultThreshold || len(got.Warnings) != 0 {
		t.Errorf("built block resolves %+v, want concurrency 5 and the rest at the default", got)
	}
}
