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
// live loop; resume.go is the crash-resume path. The code claimed this symmetry
// in two separate comments ("cannot diverge from the resume path", "the live
// dispatch loop and the crash-resume path cannot diverge") while resume.go built
// its PhaseRequest without ever calling the seeder — so a cycle that crashed
// mid-repair burned an attempt and rebuilt BLIND, in exactly the crash-resilience
// case the persisted counter was designed for. The budget half was mirrored; the
// findings half was not.
func TestAuditRepairBrief_SeededOnBothDispatchSurfaces(t *testing.T) {
	for _, f := range []string{"cyclerun_dispatch.go", "resume.go"} {
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
