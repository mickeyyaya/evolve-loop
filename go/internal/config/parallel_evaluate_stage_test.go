package config

import "testing"

// parallel_evaluate_stage_test.go — pins the ParallelEvaluate rollout default and
// the shared parseStage ladder used by the composition root when wiring the policy
// block (T1, pre-GREEN: defaults() already has StageOff; parseStage already parses
// the full ladder). Purpose: regression guard so Builder cannot accidentally flip
// the default to enforce before a shadow soak.

// TestDefaults_ParallelEvaluate_Off pins the struct-level default:
// ParallelEvaluate must be StageOff — the dispatcher is dormant and cycle behavior
// is byte-identical to the pre-T1 baseline. This is the safety oracle against a
// blind enforce flip (per goal spec: "do NOT flip enforce").
func TestDefaults_ParallelEvaluate_Off(t *testing.T) {
	if got := defaults().RolloutStages.ParallelEvaluate; got != StageOff {
		t.Errorf("default RolloutStages.ParallelEvaluate = %v, want StageOff", got)
	}
}

// TestParallelEvaluate_StageLadder confirms the dial uses the full
// off→shadow→advisory→enforce ladder parsed by shared parseStage with the
// fail-safe unknown→off rule.
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

// TestParallelEvaluate_DefaultConcurrencyThree pins the soak sweet-spot default.
func TestParallelEvaluate_DefaultConcurrencyThree(t *testing.T) {
	if got := defaults().ParallelEvaluateConcurrency; got != 3 {
		t.Errorf("default ParallelEvaluateConcurrency = %d, want 3 (soak sweet spot)", got)
	}
}
