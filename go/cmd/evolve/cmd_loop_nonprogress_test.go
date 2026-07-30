package main

// cmd_loop_nonprogress_test.go — RED contract for the inbox defect
// `nonprogress-breaker-interleaved-fail-empty`.
//
// The loop had TWO non-progress breakers, each keyed to ONE outcome class:
//
//   - consecutiveFailBreaker (cmd_loop_control.go) counts consecutive
//     core.VerdictFAIL and HALTS the batch at max_consecutive_fails;
//   - goalStallTracker (cmd_loop_goalstall.go) counts consecutive EMPTY/BLOCKED
//     (SKIPPED_UNKNOWN / SKIPPED_AUDIT_ADVISORY) and escalates at goal_stall.threshold.
//
// Each RESETS on the other's outcome, so a goal alternating FAIL, EMPTY, FAIL,
// EMPTY … resets BOTH counters every cycle and crosses NEITHER threshold: it
// burns pipelines forever while landing nothing — exactly the class both
// breakers exist to stop. The union ("this cycle shipped nothing") is the real
// signal, and nothing counted it.
//
// ADVERSARIAL DIVERSITY: each behavioural claim is paired with its negative —
// the union breaker must fire on the interleaved stream (positive) AND must be
// reset by a genuinely shipping cycle (negative), so a naive "always escalate"
// implementation cannot pass. The call-site test additionally proves the two
// PRE-EXISTING breakers stay silent on the same stream, so the union counter is
// demonstrably load-bearing rather than redundant.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// interleavedFailEmpty is the escaping stream from the defect report: five
// cycles, alternating FAIL and EMPTY, landing nothing.
var interleavedFailEmpty = []string{
	core.VerdictFAIL,
	core.CycleOutcomeSkippedUnknown,
	core.VerdictFAIL,
	core.CycleOutcomeSkippedUnknown,
	core.VerdictFAIL,
}

// TestInterleavedFailEmpty_EscapesBothPreExistingBreakers is the RED evidence
// itself: driven by the SAME interleaved stream, neither pre-existing breaker
// ever crosses its threshold. This test does not exercise the fix — it pins the
// gap the fix closes, so a future edit that "simplifies" the union counter away
// has to confront why it exists.
func TestInterleavedFailEmpty_EscapesBothPreExistingBreakers(t *testing.T) {
	t.Parallel()
	const maxFails, goalStallThreshold = 3, 3

	failStreak := 0
	var goalStall goalStallTracker
	for i, verdict := range interleavedFailEmpty {
		var stop bool
		failStreak, stop = consecutiveFailBreaker(verdict == core.VerdictFAIL, failStreak, maxFails)
		if stop {
			t.Fatalf("cycle %d (%s): the consecutive-FAIL breaker stopped the batch — it cannot, an EMPTY cycle resets its streak", i+1, verdict)
		}
		emptyOrBlocked := verdict == core.CycleOutcomeSkippedUnknown || verdict == core.CycleOutcomeSkippedAuditAdvisory
		if esc := goalStall.observe(emptyOrBlocked, verdict, goalStallThreshold); esc != nil {
			t.Fatalf("cycle %d (%s): the goal-stall breaker escalated — it cannot, a FAIL cycle resets its streak", i+1, verdict)
		}
	}
	// Neither fired across five cycles that landed nothing: the escape is real.
}

// TestNonprogressTracker_InterleavedFailEmptyEscalates — the union counter must
// fire on the threshold-th consecutive NON-SHIPPING cycle regardless of class,
// so the interleaved stream that escapes both siblings escalates on cycle 5.
func TestNonprogressTracker_InterleavedFailEmptyEscalates(t *testing.T) {
	t.Parallel()
	const threshold = 5
	var tr goalStallTracker
	var fired *goalStallEscalation
	for i, verdict := range interleavedFailEmpty {
		esc := tr.observe(nonShippingOutcome(verdict), verdict, threshold)
		switch {
		case esc != nil && i < len(interleavedFailEmpty)-1:
			t.Fatalf("escalated early on cycle %d (%s), want the %dth", i+1, verdict, threshold)
		case esc != nil:
			fired = esc
		}
	}
	if fired == nil {
		t.Fatalf("the union breaker did NOT escalate after %d consecutive non-shipping cycles %v", threshold, interleavedFailEmpty)
	}
	if fired.streak != threshold {
		t.Errorf("streak = %d, want %d", fired.streak, threshold)
	}
	// Both classes must be named in the escalation so the filed todo says WHY.
	joined := strings.Join(fired.reasons, ";")
	if !strings.Contains(joined, core.VerdictFAIL) || !strings.Contains(joined, core.CycleOutcomeSkippedUnknown) {
		t.Errorf("reasons = %v, want both the FAIL and the EMPTY class recorded", fired.reasons)
	}
}

