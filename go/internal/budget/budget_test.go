package budget

import (
	"errors"
	"math"
	"sync"
	"testing"
)

// TestNew_Defaults verifies that a freshly-constructed Meter has zero
// accumulators and respects the unlimited cap semantics (cap=0).
func TestNew_Defaults(t *testing.T) {
	m := New(0, 0)
	if got := m.Cycle(); got != 0 {
		t.Errorf("Cycle()=%g, want 0", got)
	}
	if got := m.Batch(); got != 0 {
		t.Errorf("Batch()=%g, want 0", got)
	}
	if got := m.Builder(); got != 0 {
		t.Errorf("Builder()=%g, want 0", got)
	}
	if m.BuilderOver() {
		t.Error("BuilderOver()=true on empty meter")
	}
	if got := m.PercentOfBatchCap(); got != 0 {
		t.Errorf("PercentOfBatchCap()=%g with cap=0, want 0", got)
	}
}

// TestAdd_AccumulatesCycleAndBatch verifies the canonical happy path:
// Add() bumps both per-cycle and per-batch accumulators.
func TestAdd_AccumulatesCycleAndBatch(t *testing.T) {
	m := New(0, 0)
	if err := m.Add("scout", 0.25); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := m.Add("build", 0.5); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got, want := m.Cycle(), 0.75; got != want {
		t.Errorf("Cycle=%g, want %g", got, want)
	}
	if got, want := m.Batch(), 0.75; got != want {
		t.Errorf("Batch=%g, want %g", got, want)
	}
}

// TestAdd_BuildActorTracksBuilderAccumulator verifies that only the
// "build" actor contributes to the builder-specific accumulator.
// CLAUDE.md env-var table: EVOLVE_BUILDER_COST_THRESHOLD applies to the
// Build phase only; other actors must not poison the threshold check.
func TestAdd_BuildActorTracksBuilderAccumulator(t *testing.T) {
	m := New(0, 0)
	_ = m.Add("scout", 1.0)
	_ = m.Add("build", 0.3)
	_ = m.Add("audit", 1.5)
	if got, want := m.Builder(), 0.3; got != want {
		t.Errorf("Builder=%g, want %g (scout+audit must not contribute)", got, want)
	}
}

// TestAdd_NegativeSpendRejected — defensive: avoid silent corruption
// from a buggy actor reporting negative cost.
func TestAdd_NegativeSpendRejected(t *testing.T) {
	m := New(0, 0)
	if err := m.Add("scout", -0.01); err == nil {
		t.Error("Add(-0.01) returned nil, want error")
	}
	if got := m.Cycle(); got != 0 {
		t.Errorf("Cycle=%g after rejected add, want 0", got)
	}
}

// TestAdd_NaNRejected — defensive: NaN would silently propagate and
// poison the meter forever (NaN != NaN in comparisons).
func TestAdd_NaNRejected(t *testing.T) {
	m := New(0, 0)
	if err := m.Add("scout", math.NaN()); err == nil {
		t.Error("Add(NaN) returned nil, want error")
	}
}

// TestAdd_BatchCapTripped verifies ErrBatchCapExceeded is returned when
// cumulative spend exceeds the batch cap. The post-trip Batch() value
// is still authoritative — operators read it to decide kill scope.
func TestAdd_BatchCapTripped(t *testing.T) {
	m := New(0, 1.0)
	if err := m.Add("scout", 0.9); err != nil {
		t.Fatalf("Add 0.9: %v", err)
	}
	err := m.Add("build", 0.2)
	if err == nil {
		t.Fatal("Add over cap returned nil, want ErrBatchCapExceeded")
	}
	if !errors.Is(err, ErrBatchCapExceeded) {
		t.Errorf("err=%v, want ErrBatchCapExceeded", err)
	}
	if got := m.Batch(); got <= 1.0 {
		t.Errorf("Batch=%g after trip, want >1.0 (post-add authoritative)", got)
	}
}

