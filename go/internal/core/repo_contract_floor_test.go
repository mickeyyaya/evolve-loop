package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ratchetRedName = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"

const ratchetFailureText = "internal/cyclesimulator/characterization_test.go builds a raw git repo (1 init sites) outside internal/gittest"

type fakeRepoContractPack struct {
	reds       []string
	diagnostic string
	err        error
	dirs       []string
}

func (f *fakeRepoContractPack) run(_ context.Context, root string) ([]string, string, error) {
	f.dirs = append(f.dirs, root)
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

func TestRepoContractFloorChecks_GreenOrAnUnnamedExitPassesTheHandoff(t *testing.T) {
	wt := worktreeWithModule(t)
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: wt}
	if got := RepoContractFloorChecks((&fakeRepoContractPack{}).run)(context.Background(), in); len(got) != 0 {
		t.Fatalf("a green pack passes the handoff; got %v", got)
	}
	unnamed := &fakeRepoContractPack{diagnostic: "go: toolchain chatter before the kill\n", err: errors.New("signal: killed")}
	var got []string
	stderr := captureStderr(t, func() { got = RepoContractFloorChecks(unnamed.run)(context.Background(), in) })
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
	pack := &fakeRepoContractPack{reds: []string{"pkg.TestX"}}
	got := RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)})
	if len(got) != 1 || !strings.Contains(got[0], "pkg.TestX") {
		t.Fatalf("ship names reds only for its real red, so the floor branches on the names alone; got %v", got)
	}
}

func TestRepoContractFloorChecks_RunsThePackUnderItsOwnDeadline(t *testing.T) {
	var remaining time.Duration
	var hasDeadline bool
	run := func(ctx context.Context, _ string) ([]string, string, error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	blocking := func(ctx context.Context, _ string) ([]string, string, error) {
		<-ctx.Done()
		return nil, "partial output before the kill\n", ctx.Err()
	}
	var got []string
	stderr := captureStderr(t, func() {
		got = RepoContractFloorChecks(blocking)(ctx, ReviewInput{Phase: string(PhaseBuild), Worktree: worktreeWithModule(t)})
	})
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
