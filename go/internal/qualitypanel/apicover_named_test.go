package qualitypanel

import (
	"reflect"
	"testing"
)

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

func TestMutationResult_ACleanRunWithNothingSkippedEqualsTheEmptySkipLists(t *testing.T) {
	t.Parallel()
	fn := ChangedFunction{Package: "go/internal/proctree", File: "go/internal/proctree/a.go", Name: "Add", Source: []byte(addSrc)}
	res, err := MeasureMutation(inputFor(t, fn), &fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	want := []MutationSkip{{Function: fn, Reason: "the path go/internal/proctree/ is a kill guard"}}
	wantResult := MutationResult{TQ1: res.TQ1, TQ3: res.TQ3, Excluded: want}
	if !reflect.DeepEqual(res, wantResult) {
		t.Errorf("result = %+v, want %+v", res, wantResult)
	}
}
