package config

import "testing"

func TestDefaults_MergeGate_Shadow(t *testing.T) {
	if got := defaults().RolloutStages.MergeGate; got != StageShadow {
		t.Errorf("default RolloutStages.MergeGate = %v, want StageShadow", got)
	}
}

func TestMergeGate_StageLadder(t *testing.T) {
	var ws []Warning
	for in, want := range map[string]Stage{
		"off":      StageOff,
		"shadow":   StageShadow,
		"advisory": StageAdvisory,
		"enforce":  StageEnforce,
	} {
		if got := parseStage(in, "merge_gate.stage", &ws); got != want {
			t.Errorf("parseStage(%q) = %v, want %v", in, got, want)
		}
	}
}
