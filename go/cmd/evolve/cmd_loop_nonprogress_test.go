package main

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

// interleavedFailEmpty alternates FAIL and EMPTY, landing nothing every cycle.
var interleavedFailEmpty = []string{
	core.VerdictFAIL,
	core.CycleOutcomeSkippedUnknown,
	core.VerdictFAIL,
	core.CycleOutcomeSkippedUnknown,
	core.VerdictFAIL,
}

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
}

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
	joined := strings.Join(fired.reasons, ";")
	if !strings.Contains(joined, core.VerdictFAIL) || !strings.Contains(joined, core.CycleOutcomeSkippedUnknown) {
		t.Errorf("reasons = %v, want both the FAIL and the EMPTY class recorded", fired.reasons)
	}
}

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
		if esc := tr.observe(nonShippingOutcome(core.VerdictFAIL), core.VerdictFAIL, threshold); esc != nil {
			t.Fatalf("%s: escalated at streak 1 after a reset", shipped)
		}
		if esc := tr.observe(nonShippingOutcome(core.VerdictFAIL), core.VerdictFAIL, threshold); esc != nil {
			t.Fatalf("%s: escalated at streak 2 after a reset", shipped)
		}
	}
}

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
	if !strings.Contains(np.Title+np.Description, nonprogressKind.outcomes) {
		t.Errorf("non-progress item does not name its outcome class %q: title=%q", nonprogressKind.outcomes, np.Title)
	}
	if !strings.Contains(np.Description, core.VerdictFAIL) {
		t.Errorf("non-progress description must carry the recorded reasons: %q", np.Description)
	}
}

// verdictSeqOrch replays a fixed verdict sequence per RunCycle, since the real
// orchestrator cannot be scripted to emit an interleaved stream without a full
// phase machine.
type verdictSeqOrch struct {
	noSignals
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

func TestRunLoop_NonprogressBreaker_CallSite(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// max_consecutive_fails=2 keeps the FAIL breaker from halting (this stream
	// never hits 2 consecutive FAILs), and goal_stall.threshold=3 leaves the
	// empty-only breaker unable to fire (its longest EMPTY run is 1).
	// nonprogress_threshold is deliberately 3, not the compiled default 5, so the
	// filed streak below proves policy.json is actually read rather than
	// matching the default by accident.
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
	if got := loopOrchOverride.(*verdictSeqOrch).n; got != len(interleavedFailEmpty) {
		t.Errorf("ran %d cycles, want all %d — the escalation must not stop the queue", got, len(interleavedFailEmpty))
	}
}