// TestNonShippingOutcome_Classification — the union predicate: only a cycle that
// actually LANDED something resets the counter. The PASS / SHIPPED_VIA_BUILD
// rows are the anti-false-positive half: without them a naive "everything is
// non-progress" predicate would escalate on healthy batches.
func TestNonShippingOutcome_Classification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		verdict string
		want    bool
	}{
		{core.VerdictPASS, false},
		{core.CycleOutcomeShippedViaBuild, false},
		{core.VerdictFAIL, true},
		{core.VerdictWARN, true},
		{core.CycleOutcomeSkippedUnknown, true},
		{core.CycleOutcomeSkippedAuditAdvisory, true},
		{core.VerdictSKIPPED, true},
		{"", true}, // no verdict recorded ⇒ certainly nothing shipped
	} {
		if got := nonShippingOutcome(tc.verdict); got != tc.want {
			t.Errorf("nonShippingOutcome(%q) = %v, want %v", tc.verdict, got, tc.want)
		}
	}
}

// TestNonprogressTracker_ShippingCycleResets — a single landing anywhere in the
// run resets the union counter (the acceptance's second bullet). Paired negative
// for the escalation test above.
func TestNonprogressTracker_ShippingCycleResets(t *testing.T) {
	t.Parallel()
	for _, shipped := range []string{core.VerdictPASS, core.CycleOutcomeShippedViaBuild} {
		var tr goalStallTracker
		const threshold = 3
		tr.observe(nonShippingOutcome(core.VerdictFAIL), core.VerdictFAIL, threshold)
		tr.observe(nonShippingOutcome(core.CycleOutcomeSkippedUnknown), core.CycleOutcomeSkippedUnknown, threshold)
		if esc := tr.observe(nonShippingOutcome(shipped), shipped, threshold); esc != nil {
			t.Fatalf("%s escalated — a shipping cycle must reset, not fire", shipped)
		}
		// Post-reset the streak restarts: two more non-shipping cycles must not fire.
		if esc := tr.observe(nonShippingOutcome(core.VerdictFAIL), core.VerdictFAIL, threshold); esc != nil {
			t.Fatalf("%s: escalated at streak 1 after a reset", shipped)
		}
		if esc := tr.observe(nonShippingOutcome(core.VerdictFAIL), core.VerdictFAIL, threshold); esc != nil {
			t.Fatalf("%s: escalated at streak 2 after a reset", shipped)
		}
	}
}

// TestStallKinds_DistinctInboxIdentity — the two escalations must not clobber
// each other's todo. Both ids are goal-stable (idempotent per goal) but must
// differ per KIND, else a non-progress escalation would silently overwrite the
// goal-stall todo for the same goal.
func TestStallKinds_DistinctInboxIdentity(t *testing.T) {
	t.Parallel()
	esc := &goalStallEscalation{streak: 5, reasons: []string{core.VerdictFAIL, core.CycleOutcomeSkippedUnknown}}
	const goalHash = "805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55"
	gs := buildGoalStallItem(goalStallKind, goalHash, esc, 0.9, 7, "2026-07-30T00:00:00Z")
	np := buildGoalStallItem(nonprogressKind, goalHash, esc, 0.9, 7, "2026-07-30T00:00:00Z")
	if gs.ID == np.ID {
		t.Fatalf("both kinds filed under id %q — one escalation would overwrite the other", gs.ID)
	}
	if err := np.validate(); err != nil {
		t.Fatalf("non-progress item must validate: %v", err)
	}
	// The wording must name the mixed outcome class, not "empty/blocked" — a
	// FAIL/EMPTY streak reported as empty-only sends the scout to the wrong place.
	if !strings.Contains(np.Title+np.Description, nonprogressKind.outcomes) {
		t.Errorf("non-progress item does not name its outcome class %q: title=%q", nonprogressKind.outcomes, np.Title)
	}
	if !strings.Contains(np.Description, core.VerdictFAIL) {
		t.Errorf("non-progress description must carry the recorded reasons: %q", np.Description)
	}
}

