package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTheCheckedInPolicy_RunsRegressionTIAInShadow(t *testing.T) {
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("the checked-in policy must pass the strict decoder: %v", err)
	}
	if got := p.RegressionTIAConfig().Stage; got != "shadow" {
		t.Fatalf("regression_tia stage = %q, want shadow: the operator approved the shadow soak on 2026-10-21", got)
	}
	if got := policy.RegressionTIAStageFor(filepath.Join("..", "..", "..")); got != "shadow" {
		t.Errorf("RegressionTIAStageFor = %q, want shadow from the checked-in file", got)
	}
}
