package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type scriptedFloorReviewer struct {
	phase  Phase
	rounds [][]string
	calls  int
}

func (s *scriptedFloorReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(cmp.Or(s.phase, PhaseBuild)) {
		return ReviewResult{Approve: true}
	}
	i := s.calls
	s.calls++
	if i >= len(s.rounds) {
		return ReviewResult{Approve: true}
	}
	return ReviewResult{Retry: true, Reason: strings.Join(s.rounds[i], "; "), Failures: s.rounds[i]}
}

func findings(n int, prefix string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s finding %d", prefix, i)
	}
	return out
}

func convergenceSignals(o *Orchestrator) *[]signalcenter.Event {
	center := signalcenter.New()
	events := &[]signalcenter.Event{}
	center.Subscribe(func(e signalcenter.Event) {
		if e.Code == CodeBuildFloorConverging {
			*events = append(*events, e)
		}
	})
	o.signals = center
	return events
}

func TestFloorFindingsConverge(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		prev, cur []string
		want      bool
	}{
		{"fewer", []string{"a", "b", "c"}, []string{"a"}, true},
		{"other", []string{"a"}, []string{"b"}, true},
		{"same", []string{"a", "b"}, []string{"b", "a"}, false},
		{"none now", []string{"a"}, nil, false},
		{"none before", nil, []string{"a"}, false},
		{"one to five", []string{"a"}, []string{"a", "b", "c", "d", "e"}, false},
		{"one to three with a superset", []string{"a"}, []string{"a", "b", "c"}, false},
		{"same count other findings", []string{"a", "b"}, []string{"a", "c"}, true},
		{"only the durations differ", []string{failedTestFinding("TestA", "0.12", "1.203")}, []string{failedTestFinding("TestA", "0.34", "2.5")}, false},
		{"another failed test with the same durations", []string{failedTestFinding("TestA", "0.12", "1.203")}, []string{failedTestFinding("TestB", "0.12", "1.203")}, true},
		{"only the detail differs", []string{"x [[detail: one]]"}, []string{"x [[detail: two]]"}, false},
	} {
		if got := floorFindingsConverge(tc.prev, tc.cur); got != tc.want {
			t.Errorf("%s: floorFindingsConverge(%v, %v) = %v, want %v", tc.name, tc.prev, tc.cur, got, tc.want)
		}
	}
}

func TestFloorConvergence_GrantsWithinTheBoundAndEmitsTheCounts(t *testing.T) {
	t.Parallel()

	retries := policy.Policy{}.RetryConfig()
	retries.FloorConvergenceRounds = 1
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRetryConfig(retries))
	events := convergenceSignals(o)
	fc := o.newFloorConvergence(7, PhaseBuild, findings(5, "a"))

	if fc.grant() {
		t.Fatal("a grant before any later round")
	}
	fc.observe(findings(1, "a"))
	if !fc.grant() {
		t.Fatal("5 findings then 1 must earn a further correction")
	}
	fc.observe(findings(1, "b"))
	if fc.grant() {
		t.Fatal("the bound of 1 round was already used")
	}
	if len(*events) != 1 {
		t.Fatalf("signals = %+v, want one %s", *events, CodeBuildFloorConverging)
	}
	f := (*events)[0].Fields
	if f["before"] != "5" || f["after"] != "1" || f["round"] != "1" || f["bound"] != "1" || (*events)[0].Cycle != 7 {
		t.Fatalf("signal fields = %+v, want before=5 after=1 round=1 bound=1 on cycle 7", f)
	}
}

func TestFloorConvergence_SameFindingsAndOtherPhasesGetNoRound(t *testing.T) {
	t.Parallel()

	retries := policy.Policy{}.RetryConfig()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRetryConfig(retries))
	same := o.newFloorConvergence(1, PhaseBuild, findings(2, "a"))
	same.observe(findings(2, "a"))
	if same.grant() {
		t.Fatal("the same findings twice must not earn a round")
	}
	audit := o.newFloorConvergence(1, PhaseAudit, findings(3, "a"))
	audit.observe(findings(1, "a"))
	if audit.grant() {
		t.Fatal("only the build floor earns a round")
	}
}

