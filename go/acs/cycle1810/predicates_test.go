//go:build acs

package cycle1810

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1810_001_LandPatchYieldsUncommittedWorktreeOnBranchAtOriginMain(t *testing.T) {
	h := newHub(t)
	patch := h.lanePatch(t, map[string]string{
		"clean.txt":     baseCleanText + "landed\n",
		"added/new.txt": "brand new\n",
	})
	originTip := h.advanceOrigin(t, "moved.txt", "origin moved after the hub was cloned\n")
	staleLocalMain := gitIn(t, h.store, "rev-parse", "refs/heads/main")
	if staleLocalMain == originTip {
		t.Fatalf("fixture: the store's main must lag origin so a stale base is detectable")
	}

	r := runEvolve(t, h.runtime, sandboxEnv(h.runtime, ""), "land", "--patch", patch, "--branch", "land-ok")
	if r.code != exitOK {
		t.Fatalf("evolve land --patch F --branch land-ok: want exit 0\n%s", r)
	}
	wt, ok := h.worktreeOnBranch(t, "land-ok")
	if !ok {
		t.Fatalf("no worktree of the hub store is on branch land-ok after a successful land\n%s", r)
	}
	if head := gitIn(t, wt, "rev-parse", "HEAD"); head != originTip {
		t.Errorf("land-ok worktree HEAD = %s, want the fetched origin/main tip %s (stale local main is %s)", head, originTip, staleLocalMain)
	}
	if got := gitIn(t, wt, "symbolic-ref", "--short", "HEAD"); got != "land-ok" {
		t.Errorf("worktree is on %q, want branch land-ok", got)
	}
	if got := readFile(t, filepath.Join(wt, "clean.txt")); got != baseCleanText+"landed\n" {
		t.Errorf("clean.txt = %q: the patch's edit is not applied", got)
	}
	if got := readFile(t, filepath.Join(wt, "added", "new.txt")); got != "brand new\n" {
		t.Errorf("added/new.txt = %q: the patch's new file is not applied", got)
	}
	if got := readFile(t, filepath.Join(wt, "moved.txt")); !strings.Contains(got, "origin moved") {
		t.Errorf("moved.txt = %q: the worktree does not carry origin/main's newest commit", got)
	}
	if status := gitIn(t, wt, "status", "--porcelain"); !strings.Contains(status, "clean.txt") {
		t.Errorf("the applied patch must sit uncommitted in the worktree; status:\n%s", status)
	}
	if h.branchExists(t, h.origin.Dir, "land-ok") {
		t.Errorf("land pushed branch land-ok to origin; it must never push")
	}
	if got := h.originMain(t); got != originTip {
		t.Errorf("origin main moved to %s during land; it must never push", got)
	}
	if !strings.Contains(r.combined(), "evolve ship") {
		t.Errorf("land must print the next commands (commit gate, evolve ship); output:\n%s", r)
	}
}

func TestC1810_002_LandSalvageRestoresLeafPatchAndUntrackedFiles(t *testing.T) {
	h := newHub(t)
	leaf := salvageLeaf{
		name:    "cycle-abc12345-77",
		patched: map[string]string{"clean.txt": baseCleanText + "salvaged edit\n"},
		untracked: map[string]string{
			"top.txt":                 "top-level untracked\n",
			"notes/deep/salvaged.txt": "nested untracked\n",
		},
	}
	h.writeSalvageLeaf(t, leaf)
	originTip := h.advanceOrigin(t, "moved.txt", "origin moved after the leaf was salvaged\n")
	salvageDir := filepath.Join(h.runtime, ".evolve", "operator-salvage")

	r := runEvolve(t, salvageDir, sandboxEnv(h.runtime, ""), "land", "--salvage", leaf.name, "--branch", "land-salvage")
	if r.code != exitOK {
		t.Fatalf("evolve land --salvage %s --branch land-salvage: want exit 0\n%s", leaf.name, r)
	}
	wt, ok := h.worktreeOnBranch(t, "land-salvage")
	if !ok {
		t.Fatalf("no worktree of the hub store is on branch land-salvage\n%s", r)
	}
	if head := gitIn(t, wt, "rev-parse", "HEAD"); head != originTip {
		t.Errorf("land-salvage worktree HEAD = %s, want origin/main tip %s with nothing committed", head, originTip)
	}
	if got := readFile(t, filepath.Join(wt, "clean.txt")); got != baseCleanText+"salvaged edit\n" {
		t.Errorf("clean.txt = %q: the leaf's uncommitted.patch is not applied", got)
	}
	for name, want := range leaf.untracked {
		if got := readFile(t, filepath.Join(wt, filepath.FromSlash(name))); got != want {
			t.Errorf("%s = %q, want %q: the leaf's untracked file is not restored", name, got, want)
		}
	}
	if h.branchExists(t, h.origin.Dir, "land-salvage") {
		t.Errorf("land pushed branch land-salvage to origin; it must never push")
	}
}

