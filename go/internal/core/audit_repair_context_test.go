package core

// audit_repair_context_test.go — the other half of the wiring proof.
//
// audit_repair_prompt_test.go (build + tdd) proves the prompts RENDER the key.
// This proves the dispatch SETS it. Both halves are required: a rendered key
// nobody sets, or a set key nobody renders, are each silently inert — and an
// inert repair rebuilds blind and re-earns the same verdict at full cost.
//
// Derived from PERSISTED cycle state rather than pushed at grant time, so the
// live loop and the crash-resume path cannot diverge: there is one rule, and it
// reads a field that survives both.

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
		// THE LEAK (adversarial review, MEDIUM). AuditRepairAttempts is a
		// monotonic COUNTER, not a "currently repairing" flag. Gating on it meant
		// that once a cycle had ever repaired, ANY later re-entry into tdd/build —
		// Ship->Build or Debugger->TDD, both legal edges — re-injected a stale,
		// possibly already-resolved rejection with the prose "this cycle's audit
		// REJECTED your previous build", misdirecting an agent doing unrelated
		// ship-error recovery.
		{name: "a FINISHED repair does not leak into a later unrelated dispatch", next: PhaseBuild, active: false, attempts: 2, wantSet: false},
		// Audit re-reads its own artifacts; re-injecting its own rejection would
		// be circular. Retro already holds the full dossier.
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

// A repair whose fail-reason artifact is missing must not fabricate one, and
// must not crash the dispatch — it degrades to today's blind rebuild, loudly.
func TestSeedAuditRepairContext_MissingArtifactDegradesQuietly(t *testing.T) {
	cs := CycleState{WorkspacePath: t.TempDir(), AuditRepairAttempts: 1, AuditRepairActive: true}

	got := seedAuditRepairContext(map[string]string{}, PhaseBuild, cs)

	if v, ok := got[CtxKeyAuditRepairFindings]; ok && v == "" {
		t.Error("an empty findings value must not be set at all; the prompt keys on non-empty")
	}
}

// The caller's map must not be mutated — the dispatch loop reuses ctxSnap across
// iterations, so an in-place write would leak a stale repair brief into every
// later phase of the cycle.
func TestSeedAuditRepairContext_DoesNotMutateCallerMap(t *testing.T) {
	dir := t.TempDir()
	writeAuditFailReason(t, dir, "audit", "x")
	original := map[string]string{"a": "b"}

	_ = seedAuditRepairContext(original, PhaseBuild, CycleState{WorkspacePath: dir, AuditRepairAttempts: 1, AuditRepairActive: true})

	if _, leaked := original[CtxKeyAuditRepairFindings]; leaked {
		t.Error("seedAuditRepairContext mutated the caller's map; the brief would leak into later phases")
	}
}

// BOTH dispatch surfaces must seed the repair brief. cyclerun_dispatch.go is the
// live loop; resume_execution.go is the crash-resume path. The code claimed this symmetry
// in two separate comments ("cannot diverge from the resume path", "the live
// dispatch loop and the crash-resume path cannot diverge") while resume.go built
// its PhaseRequest without ever calling the seeder — so a cycle that crashed
// mid-repair burned an attempt and rebuilt BLIND, in exactly the crash-resilience
// case the persisted counter was designed for. The budget half was mirrored; the
// findings half was not.
func TestAuditRepairBrief_SeededOnBothDispatchSurfaces(t *testing.T) {
	for _, f := range []string{"cyclerun_dispatch.go", "resume_execution.go"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if !strings.Contains(string(body), "seedAuditRepairContext(") {
			t.Errorf("%s never calls seedAuditRepairContext; a repair dispatched from this surface rebuilds blind", f)
		}
		// ARGUMENT, not just presence. The first version of this guard checked
		// only that the call existed, and passed while resume.go passed the
		// PREVIOUS phase (`current`) instead of the one being dispatched — so the
		// seeding was wired and inert. Both surfaces name the dispatched phase
		// `next`; a call keyed on anything else is the same bug returning.
		if !strings.Contains(string(body), "seedAuditRepairContext(ctxSnap, next, cs)") &&
			!strings.Contains(string(body), "seedAuditRepairContext(phaseCtx, next, cr.cs)") {
			t.Errorf("%s calls seedAuditRepairContext with something other than the DISPATCHED phase; presence is not correctness", f)
		}
	}
}

// Cycle 1679 (2026-09-15): audit round 4 PASSED with WARN (three MEDIUM
// defects) and the cycle went to ship; GIT_FLEET_REBASE_NEEDED sent it back
// to build, and that rebuild's brief carried no audit section — the repair
// brief seeds only behind a rejection grant — so round 5 found the same
// defects standing. A recovery rebuild is re-audited by the same rubric: the
// last audit's actionable findings ride the brief under their own key, and a
// rejection grant (the repair path) still outranks them.
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

// Cycle 1684 (2026-09-15): the audit FAILed with a class outside the
// vocabulary, the envelope declined the direct grant, the retrospective
// adjudicated a retry, and the tdd/build re-entry carried NONE of the audit's
// findings (only a generic "audit.failure_class" label) — the builder rebuilt
// blind to "retire the superseded predicate". A retro-routed re-entry is
// re-audited by the same rubric, so it carries the standing findings exactly
// as a ship-error recovery does; the intro names which route brought it back.
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
	// A retro reached from a dispatch error or an exhausted correction ladder
	// (no audit decline) is not re-audited work: nothing is seeded, and the
	// prompt never claims an audit FAIL that did not happen.
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
