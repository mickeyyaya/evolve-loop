package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestGuardDenial_TheCauseAndTheTieringRuleShareOneName(t *testing.T) {
	t.Parallel()
	if CauseGuardRefused != "guard_refused" || llmroute.TieringExit(81, CauseGuardRefused) != llmroute.ExitGuardRefused {
		t.Fatalf("cause %q must be the name TieringExit ends the chain on", CauseGuardRefused)
	}
}

func TestGuardDenial_TheCodeIsRegisteredUnderTheIsolationModule(t *testing.T) {
	t.Parallel()
	if m, ok := signalcenter.IsRegistered(CodeGuardDenied); !ok || m != signalcenter.ModuleIsolation || CodeGuardDenied != "ISOLATION_GUARD_DENIED" {
		t.Fatalf("code %q registered=%v module=%v", CodeGuardDenied, ok, m)
	}
}

func TestWall_AGuardRefusalIsNoQuotaAndNoLoginWall(t *testing.T) {
	t.Parallel()
	w := Wall{}.Observe(FamilyEnd{CLI: "codex-tmux", Pattern: CauseGuardRefused})
	if w.HasQuota() || w.HasLogin() || w.CountsAsQuotaPause() || w.StopReason() == stopReasonQuota {
		t.Fatalf("a guard refusal is a stop by the denial, not a fault of the family: %+v quota=%v login=%v reason=%s", w, w.HasQuota(), w.HasLogin(), w.StopReason())
	}
}

func TestGuardDenial_TheFieldNamesAreTheWireContractOfTheHookAndTheBridge(t *testing.T) {
	t.Parallel()
	if DenialFieldGuard != "guard" || DenialFieldPath != "path" || DenialFieldCWD != "cwd" || DenialFieldNeutral != "neutral" || DenialFieldReason != "reason" {
		t.Fatalf("the hook writes and the bridge reads these keys: %q %q %q %q", DenialFieldGuard, DenialFieldPath, DenialFieldCWD, DenialFieldNeutral)
	}
}
