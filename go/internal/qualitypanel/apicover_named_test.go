package qualitypanel

import "testing"

func TestDirection_RatesThatMustRiseAndRatesThatMustFall(t *testing.T) {
	t.Parallel()
	want := map[string]Direction{"PQ1": HigherIsBetter, "TQ1": HigherIsBetter, "PQ4": LowerIsBetter, "JQ3": LowerIsBetter, "OQ2": LowerIsBetter}
	for id, dir := range want {
		if ix, _ := Lookup(id); ix.Direction != dir {
			t.Errorf("%s direction = %d, want %d", id, ix.Direction, dir)
		}
	}
}

func TestMethod_TheFourWireNamesOfSpec52(t *testing.T) {
	t.Parallel()
	got := []Method{MethodWilson95, MethodBootstrap95, MethodCensus, MethodPassK}
	want := []string{"wilson95", "bootstrap95", "census", "pass_k_unbiased"}
	for i := range got {
		if string(got[i]) != want[i] {
			t.Errorf("method %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestJudge_NoSampleIsStatusNoDataEvenWithoutAThreshold(t *testing.T) {
	t.Parallel()
	ix, _ := Lookup("JQ4")
	if got := Judge(ix, Reading{}); got != StatusNoData {
		t.Errorf("Judge(empty) = %s, want %s", got, StatusNoData)
	}
}
