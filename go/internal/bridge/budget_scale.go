package bridge

import "math"

func scaledArtifactBudget(base int, scale float64) int {
	if scale <= 1 {
		return base
	}
	if base <= 0 {
		base = tmuxArtifactTimeoutS
	}
	return int(math.Round(float64(base) * scale))
}
