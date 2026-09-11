package core

import "testing"

func TestPlanStage_Initial(t *testing.T) {
	assertPlanStageMappings(t, stageInitial, "routing-plan.json", "plan", 0)
}

func TestPlanStage_PostScout(t *testing.T) {
	assertPlanStageMappings(t, stagePostScout, "routing-replan.json", "replan", 1)
}

func TestPlanStage_ZeroValueIsInitial(t *testing.T) {
	var stage planStage
	if stage != stageInitial {
		t.Fatalf("zero-value planStage = %d, want stageInitial (%d)", stage, stageInitial)
	}
}

func assertPlanStageMappings(t *testing.T, stage planStage, artifact, kind string, depth int) {
	t.Helper()
	if got := stage.artifactFile(); got != artifact {
		t.Errorf("artifactFile() = %q, want %q", got, artifact)
	}
	if got := stage.captureKind(); got != kind {
		t.Errorf("captureKind() = %q, want %q", got, kind)
	}
	if got := stage.replanDepth(); got != depth {
		t.Errorf("replanDepth() = %d, want %d", got, depth)
	}
}
