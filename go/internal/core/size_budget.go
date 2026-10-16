package core

import (
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (cr *cycleRun) buildBudgetScale() float64 {
	sig, err := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
	if err != nil {
		return 1.0
	}
	return sizeBudgetMultiplier(sig.CycleSize(), cr.o.workflowConfig.SizeBudgetMultipliers)
}

func (cr *cycleRun) correctionLimitFor(phase Phase, base int) int {
	if phase != PhaseBuild {
		return base
	}
	return scaledCorrectionLimit(base, cr.buildBudgetScale())
}

func sizeBudgetMultiplier(size string, multipliers map[string]float64) float64 {
	if m, ok := multipliers[size]; ok && m > 0 {
		return m
	}
	return 1.0
}

func scaledCorrectionLimit(base int, mult float64) int {
	if mult <= 0 || mult == 1.0 {
		return base
	}
	scaled := int(math.Round(float64(base) * mult))
	if scaled > policy.MaxContractCorrectionRetries {
		return policy.MaxContractCorrectionRetries
	}
	if scaled < base {
		return base
	}
	return scaled
}
