package policy

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestParse_TheBuilderSelfCheckStageDefaultsToShadow(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"workflow": {}}`, `{"workflow": {"builder_selfcheck": {}}}`} {
		p, err := Parse([]byte(body))
		c := p.WorkflowConfig().BuilderSelfCheck
		if err != nil || c.Stage != config.StageShadow {
			t.Errorf("%s: config = %+v, %v", body, c, err)
		}
	}
}

func TestParse_TheBuilderSelfCheckStageReadsEachStage(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]config.Stage{"off": config.StageOff, "shadow": config.StageShadow, "enforce": config.StageEnforce} {
		p, err := Parse([]byte(`{"workflow": {"builder_selfcheck": {"stage": "` + body + `"}}}`))
		if got := p.WorkflowConfig().BuilderSelfCheck.Stage; err != nil || got != want {
			t.Errorf("stage %s: got %v, %v; want %v", body, got, err, want)
		}
	}
}

func TestParse_TheBuilderSelfCheckRefusesAnUnknownStageOrKey(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"workflow": {"builder_selfcheck": {"stage": "advisory"}}}`,
		`{"workflow": {"builder_selfcheck": {"mode": "enforce"}}}`,
	} {
		if _, err := Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), "builder_selfcheck") {
			t.Errorf("%s: err = %v, want a refusal that names workflow.builder_selfcheck", body, err)
		}
	}
}

func TestBuilderSelfCheckConfig_ABuiltPolicyWithAnUnknownStageHoldsShadow(t *testing.T) {
	t.Parallel()
	p := Policy{Workflow: &WorkflowPolicy{BuilderSelfCheck: &BuilderSelfCheckPolicy{Stage: "advisory"}}}
	if got := p.builderSelfCheckConfig(); got != (BuilderSelfCheckConfig{Stage: config.StageShadow}) {
		t.Errorf("config = %+v, want the shadow stage", got)
	}
}
