package policy

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestRegressionProofStage_DefaultsToShadowAndHonorsOff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		block *RegressionProofPolicy
		want  config.Stage
	}{{nil, config.StageShadow}, {&RegressionProofPolicy{}, config.StageShadow}, {&RegressionProofPolicy{Stage: "shadow"}, config.StageShadow}, {&RegressionProofPolicy{Stage: "off"}, config.StageOff}}
	for _, c := range cases {
		if got := (Policy{RegressionProof: c.block}).RegressionProofStage(); got != c.want {
			t.Errorf("RegressionProofStage with %+v = %v, want %v", c.block, got, c.want)
		}
	}
}

func TestRegressionProofPolicy_RefusesAStageOutsideOffAndShadowAndAnUnknownKey(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"regression_proof":{"stage":"enforce"}}`, `{"regression_proof":{"stage":"Shadow"}}`, `{"regression_proof":{"stgae":"off"}}`} {
		if _, err := Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), "regression_proof") {
			t.Errorf("Parse(%s) = %v, want a regression_proof refusal", body, err)
		}
	}
	if p, err := Parse([]byte(`{"regression_proof":{"stage":"off"}}`)); err != nil || p.RegressionProofStage() != config.StageOff {
		t.Fatalf("Parse(off) = %v, stage %v; want off", err, p.RegressionProofStage())
	}
}

func TestRegressionProofStageFor_ReadsThePolicyAndReturnsTheLoadError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		root    string
		want    config.Stage
		failure bool
	}{
		{writePolicyJSON(t, `{"regression_proof":{"stage":"off"}}`), config.StageOff, false},
		{writePolicyJSON(t, `{"regression_proof":{"stage":"shadow"}}`), config.StageShadow, false},
		{writePolicyJSON(t, `{"regression_proof":`), config.StageShadow, true},
		{t.TempDir(), config.StageShadow, false},
	}
	for _, c := range cases {
		got, err := RegressionProofStageFor(c.root)
		if got != c.want || (err != nil) != c.failure || err != nil && !strings.Contains(err.Error(), "regression_proof") {
			t.Errorf("RegressionProofStageFor(%s) = %v, %v; want %v and failure %v", c.root, got, err, c.want, c.failure)
		}
	}
}

func TestRegressionProofPolicy_TheBlockIsStrictDecoded(t *testing.T) {
	t.Parallel()
	var block RegressionProofPolicy

	if err := block.UnmarshalJSON([]byte(`{"stgae":"off"}`)); err == nil || !strings.Contains(err.Error(), "regression_proof") {
		t.Fatalf("UnmarshalJSON of an unknown key = %v, want a regression_proof refusal", err)
	}
}
