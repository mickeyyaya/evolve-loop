package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestParse_TheFixabilityStageIsOffShadowOrEnforce(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]config.Stage{
		`{}`:                           config.StageShadow,
		`{"workflow": {}}`:             config.StageShadow,
		`{"workflow": {"repair": {}}}`: config.StageShadow,
		`{"workflow": {"repair": {"fixability": {}}}}`:                   config.StageShadow,
		`{"workflow": {"repair": {"fixability": {"stage": "enforce"}}}}`: config.StageEnforce,
		`{"workflow": {"repair": {"fixability": {"stage": "off"}}}}`:     config.StageOff,
	} {
		p, err := Parse([]byte(body))
		if err != nil || p.RepairFixabilityStage() != want || p.WorkflowConfig().RepairFixability != want {
			t.Errorf("%s: stage = %v, %v; want %v", body, p.RepairFixabilityStage(), err, want)
		}
	}
	built := Policy{Workflow: &WorkflowPolicy{Repair: &RepairPolicy{Fixability: &RepairFixabilityPolicy{Stage: "advisory"}}}}
	if built.RepairFixabilityStage() != config.StageShadow {
		t.Error("a policy built in code with an unknown stage must hold shadow, not off")
	}
	_, err := Parse([]byte(`{"workflow": {"repair": {"fixability": {"stage": "advisory"}}}}`))
	if err == nil || !strings.Contains(err.Error(), "workflow.repair.fixability.stage") {
		t.Errorf("err = %v, want a refusal that names the key", err)
	}
}

func TestRepairFixabilityStageFor_ReadsThePolicyOfTheRootAndHoldsShadowOnARefusal(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]config.Stage{
		`{"workflow": {"repair": {"fixability": {"stage": "enforce"}}}}`: config.StageEnforce,
		`{"workflow": {"repair": {"fixability": {"stage": "on"}}}}`:      config.StageShadow,
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		var warn strings.Builder
		got := RepairFixabilityStageFor(root, "test", &warn)
		warned := strings.Contains(warn.String(), "[test] WARN policy: workflow.repair.fixability")
		if got != want || warned != (want == config.StageShadow) {
			t.Errorf("%s: stage = %v (warned %t); want %v", body, got, warned, want)
		}
	}
}
