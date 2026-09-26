package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedAuditRepairContext(t *testing.T) {
	tests := []struct {
		name     string
		next     Phase
		active   bool
		attempts int
		wantSet  bool
	}{
		{name: "an active repair seeds the builder", next: PhaseBuild, active: true, attempts: 1, wantSet: true},
		{name: "an active repair seeds the test-first phase", next: PhaseTDD, active: true, attempts: 1, wantSet: true},
		{name: "no repair in flight seeds nothing", next: PhaseBuild, active: false, attempts: 0, wantSet: false},
		// AuditRepairAttempts is a monotonic counter, not a currently-repairing flag.
		{name: "a FINISHED repair does not leak into a later unrelated dispatch", next: PhaseBuild, active: false, attempts: 2, wantSet: false},
		// Audit re-reads its own artifacts, so re-injecting its own rejection would be circular.
		{name: "audit is not seeded", next: PhaseAudit, active: true, attempts: 1, wantSet: false},
		{name: "retro is not seeded", next: PhaseRetro, active: true, attempts: 1, wantSet: false},
		{name: "ship is not seeded", next: PhaseShip, active: true, attempts: 1, wantSet: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAuditFailReason(t, dir, "audit", "EGPS: red_count=1 [record_absent_from_inbox_root_exactly_once]")
			cs := CycleState{WorkspacePath: dir, AuditRepairAttempts: tc.attempts, AuditRepairActive: tc.active}

			got := seedAuditRepairContext(map[string]string{"keep": "me"}, tc.next, cs)

			if _, ok := got[CtxKeyAuditRepairFindings]; ok != tc.wantSet {
				t.Errorf("key set = %v, want %v", ok, tc.wantSet)
			}
			if got["keep"] != "me" {
				t.Error("seeding must preserve the existing context entries")
			}
		})
	}
}

func TestSeedAuditRepairContext_MissingArtifactDegradesQuietly(t *testing.T) {
	cs := CycleState{WorkspacePath: t.TempDir(), AuditRepairAttempts: 1, AuditRepairActive: true}

	got := seedAuditRepairContext(map[string]string{}, PhaseBuild, cs)

	if v, ok := got[CtxKeyAuditRepairFindings]; ok && v == "" {
		t.Error("an empty findings value must not be set at all; the prompt keys on non-empty")
	}
}

func TestSeedAuditRepairContext_DoesNotMutateCallerMap(t *testing.T) {
	dir := t.TempDir()
	writeAuditFailReason(t, dir, "audit", "x")
	original := map[string]string{"a": "b"}

	_ = seedAuditRepairContext(original, PhaseBuild, CycleState{WorkspacePath: dir, AuditRepairAttempts: 1, AuditRepairActive: true})

	if _, leaked := original[CtxKeyAuditRepairFindings]; leaked {
		t.Error("seedAuditRepairContext mutated the caller's map; the brief would leak into later phases")
	}
}

func TestAuditRepairBrief_SeededOnBothDispatchSurfaces(t *testing.T) {
	for _, f := range []string{"cyclerun_dispatch.go", "resume_execution.go"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(body), "seedAuditRepairContext(") {
			t.Errorf("%s never calls seedAuditRepairContext; a repair dispatched from this surface rebuilds blind", f)
		}
		// Both surfaces must pass the dispatched phase `next`, not merely call the seeder.
		if !strings.Contains(string(body), "seedAuditRepairContext(ctxSnap, next, cs)") &&
			!strings.Contains(string(body), "seedAuditRepairContext(phaseCtx, next, cr.cs)") {
			t.Errorf("%s calls seedAuditRepairContext with something other than the DISPATCHED phase; presence is not correctness", f)
		}
	}
}