func resumedConvergenceHarness(t *testing.T, phase Phase, rounds [][]string) (*Orchestrator, *fakeRunner, *scriptedFloorReviewer, string) {
	t.Helper()
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &fakeStorage{
		state:      State{LastCycleNumber: 9},
		cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: t.TempDir()},
	}
	builder := &fakeRunner{name: string(phase)}
	runners := buildRunners(nil)
	runners[phase] = builder
	reviewer := &scriptedFloorReviewer{phase: phase, rounds: rounds}
	retries := policy.Policy{}.RetryConfig()
	retries.ContractCorrectionRetries = 1
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}),
		WithReviewer(reviewer), WithRetryConfig(retries))
	return o, builder, reviewer, projectRoot
}

func TestRunCycleFromPhase_ResumedBuildGetsAFurtherCorrectionWhileTheFloorConverges(t *testing.T) {
	t.Parallel()

	o, builder, reviewer, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(5, "a"), findings(1, "a")})
	events := convergenceSignals(o)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9})

	if err != nil {
		t.Fatalf("a resumed build that goes from 5 findings to 1 must pass the floor: %v", err)
	}
	if builder.calls != 3 || reviewer.calls != 3 {
		t.Fatalf("builder/review calls = %d/%d, want the dispatch, 1 correction and the further one, with 3 reviews", builder.calls, reviewer.calls)
	}
	if len(*events) != 1 || (*events)[0].Fields["before"] != "5" || (*events)[0].Fields["after"] != "1" {
		t.Fatalf("signals = %+v, want one %s with before=5 after=1", *events, CodeBuildFloorConverging)
	}
}

func TestRunCycleFromPhase_ResumedBuildWithTheSameFindingTwiceEndsAndIsLearnedFrom(t *testing.T) {
	t.Parallel()

	o, builder, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(1, "a"), findings(1, "a"), findings(1, "a")})
	events := convergenceSignals(o)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9})

	if err == nil || !strings.Contains(err.Error(), "rejected after 1 correction") {
		t.Fatalf("err = %v, want the rejection after 1 correction", err)
	}
	if builder.calls != 2 || len(*events) != 0 {
		t.Fatalf("builder calls = %d, signals = %d, want 2 (the dispatch and 1 correction) and 0: the same finding twice earns no round", builder.calls, len(*events))
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
	timing, terr := os.ReadFile(phasetiming.Path(ws))
	if terr != nil || !strings.Contains(string(timing), cyclestate.DiagCodeBuildRejected) {
		t.Fatalf("the resumed build rejection must carry %s (%v): %s", cyclestate.DiagCodeBuildRejected, terr, timing)
	}
	digest, rerr := os.ReadFile(filepath.Join(ws, "failure-digest.json"))
	if rerr != nil || strings.Contains(string(digest), "unknown") {
		t.Fatalf("failure digest (%v) must exist and name a class: %s", rerr, digest)
	}
}

func inCycleConvergenceHarness(t *testing.T, rounds [][]string) (exhaustion, *scriptedFloorReviewer) {
	t.Helper()
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")
	reviewer := &scriptedFloorReviewer{rounds: rounds}
	h.cr.o.reviewer = reviewer
	h.cr.o.retryConfig.FloorConvergenceRounds = 1
	return h, reviewer
}

func TestReviewWithCorrections_ABuildFloorThatConvergesGetsAFurtherCorrection(t *testing.T) {
	t.Parallel()
	h, reviewer := inCycleConvergenceHarness(t, [][]string{findings(5, "a"), findings(4, "a"), findings(1, "a")})
	events := convergenceSignals(h.cr.o)

	act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if act != loopNext || err != nil {
		t.Fatalf("reviewWithCorrections = (%v, %v), want the floor to pass after the further correction", act, err)
	}
	if h.runner.calls != 3 || reviewer.calls != 4 || len(*events) != 1 {
		t.Fatalf("corrections %d, reviews %d, signals %d; want 3, 4 and 1", h.runner.calls, reviewer.calls, len(*events))
	}
}

func TestReviewWithCorrections_ABuildFloorThatRepeatsItsFindingsEndsAsBefore(t *testing.T) {
	t.Parallel()
	h, _ := inCycleConvergenceHarness(t, [][]string{findings(2, "a"), findings(2, "a"), findings(2, "a"), findings(2, "a")})
	events := convergenceSignals(h.cr.o)

	act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if act != loopAbort || err == nil || h.runner.calls != 2 || len(*events) != 0 {
		t.Fatalf("(%v, %v) after %d corrections, %d signals; want an abort after 2 corrections and no signal", act, err, h.runner.calls, len(*events))
	}
}

