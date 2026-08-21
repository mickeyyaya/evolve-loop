package core

// judgment_lesson_test.go — a JUDGMENT phase's FAIL verdict must leave a lesson.
//
// The defect: a judgment phase (premise-challenge, plan-review,
// adversarial-review) returns FAIL as a VERDICT with err == nil. Neither
// learning path catches that shape:
//
//   - recordFloorVerdictFailure fires only for isAuthoritativePhase() — the
//     resolved ship floor {tdd,build,audit} plus ship. No judgment phase is in it.
//   - recordFailureLearning early-returns unless fl.Err != nil. A FAIL verdict
//     carries no error.
//
// So a challenged premise left NO trace: the next cycle's Scout had no memory of
// it and re-derived the same doomed premise. Live instance — cycle-1528's
// premise-challenge FAIL produced a CRITICAL objection that survived only because
// a human copied it into an inbox item by hand.
//
// The fix records ONLY a carryover todo — deliberately NOT a FailedRecord.
// Appending to state.FailedAt would feed the failure adapter, which carries two
// documented halt vectors: tailInfraTransientStreak breaks on any foreign class,
// and sameClassStreak manufactures a streak from consecutive same-class records.
// A phase that TEACHES must not also be able to HALT the batch.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// judgmentLessonTodos returns the carryover todos whose action names phase.
func judgmentLessonTodos(cr *cycleRun, phase Phase) []CarryoverTodo {
	var out []CarryoverTodo
	for _, td := range cr.state.CarryoverTodos {
		if strings.Contains(td.Action, string(phase)) || strings.Contains(td.ID, string(phase)) {
			out = append(out, td)
		}
	}
	return out
}

// TestRecordAndBranch_PremiseChallengeFAILTeaches: the live trigger point. A
// premise-challenge FAIL verdict with NO dispatch error must leave a
// Scout-visible carryover todo carrying the objection.
func TestRecordAndBranch_PremiseChallengeFAILTeaches(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	dr := dispatchResult{resp: PhaseResponse{
		Verdict: VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error",
			Message: "premise falsified: the stderr buffer never contains provider error text"}},
	}, attemptCount: 1}

	if _, err := cr.recordAndBranch(Phase("premise-challenge"), dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	todos := judgmentLessonTodos(cr, Phase("premise-challenge"))
	if len(todos) != 1 {
		t.Fatalf("a premise-challenge FAIL must leave exactly ONE carryover todo so Scout cannot "+
			"re-derive the falsified premise; got %d (all todos: %+v)", len(todos), cr.state.CarryoverTodos)
	}
	if !strings.Contains(todos[0].Action, "stderr buffer never contains") {
		t.Errorf("the todo must carry the OBJECTION, not merely the fact of failure — a todo that says "+
			"'premise-challenge failed' teaches nothing\n  got: %q", todos[0].Action)
	}
}

// TestRecordAndBranch_PremiseChallengeFAILDoesNotRecordFailedApproach is the
// other half of the contract, and the reason the first attempt at this fix was
// blocked: teaching must not import a halt vector. state.FailedAt feeds
// failureadapter.Decide; a judgment phase appending there can manufacture a
// same-class streak and break the infra-transient tail streak.
func TestRecordAndBranch_PremiseChallengeFAILDoesNotRecordFailedApproach(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	dr := dispatchResult{resp: PhaseResponse{
		Verdict:     VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "premise falsified"}},
	}, attemptCount: 1}

	if _, err := cr.recordAndBranch(Phase("premise-challenge"), dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if len(cr.state.FailedAt) != 0 {
		t.Errorf("a judgment phase must TEACH without HALTING: appending to state.FailedAt feeds the "+
			"failure adapter (sameClassStreak / tailInfraTransientStreak), so a phase whose whole job is "+
			"to object could halt the batch by objecting; got %d record(s): %+v",
			len(cr.state.FailedAt), cr.state.FailedAt)
	}
}

// TestJudgmentLesson_PriorityIsNotP0: the failure path's todos are P0 because a
// floor failure blocks the cycle. A judgment lesson is advice for the NEXT
// cycle's planner — filed at P0 it outranks the actual blocking work and
// distorts triage.
func TestJudgmentLesson_PriorityIsNotP0(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	dr := dispatchResult{resp: PhaseResponse{
		Verdict:     VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "premise falsified"}},
	}, attemptCount: 1}
	if _, err := cr.recordAndBranch(Phase("premise-challenge"), dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}
	todos := judgmentLessonTodos(cr, Phase("premise-challenge"))
	if len(todos) != 1 {
		t.Fatalf("want 1 lesson todo, got %d", len(todos))
	}
	if todos[0].Priority == "P0" {
		t.Errorf("a judgment lesson must not be filed P0 — it would outrank the cycle's actual blocking "+
			"work in the next planner pass; got %q", todos[0].Priority)
	}
	if todos[0].Priority == "" {
		t.Errorf("lesson todo has no priority — the planner cannot rank it")
	}
}

