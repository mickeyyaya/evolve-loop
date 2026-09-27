package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func runConflictCycle(t *testing.T, f conflictFixture) core.CycleResult {
	t.Helper()
	result, _ := f.o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "ship-recovery-end",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	return result
}

func TestRecoverFromShipError_ACycleWhoseRebaseReentryAbortsEndsFAIL(t *testing.T) {
	f := newConflictFixture(t, "lane line\n", nil)
	result := runConflictCycle(t, f)
	if f.ship.calls != 1 || f.dbg.calls != 1 {
		t.Fatalf("calls ship=%d debugger=%d, want 1/1", f.ship.calls, f.dbg.calls)
	}
	if result.FinalVerdict != core.VerdictFAIL {
		t.Fatalf("FinalVerdict = %q: a cycle whose ship never landed ends FAIL, so the closeout releases its claim", result.FinalVerdict)
	}
}

func TestRecoverFromShipError_ADebuggerThatBlocksAShipRecoveryEndsFAIL(t *testing.T) {
	f := newConflictFixture(t, "lane line\n", map[string]any{"debugger.action": "BLOCK"})
	result := runConflictCycle(t, f)
	if f.ship.calls != 1 || f.dbg.calls != 1 {
		t.Fatalf("calls ship=%d debugger=%d, want 1/1", f.ship.calls, f.dbg.calls)
	}
	if result.FinalVerdict != core.VerdictFAIL {
		t.Fatalf("FinalVerdict = %q: a ship recovery the debugger blocks ends FAIL", result.FinalVerdict)
	}
}

type greenAuditDebugger struct{ inner *resolvingDebugger }

func (d greenAuditDebugger) Name() string { return string(core.PhaseDebugger) }
func (d greenAuditDebugger) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return core.PhaseResponse{}, err
	}
	audit := "# Audit\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "PASS", nil) + "\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, phasecontract.ArtifactFilename("audit")), []byte(audit), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "acs-verdict.json"), []byte(`{"verdict":"PASS"}`), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	return d.inner.Run(ctx, req)
}

type deliverableVerdict bool

func (v deliverableVerdict) VerifyDeliverable(context.Context, core.ReviewInput) (core.ContractVerification, error) {
	return core.ContractVerification{OK: bool(v)}, nil
}

func greenAuditConflictFixture(t *testing.T, signals map[string]any, verifies deliverableVerdict) conflictFixture {
	t.Helper()
	f := newConflictFixture(t, "lane line\n", signals)
	f.o = core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip: f.ship, core.PhaseAudit: f.audit, core.PhaseBuild: f.build, core.PhaseTDD: f.tdd,
		core.PhaseDebugger: greenAuditDebugger{inner: f.dbg},
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: f.dir}), core.WithContractVerifier(verifies))
	return f
}

func TestRecoverFromShipError_AnUnlandedShipOverAGreenAuditIsADiagnosedFAILNotAForgedVerdict(t *testing.T) {
	for name, signals := range map[string]map[string]any{"block": {"debugger.action": "BLOCK"}, "reentry-abort": nil} {
		for _, verifies := range []deliverableVerdict{false, true} {
			result := runConflictCycle(t, greenAuditConflictFixture(t, signals, verifies))
			if result.FinalVerdict != core.VerdictFAIL || result.SystemFailure != nil {
				t.Errorf("%s (deliverable verifies=%v): FinalVerdict=%q SystemFailure=%+v: a ship that never landed is a diagnosed FAIL, never a verdict-incoherence halt or a reconciled PASS", name, verifies, result.FinalVerdict, result.SystemFailure)
			}
		}
	}
}
