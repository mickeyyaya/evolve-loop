package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ratchetRedName = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"

type fakeRepoContractPack struct {
	reds []string
	err  error
	dirs []string
}

func (f *fakeRepoContractPack) run(_ context.Context, moduleDir string) ([]string, error) {
	f.dirs = append(f.dirs, moduleDir)
	return f.reds, f.err
}

func ratchetRedPack() *fakeRepoContractPack {
	return &fakeRepoContractPack{reds: []string{ratchetRedName}, err: errors.New("exit status 1")}
}

func worktreeWithModule(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestRepoContractFloorChecks_ARawGitFixtureIsCorrectedAtBuildExitByTheRatchetsName(t *testing.T) {
	wt := worktreeWithModule(t)
	pack := ratchetRedPack()
	var run RepoContractPackFn = pack.run
	res := NewBuildFloorReviewer(RepoContractFloorChecks(run)).Review(context.Background(),
		ReviewInput{Phase: string(PhaseBuild), Worktree: wt, ProjectRoot: t.TempDir()})
	if res.Approve || !res.Retry {
		t.Fatalf("a red repo-contract pack goes back to the build correction; got Approve=%v Retry=%v", res.Approve, res.Retry)
	}
	if !strings.Contains(res.Reason, "TestRatchet_NoNewRawGitFixtures") || !strings.Contains(res.Reason, "repo-contract scanner pack") {
		t.Fatalf("the correction names the pack and the ratchet's failing test; got %q", res.Reason)
	}
	if len(pack.dirs) != 1 || pack.dirs[0] != filepath.Join(wt, "go") {
		t.Fatalf("the pack runs once in the build worktree's module; ran in %v", pack.dirs)
	}
}

func TestRepoContractFloorChecks_GreenOrAnUnnamedExitPassesTheHandoff(t *testing.T) {
	wt := worktreeWithModule(t)
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: wt}
	for name, pack := range map[string]*fakeRepoContractPack{
		"green":                        {},
		"nonzero exit naming no test":  {err: errors.New("signal: killed")},
		"named test with a clean exit": {reds: []string{"pkg.TestX"}},
	} {
		if got := RepoContractFloorChecks(pack.run)(context.Background(), in); len(got) != 0 {
			t.Errorf("%s: only a named red with a failed run corrects the build (ship's gate classes the rest); got %v", name, got)
		}
	}
	pack := ratchetRedPack()
	if got := RepoContractFloorChecks(pack.run)(context.Background(), ReviewInput{Phase: string(PhaseBuild)}); len(got) != 0 || len(pack.dirs) != 0 {
		t.Fatalf("no worktree means nothing to run; got %v, ran in %v", got, pack.dirs)
	}
}

func TestRepoContractFloorChecks_FollowsTheShipGatesDial(t *testing.T) {
	wt := worktreeWithModule(t)
	for word, wantRun := range map[string]bool{"off": false, "enforce": true, "": true} {
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

func writePolicyGate(t *testing.T, root, word string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"gates":{"repo_contract_gate":"` + word + `"}}`
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
		WithReviewer(NewBuildFloorReviewer(RepoContractFloorChecks(ratchetRedPack().run))))
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
