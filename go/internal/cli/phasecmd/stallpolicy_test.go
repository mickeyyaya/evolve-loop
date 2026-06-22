package phasecmd

import "testing"

// resolveStallPolicy activates the ADR-0044 chain-backed stall policy ONLY at
// enforce, from either the operator's --enforce flag (the live signal for the
// standalone `evolve phase-observer` subcommand) or an injected IPC stage key.
// These tests pin that contract — the behavioral signal the deterministic gates
// (build/vet/test, flagreaders) structurally cannot provide, which is why a
// prior cycle's rename to a never-injected env key silently disabled the manual
// --enforce path and shipped green.
//
// All cases force EVOLVE_PROJECT_ROOT to a temp dir so policy.Load fails over to
// a zero Policy (hermetic — no dependency on a real policy.json).

func TestResolveStallPolicy_EnforceFlagActivates(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	t.Setenv(envIPCPhaseRecoveryStage, "") // key absent — only --enforce should drive
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
	t.Setenv(envIPCPhaseRecoveryStage, "enforce") // a parent that injects the stage
	if got := resolveStallPolicy(false); got == nil {
		t.Fatal("resolveStallPolicy(false) with IPC env=enforce = nil; the injected-stage path must still activate")
	}
}

func TestResolveStallPolicy_TypoEnvIsNil(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	const typo = "enfoce" // misspelled (missing 'r') — must NOT activate the kill-path
	t.Setenv(envIPCPhaseRecoveryStage, typo)
	if got := resolveStallPolicy(false); got != nil {
		t.Fatalf("resolveStallPolicy(false) with env typo %q = %v; want nil (only exact \"enforce\" activates)", typo, got)
	}
}
