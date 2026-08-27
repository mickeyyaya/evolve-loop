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

import "testing"

func TestSeedAuditRepairContext(t *testing.T) {
	tests := []struct {
		name     string
		next     Phase
		attempts int
		wantSet  bool
	}{
		{name: "repair in flight seeds the builder", next: PhaseBuild, attempts: 1, wantSet: true},
		{name: "repair in flight seeds the test-first phase", next: PhaseTDD, attempts: 1, wantSet: true},
		{name: "no repair in flight seeds nothing", next: PhaseBuild, attempts: 0, wantSet: false},
		// Audit re-reads its own artifacts; re-injecting its own rejection
		// would be circular. Retro already has the full dossier.
		{name: "audit is not seeded", next: PhaseAudit, attempts: 1, wantSet: false},
		{name: "retro is not seeded", next: PhaseRetro, attempts: 1, wantSet: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAuditFailReason(t, dir, "audit", "EGPS: red_count=1 [record_absent_from_inbox_root_exactly_once]")
			cs := CycleState{WorkspacePath: dir, AuditRepairAttempts: tc.attempts}

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
	cs := CycleState{WorkspacePath: t.TempDir(), AuditRepairAttempts: 1}

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

	_ = seedAuditRepairContext(original, PhaseBuild, CycleState{WorkspacePath: dir, AuditRepairAttempts: 1})

	if _, leaked := original[CtxKeyAuditRepairFindings]; leaked {
		t.Error("seedAuditRepairContext mutated the caller's map; the brief would leak into later phases")
	}
}
