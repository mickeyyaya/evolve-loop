package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const ratchetRedName = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"

const ratchetFailureText = "internal/cyclesimulator/characterization_test.go builds a raw git repo (1 init sites) outside internal/gittest"

type fakeRepoContractPack struct {
	reds       []string
	diagnostic string
	err        error
	dirs       []string
	projects   []string
	changed    [][]string
}

func (f *fakeRepoContractPack) run(_ context.Context, projectRoot, root string, changed []string) ([]string, string, error) {
	f.changed = append(f.changed, changed)
	f.dirs = append(f.dirs, root)
	f.projects = append(f.projects, projectRoot)
	return f.reds, f.diagnostic, f.err
}

func ratchetRedPack() *fakeRepoContractPack {
	return &fakeRepoContractPack{
		reds:       []string{ratchetRedName},
		diagnostic: "=== RUN   TestRatchet_NoNewRawGitFixtures\n    rawgitratchet_test.go:49: " + ratchetFailureText + "\n--- FAIL: TestRatchet_NoNewRawGitFixtures (0.27s)\n",
		err:        errors.New("exit status 1"),
	}
}

func worktreeWithModule(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "go", "go.mod"), []byte("module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestRepoContractFloorChecks_ARawGitFixtureIsCorrectedAtBuildExitByTheRatchetsName(t *testing.T) {
	t.Parallel()

	wt := worktreeWithModule(t)
	pack := ratchetRedPack()
	var run RepoContractPackFn = pack.run
	res := BuildHandoffFloor{{Name: "repo-contract", Run: RepoContractFloorChecks(run)}}.Review(context.Background(),
		ReviewInput{Phase: string(PhaseBuild), Worktree: wt, ProjectRoot: t.TempDir()})
	if res.Approve || !res.Retry {
		t.Fatalf("a red repo-contract pack goes back to the build correction; got Approve=%v Retry=%v", res.Approve, res.Retry)
	}
	if !strings.Contains(res.Reason, "TestRatchet_NoNewRawGitFixtures") || !strings.Contains(res.Reason, "repo-contract scanner pack") || !strings.Contains(res.Reason, "go test -count=1 <package>") {
		t.Fatalf("the correction names the pack and the ratchet's failing test; got %q", res.Reason)
	}
	if !strings.Contains(res.Reason, ratchetFailureText) {
		t.Fatalf("the correction carries the ratchet's own message naming the offending file; got %q", res.Reason)
	}
	if len(pack.dirs) != 1 || pack.dirs[0] != wt {
		t.Fatalf("the pack runs once, handed the build worktree so ship picks its module exactly as the gate does; ran in %v", pack.dirs)
	}
}

func TestRepoContractFloorChecks_HandsThePackTheProjectRootOfTheSharedVerdictStore(t *testing.T) {
	t.Parallel()

	wt, project := worktreeWithModule(t), t.TempDir()
	pack := &fakeRepoContractPack{}
	RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: wt, ProjectRoot: project})
	if len(pack.projects) != 1 || pack.projects[0] != project || pack.dirs[0] != wt {
		t.Fatalf("the floor writes the store of the project that the ship reads, and tests the worktree; got projects %v dirs %v", pack.projects, pack.dirs)
	}
}

func TestRepoContractFloorChecks_GreenOrAnUnnamedExitPassesTheHandoff(t *testing.T) {
	t.Parallel()

	wt := worktreeWithModule(t)
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: wt}
	if got := RepoContractFloorChecks((&fakeRepoContractPack{}).run)(context.Background(), in); len(got) != 0 {
		t.Fatalf("a green pack passes the handoff; got %v", got)
	}
	unnamed := &fakeRepoContractPack{diagnostic: "go: toolchain chatter before the kill\n", err: errors.New("signal: killed")}
	var got []string
	var sink injectedStderr
	in.Stderr = &sink
	got = RepoContractFloorChecks(unnamed.run)(context.Background(), in)
	stderr := sink.String()
	if len(got) != 0 {
		t.Fatalf("an exit that names no test passes the handoff, since ship's gate classes it; got %v", got)
	}
	if !strings.Contains(stderr, "WARN repo-contract scanner pack exited nonzero naming no test") || !strings.Contains(stderr, "signal: killed") || !strings.Contains(stderr, "go: toolchain chatter before the kill") {
		t.Fatalf("the unnamed exit WARNs with its error and the pack's own output; stderr=%q", stderr)
	}
	pack := ratchetRedPack()
	if got := RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild)}); len(got) != 0 || len(pack.dirs) != 0 {
		t.Fatalf("no worktree means nothing to run; got %v, ran in %v", got, pack.dirs)
	}
}