func TestC1810_003_LandConflictingPatchExits1NamingConflictedFilesWithMarkers(t *testing.T) {
	h := newHub(t)
	patch := h.lanePatch(t, map[string]string{
		"conflict.txt": "one\nLANE\nthree\n",
		"clean.txt":    baseCleanText + "lane line\n",
	})
	originTip := h.advanceOrigin(t, "conflict.txt", "one\nMAIN\nthree\n")

	r := runEvolve(t, h.runtime, sandboxEnv(h.runtime, ""), "land", "--patch", patch, "--branch", "land-conflict")
	if r.code != exitRefused {
		t.Fatalf("a patch conflicting with origin/main: want exit 1\n%s", r)
	}
	if !strings.Contains(r.combined(), "conflict.txt") {
		t.Errorf("the conflict report must name the conflicted file conflict.txt\n%s", r)
	}
	wt, ok := h.worktreeOnBranch(t, "land-conflict")
	if !ok {
		t.Fatalf("a conflict must leave the worktree on land-conflict for resolution\n%s", r)
	}
	got := readFile(t, filepath.Join(wt, "conflict.txt"))
	if !strings.Contains(got, "<<<<<<<") || !strings.Contains(got, ">>>>>>>") || !strings.Contains(got, "LANE") || !strings.Contains(got, "MAIN") {
		t.Errorf("conflict.txt must keep both sides between conflict markers (a --3way apply); got:\n%s", got)
	}
	if head := gitIn(t, wt, "rev-parse", "HEAD"); head != originTip {
		t.Errorf("land-conflict HEAD = %s, want origin/main tip %s with nothing committed", head, originTip)
	}
	if h.branchExists(t, h.origin.Dir, "land-conflict") {
		t.Errorf("land pushed branch land-conflict to origin; it must never push")
	}
}

func TestC1810_004_LandOntoExistingBranchExits1AndCreatesNoWorktree(t *testing.T) {
	h := newHub(t)
	patch := h.lanePatch(t, map[string]string{"clean.txt": baseCleanText + "landed\n"})
	h.advanceOrigin(t, "moved.txt", "origin moved\n")
	gitIn(t, h.store, "branch", "taken", "refs/heads/main")
	takenBefore := gitIn(t, h.store, "rev-parse", "refs/heads/taken")
	worktreesBefore := h.worktreeCount(t)

	r := runEvolve(t, h.runtime, sandboxEnv(h.runtime, ""), "land", "--patch", patch, "--branch", "taken")
	if r.code != exitRefused {
		t.Fatalf("land onto an existing branch: want exit 1\n%s", r)
	}
	if !strings.Contains(r.combined(), "taken") {
		t.Errorf("the refusal must name the existing branch taken\n%s", r)
	}
	if got := h.worktreeCount(t); got != worktreesBefore {
		t.Errorf("a refused land changed the worktree count %d -> %d", worktreesBefore, got)
	}
	if got := gitIn(t, h.store, "rev-parse", "refs/heads/taken"); got != takenBefore {
		t.Errorf("a refused land moved the existing branch taken %s -> %s", takenBefore, got)
	}
}

