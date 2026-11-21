package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTheCheckedInPolicy_EnforcesTheStagesTheOperatorMoved(t *testing.T) {
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("the checked-in policy must pass the strict decoder: %v", err)
	}
	catalog := p.CatalogConfig()
	if catalog.AutoRefresh == nil || !*catalog.AutoRefresh {
		t.Error("catalog.auto_refresh is not true, want true")
	}
	stages := map[string]string{
		"catalog.refresh_stage":        catalog.RefreshStage,
		"comment_floor.stage":          p.CommentFloorConfig().Stage,
		"retrospectives.stage":         p.RetrospectivesConfig().Stage,
		"fleet.orchestrator.admission": p.FleetConfig().Orchestrator.Admission,
	}
	for key, got := range stages {
		if got != "enforce" {
			t.Errorf("%s = %q, want enforce", key, got)
		}
	}
}

func TestTheCheckedInPolicy_HasTheAdaptiveShadowBlock(t *testing.T) {
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("the checked-in policy must pass the strict decoder: %v", err)
	}
	adaptive := p.FleetConfig().Orchestrator.Adaptive
	if !adaptive.Enabled || adaptive.Stage != string(policy.AdaptiveStageShadow) {
		t.Errorf("fleet.orchestrator.window.adaptive = %+v, want enabled at stage shadow so the load sampler runs without capping the window", adaptive)
	}
}

func TestTheCheckedInPolicy_SetsTheDiskFloorAndWarnAboveTheCycleStartGuardLine(t *testing.T) {
	gcBlock := policy.GCFor(filepath.Join("..", "..", ".."))
	if gcBlock.DiskFloorGB != 25 || gcBlock.DiskWarnGB != 40 {
		t.Errorf("gc.disk_floor_gb = %d gc.disk_warn_gb = %d, want 25 and 40: both sit above the 5%% guard line of the cycle start", gcBlock.DiskFloorGB, gcBlock.DiskWarnGB)
	}
	if gcBlock.Sentinel.HoldEnforced() || gcBlock.Sentinel.ReleaseEnforced() {
		t.Error("the checked-in policy enforces a sentinel stage before shadow evidence exists")
	}
}