func failedTestFinding(test, testTime, pkgTime string) string {
	output := fmt.Sprintf("--- FAIL: %s (%ss)\n    x_test.go:12: got 1 want 2\nFAIL\nFAIL\tgithub.com/x/y\t%ss\n", test, testTime, pkgTime)
	return floorLinesFor([]selfCheckFailure{{Pkg: "./y", Output: output}}, nil, nil)[0]
}

func longFailedTestFinding(test, pkgTime string) string {
	noise := strings.Repeat("    x_test.go:34: open /tmp/work/fixture: no such file or directory\n", 8)
	output := fmt.Sprintf("--- FAIL: %s (0.01s)\n%sFAIL\nFAIL\tgithub.com/x/y\t%ss\n", test, noise, pkgTime)
	return floorLinesFor([]selfCheckFailure{{Pkg: "./y", Output: output}}, nil, nil)[0]
}

func longGoTestRounds(test string, pkgTimes ...string) [][]string {
	rounds := make([][]string, len(pkgTimes))
	for i, d := range pkgTimes {
		rounds[i] = []string{longFailedTestFinding(test, d)}
	}
	return rounds
}

func TestFloorFindingsConverge_AFailedTestPastTheDiagnosticWindowKeepsOneIdentity(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		prev, cur string
		want      bool
	}{
		{"longer package duration", longFailedTestFinding("TestA", "1.203"), longFailedTestFinding("TestA", "112.345"), false},
		{"temp dir names", "./y: unit tests FAIL\n--- FAIL: TestA (0.1s)\n    a_test.go:3: /t/TestA1234567/001", "./y: unit tests FAIL\n--- FAIL: TestA (0.1s)\n    a_test.go:3: /t/TestA7654321/001", false},
		{"addresses", "./y: unit tests FAIL\n--- FAIL: TestA (0.1s)\n    a_test.go:3: ptr 0xc000123000", "./y: unit tests FAIL\n--- FAIL: TestA (0.1s)\n    a_test.go:3: ptr 0xc000999000", false},
		{"another test", longFailedTestFinding("TestA", "1.2"), longFailedTestFinding("TestB", "1.2"), true},
		{"another package", "./y: unit tests FAIL\n--- FAIL: TestA (0.1s)", "./z: unit tests FAIL\n--- FAIL: TestA (0.1s)", true},
		{"a subtest is its top test", "./y: unit tests FAIL\n--- FAIL: TestA/one (0.1s)", "./y: unit tests FAIL\n--- FAIL: TestA/two (0.1s)", false},
	} {
		if got := floorFindingsConverge([]string{tc.prev}, []string{tc.cur}); got != tc.want {
			t.Errorf("%s: converge = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestReviewWithCorrections_ABuildFloorThatRepeatsALongFailureEndsAsBefore(t *testing.T) {
	t.Parallel()
	h, _ := inCycleConvergenceHarness(t, longGoTestRounds("TestA", "1.203", "112.345", "9.1", "10.22"))
	events := convergenceSignals(h.cr.o)

	act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if act != loopAbort || err == nil || h.runner.calls != 2 || len(*events) != 0 {
		t.Fatalf("(%v, %v) after %d corrections, %d signals; want an abort after 2 corrections and no signal", act, err, h.runner.calls, len(*events))
	}
}

func TestRunCycleFromPhase_ResumedBuildThatRepeatsALongFailureEarnsNoRound(t *testing.T) {
	t.Parallel()

	o, builder, _, root := resumedConvergenceHarness(t, PhaseBuild, longGoTestRounds("TestA", "1.203", "112.345", "9.1"))
	events := convergenceSignals(o)

	err := resumeAt(o, root, PhaseBuild)

	if err == nil || builder.calls != 2 || len(*events) != 0 {
		t.Fatalf("err = %v, builder calls = %d, signals = %d; want the rejection after 2 builder calls and no signal", err, builder.calls, len(*events))
	}
}

func TestFloorFindingsConverge_TheNormalizerKeepsDifferentFindingsApart(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{
		{failedTestFinding("TestA", "0.12", "1.2"), failedTestFinding("TestB", "0.12", "1.2")},
		{"./y: unit tests FAIL", "./z: unit tests FAIL"},
		{"naming: Foo lacks a test [[detail: one]]", "naming: Bar lacks a test [[detail: one]]"},
	}
	for _, pair := range pairs {
		if normalizeReasonForFingerprint(pair[0]) == normalizeReasonForFingerprint(pair[1]) {
			t.Errorf("two different findings read as one:\n%q\n%q", pair[0], pair[1])
		}
		if !floorFindingsConverge([]string{pair[0]}, []string{pair[1]}) {
			t.Errorf("one finding replaced by another must converge:\n%q\n%q", pair[0], pair[1])
		}
	}
}

func goTestRounds(tests ...string) [][]string {
	rounds := make([][]string, len(tests))
	for i, test := range tests {
		rounds[i] = []string{failedTestFinding(test, fmt.Sprintf("0.%02d", i+10), fmt.Sprintf("1.%d", 200+i*37))}
	}
	return rounds
}

func floorOrder(o *Orchestrator) *[]string {
	order := &[]string{}
	center := signalcenter.New()
	center.Subscribe(func(e signalcenter.Event) {
		switch e.Code {
		case CodeLadderDecision:
			*order = append(*order, "ladder")
		case CodeBuildFloorConverging:
			*order = append(*order, "converging")
		}
	})
	o.signals = center
	return order
}

func resumeAt(o *Orchestrator, root string, phase Phase) error {
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}, &ResumePoint{Phase: string(phase), CycleID: 9})
	return err
}

