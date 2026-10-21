package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func TestQualityStages_ReadsEachIndexStageFromPolicy(t *testing.T) {
	t.Parallel()

	p, err := Parse([]byte(`{"quality":{"indexes":{"TQ1":{"stage":"report"},"PQ1":{"stage":"shadow"}}}}`))

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := p.QualityStages()
	if len(got) != 2 || got["TQ1"] != qualitypanel.StageReport || got["PQ1"] != qualitypanel.StageShadow {
		t.Errorf("stages = %v, want TQ1 report and PQ1 shadow", got)
	}
}

func TestQualityStages_NoBlockGivesNoOverride(t *testing.T) {
	t.Parallel()
	if got := (Policy{}).QualityStages(); len(got) != 0 {
		t.Errorf("stages = %v, want none", got)
	}
}

func TestQualityPolicy_StrictDecodeRefusesEachMistake(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"unknown index":     `{"quality":{"indexes":{"ZQ9":{"stage":"report"}}}}`,
		"unknown stage":     `{"quality":{"indexes":{"PQ1":{"stage":"enforce"}}}}`,
		"unknown index key": `{"quality":{"indexes":{"PQ1":{"stage":"report","threshold":0.5}}}}`,
		"unknown block key": `{"quality":{"index":{}}}`,
	}
	for name, raw := range cases {
		if _, err := Parse([]byte(raw)); err == nil || !strings.Contains(err.Error(), "quality") {
			t.Errorf("%s: err = %v, want a quality error", name, err)
		}
	}
}

func TestQualityStagesFor_LoadsThePolicyFileOrReturnsItsError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got, err := QualityStagesFor(root); err != nil || len(got) != 0 {
		t.Errorf("no policy file: %v, %v; want no override and no error", got, err)
	}
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(`{"quality":{"indexes":{"OQ2":{"stage":"gate"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := QualityStagesFor(root); err != nil || got["OQ2"] != qualitypanel.StageGate {
		t.Errorf("stages = %v, %v; want OQ2 gate", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(`{"quality":{"indexes":{"OQ2":{"stage":"on"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := QualityStagesFor(root); err == nil {
		t.Error("a bad stage must return the policy error")
	}
}

func TestQualityPanelPolicy_DecodesOnItsOwn(t *testing.T) {
	t.Parallel()
	var q QualityPanelPolicy

	err := json.Unmarshal([]byte(`{"indexes":{"JQ3":{"stage":"shadow"}}}`), &q)

	if err != nil || len(q.Stages) != 1 || q.Stages["JQ3"] != qualitypanel.StageShadow {
		t.Errorf("QualityPanelPolicy = %+v, %v; want JQ3 shadow", q, err)
	}
}