// TestJudgmentLesson_ControlPhasesDoNotTeach pins the exact defect that blocked
// the first attempt. Deriving the teaching set as
// `remediationDenied[p] && !isAuthoritativePhase(p)` silently sweeps in retro and
// debugger — control phases, not judgment phases. recordFailureLearning already
// excludes PhaseRetro explicitly (a direct call routes AROUND that guard), and a
// retro that FAILs must not file a lesson about itself: retro is the mechanism
// that WRITES lessons, so it would recurse into its own output.
func TestJudgmentLesson_ControlPhasesDoNotTeach(t *testing.T) {
	for _, phase := range []Phase{PhaseRetro, PhaseDebugger} {
		t.Run(string(phase), func(t *testing.T) {
			cr := retroGateHarness(t, phasespec.Catalog{})
			dr := dispatchResult{resp: PhaseResponse{
				Verdict:     VerdictFAIL,
				Diagnostics: []Diagnostic{{Severity: "error", Message: "boom"}},
			}, attemptCount: 1}
			if _, err := cr.recordAndBranch(phase, dr); err != nil {
				t.Fatalf("recordAndBranch: %v", err)
			}
			if got := judgmentLessonTodos(cr, phase); len(got) != 0 {
				t.Errorf("%s is a CONTROL phase, not a judgment phase — it must not file a judgment "+
					"lesson (a derived remediationDenied set sweeps it in silently); got %+v", phase, got)
			}
		})
	}
}

// TestJudgmentLesson_PersistedToStorage: durability. recordAndBranch runs
// mid-loop and several abort branches return before finalizeCycle persists, so
// an in-memory-only append is lost exactly when the cycle dies — which is the
// case this fix exists for.
func TestJudgmentLesson_PersistedToStorage(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	dr := dispatchResult{resp: PhaseResponse{
		Verdict:     VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "premise falsified"}},
	}, attemptCount: 1}
	if _, err := cr.recordAndBranch(Phase("premise-challenge"), dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}
	persisted, err := cr.o.storage.ReadState(context.Background())
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	found := 0
	for _, td := range persisted.CarryoverTodos {
		if strings.Contains(td.ID, "premise-challenge") {
			found++
		}
	}
	if found != 1 {
		t.Errorf("the lesson must be PERSISTED, not only in-memory — the abort branches that make this "+
			"fix necessary return before finalizeCycle; got %d on disk", found)
	}
}

// TestJudgmentLesson_DedupedByID: a re-dispatched judgment phase failing twice in
// one cycle must not file the lesson twice.
func TestJudgmentLesson_DedupedByID(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	dr := dispatchResult{resp: PhaseResponse{
		Verdict:     VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "premise falsified"}},
	}, attemptCount: 1}
	for i := 0; i < 2; i++ {
		if _, err := cr.recordAndBranch(Phase("premise-challenge"), dr); err != nil {
			t.Fatalf("recordAndBranch #%d: %v", i+1, err)
		}
	}
	if got := judgmentLessonTodos(cr, Phase("premise-challenge")); len(got) != 1 {
		t.Errorf("lesson todo must dedupe by id across re-dispatch within a cycle; got %d", len(got))
	}
}

