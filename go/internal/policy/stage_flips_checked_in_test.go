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
		"catalog.refresh_stage": catalog.RefreshStage,
		"comment_floor.stage":   p.CommentFloorConfig().Stage,
		"retrospectives.stage":  p.RetrospectivesConfig().Stage,
	}
	for key, got := range stages {
		if got != "enforce" {
			t.Errorf("%s = %q, want enforce", key, got)
		}
	}
}
