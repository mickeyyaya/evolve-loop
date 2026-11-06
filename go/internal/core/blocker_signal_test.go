package core

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const wantRouteStep = "at the boundary, the console routes each item that needs this path with evolve inbox route-console <id> <reason> <cycle>. A quarantined item needs evolve inbox quarantine release <id> first."

func routeWorkspace(t *testing.T, pin []string, decision string) string {
	t.Helper()
	ws := t.TempDir()
	if pin != nil {
		body := `{"todo_ids":["` + strings.Join(pin, `","`) + `"]}`
		if err := os.WriteFile(filepath.Join(ws, committedset.LanePinFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if decision != "" {
		if err := os.WriteFile(filepath.Join(ws, committedset.DecisionFile), []byte(decision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func TestBlockerSignal_TheUnroutableTextNamesTheSourceOfTheSet(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pin  []string
		dec  string
		want func(ws string) string
	}{
		{"pin", []string{"a", "b"}, "", func(string) string {
			return "the blocker is proven, and this lane is pinned on items a, b, and a console route needs exactly one, so this cycle ends with no route. Next: " + wantRouteStep
		}},
		{"decision", nil, `{"top_n":[{"id":"a"},{"id":"b"}]}`, func(string) string {
			return "the blocker is proven, and the triage decision commits items a, b, and a console route needs exactly one, so this cycle ends with no route. Next: " + wantRouteStep
		}},
		{"empty", nil, `{"top_n":[]}`, func(ws string) string {
			return "the blocker is proven, and the triage decision commits no item, so this cycle ends with no route. Next: the console reads " + filepath.Join(ws, committedset.DecisionFile) + "."
		}},
		{"unknown", nil, "", func(ws string) string {
			return "the blocker is proven, and neither " + filepath.Join(ws, committedset.LanePinFile) + " nor " + filepath.Join(ws, committedset.DecisionFile) + " gives a commitment, so this cycle ends with no route. Next: the console reads both paths."
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := routeWorkspace(t, tc.pin, tc.dec)
			item, gap, ok := consoleRouteChoice(ws)
			if ok || item != "" {
				t.Fatalf("consoleRouteChoice = %q, %v; want no route", item, ok)
			}
			if got := gap.unroutable().text(); got != tc.want(ws) {
				t.Errorf("text = %q\nwant %q", got, tc.want(ws))
			}
		})
	}
}

func TestBlockerSignal_OneCommittedItemRoutes(t *testing.T) {
	t.Parallel()
	item, _, ok := consoleRouteChoice(routeWorkspace(t, nil, `{"top_n":[{"id":"only"}]}`))
	if !ok || item != "only" {
		t.Errorf("consoleRouteChoice = %q, %v; want only, true", item, ok)
	}
}

func TestBlockerSignal_TheHandoffTextNamesTheSourceAndTheReason(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pin  []string
		dec  string
		want func(ws string) string
	}{
		{"pin", []string{"a", "b"}, "", func(string) string {
			return "cycle 7 handed off. No item routes to the console: this lane is pinned on items a, b, and a console route needs exactly one. Next: " + wantRouteStep + " The hand-off reason: blockers a, b; console items c"
		}},
		{"decision", nil, `{"top_n":[{"id":"a"},{"id":"b"}]}`, func(string) string {
			return "cycle 7 handed off. No item routes to the console: the triage decision commits items a, b, and a console route needs exactly one. Next: " + wantRouteStep + " The hand-off reason: blockers a, b; console items c"
		}},
		{"empty", nil, `{"top_n":[]}`, func(ws string) string {
			return "cycle 7 handed off. No item routes to the console: the triage decision commits no item. Next: the console reads " + filepath.Join(ws, committedset.DecisionFile) + ". The hand-off reason: blockers a, b; console items c"
		}},
		{"unknown", nil, "", func(ws string) string {
			return "cycle 7 handed off. No item routes to the console: neither " + filepath.Join(ws, committedset.LanePinFile) + " nor " + filepath.Join(ws, committedset.DecisionFile) + " gives a commitment. Next: the console reads both paths. The hand-off reason: blockers a, b; console items c"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := routeWorkspace(t, tc.pin, tc.dec)
			_, gap, _ := consoleRouteChoice(ws)
			if got := gap.handoff(7, "blockers a, b; console items c"); got != tc.want(ws) {
				t.Errorf("text = %q\nwant %q", got, tc.want(ws))
			}
		})
	}
}

func TestBlockerSignal_TheIncompleteTextsSayWhatToDo(t *testing.T) {
	t.Parallel()
	tail := ". This cycle ends with no charge to its items, because no correction can supply a base. The resume batch stops, and starting a new batch seals a new base. Next: start a new batch. If it repeats, file a pipeline item."
	cases := []struct {
		name string
		base func(*testing.T, string, string)
		want func(ws string) string
	}{
		{"missing", noBase, func(ws string) string {
			return "the declared blocker cannot be judged: no predicate base at " + filepath.Join(ws, predicateSnapshotFile) + " for this Build entry" + tail
		}},
		{"corrupt", corruptBase, func(ws string) string {
			return "the declared blocker cannot be judged: the predicate base at " + filepath.Join(ws, predicateSnapshotFile) + " does not parse: unexpected end of JSON input" + tail
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run := runResumedWith(t, resumedSetup{failures: []string{cycle1946PredicateFailure}, defects: []string{cycle1946Claim}, decision: oneCardDecision, env: laneEnv, base: tc.base})
			ev, ok := run.signal(CodeBlockerRefused)
			if !ok || ev.Reason != tc.want(run.workspace) {
				t.Errorf("signal = %+v ok=%v\nwant %q", ev, ok, tc.want(run.workspace))
			}
		})
	}
}

func TestBlockerSignal_TheEditTextNamesEachChangedAddedAndRemovedFile(t *testing.T) {
	t.Parallel()
	c := BlockerCase{
		Cycle:               9,
		Worktree:            blockerWorktree(t),
		PredicatesAtRequest: map[string]string{"kept_test.go": "a", "edited_test.go": "b", "gone_test.go": "c"},
		PredicatesNow:       map[string]string{"kept_test.go": "a", "edited_test.go": "B", "new_test.go": "d"},
		Failures:            []string{cycle1946PredicateFailure},
		Defects:             []string{cycle1946Claim},
		Explainable:         buildFloorExplainable,
		Protected:           protectsSelfcheck,
	}
	want := "the declared blocker is not proven: the lane changed the predicates of the cycle after the phase request: go/acs/cycle9/edited_test.go, go/acs/cycle9/gone_test.go (removed), go/acs/cycle9/new_test.go (added). Next: restore each changed file and remove each added file, then declare the blocker again."
	if got := NewBlockerProver().Prove(c).Statement.text(); got != want {
		t.Errorf("text = %q\nwant %q", got, want)
	}
}

func TestBlockerSignal_TheGitFaultTextAsksForAnotherDeclaration(t *testing.T) {
	t.Parallel()
	c := BlockerCase{Failures: []string{cycle1946PredicateFailure}, Defects: []string{cycle1946Claim}, Explainable: buildFloorExplainable, EvidenceFault: "git diff --numstat abc failed (rc=128, <nil>)"}
	want := "the declared blocker is not proven: the evidence could not be read: git diff --numstat abc failed (rc=128, <nil>). Next: declare the blocker again. The host reads the evidence again."
	if got := NewBlockerProver().Prove(c).Statement.text(); got != want {
		t.Errorf("text = %q\nwant %q", got, want)
	}
}

func TestBlockerSignal_TheVerifierRefusalTextNamesTheClaim(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	c := BlockerCase{
		Worktree: blockerWorktree(t), Failures: []string{cycle1950Failure}, Defects: []string{fake},
		Explainable: buildFloorExplainable, Protected: protectsSelfcheck,
	}
	want := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it. Next: correct the BLOCKER line of your report so each claim is true, or fix the failures without declaring a blocker."
	if got := NewBlockerProver().Prove(c).Statement.text(); got != want {
		t.Errorf("text = %q\nwant %q", got, want)
	}
}

func TestBlockerSignal_ThePartialTextKeepsTheProvenPathClosed(t *testing.T) {
	t.Parallel()
	other := "./acs/cycle9 (-tags acs): unit tests FAIL\n--- FAIL: TestC9_001_Other (0.10s)\n    predicates_test.go:5: in go/internal/core/other.go the thing is wrong\nFAIL\n"
	c := BlockerCase{
		Worktree: blockerWorktree(t), Failures: []string{cycle1946PredicateFailure, other}, Defects: []string{cycle1946Claim},
		Explainable: buildFloorExplainable, Protected: protectsSelfcheck,
	}
	want := "the blocker on " + selfcheckPath + " is proven, and these failures are not explained: ./acs/cycle9 (-tags acs): unit tests FAIL. Next: fix only these failures. Do not change " + selfcheckPath + "."
	proof := NewBlockerProver().Prove(c)
	if proof.Blocked() || proof.Statement.text() != want {
		t.Errorf("blocked=%v text = %q\nwant %q", proof.Blocked(), proof.Statement.text(), want)
	}
}

func TestBlockerSignal_TheLastRoundEndsTheCycleWithTheQueueStep(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	fact := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it"
	standard := fact + ". Next: correct the BLOCKER line of your report so each claim is true, or fix the failures without declaring a blocker."
	last := fact + ". This was the last correction, so this cycle ends. Next: the item returns to the queue. A quarantined item needs evolve inbox quarantine release <id>."
	run := runResumedWith(t, resumedSetup{
		failures: []string{cycle1950Failure}, defects: []string{fake}, decision: oneCardDecision, env: laneEnv,
		retry: &policy.RetryConfig{PhaseMaxAttempts: 1, ContractCorrectionRetries: 1},
	})
	var got []string
	for _, e := range run.signals {
		if e.Code == CodeBlockerRefused {
			got = append(got, e.Reason)
		}
	}
	if len(got) != 2 || got[0] != standard || got[1] != last {
		t.Fatalf("refused signals = %q\nwant [%q, %q]", got, standard, last)
	}
	if run.dispatches != 2 || len(run.directives) != 2 || !strings.HasSuffix(run.directives[1], standard) {
		t.Errorf("dispatches=%d directives=%q; the correction carries the standard text once", run.dispatches, run.directives)
	}
}

func TestBlockerSignal_TheDirectiveHoldsTheRefusalOnceAfterTheDigestChoice(t *testing.T) {
	t.Parallel()
	req := PhaseRequest{CorrectionDirective: "digest body"}
	if got := withBlockerRefusal(preferDigest("plain", req), "why"); got != "plain\n\nThe host refused your BLOCKER claim: why" {
		t.Errorf("plain = %q", got)
	}
	if got := withBlockerRefusal("digest body", ""); got != "digest body" {
		t.Errorf("empty refusal = %q", got)
	}
}

const twoItemDecision = `{"top_n":[{"id":"a"},{"id":"b"}]}`

func capped(s string) string {
	if r := []rune(s); len(r) > 512 {
		return string(r[:511]) + "…"
	}
	return s
}

func wantHandoffNoRoute(cycle int, reason string) string {
	return capped("cycle " + strconv.Itoa(cycle) + " handed off. No item routes to the console: the triage decision commits items a, b, and a console route needs exactly one. Next: " + wantRouteStep +
		" The hand-off reason: " + strings.TrimPrefix(reason, repairHandoffReasonPrefix))
}

func noRouteSignals(events []signalcenter.Event) []signalcenter.Event {
	return eventsWithCode(events, CodeHandoffNoRoute)
}

func TestBlockerSignal_AFixabilityHandoffAtTheLiveAuditBoundaryNamesTheMissingRoute(t *testing.T) {
	t.Parallel()
	o, _, events := handoffOrchestrator(policy.FixabilityEnforce, handoffStop)
	cs := handoffState(t)
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, committedset.DecisionFile), []byte(twoItemDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	_, reason, _ := o.decideAfterAuditFail(cs)
	*events = nil
	cr := &cycleRun{o: o, ctx: context.Background(), cycle: cs.CycleID, cs: cs, req: CycleRequest{ProjectRoot: t.TempDir()}}

	if _, err := cr.recordAndBranch(PhaseAudit, dispatchResult{resp: PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL}, attemptCount: 1}); err != nil {
		t.Fatal(err)
	}

	got := noRouteSignals(*events)
	if len(got) != 1 || got[0].Reason != wantHandoffNoRoute(cs.CycleID, reason) || cr.result.TerminationReason != CycleTerminationHandedOff {
		t.Errorf("signals = %+v termination %q\nwant one %s with %q", got, cr.result.TerminationReason, CodeHandoffNoRoute, wantHandoffNoRoute(cs.CycleID, reason))
	}
}

func TestBlockerSignal_AFixabilityHandoffOnResumeNamesTheMissingRoute(t *testing.T) {
	t.Parallel()
	o, _, events := handoffOrchestrator(policy.FixabilityEnforce, handoffStop)
	cs := handoffState(t)
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, committedset.DecisionFile), []byte(twoItemDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	_, reason, _ := o.decideAfterAuditFail(cs)
	*events = nil
	cursor := newResumeCursor(PhaseAudit)
	var result CycleResult

	if err := o.resumeAfterAudit(&cs, &cursor, &result, []phaseTimingEntry{{Phase: "audit", Verdict: VerdictFAIL}}, VerdictFAIL); err != nil {
		t.Fatal(err)
	}

	got := noRouteSignals(*events)
	if len(got) != 1 || got[0].Reason != wantHandoffNoRoute(cs.CycleID, reason) || cursor.termination.reason != CycleTerminationHandedOff {
		t.Errorf("signals = %+v\nwant one %s with %q", got, CodeHandoffNoRoute, wantHandoffNoRoute(cs.CycleID, reason))
	}
}

func TestBlockerSignal_ASpendLimitHandoffNamesTheMissingRoute(t *testing.T) {
	t.Parallel()
	cr, events := spendRun(t, config.StageEnforce, PhaseAudit)
	cr.cs.ActiveWorktree = "/dev/cl-c2035-lane"
	cr.lastVerdict = VerdictFAIL
	if err := os.WriteFile(filepath.Join(cr.cs.WorkspacePath, committedset.DecisionFile), []byte(twoItemDecision), 0o644); err != nil {
		t.Fatal(err)
	}

	cr.checkCycleSpend()

	got := noRouteSignals(*events)
	reason := "spend_limit: max_llm_minutes 160.1 of 90; salvage /dev/cl-c2035-lane"
	if len(got) != 1 || got[0].Reason != wantHandoffNoRoute(2035, reason) {
		t.Errorf("signals = %+v\nwant one %s with %q", got, CodeHandoffNoRoute, wantHandoffNoRoute(2035, reason))
	}
}

func TestBlockerSignal_AFloorResidualHandoffNamesTheMissingRoute(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, RungResult{Handled: true, Stop: true, Reason: "audit_fix_exhausted"})
	if err := os.WriteFile(filepath.Join(c.cr.cs.WorkspacePath, committedset.DecisionFile), []byte(twoItemDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	held := blockerStatement{fact: "held", step: "none"}

	if act, err := c.cr.rejectRefused(PhaseBuild, c.dr, floorRejected([]string{omissionFailure}, omissionResidualFixture()), 1, held); act != loopBreak || err != nil {
		t.Fatalf("rejectAfterCorrections = %v, %v", act, err)
	}

	got := noRouteSignals(*c.events)
	if len(got) != 1 || !strings.HasPrefix(got[0].Reason, wantHandoffHead+" The hand-off reason: audit_fix_exhausted: ") {
		t.Errorf("signals = %+v; want one %s for cycle 2093", got, CodeHandoffNoRoute)
	}
	if len(eventsWithCode(*c.events, CodeBlockerRefused)) != 0 {
		t.Errorf("a handed-off cycle emits no refusal text: %+v", *c.events)
	}
}

const wantHandoffHead = "cycle 2093 handed off. No item routes to the console: the triage decision commits items a, b, and a console route needs exactly one. Next: " + wantRouteStep

func TestBlockerSignal_TheDeliveredBuildHasItsOwnText(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	claim := blockerStatement{fact: "the declared blocker is not proven: a claim", step: "none"}

	act, err := c.cr.rejectRefused(PhaseBuild, c.dr, floorRejected([]string{coverResidualFailure}, coverResidualsFixture()), 1, claim)

	want := "the declared blocker is not proven: a claim. This was the last correction, and the audit_fix escalation delivers the build. Next: the audit reads the build."
	got := eventsWithCode(*c.events, CodeBlockerRefused)
	if act != loopNext || err != nil || len(got) != 1 || got[0].Reason != want {
		t.Errorf("act=%v err=%v signals=%+v\nwant one refusal %q", act, err, got, want)
	}
}

func TestBlockerSignal_AnAbortedBuildEmitsTheLastRoundText(t *testing.T) {
	t.Parallel()
	c := floorResidualHarness(t, policy.RetryLadderEnforce, passedFix)
	claim := blockerStatement{fact: "the declared blocker is not proven: a claim", step: "none"}

	act, _ := c.cr.rejectRefused(PhaseBuild, c.dr, floorRejected([]string{testFailure}, nil), 1, claim)

	want := "the declared blocker is not proven: a claim. This was the last correction, so this cycle ends. Next: the item returns to the queue. A quarantined item needs evolve inbox quarantine release <id>."
	got := eventsWithCode(*c.events, CodeBlockerRefused)
	if act != loopAbort || len(got) != 1 || got[0].Reason != want {
		t.Errorf("act=%v signals=%+v\nwant one refusal %q", act, got, want)
	}
}

func TestBlockerSignal_AFreshCycleEndsItsRefusedClaimWithTheLastRoundText(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	run := runBlockerCycle(t, []string{cycle1950Failure}, []string{fake}, protectsSelfcheck, oneCardDecision, laneEnv)
	var got []string
	for _, e := range run.signals {
		if e.Code == CodeBlockerRefused {
			got = append(got, e.Reason)
		}
	}
	last := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it. This was the last correction, so this cycle ends. Next: the item returns to the queue. A quarantined item needs evolve inbox quarantine release <id>."
	if len(got) < 2 || got[len(got)-1] != last || strings.Count(strings.Join(got, "\n"), "This was the last correction") != 1 {
		t.Errorf("refused signals = %q\nwant the last to be %q", got, last)
	}
}

func TestBlockerSignal_ARaisedReentryAtEnforceHoldsTheRefusalOnceInTheDigest(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	standard := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it. Next: correct the BLOCKER line of your report so each claim is true, or fix the failures without declaring a blocker."
	run := runResumedWith(t, resumedSetup{
		failures: []string{cycle1950Failure}, defects: []string{fake}, decision: oneCardDecision, env: laneEnv,
		retry: &policy.RetryConfig{PhaseMaxAttempts: 1, ContractCorrectionRetries: 1}, enforceLadder: true,
	})
	if len(run.directives) < 3 {
		t.Fatalf("directives = %q; want a plain correction and a raised re-entry", run.directives)
	}
	raised := run.directives[2]
	const marker = "The host refused your BLOCKER claim: "
	if strings.Count(raised, marker) != 1 || !strings.HasSuffix(raised, marker+standard) {
		t.Errorf("raised directive = %q\nwant the refusal once, last", raised)
	}
	if plain := run.directives[1]; strings.Count(plain, marker) != 1 || !strings.HasSuffix(plain, marker+standard) {
		t.Errorf("plain directive = %q", plain)
	}
}

func TestBlockerSignal_AFreshRaisedReentryAtEnforceHoldsTheRefusalOnceInTheDigest(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	enforce := func(t *testing.T, root string) []Option {
		writeCLIProfile(t, root, "builder", "agy-tmux", nil)
		return []Option{WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 1, ContractCorrectionRetries: 1}), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce))}
	}
	run := runBlockerCycleWith(t, []string{cycle1950Failure}, []string{fake}, protectsSelfcheck, oneCardDecision, laneEnv, nil, enforce)
	const marker = "The host refused your BLOCKER claim: "
	if len(run.directives) < 3 {
		t.Fatalf("directives = %q; want a plain correction and a raised re-entry", run.directives)
	}
	for i, d := range run.directives[1:] {
		if strings.Count(d, marker) != 1 || !strings.Contains(d, "is not on the protected-surface manifest") {
			t.Errorf("directive %d = %q\nwant the refusal once", i+1, d)
		}
	}
	if !strings.HasPrefix(run.directives[2], "## Retry digest") {
		t.Errorf("directive 2 = %q; want the digest the ladder rendered", run.directives[2])
	}
}

type remediatingFloor struct{ floorRejecting }

func (r remediatingFloor) Review(ctx context.Context, in ReviewInput) ReviewResult {
	out := r.floorRejecting.Review(ctx, in)
	out.Remediation = "fix the named failure"
	return out
}

func TestBlockerSignal_ASalvageRetryDirectiveHoldsTheRefusalOnce(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	retry := func(*testing.T, string) []Option {
		return []Option{WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 1, ContractCorrectionRetries: 2}), WithReviewer(remediatingFloor{floorRejecting{failures: []string{cycle1950Failure}}})}
	}
	run := runBlockerCycleWith(t, []string{cycle1950Failure}, []string{fake}, protectsSelfcheck, oneCardDecision, laneEnv, nil, retry)
	const marker = "The host refused your BLOCKER claim: "
	if len(run.directives) < 3 {
		t.Fatalf("directives = %q; want two corrections", run.directives)
	}
	if !strings.Contains(run.directives[2], contractSalvageRetryDirectiveHeading) {
		t.Fatalf("directive 2 = %q; want the salvage retry directive", run.directives[2])
	}
	for i, d := range run.directives[1:] {
		if strings.Count(d, marker) != 1 {
			t.Errorf("directive %d = %q\nwant the refusal once", i+1, d)
		}
	}
}

