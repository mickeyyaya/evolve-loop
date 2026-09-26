package bridge

import "math"

// scaledArtifactBudget stretches the artifact-wait budget by scale. scale <= 1 is identity, so a degenerate signal never
// shrinks a budget; an unlisted agent (base 0) scales from the driver's builtin.
// See ADR-0076.
func scaledArtifactBudget(base int, scale float64) int {
	if scale <= 1 {
		return base
	}
	if base <= 0 {
		base = tmuxArtifactTimeoutS
	}
	return int(math.Round(float64(base) * scale))
}
