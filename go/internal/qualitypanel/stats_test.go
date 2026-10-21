package qualitypanel

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.0005 }

func TestWilson_MatchesTheResearchWorkedExamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k, n      int
		p, lo, hi float64
	}{{3, 4, 0.750, 0.301, 0.954}, {119, 186, 0.640, 0.569, 0.705}, {0, 234, 0, 0, 0.016}, {5, 5, 1, 0.566, 1}}
	for _, c := range cases {
		got := Wilson(c.k, c.n)
		if !near(got.Value, c.p) || !near(got.Lo, c.lo) || !near(got.Hi, c.hi) {
			t.Errorf("Wilson(%d, %d) = %+v, want %.3f [%.3f, %.3f]", c.k, c.n, got, c.p, c.lo, c.hi)
		}
	}
	if got := Wilson(0, 0); got != (Interval{}) {
		t.Errorf("Wilson(0, 0) = %+v, want the zero interval", got)
	}
}

func TestMDE_MatchesTheSampleRuleOfTheSpec(t *testing.T) {
	t.Parallel()
	if got := MDE(0.42, 66); !near(got, 0.2406) {
		t.Errorf("MDE(0.42, 66) = %.4f, want 0.2406", got)
	}
	if got := MDE(0.42, 5); !near(got, 0.8742) {
		t.Errorf("MDE(0.42, 5) = %.4f, want 0.8742", got)
	}
	if got := MDE(0.42, 0); got != 1 {
		t.Errorf("MDE with no sample = %v, want 1 (nothing is detectable)", got)
	}
}

func TestMedian_OddEvenAndEmpty(t *testing.T) {
	t.Parallel()
	in := []float64{9, 1, 7}
	if Median(in) != 7 || Median([]float64{4, 1, 3, 2}) != 2.5 || Median(nil) != 0 {
		t.Error("median of {9,1,7} is 7, of {4,1,3,2} is 2.5, of nothing is 0")
	}
	if in[0] != 9 {
		t.Errorf("Median sorted its input in place: %v", in)
	}
}

func TestClusterBootstrapMedian_IsSeededAndBracketsTheMedian(t *testing.T) {
	t.Parallel()
	clusters := [][]float64{{1, 2, 2}, {7, 9}, {3, 4, 5, 6}, {2}, {8, 8, 10}}

	a := ClusterBootstrapMedian(clusters, 2000, 20261021)
	b := ClusterBootstrapMedian(clusters, 2000, 20261021)

	if a != b {
		t.Errorf("same seed gave %+v and %+v, want one repeatable reading", a, b)
	}
	if a.Value != 5 || a.Lo > a.Value || a.Hi < a.Value || a.Lo == a.Hi {
		t.Errorf("interval %+v, want median 5 strictly inside a non-empty interval", a)
	}
	if got := ClusterBootstrapMedian([][]float64{{4, 4}, {4}}, 50, 1); got != (Interval{Value: 4, Lo: 4, Hi: 4}) {
		t.Errorf("constant data = %+v, want a point interval at 4", got)
	}
	if ClusterBootstrapMedian(nil, 10, 1) != (Interval{}) || ClusterBootstrapMedian(clusters, 0, 1) != (Interval{}) {
		t.Error("no data or no resamples must give the zero interval")
	}
}

func TestClusterBootstrapMedian_EmptyClustersDoNotDragTheMedianToZero(t *testing.T) {
	t.Parallel()
	got := ClusterBootstrapMedian([][]float64{{}, {6, 6}, {}, {6}}, 200, 7)
	if got != (Interval{Value: 6, Lo: 6, Hi: 6}) {
		t.Errorf("interval = %+v, want a point at 6: an empty cluster is no data, not a zero", got)
	}
}