func TestRunCycleFromPhase_ResumedBuildWhoseFailedTestOnlyChangesDurationsEarnsNoRound(t *testing.T) {
	t.Parallel()

	o, builder, _, root := resumedConvergenceHarness(t, PhaseBuild, goTestRounds("TestA", "TestA", "TestA"))
	events := convergenceSignals(o)

	err := resumeAt(o, root, PhaseBuild)

	if err == nil || builder.calls != 2 || len(*events) != 0 {
		t.Fatalf("err = %v, builder calls = %d, signals = %d; want the rejection after 2 builder calls and no signal", err, builder.calls, len(*events))
	}
}

func TestRunCycleFromPhase_ResumedBuildThatGrowsFromOneFindingToFiveEarnsNoRound(t *testing.T) {
	t.Parallel()

	o, builder, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(1, "a"), findings(5, "a"), findings(1, "a")})
	events := convergenceSignals(o)

	err := resumeAt(o, root, PhaseBuild)

	if err == nil || builder.calls != 2 || len(*events) != 0 {
		t.Fatalf("err = %v, builder calls = %d, signals = %d; want the rejection after 2 builder calls and no signal", err, builder.calls, len(*events))
	}
}

func TestReviewWithCorrections_ABuildFloorWhoseFailedTestOnlyChangesDurationsEndsAsBefore(t *testing.T) {
	t.Parallel()
	h, _ := inCycleConvergenceHarness(t, goTestRounds("TestA", "TestA", "TestA", "TestA"))
	events := convergenceSignals(h.cr.o)

	act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if act != loopAbort || err == nil || h.runner.calls != 2 || len(*events) != 0 {
		t.Fatalf("(%v, %v) after %d corrections, %d signals; want an abort after 2 corrections and no signal", act, err, h.runner.calls, len(*events))
	}
}

func TestReviewWithCorrections_ABuildFloorThatGrowsEarnsNoFurtherCorrection(t *testing.T) {
	t.Parallel()
	h, _ := inCycleConvergenceHarness(t, [][]string{findings(1, "a"), findings(5, "a"), findings(6, "a"), findings(1, "a")})
	events := convergenceSignals(h.cr.o)

	act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if act != loopAbort || err == nil || h.runner.calls != 2 || len(*events) != 0 {
		t.Fatalf("(%v, %v) after %d corrections, %d signals; want an abort after 2 corrections and no signal", act, err, h.runner.calls, len(*events))
	}
}

func TestReviewWithCorrections_ConvergingInsideTheBudgetDoesNotSpendTheRound(t *testing.T) {
	t.Parallel()
	h, _ := inCycleConvergenceHarness(t, [][]string{findings(5, "a"), findings(4, "a"), findings(4, "a"), findings(4, "a")})
	events := convergenceSignals(h.cr.o)

	act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if act != loopAbort || err == nil || h.runner.calls != 2 || len(*events) != 0 {
		t.Fatalf("(%v, %v) after %d corrections, %d signals; want an abort after 2 corrections and no signal", act, err, h.runner.calls, len(*events))
	}
}

