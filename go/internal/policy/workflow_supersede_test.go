package policy

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestParse_TheSupersedeStageDefaultsToShadow(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"workflow": {}}`, `{"workflow": {"supersede": {}}}`} {
		p, err := Parse([]byte(body))
		var c SupersedeConfig = p.WorkflowConfig().Supersede
		if err != nil || c.Stage != config.StageShadow {
			t.Errorf("%s: config = %+v, %v", body, c, err)
		}
	}
}

func TestParse_TheSupersedeStageReadsEachStage(t *testing.T) {
	t.Parallel()
	for word, want := range map[string]config.Stage{"0": config.StageOff, "off": config.StageOff, "shadow": config.StageShadow, "enforce": config.StageEnforce} {
		p, err := Parse([]byte(`{"workflow": {"supersede": {"stage": "` + word + `"}}}`))
		if got := p.WorkflowConfig().Supersede.Stage; err != nil || got != want {
			t.Errorf("stage %s: got %v, %v; want %v", word, got, err, want)
		}
	}
}

func TestParse_TheSupersedeBlockRefusesAnUnknownStageOrKey(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"workflow": {"supersede": {"stage": "advisory"}}}`,
		`{"workflow": {"supersede": {"mode": "enforce"}}}`,
	} {
		if _, err := Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), "supersede") {
			t.Errorf("%s: err = %v, want a refusal that names supersede", body, err)
		}
	}
}

func TestSupersedeConfig_ABuiltPolicyWithAnUnknownStageHoldsShadow(t *testing.T) {
	t.Parallel()
	p := Policy{Workflow: &WorkflowPolicy{Supersede: &SupersedePolicy{Stage: "advisory"}}}
	if got := p.supersedeConfig(); got != (SupersedeConfig{Stage: config.StageShadow}) {
		t.Errorf("config = %+v, want the shadow stage", got)
	}
	if err := (*WorkflowPolicy)(nil).validateSupersede(); err != nil {
		t.Errorf("a nil workflow block refused: %v", err)
	}
}
