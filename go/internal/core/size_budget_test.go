package core

// size_budget_test.go — ADR-0076 slice A consumption pins: the cycle-size
// multiplier scales the correction limit (clamped at the policy ceiling) and
// the build launch's artifact budget via PhaseRequest.BudgetScale. Absent /
// unknown size = 1.0 = byte-identical legacy behavior.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestSizeBudgetMultiplier_MapsAndDefaults(t *testing.T) {
	m := map[string]float64{"large": 1.5, "medium": 1.25}
	if got := sizeBudgetMultiplier("large", m); got != 1.5 {
		t.Fatalf("large: got %v", got)
	}
	if got := sizeBudgetMultiplier("", m); got != 1.0 {
		t.Fatalf("absent size must be 1.0, got %v", got)
	}
	if got := sizeBudgetMultiplier("weird", m); got != 1.0 {
		t.Fatalf("unknown size must be 1.0, got %v", got)
	}
	if got := sizeBudgetMultiplier("large", nil); got != 1.0 {
		t.Fatalf("nil map must be 1.0, got %v", got)
	}
}

func TestScaledCorrectionLimit_RoundsAndClamps(t *testing.T) {
	if got := scaledCorrectionLimit(2, 1.0); got != 2 {
		t.Fatalf("mult 1.0 must be identity, got %d", got)
	}
	if got := scaledCorrectionLimit(2, 1.5); got != 3 {
		t.Fatalf("2×1.5 must round to 3, got %d", got)
	}
	if got := scaledCorrectionLimit(4, 2.0); got != policy.MaxContractCorrectionRetries {
		t.Fatalf("scale must clamp at the policy ceiling, got %d", got)
	}
	if got := scaledCorrectionLimit(2, 0); got != 2 {
		t.Fatalf("zero/absent mult must be identity, got %d", got)
	}
}