func TestReviewWithCorrections_ARejectionAfterTheFurtherCorrectionCountsIt(t *testing.T) {
	t.Parallel()
	h, _ := inCycleConvergenceHarness(t, [][]string{findings(5, "a"), findings(4, "a"), findings(1, "a"), findings(1, "a")})

	_, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	if err == nil || !strings.Contains(err.Error(), "after 3 correction(s)") {
		t.Fatalf("err = %v, want the rejection after 3 corrections", err)
	}
}

func TestRunCycleFromPhase_AResumedRejectionAfterTheFurtherCorrectionCountsIt(t *testing.T) {
	t.Parallel()

	o, _, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(5, "a"), findings(1, "a"), findings(1, "a")})

	err := resumeAt(o, root, PhaseBuild)

	if err == nil || !strings.Contains(err.Error(), "rejected after 2 correction(s)") {
		t.Fatalf("err = %v, want the rejection after 2 corrections", err)
	}
}

func TestRunCycleFromPhase_AResumedRejectionIsACycleLevelFailure(t *testing.T) {
	t.Parallel()

	o, _, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(1, "a"), findings(1, "a")})

	err := resumeAt(o, root, PhaseBuild)

	var cycleLevel *ErrCycleLevelFailure
	if !errors.As(err, &cycleLevel) || cycleLevel.Phase != string(PhaseBuild) {
		t.Fatalf("err = %v, want a cycle-level failure of the build phase", err)
	}
}

func TestRunCycleFromPhase_AResumedAuditRejectionIsLearnedFrom(t *testing.T) {
	t.Parallel()

	o, _, _, root := resumedConvergenceHarness(t, PhaseAudit, [][]string{findings(1, "a"), findings(1, "a")})

	err := resumeAt(o, root, PhaseAudit)

	var cycleLevel *ErrCycleLevelFailure
	if !errors.As(err, &cycleLevel) || cycleLevel.Phase != string(PhaseAudit) {
		t.Fatalf("err = %v, want a cycle-level failure of the audit phase", err)
	}
	if _, rerr := os.Stat(filepath.Join(root, ".evolve", "runs", "cycle-9", "failure-digest.json")); rerr != nil {
		t.Fatalf("a resumed rejection of any phase must write the failure digest: %v", rerr)
	}
}

func TestRunCycleFromPhase_AResumedFurtherCorrectionReportsABudgetThatHoldsIt(t *testing.T) {
	t.Parallel()

	o, _, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(5, "a"), findings(1, "a")})
	var corrections []signalcenter.Event
	center := signalcenter.New()
	center.Subscribe(func(e signalcenter.Event) {
		if e.Code == CodeGateCorrection {
			corrections = append(corrections, e)
		}
	})
	o.signals = center

	if err := resumeAt(o, root, PhaseBuild); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if len(corrections) != 2 || corrections[1].Fields["correction"] != "2" || corrections[1].Fields["max"] != "2" {
		t.Fatalf("gate corrections = %+v, want the second with correction=2 and max=2", corrections)
	}
}

func TestFloorGrant_BothLoopsAskTheLadderBeforeTheyGrantARound(t *testing.T) {
	t.Parallel()
	want := []string{"ladder", "converging"}

	h, _ := inCycleConvergenceHarness(t, [][]string{findings(5, "a"), findings(4, "a")})
	h.cr.retryConfig.ContractCorrectionRetries = 1
	h.cr.o.retryConfig.ContractCorrectionRetries = 1
	fresh := floorOrder(h.cr.o)
	if act, err := h.cr.reviewWithCorrections(PhaseBuild, h.dr); act != loopNext || err != nil {
		t.Fatalf("fresh loop = (%v, %v)", act, err)
	}
	if !slices.Equal(*fresh, want) {
		t.Errorf("fresh loop order = %v, want %v", *fresh, want)
	}

	o, _, _, root := resumedConvergenceHarness(t, PhaseBuild, [][]string{findings(5, "a"), findings(4, "a")})
	resumed := floorOrder(o)
	if err := resumeAt(o, root, PhaseBuild); err != nil {
		t.Fatalf("resumed loop: %v", err)
	}
	if !slices.Equal(*resumed, want) {
		t.Errorf("resumed loop order = %v, want %v", *resumed, want)
	}
}
