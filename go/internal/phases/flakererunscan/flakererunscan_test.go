package flakererunscan

import (
	"reflect"
	"testing"
)

func TestRerun_StablePass(t *testing.T) {
	got := Rerun(5, func(i int) bool { return true })
	want := Result{Runs: 5, Passes: 5, Failures: 0, Flaky: false}
	if got != want {
		t.Errorf("stable Rerun(5) = %+v, want %+v", got, want)
	}
	if v := got.Verdict(); v != "PASS" {
		t.Errorf("stable verdict = %q, want PASS", v)
	}
}

func TestRerun_ConsistentFail(t *testing.T) {
	got := Rerun(4, func(i int) bool { return false })
	if got.Passes != 0 || got.Failures != 4 || got.Flaky {
		t.Errorf("consistent-fail = %+v, want Passes=0 Failures=4 Flaky=false", got)
	}
	if v := got.Verdict(); v != "FAIL" {
		t.Errorf("consistent-fail verdict = %q, want FAIL", v)
	}
}

func TestRerun_FlakyAlternating(t *testing.T) {
	got := Rerun(6, func(i int) bool { return i%2 == 0 })
	if !got.Flaky {
		t.Errorf("alternating not flagged flaky: %+v", got)
	}
	if got.Passes == 0 || got.Failures == 0 {
		t.Errorf("alternating should record both outcomes: %+v", got)
	}
	if v := got.Verdict(); v != "WARN" {
		t.Errorf("flaky verdict = %q, want WARN (non-PASS canonical)", v)
	}
}

func TestRerun_ZeroRunsSkipped(t *testing.T) {
	got := Rerun(0, func(i int) bool { return true })
	if got.Runs != 0 || got.Flaky {
		t.Errorf("Rerun(0) = %+v, want empty", got)
	}
	if v := got.Verdict(); v != "SKIPPED" {
		t.Errorf("zero-run verdict = %q, want SKIPPED", v)
	}
}

func TestRerun_Deterministic(t *testing.T) {
	fn := func(i int) bool { return i%3 != 0 }
	a := Rerun(9, fn)
	b := Rerun(9, fn)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Rerun not deterministic: %+v vs %+v", a, b)
	}
}
