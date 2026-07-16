package audit

// ciparity_apicover_ctx_test.go — apicover-inprocess-ctx-timeout: the gate's
// apicoverTimeout must bound the IN-PROCESS measurement (apicover.Run), and a
// ctx interruption must fail OPEN (infra WARN via error return) — never FAIL
// the cycle as if the touched code had offenders. Companion to the apicover
// package's own run_ctx_test.go, which pins the ctx plumbing; this pins the
// gate-level wiring + error mapping.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestApicoverGate_CtxInterruption_FailsOpen: with the gate deadline already
// expired, the measurement is interrupted — the gate must return (nil, err)
// with the context error in the chain (the fail-open WARN contract every other
// ctx-bounded step of this gate follows), NOT an offender FAIL. The fixture
// deliberately contains an offender so a wrong FAIL-path mapping is caught.
func TestApicoverGate_CtxInterruption_FailsOpen(t *testing.T) {
	root, goDir := writeApicoverFixture(t, apicoverOffenderPkg)
	withFakeRunner(t, apicoverPipelineRunner(goDir, nil))

	orig := apicoverTimeout
	apicoverTimeout = -time.Nanosecond // ctx born expired: the fake pre-steps ignore it; apicover.Run must not
	t.Cleanup(func() { apicoverTimeout = orig })

	off, err := apicoverEnforceChangedDefault(core.PhaseRequest{ProjectRoot: root, Worktree: root, Cycle: 1})

	if err == nil {
		t.Fatalf("interrupted measurement must fail OPEN with an error; got (off=%v, err=nil)", off)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error must chain to context.DeadlineExceeded so operators see the interruption, got %v", err)
	}
	if off != nil {
		t.Errorf("interruption must not report offenders (that would FAIL the cycle for infra weather); got %v", off)
	}
}
