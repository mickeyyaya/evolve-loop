package core

// reset_autoseal_role_test.go — cycle-1194 continuation (ADR-0081 audit
// defect): the in-band ledger seal's trust anchor must not be reachable via
// the unattended boot self-heal path. AutosealStaleMarker is triggered
// merely by a dead owner PID (trivially arrangeable — kill the owning
// process), so before SealOptions.AutomatedRecovery existed, that path wrote
// the identical Role:"operator" a genuine human `evolve cycle reset` writes.
// Ledger verify's epoch-anchor resolver (go/internal/adapters/ledger/anchor.go)
// trusts ANY ledger line with Role=="operator" — Role/CycleLabel are
// otherwise self-declared, unauthenticated fields — so the automated path was
// enough to mint a trust-anchor-eligible seal with no human sign-off at all.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// TestSealCycle_ManualSeal_RoleIsOperator pins the human-invoked contract: a
// direct SealCycle call (as `evolve cycle reset` makes) still writes
// Role:"operator", the only role the ledger verify epoch-anchor resolver
// trusts.
func TestSealCycle_ManualSeal_RoleIsOperator(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	sealFixture(t, ev, 201)

	led := &recordingLedger{}
	if _, err := SealCycle(context.Background(), led, sealOpts(ev)); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}
	if len(led.entries) != 1 {
		t.Fatalf("want exactly one ledger append, got %d", len(led.entries))
	}
	if got := led.entries[0].Role; got != "operator" {
		t.Errorf("manual seal Role = %q, want \"operator\"", got)
	}
}

// TestSealCycle_AutomatedRecovery_RoleIsNotOperator is the regression guard
// for the audit defect: when SealOptions.AutomatedRecovery is set (the only
// way AutosealStaleMarker seals), the ledger entry's Role must NOT be
// "operator" — it must never be mistaken for a human trust decision by the
// epoch-anchor resolver.
func TestSealCycle_AutomatedRecovery_RoleIsNotOperator(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	sealFixture(t, ev, 202)

	opts := sealOpts(ev)
	opts.AutomatedRecovery = true
	led := &recordingLedger{}
	if _, err := SealCycle(context.Background(), led, opts); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}
	if len(led.entries) != 1 {
		t.Fatalf("want exactly one ledger append, got %d", len(led.entries))
	}
	if got := led.entries[0].Role; got == "operator" {
		t.Errorf("automated-recovery seal must not carry Role=%q — that would silently gain operator trust for chain verification with no human involved", got)
	}
}

// TestAutosealStaleMarker_SealsWithAutomatedRecoveryRole proves the wiring
// end-to-end: the real boot self-heal path (AutosealStaleMarker) — not just a
// hand-set SealOptions — produces a non-operator-role ledger entry.
func TestAutosealStaleMarker_SealsWithAutomatedRecoveryRole(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	workspace := sealFixture(t, evolveDir, 203)
	frozen := sealOpts(evolveDir).Now()
	if err := runlease.Write(workspace, runlease.Lease{RunID: "run-203", OwnerPID: 999999}, frozen); err != nil {
		t.Fatalf("seed lease: %v", err)
	}
	dead := func(int) bool { return false }

	led := &recordingLedger{}
	_, sealed, err := AutosealStaleMarker(context.Background(), led, sealOpts(evolveDir), dead)
	if err != nil {
		t.Fatalf("AutosealStaleMarker: %v", err)
	}
	if !sealed {
		t.Fatal("dead-owner marker must be auto-sealed")
	}
	if len(led.entries) != 1 {
		t.Fatalf("want exactly one ledger append, got %d", len(led.entries))
	}
	if got := led.entries[0].Role; got == "operator" {
		t.Errorf("AutosealStaleMarker must never write Role=%q — it runs unattended at boot with no human sign-off", got)
	}
}
