package triagecap

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestK_EmptyWindowSeedsCycle281Baseline: with no observed history the
// throughput estimate is the cycle-281 PASS baseline (~5 floors/turn).
func TestK_EmptyWindowSeedsCycle281Baseline(t *testing.T) {
	if got := K(nil); got != 5 {
		t.Errorf("K(empty) = %d, want 5 (cycle-281 seed)", got)
	}
}

func TestK_MeanOverWindow(t *testing.T) {
	tests := []struct {
		name   string
		window []core.TriageThroughputEntry
		want   int
	}{
		{
			name:   "single entry",
			window: []core.TriageThroughputEntry{{Cycle: 281, Floors: 5}},
			want:   5,
		},
		{
			name: "mean rounds half up",
			window: []core.TriageThroughputEntry{
				{Cycle: 281, Floors: 5},
				{Cycle: 285, Floors: 4},
			},
			want: 5, // 4.5 → 5
		},
		{
			name: "mean floors down below half",
			window: []core.TriageThroughputEntry{
				{Cycle: 281, Floors: 5},
				{Cycle: 285, Floors: 4},
				{Cycle: 286, Floors: 3},
			},
			want: 4, // 4.0
		},
		{
			name:   "floor of one is preserved",
			window: []core.TriageThroughputEntry{{Cycle: 290, Floors: 1}},
			want:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := K(tt.window); got != tt.want {
				t.Errorf("K = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCap_CeilOnePointTwoFiveK(t *testing.T) {
	tests := []struct{ k, want int }{
		{5, 7},  // ceil(6.25)
		{4, 5},  // ceil(5.0)
		{1, 2},  // ceil(1.25)
		{0, 2},  // degenerate K clamps to 1 → ceil(1.25)=2 (never reject-everything)
		{-3, 2}, // negative is degenerate too
	}
	for _, tt := range tests {
		if got := Cap(tt.k); got != tt.want {
			t.Errorf("Cap(%d) = %d, want %d", tt.k, got, tt.want)
		}
	}
}

// TestRecord_AppendsAndCapsAtFive: rolling window of the last 5 floor-bearing
// PASS cycles; the input slice is never mutated (immutability rule).
func TestRecord_AppendsAndCapsAtFive(t *testing.T) {
	var w []core.TriageThroughputEntry
	for c := 1; c <= 7; c++ {
		w = Record(w, c, c)
	}
	want := []core.TriageThroughputEntry{
		{Cycle: 3, Floors: 3}, {Cycle: 4, Floors: 4}, {Cycle: 5, Floors: 5},
		{Cycle: 6, Floors: 6}, {Cycle: 7, Floors: 7},
	}
	if !reflect.DeepEqual(w, want) {
		t.Errorf("window after 7 records = %+v, want %+v", w, want)
	}
}

func TestRecord_ZeroFloorsIsNoOp(t *testing.T) {
	in := []core.TriageThroughputEntry{{Cycle: 281, Floors: 5}}
	out := Record(in, 282, 0)
	if !reflect.DeepEqual(out, in) {
		t.Errorf("Record(floors=0) changed window: %+v", out)
	}
}

func TestRecord_DoesNotMutateInput(t *testing.T) {
	in := make([]core.TriageThroughputEntry, 0, 8) // spare capacity invites aliasing bugs
	in = append(in, core.TriageThroughputEntry{Cycle: 281, Floors: 5})
	snapshot := []core.TriageThroughputEntry{{Cycle: 281, Floors: 5}}
	_ = Record(in, 282, 3)
	if !reflect.DeepEqual(in, snapshot) {
		t.Errorf("input window mutated: %+v", in)
	}
}