func TestC1810_005_LandMissingPatchOrLeafExits2BeforeAnySideEffect(t *testing.T) {
	cases := []struct {
		name, branch, missing string
		args                  func(h hubFixture) (cwd string, args []string)
	}{
		{"missing patch file", "no-patch", "absent.patch", func(h hubFixture) (string, []string) {
			return h.runtime, []string{"land", "--patch", filepath.Join(h.root, "absent.patch"), "--branch", "no-patch"}
		}},
		{"missing salvage leaf", "no-leaf", "cycle-deadbeef-404", func(h hubFixture) (string, []string) {
			dir := filepath.Join(h.runtime, ".evolve", "operator-salvage")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			return dir, []string{"land", "--salvage", "cycle-deadbeef-404", "--branch", "no-leaf"}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHub(t)
			worktreesBefore := h.worktreeCount(t)
			cwd, args := c.args(h)
			r := runEvolve(t, cwd, sandboxEnv(h.runtime, ""), args...)
			if r.code != exitIO {
				t.Fatalf("%s: want exit 2\n%s", c.name, r)
			}
			if out := r.combined(); !strings.Contains(out, c.missing) || strings.Contains(out, "unknown command") {
				t.Errorf("%s: the exit-2 report must name the missing input %s, not be an unknown-command error\n%s", c.name, c.missing, r)
			}
			if h.branchExists(t, h.store, c.branch) {
				t.Errorf("%s: land created branch %s before finding its input missing", c.name, c.branch)
			}
			if got := h.worktreeCount(t); got != worktreesBefore {
				t.Errorf("%s: land changed the worktree count %d -> %d", c.name, worktreesBefore, got)
			}
		})
	}
}

func TestC1810_006_BoundaryDryRunPrintsSixStepsInOrderWithoutSideEffects(t *testing.T) {
	h := newHub(t)
	h.advanceOrigin(t, "moved.txt", "origin moved\n")
	fb := newFakeBin(t)
	goal := filepath.Join(t.TempDir(), "goal.txt")
	writeFile(t, goal, "drain the queue\n")
	headBefore := h.runtimeHead(t)

	r := runEvolve(t, h.runtime, sandboxEnv(h.runtime, fb.dir), "boundary", "run", "--dry-run", "--merge", "12", "--goal-text-file", goal)
	if r.code != exitOK {
		t.Fatalf("evolve boundary run --dry-run --merge 12 --goal-text-file F: want exit 0\n%s", r)
	}
	if lines, missing := stepLinesInOrder(r.stdout, boundaryPlanSteps("12")); missing != "" {
		t.Errorf("dry-run plan is missing step %q after lines %v (want six steps in order: loop-stop --wait, pr merge 12, sync-main, gc, loop-stop --release, loop --detach)\n%s", missing, lines, r)
	}
	if _, err := os.Stat(h.brakePath()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dry-run engaged the loop-stop brake (%s): a dry run must run no step", h.brakePath())
	}
	if calls := fb.calls(t); len(calls) != 0 {
		t.Errorf("dry-run called gh %v: a dry run must merge nothing", calls)
	}
	if got := h.runtimeHead(t); got != headBefore {
		t.Errorf("dry-run moved the plane %s -> %s: a dry run must not sync", headBefore, got)
	}
	if _, err := os.Stat(filepath.Join(h.runtime, ".evolve", "runs")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dry-run created .evolve/runs: a dry run must launch nothing")
	}
}