func TestSeedAuditRepairContext_ShipRecoveryRebuildCarriesStandingFindings(t *testing.T) {
	dir := t.TempDir()
	report := "# Audit Report\n\n## Verdict\nWARN\n\n## Issues\n\n### M1 (MEDIUM) — claim-discrepancy: the record says five cycle predicates while the tree carries eight\nb\n\n### L1 (LOW) — a nit the builder may ignore\nb\n"
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{WorkspacePath: dir, AuditDispatches: 4, ShipRecoveryCode: "GIT_FLEET_REBASE_NEEDED"}
	base := map[string]string{"keep": "me"} // a resumed dispatch: no ship_error_code snapshot

	got := seedAuditRepairContext(base, PhaseBuild, cs)

	findings := got[CtxKeyStandingAuditFindings]
	if !strings.Contains(findings, "claim-discrepancy") {
		t.Fatalf("a rebuild after a ship error carries the last audit's actionable findings, got %q", findings)
	}
	if strings.Contains(findings, "nit the builder may ignore") {
		t.Errorf("LOW findings are not actionable and stay out of the brief: %q", findings)
	}
	if got["keep"] != "me" || base[CtxKeyStandingAuditFindings] != "" {
		t.Error("seeding copies on write and preserves the existing entries")
	}
	if got[CtxKeyShipErrorCode] != "GIT_FLEET_REBASE_NEEDED" {
		t.Errorf("the prompt names the code from persisted state when the snapshot lacks it, got %q", got[CtxKeyShipErrorCode])
	}
	if _, ok := seedAuditRepairContext(base, PhaseBuild, CycleState{WorkspacePath: dir, AuditDispatches: 4})[CtxKeyStandingAuditFindings]; ok {
		t.Error("no ship-error recovery in flight → an ordinary build is not seeded with standing findings")
	}
	latched := cs
	if !latchShippedState(&latched, PhaseShip, VerdictPASS) || latched.ShipRecoveryCode != "" {
		t.Errorf("the ship latch ends the recovery: ShipRecoveryCode=%q", latched.ShipRecoveryCode)
	}
	if _, ok := seedAuditRepairContext(base, PhaseAudit, cs)[CtxKeyStandingAuditFindings]; ok {
		t.Error("the audit re-reads its own report; it is never seeded")
	}
	writeAuditFailReason(t, dir, "audit", "EGPS: red_count=1 [x]")
	rejected := seedAuditRepairContext(base, PhaseBuild, CycleState{WorkspacePath: dir, AuditDispatches: 4, AuditRepairActive: true, AuditRepairAttempts: 1, ShipRecoveryCode: "GIT_FLEET_REBASE_NEEDED"})
	if rejected[CtxKeyAuditRepairFindings] == "" || rejected[CtxKeyStandingAuditFindings] != "" {
		t.Errorf("a rejection grant is the repair path and outranks standing findings: %v", rejected)
	}
}

func TestSeedAuditRepairContext_RetroRoutedReentryCarriesStandingFindings(t *testing.T) {
	dir := t.TempDir()
	report := "# Audit Report\n\n## Verdict\nFAIL\n\n## Issues\n\n### M1 (MEDIUM) — superseded predicate: TestC1515_006 contradicts the commissioned change; retire it in-phase\nb\n"
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	declined := CycleState{WorkspacePath: dir, AuditDispatches: 1, CompletedPhases: []string{"scout", "triage", "tdd", "build", "audit", "retro"}}
	consumeAuditRepairGrant(&declined, auditDeclineReasonPrefix+"audit declared an unrecognised class superseded-predicate-contradiction")
	if declined.AuditDeclineReason == "" || declined.AuditRepairActive {
		t.Fatalf("a decline records its reason on persisted state without granting: %+v", declined)
	}
	got := seedAuditRepairContext(map[string]string{"keep": "me"}, PhaseTDD, declined)
	if !strings.Contains(got[CtxKeyStandingAuditFindings], "retire it in-phase") {
		t.Fatalf("a retro-routed tdd re-entry after a decline carries the last audit's actionable findings, got %q", got[CtxKeyStandingAuditFindings])
	}
	if got[CtxKeyShipErrorCode] != "" {
		t.Errorf("no ship error was involved; the code must not be fabricated: %q", got[CtxKeyShipErrorCode])
	}
	if !strings.Contains(got[CtxKeyAuditDeclineReason], "unrecognised class") {
		t.Errorf("the envelope's decline reason rides the context for the prompt: %q", got[CtxKeyAuditDeclineReason])
	}
	if intro := StandingFindingsIntro(got); !strings.Contains(intro, "retrospective") || !strings.Contains(intro, "unrecognised class") || strings.Contains(intro, "ship-time error") {
		t.Errorf("the intro names the retro route and the envelope's reason, not a ship error: %q", intro)
	}
	// A retro reached without an audit decline is not re-audited work, so nothing is seeded.
	viaDispatchError := CycleState{WorkspacePath: dir, AuditDispatches: 1, CompletedPhases: []string{"scout", "triage", "tdd", "build", "retro"}}
	if _, ok := seedAuditRepairContext(map[string]string{}, PhaseBuild, viaDispatchError)[CtxKeyStandingAuditFindings]; ok {
		t.Error("a retro re-entry without an audit decline is not seeded")
	}
	granted := declined
	consumeAuditRepairGrant(&granted, auditRepairReasonPrefix+"retry-build: within budget")
	if granted.AuditDeclineReason != "" || !granted.AuditRepairActive {
		t.Errorf("a later grant clears the decline: %+v", granted)
	}
	latched := declined
	if !latchShippedState(&latched, PhaseShip, VerdictPASS) || latched.AuditDeclineReason != "" {
		t.Errorf("the ship latch spends the decline: %q", latched.AuditDeclineReason)
	}
	if intro := StandingFindingsIntro(map[string]string{CtxKeyShipErrorCode: "GIT_FLEET_REBASE_NEEDED"}); !strings.Contains(intro, "GIT_FLEET_REBASE_NEEDED") {
		t.Errorf("the intro names the ship error when one brought the cycle back: %q", intro)
	}
	ordinary := CycleState{WorkspacePath: dir, AuditDispatches: 1, CompletedPhases: []string{"scout", "triage", "tdd"}}
	if _, ok := seedAuditRepairContext(map[string]string{}, PhaseBuild, ordinary)[CtxKeyStandingAuditFindings]; ok {
		t.Error("a first-pass build (no retro, no ship error) is not seeded")
	}
}