func TestBlockerSignal_AProvenBlockerCarriesNoRefusalStatement(t *testing.T) {
	t.Parallel()
	c := BlockerCase{
		Worktree: blockerWorktree(t), Failures: []string{cycle1946StrictFailure}, Defects: []string{cycle1946Claim},
		Explainable: buildFloorExplainable, Protected: protectsSelfcheck,
	}
	proof := NewBlockerProver().Prove(c)
	if !proof.Blocked() || !proof.Statement.empty() {
		t.Errorf("proof = %+v; a proven blocker has no refusal statement", proof)
	}
}

func TestBlockerSignal_AProvenClaimBesideARefusedClaimIsStillNotProven(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	c := BlockerCase{
		Worktree: blockerWorktree(t), Failures: []string{cycle1946PredicateFailure}, Defects: []string{cycle1946Claim, fake},
		Explainable: buildFloorExplainable, Protected: protectsSelfcheck,
	}
	want := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it. Next: " + "correct the BLOCKER line of your report so each claim is true, or fix the failures without declaring a blocker."
	proof := NewBlockerProver().Prove(c)
	if proof.Blocked() || proof.Statement.text() != want {
		t.Errorf("blocked=%v text = %q\nwant %q", proof.Blocked(), proof.Statement.text(), want)
	}
}

func TestBlockerSignal_ARefusedClaimOnThePredicateRerouteIsNotSwallowed(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	run := runBlockerRounds(t, []string{cycle1761Failure}, claimingBuild{rounds: [][]string{nil, {fake}, nil}}, protectsSelfcheck, oneCardDecision, laneEnv, nil)
	got := run.signalReasons(CodeBlockerRefused)
	if len(got) != 1 || !strings.Contains(got[0], "is not on the protected-surface manifest") {
		t.Errorf("refused signals = %q; want the false claim of the round that reroutes to TDD recorded once", got)
	}
}

func (r blockerRun) signalReasons(code signalcenter.Code) []string {
	var out []string
	for _, e := range r.signals {
		if e.Code == code {
			out = append(out, e.Reason)
		}
	}
	return out
}