// verdictSeqOrch replays a fixed verdict sequence, one per RunCycle — the seam
// that exercises the loop's post-cycle breaker WIRING (the real orchestrator
// cannot be scripted to emit an interleaved FAIL/EMPTY stream without a faithful
// phase machine). Mirrors failingOrch (cmd_loop_failbreaker_test.go).
type verdictSeqOrch struct {
	verdicts []string
	n        int
}

func (s *verdictSeqOrch) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	v := ""
	if s.n < len(s.verdicts) {
		v = s.verdicts[s.n]
	}
	s.n++
	return core.CycleResult{Cycle: s.n, FinalVerdict: v}, nil
}

func (s *verdictSeqOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return s.RunCycle(ctx, req)
}

// TestRunLoop_NonprogressBreaker_CallSite is the WIRING proof: runLoop itself
// (the production caller) must file the non-progress todo when a goal alternates
// FAIL/EMPTY for the configured threshold — and must NOT stop the batch doing it
// (never_stop_queue_inject_inbox: escalate and continue; the hard-halt decision
// stays with the consecutive-FAIL breaker). It also pins that the sibling
// goal-stall todo is NOT filed on this stream, so the two breakers remain
// distinguishable in the artifacts an operator greps.
func TestRunLoop_NonprogressBreaker_CallSite(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// max_consecutive_fails=2 keeps the FAIL breaker from halting the batch (the
	// interleaved stream never reaches 2 CONSECUTIVE FAILs) and goal_stall.threshold=3
	// leaves the empty-only breaker demonstrably unable to fire on this stream
	// (its longest EMPTY run is 1). nonprogress_threshold is deliberately 3 —
	// NOT the compiled default of 5 — so the assertion below on the filed streak
	// proves the policy.json field is actually parsed and honoured, rather than
	// passing on a default that happens to match.
	policyJSON := `{"dispatch":{"policy":"off"},"workflow":{"max_consecutive_fails":2},` +
		`"goal_stall":{"threshold":3,"nonprogress_threshold":3}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"failedApproaches":[],"lastCycleNumber":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	storage := &fixtures.FakeStorage{}
	defer installStubDeps(t, storage, newFakeLedger())()
	prev := loopOrchOverride
	loopOrchOverride = &verdictSeqOrch{verdicts: interleavedFailEmpty}
	defer func() { loopOrchOverride = prev }()

	var stdout, stderr bytes.Buffer
	runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "interleaved fail/empty goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)

	inbox := filepath.Join(evolveDir, "inbox")
	np, err := filepath.Glob(filepath.Join(inbox, nonprogressKind.idPrefix+"*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(np) != 1 {
		t.Fatalf("want exactly 1 self-filed non-progress todo in %s, got %v\nstderr=%q", inbox, np, stderr.String())
	}
	// The filed streak must be the CONFIGURED 3, not the compiled default 5 —
	// the proof that goal_stall.nonprogress_threshold is read from policy.json.
	raw, err := os.ReadFile(np[0])
	if err != nil {
		t.Fatal(err)
	}
	var filed goalStallItem
	if err := json.Unmarshal(raw, &filed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filed.Title, "3 consecutive") {
		t.Errorf("filed title = %q, want the policy-configured threshold of 3 (a compiled default would say 5)", filed.Title)
	}
	if filed.Source != nonprogressKind.source {
		t.Errorf("filed source = %q, want %q so the two breakers stay distinguishable", filed.Source, nonprogressKind.source)
	}
	gs, err := filepath.Glob(filepath.Join(inbox, goalStallKind.idPrefix+"*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 0 {
		t.Errorf("the empty-only goal-stall breaker must NOT fire on an interleaved stream; got %v", gs)
	}
	if !strings.Contains(stderr.String(), "NONPROGRESS") {
		t.Errorf("no loud stderr escalation line; stderr=%q", stderr.String())
	}
	// All five scripted cycles must have run: the escalation is diagnostic, never a halt.
	if got := loopOrchOverride.(*verdictSeqOrch).n; got != len(interleavedFailEmpty) {
		t.Errorf("ran %d cycles, want all %d — the escalation must not stop the queue", got, len(interleavedFailEmpty))
	}
}