func TestC1810_007_BoundaryRefusalAtPRMergeStopsBeforeSyncMainWithItsExitCode(t *testing.T) {
	h := newHub(t)
	originTip := h.advanceOrigin(t, "moved.txt", "origin moved\n")
	fb := newFakeBin(t)
	env := sandboxEnv(h.runtime, fb.dir)
	goal := filepath.Join(t.TempDir(), "goal.txt")
	writeFile(t, goal, "drain the queue\n")

	alone := runEvolve(t, h.runtime, env, "pr", "merge", "12")
	if alone.code != exitRefused {
		t.Fatalf("fixture: evolve pr merge 12 on a CLOSED PR must refuse with exit 1\n%s", alone)
	}
	fb.reset(t)
	headBefore := h.runtimeHead(t)
	if headBefore == originTip {
		t.Fatalf("fixture: the plane must lag origin so a sync-main run is observable")
	}

	r := runEvolve(t, h.runtime, env, "boundary", "run", "--merge", "12", "--goal-text-file", goal)
	if r.code != alone.code {
		t.Fatalf("boundary run must return pr merge's own exit code %d at its refusal\n%s", alone.code, r)
	}
	calls := fb.calls(t)
	if len(calls) == 0 || !strings.HasPrefix(calls[0], "pr view 12") {
		t.Errorf("boundary run never reached the real pr merge verb for #12; gh calls: %v", calls)
	}
	if _, err := os.Stat(h.brakePath()); err != nil {
		t.Errorf("after a refused merge the loop-stop brake must stay engaged (loop-stop --wait ran, --release must not): %v", err)
	}
	if got := h.runtimeHead(t); got != headBefore {
		t.Errorf("the plane moved %s -> %s: sync-main ran after the merge refusal", headBefore, got)
	}
	if strings.Contains(r.combined(), "detached pid") {
		t.Errorf("a loop was launched after the merge refusal\n%s", r)
	}

	sync := runEvolve(t, h.runtime, env, "sync-main")
	if sync.code != exitOK || h.runtimeHead(t) != originTip {
		t.Fatalf("fixture: evolve sync-main alone must move the plane to %s, else the not-synced check above is vacuous\n%s", originTip, sync)
	}
}

func TestC1810_008_BoundaryFakeStepsStopAtFailureAndEndWithDetachedPidAndBootVerdict(t *testing.T) {
	frozen := []string{
		"TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode",
		"TestBoundaryRun_FullFakeRunEndsWithDetachedPidAndBootVerdict",
		"TestBoundaryRun_DryRunDispatchesNoStep",
		"TestBoundaryRun_BadArgumentsDispatchNoStep",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(frozen, "|")+")$", "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.Env = sandboxEnv("", "")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the frozen boundary fake-step tests fail: %v\n%s", err, out)
	}
	for _, name := range frozen {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass\n%s", name, out)
		}
	}
}

func TestC1810_009_BoundaryLaunchStepPassesTheRealLoopArgumentParser(t *testing.T) {
	repair := []string{
		"TestBoundaryRun_LaunchStepPassesTheRealLoopArgumentParser",
		"TestBoundaryRun_RefusedLaunchReturnsTheLoopsExitCodeAfterTheRelease",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(repair, "|")+")$", "./cmd/evolve")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.Env = sandboxEnv("", "")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("boundary's launch argv is refused by evolve loop's own argument parser: %v\n%s", err, out)
	}
	for _, name := range repair {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass\n%s", name, out)
		}
	}
}

func TestC1810_010_BoundaryDryRunLaunchStepLogsUnderThePlanesEvolveDir(t *testing.T) {
	h := newHub(t)
	fb := newFakeBin(t)
	goal := filepath.Join(t.TempDir(), "goal.txt")
	writeFile(t, goal, "drain the queue\n")

	r := runEvolve(t, h.runtime, sandboxEnv(h.runtime, fb.dir), "boundary", "run", "--dry-run", "--goal-text-file", goal, "--max-cycles", "2")
	if r.code != exitOK {
		t.Fatalf("evolve boundary run --dry-run --goal-text-file F --max-cycles 2: want exit 0\n%s", r)
	}
	launch, ok := launchPlanLine(r.stdout)
	if !ok {
		t.Fatalf("dry-run plan has no `loop --detach` launch step\n%s", r)
	}
	logPath, ok := flagValueIn(launch, "--log")
	if !ok {
		t.Fatalf("the launch step %q carries no --log, and evolve loop refuses --detach without one (exit 10)", launch)
	}
	planeEvolveDir := filepath.Join(h.runtime, ".evolve")
	if rel, err := filepath.Rel(planeEvolveDir, logPath); !filepath.IsAbs(logPath) || err != nil || rel == "." || !filepath.IsLocal(rel) {
		t.Errorf("the launch logs to %q, want an absolute file under the plane's %s (a relative log lands in the operator's cwd)", logPath, planeEvolveDir)
	}
	if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a dry run created the launch log %s: it must launch nothing", logPath)
	}
}