func TestRepoContractFloorChecks_AnyNamedRedCorrectsTheBuild(t *testing.T) {
	t.Parallel()

	pack := &fakeRepoContractPack{reds: []string{"pkg.TestX"}}
	got := RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)})
	if len(got) != 1 || !strings.Contains(got[0], "pkg.TestX") {
		t.Fatalf("ship names reds only for its real red, so the floor branches on the names alone; got %v", got)
	}
}

func TestRepoContractFloorChecks_RunsThePackUnderItsOwnDeadline(t *testing.T) {
	t.Parallel()

	var remaining time.Duration
	var hasDeadline bool
	run := func(ctx context.Context, _, _ string, _ []string) ([]string, string, error) {
		var deadline time.Time
		deadline, hasDeadline = ctx.Deadline()
		remaining = time.Until(deadline)
		return nil, "", nil
	}
	RepoContractFloorChecks(run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)})
	if repoContractFloorDeadline != 120*time.Second {
		t.Fatalf("the floor's deadline follows the floor's go test convention (120s), not ship's 20m; got %s", repoContractFloorDeadline)
	}
	if !hasDeadline || remaining <= 0 || remaining > repoContractFloorDeadline {
		t.Fatalf("the pack runs under the floor's own deadline even when the caller set none; deadline=%v remaining=%s", hasDeadline, remaining)
	}
}

func TestRepoContractFloorChecks_ADeadlineKillWarnsAndPassesTheHandoff(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	blocking := func(ctx context.Context, _, _ string, _ []string) ([]string, string, error) {
		<-ctx.Done()
		return nil, "partial output before the kill\n", ctx.Err()
	}
	var got []string
	var sink injectedStderr
	got = RepoContractFloorChecks(blocking)(ctx, ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t), Stderr: &sink})
	stderr := sink.String()
	if len(got) != 0 {
		t.Fatalf("a kill names nothing, so the floor passes the handoff and ship stays the authority; got %v", got)
	}
	for _, want := range []string{"WARN repo-contract scanner pack", "context deadline exceeded", repoContractFloorDeadline.String(), "partial output before the kill"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the deadline WARN names %q; stderr=%q", want, stderr)
		}
	}
}

func TestRepoContractFloorChecks_FollowsTheShipGatesDial(t *testing.T) {
	t.Parallel()

	wt := worktreeWithModule(t)
	for word, wantRun := range map[string]bool{"off": false, "enforce": true, "": true, "shadwo": true, "{not json": true} {
		root := t.TempDir()
		if word != "" {
			writePolicyGate(t, root, word)
		}
		pack := ratchetRedPack()
		got := RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: wt, ProjectRoot: root})
		if ran := len(pack.dirs) == 1; ran != wantRun || (len(got) == 1) != wantRun {
			t.Errorf("gates.repo_contract_gate %q: ran=%v findings=%v, want run=%v", word, ran, got, wantRun)
		}
	}
}

func TestRepoContractFloorChecks_WithoutAProjectRootTheWorktreesPolicyDecides(t *testing.T) {
	t.Parallel()

	wt := t.TempDir()
	writePolicyGate(t, wt, "off")
	pack := ratchetRedPack()
	if got := RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: wt}); len(got) != 0 || len(pack.dirs) != 0 {
		t.Fatalf("selfcheck passes no project root, so the worktree's gates.repo_contract_gate off must still skip the pack; got %v, ran in %v", got, pack.dirs)
	}
}

