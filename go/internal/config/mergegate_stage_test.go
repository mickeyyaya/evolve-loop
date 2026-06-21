package config

import "testing"

// TestDefaults_MergeGate_Shadow pins the merge-to-main gate rollout dial at the
// struct-default level: StageShadow — the gate records its would-be promotion
// verdict but merges nothing. This is the byte-neutral first deploy over the
// single most dangerous action in the system (auto-merge to main); guard it
// explicitly so a future default edit can't silently flip it on.
func TestDefaults_MergeGate_Shadow(t *testing.T) {
	if got := defaults().RolloutStages.MergeGate; got != StageShadow {
		t.Errorf("default RolloutStages.MergeGate = %v, want StageShadow", got)
	}
}

// TestMergeGate_StageLadder confirms the dial uses the full
// off→shadow→advisory→enforce ladder (auto-merge activates only at enforce),
// parsed by the shared parseStage with the fail-safe unknown→off rule.
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
