package core

import (
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// buildBudgetScale resolves this cycle's build-phase budget multiplier from
// the digest-resolved size signal. Any digest failure or absent signal is
// 1.0: scaling is an upgrade on evidence, never a guess.
func (cr *cycleRun) buildBudgetScale() float64 {
	sig, err := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
	if err != nil {
		return 1.0
	}
	return sizeBudgetMultiplier(sig.CycleSize(), cr.o.workflowConfig.SizeBudgetMultipliers)
}

// correctionLimitFor scales the contract-correction limit for the BUILD phase
// only; audit/adversarial phases keep their configured ladder untouched.
func (cr *cycleRun) correctionLimitFor(phase Phase, base int) int {
	if phase != PhaseBuild {
		return base
	}
	return scaledCorrectionLimit(base, cr.buildBudgetScale())
}

// sizeBudgetMultiplier resolves the multiplier for a cycle_size_estimate.
// Unknown/absent size or nil map is 1.0.
func sizeBudgetMultiplier(size string, multipliers map[string]float64) float64 {
	if m, ok := multipliers[size]; ok && m > 0 {
		return m
	}
	return 1.0
}

// scaledCorrectionLimit scales the base correction limit by mult, rounding
// half-up, clamped to the policy ceiling. mult <= 0 is identity.
func scaledCorrectionLimit(base int, mult float64) int {
	if mult <= 0 || mult == 1.0 {
		return base
	}
	scaled := int(math.Round(float64(base) * mult))
	if scaled > policy.MaxContractCorrectionRetries {
		return policy.MaxContractCorrectionRetries
	}
	if scaled < base {
		return base // scaling never shrinks the ladder
	}
	return scaled
}