func writePolicyGate(t *testing.T, root, word string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"gates":{"repo_contract_gate":"` + word + `"}}`
	if strings.HasPrefix(word, "{") {
		body = word
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRepoContractFloor_ARedPackCorrectsTheBuildAndNeverReachesTheAudit(t *testing.T) {
	t.Parallel()

	runners := buildRunners(nil)
	build := runners[PhaseBuild].(*fakeRunner)
	audit := runners[PhaseAudit].(*fakeRunner)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: worktreeWithModule(t)}),
		WithReviewer(BuildHandoffFloor{{Name: "repo-contract", Run: RepoContractFloorChecks(ratchetRedPack().run)}}))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err == nil || !strings.Contains(err.Error(), "TestRatchet_NoNewRawGitFixtures") {
		t.Fatalf("a pack that stays red fails the build naming the ratchet's test; got %v", err)
	}
	if audit.calls != 0 {
		t.Fatalf("a red repo-contract pack never reaches the audit; audit ran %d time(s)", audit.calls)
	}
	if build.calls < 2 || !strings.Contains(build.requests[1].CorrectionDirective, "TestRatchet_NoNewRawGitFixtures") {
		t.Fatalf("the build is re-dispatched with the ratchet's test named; builds=%d", build.calls)
	}
}

func TestUnnamedPackExit_NamesThePackWallTimeAndTheFloorDeadline(t *testing.T) {
	t.Parallel()

	err := BuildFloorStage().unnamedPackExit(errors.New("signal: killed"), "chatter\n", 95400*time.Millisecond)
	if err == nil {
		t.Fatal("an exit that names no test is a WARN, never nil")
	}
	for _, want := range []string{"signal: killed", "the pack ran 1m35.4s", "the floor's deadline is 2m0s", "chatter"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the WARN names %q; got %q", want, err)
		}
	}
	if got := BuildFloorStage().unnamedPackExit(nil, "", time.Second); got != nil {
		t.Fatalf("a clean exit has no WARN; got %v", got)
	}
}

func TestRepoContractFloorChecks_AnUnnamedExitWarnsWithTheWallTimeOfItsRun(t *testing.T) {
	t.Parallel()

	unnamed := &fakeRepoContractPack{err: errors.New("signal: killed")}
	var sink injectedStderr
	RepoContractFloorChecks(unnamed.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t), Stderr: &sink})
	if !strings.Contains(sink.String(), "the pack ran ") {
		t.Fatalf("the judge measures the pack run and puts it in the WARN; stderr=%q", sink.String())
	}
}

func selectionHolds(selections []repocontract.TestSelection, pkg, test string) bool {
	return slices.ContainsFunc(selections, func(selection repocontract.TestSelection) bool {
		return selection.Package == pkg && slices.Contains(selection.Tests, test)
	})
}

func TestRepoContractFloorJudge_AWorkflowOnlyWorktreeRunsTheTestsThatReadIt(t *testing.T) {
	t.Parallel()

	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, ".github", "workflows", "required.yml"), "name: required\n")
	pack := &fakeRepoContractPack{}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: repo.Dir, WorktreeBaseSHA: base}
	if _, err := RepoContractFloorJudge(pack.run)(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(pack.changed) != 1 || !slices.Equal(pack.changed[0], []string{".github/workflows/required.yml"}) {
		t.Fatalf("the judge hands the pack the changed paths of the worktree; got %q", pack.changed)
	}
	if !selectionHolds(repocontract.ByNameTestsFor(pack.changed[0]), "./internal/ciparity", "TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies") {
		t.Fatal("the selection of a workflow-only change holds the workflow test of ciparity")
	}
}

func TestRepoContractFloorJudge_AGoOnlyWorktreeHandsThePackOnlyGoPaths(t *testing.T) {
	t.Parallel()

	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "x.go"), "package x\n")
	pack := &fakeRepoContractPack{}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: repo.Dir, WorktreeBaseSHA: base}
	if _, err := RepoContractFloorJudge(pack.run)(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(pack.changed) != 1 || !slices.Equal(pack.changed[0], []string{"go/x.go"}) {
		t.Fatalf("the judge hands the pack the Go path; got %q", pack.changed)
	}
}

func TestRepoContractFloorJudge_AnUnreadableChangeRunsTheCoarseSet(t *testing.T) {
	t.Parallel()

	pack := &fakeRepoContractPack{}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t), ProjectRoot: t.TempDir()}
	if _, err := RepoContractFloorJudge(pack.run)(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(pack.changed) != 1 {
		t.Fatalf("an unreadable change still runs the pack once; got %d runs", len(pack.changed))
	}
	if !reflect.DeepEqual(repocontract.ByNameTestsFor(pack.changed[0]), repocontract.ByNameTests()) {
		t.Fatalf("paths %q must select the coarse set of ByNameTests()", pack.changed[0])
	}
}

func TestRepoContractStage_EachCallerGetsItsOwnWords(t *testing.T) {
	t.Parallel()

	in := func(t *testing.T) ReviewInput {
		return ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)}
	}
	floorLines, _ := RepoContractFloorJudge(ratchetRedPack().run)(context.Background(), in(t))
	gateLines, _ := RepoContractJudge(CommitGateStage(), ratchetRedPack().run)(context.Background(), in(t))
	if len(floorLines) != 1 || len(gateLines) != 1 {
		t.Fatalf("each caller refuses a named red once; floor %v gate %v", floorLines, gateLines)
	}
	for _, want := range []string{"ship runs these repo-wide suites before it pushes and refuses a red one", "fix the change before handoff"} {
		if !strings.Contains(floorLines[0], want) {
			t.Errorf("the floor keeps its text %q; got %q", want, floorLines[0])
		}
	}
	for _, want := range []string{"the commit gate refuses a commit while one is red", "fix the change before the commit"} {
		if !strings.Contains(gateLines[0], want) {
			t.Errorf("the commit gate names itself with %q; got %q", want, gateLines[0])
		}
	}
	for _, banned := range []string{"ship", "floor", "handoff"} {
		if strings.Contains(gateLines[0], banned) {
			t.Errorf("the commit gate's refusal must not claim %q; got %q", banned, gateLines[0])
		}
	}
}

func TestRepoContractStage_ACallerDefinedStageSpeaksInItsOwnFieldsAndNoOtherCallersWords(t *testing.T) {
	t.Parallel()

	custom := RepoContractStage{
		Name:      "probe-stage",
		Subject:   "probe",
		Refusal:   "the probe refuses a red suite",
		FixBefore: "the probe retries",
		Rerun:     "re-run the probe",
	}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)}
	lines, err := RepoContractJudge(custom, ratchetRedPack().run)(context.Background(), in)
	if err != nil {
		t.Fatalf("a named red is a refusal line, not a fault; got %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("a named red refuses once; got %v", lines)
	}
	for _, want := range []string{custom.Refusal, "fix the change before " + custom.FixBefore} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("the refusal speaks with the stage's %q; got %q", want, lines[0])
		}
	}
	for _, other := range []RepoContractStage{BuildFloorStage(), CommitGateStage()} {
		if strings.Contains(lines[0], other.Refusal) {
			t.Errorf("the refusal must not borrow %q from another stage; got %q", other.Refusal, lines[0])
		}
	}
}

func TestRepoContractStage_AnUnnamedExitCarriesTheCallersStage(t *testing.T) {
	t.Parallel()

	unnamed := &fakeRepoContractPack{err: errors.New("exit status 2")}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)}
	_, floorErr := RepoContractFloorJudge(unnamed.run)(context.Background(), in)
	_, gateErr := RepoContractJudge(CommitGateStage(), unnamed.run)(context.Background(), in)
	var floorExit, gateExit *PackExitError
	if !errors.As(floorErr, &floorExit) || floorExit.Stage != BuildFloorStage() {
		t.Fatalf("the floor's fault carries the floor's stage; got %v", floorErr)
	}
	if !errors.As(gateErr, &gateExit) || gateExit.Stage != CommitGateStage() {
		t.Fatalf("the commit gate's fault carries the commit gate's stage; got %v", gateErr)
	}
	if !errors.Is(gateErr, unnamed.err) {
		t.Errorf("the fault wraps the pack's own error; got %v", gateErr)
	}
	if strings.Contains(gateExit.Stage.Rerun, "ship") || strings.Contains(gateExit.Stage.Subject, "floor") {
		t.Errorf("the commit gate's words must not claim a ship gate or a floor; got %+v", gateExit.Stage)
	}
	if !strings.Contains(gateExit.Stage.Rerun, "re-run the commit gate") {
		t.Errorf("the commit gate says to re-run the commit gate; got %q", gateExit.Stage.Rerun)
	}
}

func TestRepoContractJudge_APackThatOutlivesTheDeadlineIsADeadlineFaultAndNotATestFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	blocking := func(ctx context.Context, _, _ string, _ []string) ([]string, string, error) {
		<-ctx.Done()
		return nil, "partial output before the kill\n", ctx.Err()
	}
	lines, err := RepoContractJudge(CommitGateStage(), blocking)(ctx, ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)})
	var deadline *PackDeadlineError
	var exit *PackExitError
	if len(lines) != 0 || !errors.As(err, &deadline) || errors.As(err, &exit) {
		t.Fatalf("a timeout fails closed as a deadline fault and names no test; lines %v err %v", lines, err)
	}
	if deadline.Stage != CommitGateStage() || deadline.Elapsed <= 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("the fault carries the stage, the elapsed time and the cause; got %+v", deadline)
	}
	if !strings.Contains(deadline.Diagnostic, "partial output before the kill") {
		t.Errorf("the fault keeps the partial output; got %q", deadline.Diagnostic)
	}
}

func TestRepoContractJudge_ARedThatTheDeadlineKillLeftNamedStillRefusesAsARed(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	killedAfterARed := func(ctx context.Context, _, _ string, _ []string) ([]string, string, error) {
		<-ctx.Done()
		return []string{ratchetRedName}, "", ctx.Err()
	}
	for name, judge := range map[string]FloorJudge{
		"floor":       RepoContractFloorJudge(killedAfterARed),
		"commit gate": RepoContractJudge(CommitGateStage(), killedAfterARed),
	} {
		lines, err := judge(ctx, ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)})
		if err != nil || len(lines) != 1 || !strings.Contains(lines[0], ratchetRedName) {
			t.Errorf("%s: a proven red is a refusal line even when the deadline killed the rest; lines %v err %v", name, lines, err)
		}
	}
}

func TestRepoContractJudge_TheStageNamesTheStderrNoteAndTheNoteReachesTheCallersWriter(t *testing.T) {
	t.Parallel()

	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "go", "go.mod"), []byte("module e2e.local/fixture\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink injectedStderr
	if _, err := RepoContractJudge(CommitGateStage(), (&fakeRepoContractPack{}).run)(context.Background(), ReviewInput{Worktree: wt, Stderr: &sink}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sink.String(), "[commit-gate] repo-contract gate: ") {
		t.Fatalf("the note goes to the caller's writer under the caller's stage; got %q", sink.String())
	}
}

func TestRepoContractJudge_ThePolicyWarnAndTheUnreadableChangeNoteSpeakAsTheCallersStageToTheCallersWriter(t *testing.T) {
	t.Parallel()

	wt := worktreeWithModule(t)
	if err := os.MkdirAll(filepath.Join(wt, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".evolve", "policy.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink injectedStderr
	if _, err := RepoContractJudge(CommitGateStage(), (&fakeRepoContractPack{}).run)(context.Background(), ReviewInput{Worktree: wt, Stderr: &sink}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[commit-gate] WARN repo-contract gate could not read the policy", "[commit-gate] repo-contract gate: "} {
		if !strings.Contains(sink.String(), want) {
			t.Errorf("the caller's writer holds %q; got %q", want, sink.String())
		}
	}
	if strings.Contains(sink.String(), "build-floor") {
		t.Errorf("the commit gate does not speak as the build floor; got %q", sink.String())
	}
}
