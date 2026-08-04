package contextfill

import (
	"errors"
	"math"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

const epsilon = 1e-9

// TestFillRatio is the table-driven sweep over the derivation: which fields
// count, the boundary rows the acceptance criteria name, and the invalid-window
// rejection.
func TestFillRatio(t *testing.T) {
	cases := []struct {
		name   string
		tokens cyclestate.TokenUsage
		window int
		want   float64
	}{
		{
			// Asymmetric on purpose: an Input+Output-only sum yields 0.1875 and a
			// CacheWrite-blind sum yields 0.4375, so either mistake fails here.
			name:   "every token kind counts",
			tokens: cyclestate.TokenUsage{Input: 1000, Output: 500, CacheRead: 2000, CacheWrite: 500},
			window: 8000,
			want:   0.5,
		},
		{name: "zero tokens", tokens: cyclestate.TokenUsage{}, window: 1000, want: 0},
		{name: "sub threshold", tokens: cyclestate.TokenUsage{Input: 250}, window: 1000, want: 0.25},
		{
			name:   "at hot threshold",
			tokens: cyclestate.TokenUsage{Input: 500, CacheRead: 350},
			window: 1000,
			want:   HotThreshold,
		},
		{
			// Unclamped: telemetry that saturates at 1.0 cannot tell a phase that
			// just fit from one that overran by half.
			name:   "over window is not clamped",
			tokens: cyclestate.TokenUsage{Input: 1000, Output: 500},
			window: 1000,
			want:   1.5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FillRatio(tc.tokens, tc.window)
			if err != nil {
				t.Fatalf("FillRatio(%+v, %d) returned error %v, want nil", tc.tokens, tc.window, err)
			}
			if math.Abs(got-tc.want) > epsilon {
				t.Errorf("FillRatio(%+v, %d) = %v, want %v", tc.tokens, tc.window, got, tc.want)
			}
		})
	}
}

// TestFillRatio_InvalidWindow is the negative axis: a window we cannot identify
// (0, as WindowSizeForTier reports for an unknown tier) or an outright negative
// one must be REJECTED with ErrInvalidWindow. A silent divide-by-zero (+Inf /
// NaN), a panic, or a fabricated 0-with-nil-error are all failures — a fill
// number nobody can source is worse than no number.
func TestFillRatio_InvalidWindow(t *testing.T) {
	tokens := cyclestate.TokenUsage{Input: 100, Output: 100}

	for _, window := range []int{0, -1, -8000} {
		got, err := FillRatio(tokens, window)
		if !errors.Is(err, ErrInvalidWindow) {
			t.Errorf("FillRatio(%+v, %d) error = %v, want ErrInvalidWindow", tokens, window, err)
		}
		if got != 0 || math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("FillRatio(%+v, %d) = %v alongside its error, want a finite 0", tokens, window, got)
		}
	}
}

// TestIsHot pins the classification boundary against the exported threshold so
// the constant and its consumers cannot drift apart.
func TestIsHot(t *testing.T) {
	if HotThreshold <= 0 || HotThreshold > 1 {
		t.Fatalf("HotThreshold = %v, want a usable fraction in (0, 1]", HotThreshold)
	}

	cases := []struct {
		ratio float64
		want  bool
	}{
		{0, false},
		{HotThreshold - 0.01, false},
		{HotThreshold, true},
		{1.5, true},
	}
	for _, tc := range cases {
		if got := IsHot(tc.ratio); got != tc.want {
			t.Errorf("IsHot(%v) = %v, want %v", tc.ratio, got, tc.want)
		}
	}
}

// TestWindowSizeForTier drives the stub over the REAL canonical tier vocabulary
// so the map cannot silently cover a hand-copied subset that drifts from
// modelcatalog, and pins unknown -> 0 (reported as unknown, never defaulted).
func TestWindowSizeForTier(t *testing.T) {
	for _, tier := range modelcatalog.CanonicalTiers {
		got := WindowSizeForTier(tier)
		if got <= 0 {
			t.Errorf("WindowSizeForTier(%q) = %d, want a positive window size", tier, got)
			continue
		}
		if _, err := FillRatio(cyclestate.TokenUsage{Input: 1}, got); err != nil {
			t.Errorf("FillRatio(tokens, WindowSizeForTier(%q)=%d) returned error %v, want nil", tier, got, err)
		}
	}

	for _, unknown := range []string{"", "nonexistent-tier", "opus"} {
		if got := WindowSizeForTier(unknown); got != 0 {
			t.Errorf("WindowSizeForTier(%q) = %d, want 0", unknown, got)
		}
	}
}
