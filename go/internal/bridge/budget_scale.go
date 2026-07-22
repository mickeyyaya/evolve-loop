package bridge

import "math"

// budget_scale.go — ADR-0076 slice A: difficulty-conditioned artifact budgets.
// A BridgeRequest.BudgetScale > 1 (set by the orchestrator for the build phase
// of a medium/large cycle) stretches the artifact-wait deadline the engine
// passes to the driver, so hard cycles stop starving their verification tail.

// scaledArtifactBudget resolves the effective artifact-wait budget in seconds.
// base is the per-agent policy value (0 = unlisted → the driver's builtin
// deadline applies). scale <= 1 is identity — a missing or degenerate signal
// must never shrink a budget. Scaling an unlisted agent starts from the same
// builtin the driver would apply (tmuxArtifactTimeoutS), so the flag emitted
// is exactly "what would have happened, stretched".
func scaledArtifactBudget(base int, scale float64) int {
	if scale <= 1 {
		return base
	}
	if base <= 0 {
		base = tmuxArtifactTimeoutS
	}
	return int(math.Round(float64(base) * scale))
}
