package config

import "testing"

func TestDefaults_ParallelEvaluate_Off(t *testing.T) {
	if got := defaults().RolloutStages.ParallelEvaluate; got != StageOff {
		t.Errorf("default RolloutStages.ParallelEvaluate = %v, want StageOff", got)
	}
}

func TestParallelEvaluate_StageLadder(t *testing.T) {
	var ws []Warning
	cases := map[string]Stage{
		"off":     StageOff,
		"shadow":  StageShadow,
		"enforce": StageEnforce,
	}
	for in, want := range cases {
		if got := parseStage(in, "parallel_evaluate.stage", &ws); got != want {
			t.Errorf("parseStage(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParallelEvaluate_DefaultConcurrencyThree(t *testing.T) {
	if got := defaults().ParallelEvaluateConcurrency; got != 3 {
		t.Errorf("default ParallelEvaluateConcurrency = %d, want 3 (soak sweet spot)", got)
	}
}
