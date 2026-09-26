package phasecmd

import "testing"

func TestResolveStallPolicy_EnforceFlagActivates(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv(envIPCPhaseRecoveryStage, "")
	if got := resolveStallPolicy(true); got == nil {
		t.Fatal("resolveStallPolicy(true) = nil; the --enforce flag must activate the chain-backed stall policy " +
			"even when the IPC stage key is unset (the manual phase-observer path)")
	}
}

func TestResolveStallPolicy_NoEnforceNoEnvIsNil(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv(envIPCPhaseRecoveryStage, "")
	if got := resolveStallPolicy(false); got != nil {
		t.Fatalf("resolveStallPolicy(false) with no flag + unset env = %v; want nil "+
			"(legacy/fail-safe — a typo or unset must never enable the kill-path)", got)
	}
}

func TestResolveStallPolicy_InjectedEnvStillActivates(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv(envIPCPhaseRecoveryStage, "enforce")
	if got := resolveStallPolicy(false); got == nil {
		t.Fatal("resolveStallPolicy(false) with IPC env=enforce = nil; the injected-stage path must still activate")
	}
}

func TestResolveStallPolicy_TypoEnvIsNil(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	const typo = "enfoce"
	t.Setenv(envIPCPhaseRecoveryStage, typo)
	if got := resolveStallPolicy(false); got != nil {
		t.Fatalf("resolveStallPolicy(false) with env typo %q = %v; want nil (only exact \"enforce\" activates)", typo, got)
	}
}
