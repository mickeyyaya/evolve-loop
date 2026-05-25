package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// stubRunner returns a scripted PhaseResponse and error. Used to drive
// the decorator without spinning a real BaseRunner.
type stubRunner struct {
	name string
	resp core.PhaseResponse
	err  error
	hits int
}

func (s *stubRunner) Name() string { return s.name }
func (s *stubRunner) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	s.hits++
	return s.resp, s.err
}

func defaultOpts() CostGuardOptions {
	return CostGuardOptions{
		ThresholdEnvKey:     "EVOLVE_TEST_COST_THRESHOLD",
		StrictEnvKey:        "EVOLVE_TEST_COST_GUARD_STRICT",
		DefaultThresholdUSD: 2.00,
	}
}

// TestCostGuard_Name_DelegatesToInner — Decorator must preserve the
// inner runner's Name so callers see no shape change.
func TestCostGuard_Name_DelegatesToInner(t *testing.T) {
	inner := &stubRunner{name: "build"}
	d := WithCostGuard(inner, defaultOpts())
	if d.Name() != "build" {
		t.Errorf("Name=%q, want build", d.Name())
	}
}

// TestCostGuard_BelowThreshold_NoDiagnostic — cost <= threshold means
// the decorator is invisible.
func TestCostGuard_BelowThreshold_NoDiagnostic(t *testing.T) {
	inner := &stubRunner{
		name: "build",
		resp: core.PhaseResponse{Verdict: core.VerdictPASS, CostUSD: 1.50},
	}
	d := WithCostGuard(inner, defaultOpts())
	resp, err := d.Run(context.Background(), core.PhaseRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("expected no diagnostics; got %+v", resp.Diagnostics)
	}
}

// TestCostGuard_AboveThreshold_Advisory_StillPASSWithDiag — overrun
// without strict mode emits a warning diagnostic and preserves the
// inner runner's verdict.
func TestCostGuard_AboveThreshold_Advisory_StillPASSWithDiag(t *testing.T) {
	inner := &stubRunner{
		name: "build",
		resp: core.PhaseResponse{Verdict: core.VerdictPASS, CostUSD: 3.50},
	}
	d := WithCostGuard(inner, defaultOpts())
	resp, _ := d.Run(context.Background(), core.PhaseRequest{
		Env: map[string]string{"EVOLVE_TEST_COST_THRESHOLD": "2.00"},
	})
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS (advisory)", resp.Verdict)
	}
	if len(resp.Diagnostics) != 1 {
		t.Fatalf("want 1 diagnostic; got %+v", resp.Diagnostics)
	}
	if resp.Diagnostics[0].Severity != "warning" {
		t.Errorf("Severity=%q, want warning", resp.Diagnostics[0].Severity)
	}
	if !strings.Contains(resp.Diagnostics[0].Message, "3.50") {
		t.Errorf("Message lost cost value: %q", resp.Diagnostics[0].Message)
	}
}

// TestCostGuard_AboveThreshold_Strict_PromotesToFAIL — strict mode
// promotes the verdict and the diagnostic severity.
func TestCostGuard_AboveThreshold_Strict_PromotesToFAIL(t *testing.T) {
	inner := &stubRunner{
		name: "build",
		resp: core.PhaseResponse{Verdict: core.VerdictPASS, CostUSD: 3.50},
	}
	d := WithCostGuard(inner, defaultOpts())
	resp, _ := d.Run(context.Background(), core.PhaseRequest{
		Env: map[string]string{
			"EVOLVE_TEST_COST_THRESHOLD":    "2.00",
			"EVOLVE_TEST_COST_GUARD_STRICT": "1",
		},
	})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL (strict)", resp.Verdict)
	}
	if resp.Diagnostics[0].Severity != "error" {
		t.Errorf("Severity=%q, want error", resp.Diagnostics[0].Severity)
	}
}

// TestCostGuard_PreservesInnerDiagnostics — the decorator appends, it
// does not replace.
func TestCostGuard_PreservesInnerDiagnostics(t *testing.T) {
	inner := &stubRunner{
		name: "build",
		resp: core.PhaseResponse{
			Verdict:     core.VerdictPASS,
			CostUSD:     3.50,
			Diagnostics: []core.Diagnostic{{Severity: "info", Message: "from inner"}},
		},
	}
	d := WithCostGuard(inner, defaultOpts())
	resp, _ := d.Run(context.Background(), core.PhaseRequest{
		Env: map[string]string{"EVOLVE_TEST_COST_THRESHOLD": "2.00"},
	})
	if len(resp.Diagnostics) != 2 {
		t.Fatalf("want 2 diagnostics; got %+v", resp.Diagnostics)
	}
	if resp.Diagnostics[0].Message != "from inner" {
		t.Errorf("inner diagnostic clobbered: %q", resp.Diagnostics[0].Message)
	}
}

// TestCostGuard_InnerError_PassesThroughUnchanged — transport errors
// from the wrapped runner must not be masked by cost-overrun logic.
func TestCostGuard_InnerError_PassesThroughUnchanged(t *testing.T) {
	innerErr := errors.New("bridge boom")
	inner := &stubRunner{name: "build", err: innerErr}
	d := WithCostGuard(inner, defaultOpts())
	_, err := d.Run(context.Background(), core.PhaseRequest{
		Env: map[string]string{"EVOLVE_TEST_COST_THRESHOLD": "2.00"},
	})
	if !errors.Is(err, innerErr) {
		t.Errorf("err=%v, want wraps inner err", err)
	}
}

// TestCostGuard_UnparseableThreshold_FallsBackToDefault — operators
// who set "auto" or similar non-numeric values keep working.
func TestCostGuard_UnparseableThreshold_FallsBackToDefault(t *testing.T) {
	inner := &stubRunner{
		name: "build",
		resp: core.PhaseResponse{Verdict: core.VerdictPASS, CostUSD: 1.50},
	}
	d := WithCostGuard(inner, defaultOpts())
	resp, _ := d.Run(context.Background(), core.PhaseRequest{
		Env: map[string]string{"EVOLVE_TEST_COST_THRESHOLD": "auto"},
	})
	// Default 2.00 means 1.50 is below threshold.
	if len(resp.Diagnostics) != 0 {
		t.Errorf("unparseable threshold should fall back to default; got %+v", resp.Diagnostics)
	}
}

// TestCostGuard_EmptyThresholdEnv_UsesDefault — same path but exercised
// via the empty-string branch.
func TestCostGuard_EmptyThresholdEnv_UsesDefault(t *testing.T) {
	inner := &stubRunner{
		name: "build",
		resp: core.PhaseResponse{Verdict: core.VerdictPASS, CostUSD: 2.50},
	}
	d := WithCostGuard(inner, defaultOpts())
	resp, _ := d.Run(context.Background(), core.PhaseRequest{})
	// 2.50 > default 2.00 → expect a warning.
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Severity != "warning" {
		t.Errorf("expected single warning at cost 2.50 vs default 2.00; got %+v", resp.Diagnostics)
	}
}
