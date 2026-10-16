package contextfill

import (
	"errors"
	"math"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestAPICoverNamedExports(t *testing.T) {
	window := WindowSizeForTier("deep")
	if window <= 0 {
		t.Fatalf("WindowSizeForTier(%q) = %d, want a positive window", "deep", window)
	}

	hotTokens := cyclestate.TokenUsage{Input: int(math.Round(float64(window) * HotThreshold))}
	ratio, err := FillRatio(hotTokens, window)
	if err != nil {
		t.Fatalf("FillRatio(%+v, %d) returned error %v, want nil", hotTokens, window, err)
	}
	if math.Abs(ratio-HotThreshold) > 1e-9 {
		t.Fatalf("FillRatio(%+v, %d) = %v, want %v", hotTokens, window, ratio, HotThreshold)
	}
	if !IsHot(ratio) {
		t.Errorf("IsHot(%v) = false, want true at HotThreshold", ratio)
	}

	if got := WindowSizeForTier("unrecognised"); got != 0 {
		t.Fatalf("WindowSizeForTier(%q) = %d, want 0", "unrecognised", got)
	}
	if _, err := FillRatio(hotTokens, WindowSizeForTier("unrecognised")); !errors.Is(err, ErrInvalidWindow) {
		t.Errorf("FillRatio with an unknown tier's window: error = %v, want ErrInvalidWindow", err)
	}
}