// TestJudgmentLesson_WiredInLockstepWithFloorRecorder is a WIRING proof, not a
// behavioral one. The two FAIL-verdict learning recorders must be wired at the
// same call sites: today the live loop (recordAndBranch) and the resume path,
// which re-enters mid-cycle after a crash.
//
// This exists because the failure mode is silent and path-specific. resume.go's
// own comment on the floor recorder records the precedent: wiring it on one path
// only "silently reproduce[s] the storm class for resumed cycles specifically" —
// every unit test passes, and the defect appears only after a mid-batch recovery.
// A third dispatch path added later must wire BOTH recorders or fail here.
func TestJudgmentLesson_WiredInLockstepWithFloorRecorder(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(src)
		// Call sites only — skip the file that DEFINES the floor recorder.
		if !strings.Contains(body, "o.recordFloorVerdictFailure(") {
			continue
		}
		if strings.Contains(body, "func (o *Orchestrator) recordFloorVerdictFailure(") {
			continue
		}
		checked++
		if !strings.Contains(body, "o.recordJudgmentLesson(") {
			t.Errorf("%s records a FLOOR-phase FAIL verdict but never records a JUDGMENT lesson — "+
				"a judgment phase failing on this path leaves no trace and Scout re-derives the "+
				"falsified premise. Wire recordJudgmentLesson beside recordFloorVerdictFailure.", name)
		}
	}
	// Guard the guard: if the call sites are ever renamed, this test must fail
	// loudly rather than silently vacuously passing over zero files.
	if checked < 2 {
		t.Errorf("expected at least 2 dispatch paths recording floor failures (live loop + resume); "+
			"found %d — the scan matched nothing, so this test proves nothing", checked)
	}
}

// TestJudgmentLesson_ExpiresAtSurvivesALoopPause: the lesson's TTL must not land
// in the 1-day fallback bucket. "cycle-mid-execution-fail" — the failure path's
// default — is OUTSIDE the taxonomy, so it normalizes to UnknownClassification
// and ages out in 24h; this repo routinely pauses the loop for days, and an
// expired lesson silently reproduces the "Scout has no memory" defect this whole
// file exists to prevent. A falsified premise does not become true again, so the
// lesson is classified IntentRejected (effectively never ages out).
//
// This is the mutation the rest of the suite misses: dedupe keys only off
// Action/ID, so breaking the TTL passes every other assertion here.
func TestJudgmentLesson_ExpiresAtSurvivesALoopPause(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	dr := dispatchResult{resp: PhaseResponse{
		Verdict:     VerdictFAIL,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "premise falsified"}},
	}, attemptCount: 1}
	if _, err := cr.recordAndBranch(Phase("premise-challenge"), dr); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}
	todos := judgmentLessonTodos(cr, Phase("premise-challenge"))
	if len(todos) != 1 {
		t.Fatalf("want 1 lesson todo, got %d", len(todos))
	}
	if todos[0].ExpiresAt == "" {
		t.Fatalf("lesson todo has no ExpiresAt — the boot prune (PruneExpiredCarryoverTodos) keys on it")
	}
	got, err := time.Parse(time.RFC3339, todos[0].ExpiresAt)
	if err != nil {
		t.Fatalf("ExpiresAt %q is not RFC3339: %v", todos[0].ExpiresAt, err)
	}
	// A week is the discriminator: the 1-day fallback bucket fails this, every
	// long-lived classification passes it. Asserting the BEHAVIOR (survives a
	// realistic pause) rather than the exact constant keeps this from breaking
	// when a TTL is retuned.
	if minWanted := cr.o.now().UTC().Add(7 * 24 * time.Hour); got.Before(minWanted) {
		t.Errorf("lesson expires at %s, less than a week out — a multi-day loop pause would drop it "+
			"before Scout ever reads it (the 1-day unknown-classification fallback bucket)", got)
	}
}

// TestJudgmentLesson_AuthoritativePhaseYieldsToFloorRecorder: policy.ship_floor is
// operator-configurable, so a judgment phase CAN be made authoritative. Both
// recorders would then fire for one FAIL, and because they synthesize an
// identical summary the fingerprint dedupe would collapse the floor path's P0
// todo into this path's P1 one — leaving an "advice" label shadowing a
// halt-capable FailedRecord. The lesson path yields.
func TestJudgmentLesson_AuthoritativePhaseYieldsToFloorRecorder(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	// Make the judgment phase authoritative, as an operator ship_floor override would.
	cr.o.shipFloor = []string{"premise-challenge"}
	if !cr.o.isAuthoritativePhase(Phase("premise-challenge")) {
		t.Skip("ship_floor override did not take effect through this harness; guard is untestable here")
	}
	cr.o.recordJudgmentLesson(context.Background(), 5, Phase("premise-challenge"), &cr.state,
		[]Diagnostic{{Severity: "error", Message: "premise falsified"}})

	for _, td := range cr.state.CarryoverTodos {
		if td.Priority == carryoverPriorityLesson {
			t.Errorf("an AUTHORITATIVE judgment phase must yield to recordFloorVerdictFailure — a P1 "+
				"advice todo would shadow a halt-capable FailedRecord for the same FAIL; got %+v", td)
		}
	}
}