// TestAdd_MaxBudgetTripped — distinct sentinel for per-invocation
// EVOLVE_MAX_BUDGET_USD cap (vs EVOLVE_BATCH_BUDGET_CAP). Operators
// distinguish so DISPATCH_RC mapping is correct.
func TestAdd_MaxBudgetTripped(t *testing.T) {
	m := New(0.5, 100.0)
	err := m.Add("scout", 0.6)
	if err == nil {
		t.Fatal("Add over max returned nil, want ErrMaxBudgetExceeded")
	}
	if !errors.Is(err, ErrMaxBudgetExceeded) {
		t.Errorf("err=%v, want ErrMaxBudgetExceeded", err)
	}
}

// TestAdvanceCycle_ResetsCycleNotBatch verifies the cycle-boundary
// semantics: per-cycle accumulators reset, batch persists.
func TestAdvanceCycle_ResetsCycleNotBatch(t *testing.T) {
	m := New(0, 0)
	_ = m.Add("scout", 0.4)
	_ = m.Add("build", 0.3)
	m.AdvanceCycle()
	if got := m.Cycle(); got != 0 {
		t.Errorf("Cycle=%g after AdvanceCycle, want 0", got)
	}
	if got := m.Builder(); got != 0 {
		t.Errorf("Builder=%g after AdvanceCycle, want 0", got)
	}
	if got, want := m.Batch(), 0.7; got != want {
		t.Errorf("Batch=%g after AdvanceCycle, want %g (must persist)", got, want)
	}
}

// TestBuilderOver_ThresholdSet verifies the EVOLVE_BUILDER_COST_THRESHOLD
// trigger. Default threshold is 2.00 USD per CLAUDE.md.
func TestBuilderOver_ThresholdSet(t *testing.T) {
	m := New(0, 0)
	m.SetBuilderThreshold(2.00)
	_ = m.Add("build", 1.99)
	if m.BuilderOver() {
		t.Error("BuilderOver=true under threshold")
	}
	_ = m.Add("build", 0.02)
	if !m.BuilderOver() {
		t.Errorf("BuilderOver=false at builder=%g, threshold=2.00", m.Builder())
	}
}

// TestBuilderOver_NoThreshold — threshold=0 means unconfigured.
// BuilderOver must always return false.
func TestBuilderOver_NoThreshold(t *testing.T) {
	m := New(0, 0)
	_ = m.Add("build", 1000.0)
	if m.BuilderOver() {
		t.Error("BuilderOver=true with no threshold set")
	}
}

// TestPercentOfBatchCap_ReportsRatio.
func TestPercentOfBatchCap_ReportsRatio(t *testing.T) {
	m := New(0, 10.0)
	_ = m.Add("scout", 2.5)
	if got, want := m.PercentOfBatchCap(), 25.0; got != want {
		t.Errorf("PercentOfBatchCap=%g, want %g", got, want)
	}
	// EVOLVE_CHECKPOINT_AT_PCT semantics: 95% trigger.
	_ = m.Add("scout", 7.0)
	if got := m.PercentOfBatchCap(); got <= 94 || got >= 96 {
		t.Errorf("PercentOfBatchCap=%g near 95, want 95±1", got)
	}
}

// TestConcurrent_AddIsRaceFree exercises the mutex with -race.
// Multi-project parallel cycles (plan §2 decision #11) share one Meter.
func TestConcurrent_AddIsRaceFree(t *testing.T) {
	m := New(0, 0)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Add("scout", 0.01)
		}()
	}
	wg.Wait()
	// 100 adds of 0.01 = ~1.0; use tolerance for float accumulation.
	if got := m.Batch(); math.Abs(got-1.0) > 1e-6 {
		t.Errorf("Batch=%g after 100 concurrent adds, want ~1.0", got)
	}
}

// TestErrBatchCapExceeded_PreservesContext — error message must include
// the cap and observed batch so operator logs are actionable.
func TestErrBatchCapExceeded_PreservesContext(t *testing.T) {
	m := New(0, 1.0)
	err := m.Add("scout", 1.5)
	if err == nil {
		t.Fatal("want error")
	}
	msg := err.Error()
	for _, frag := range []string{"1.5", "1"} {
		if !contains(msg, frag) {
			t.Errorf("err=%q missing %q (operator-facing context)", msg, frag)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
