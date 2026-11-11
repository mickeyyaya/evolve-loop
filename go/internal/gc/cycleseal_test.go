package gc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const sealLeafPrefix = "cycle-cd3ae73e-"

type sealEnv struct {
	t         *testing.T
	repo      *gittest.Repo
	base      string
	evolveDir string
	now       time.Time
	listing   string
	signaled  []int
	pidAlive  bool
}

func newSealEnv(t *testing.T) *sealEnv {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("f.txt", "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	return &sealEnv{
		t:         t,
		repo:      repo,
		base:      filepath.Join(repo.Dir, ".evolve", "worktrees"),
		evolveDir: filepath.Join(repo.Dir, ".evolve"),
		now:       time.Now(),
	}
}

func (e *sealEnv) leaf(cycle int) string { return fmt.Sprintf("%s%d", sealLeafPrefix, cycle) }

func (e *sealEnv) lane(cycle int, dirty bool) string {
	e.t.Helper()
	wt := filepath.Join(e.base, e.leaf(cycle))
	e.repo.Git("worktree", "add", "-q", wt, "-b", e.leaf(cycle))
	writeFile(e.t, filepath.Join(wt, "lane.txt"), "lane work\n")
	e.repo.Git("-C", wt, "add", "lane.txt")
	e.repo.Git("-C", wt, "commit", "-q", "-m", "lane work")
	if dirty {
		writeFile(e.t, filepath.Join(wt, "f.txt"), "base\nuncommitted edit\n")
		writeFile(e.t, filepath.Join(wt, "sub", "new.txt"), "untracked\n")
	}
	return wt
}

func (e *sealEnv) orphan(pid int, cwd string) {
	e.listing += fmt.Sprintf("p%d\nR1\nfcwd\nn%s\n", pid, cwd)
}

func (e *sealEnv) attached(pid, ppid int, cwd string) {
	e.listing += fmt.Sprintf("p%d\nR%d\nfcwd\nn%s\n", pid, ppid, cwd)
}

func (e *sealEnv) exec(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errOut io.Writer) (int, error) {
	if name == "lsof" {
		_, _ = io.WriteString(out, e.listing)
		return 0, nil
	}
	return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errOut)
}

func (e *sealEnv) kill(pid int) error {
	e.signaled = append(e.signaled, pid)
	return nil
}

func (e *sealEnv) opts() WorktreeOptions {
	return WorktreeOptions{
		ProjectRoot:  e.repo.Dir,
		WorktreeBase: e.base,
		EvolveDir:    e.evolveDir,
		Policy:       WorktreesPolicy{KeepRecent: 99, MinAgeMinutes: 1_000_000, SalvageAfterHours: 1_000_000},
		Now:          func() time.Time { return e.now },
		Exec:         e.exec,
		PidAlive:     func(int) bool { return e.pidAlive },
		LeaseTTL:     runlease.DefaultTTL,
	}
}

func (e *sealEnv) sweep(cycle int) SealedCycleReport {
	e.t.Helper()
	return SweepSealedCycle(context.Background(), e.opts(), cycle, e.kill)
}

func (e *sealEnv) leaseRun(cycle int) {
	e.t.Helper()
	e.leaseRunOwnedBy(cycle, os.Getppid())
}

func (e *sealEnv) leaseRunOwnedBy(cycle, owner int) {
	e.t.Helper()
	dir := filepath.Join(e.evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := runlease.Write(dir, runlease.Lease{RunID: fmt.Sprintf("cycle-%d", cycle), OwnerPID: owner}, e.now); err != nil {
		e.t.Fatal(err)
	}
	e.pidAlive = true
}

func (e *sealEnv) pauseRun(cycle int, worktree string) {
	e.t.Helper()
	dir := filepath.Join(e.evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle))
	state := map[string]any{"cycle_id": cycle, "phase": "build", "checkpoint": map[string]any{
		"enabled": true, "reason": "quota-likely", "resumeFromPhase": "build", "worktreePath": worktree,
		"savedAt": e.now.UTC().Format(time.RFC3339), "autoResumeAttempts": 0, "autoResumeMaxAttempts": 3,
	}}
	raw, err := json.Marshal(state)
	if err != nil {
		e.t.Fatal(err)
	}
	writeFile(e.t, filepath.Join(dir, core.CycleStateFile), string(raw))
	scope, err := json.Marshal(core.LaneScope{TodoIDs: []string{fmt.Sprintf("item-%d", cycle)}, GoalHash: "wave-goal"})
	if err != nil {
		e.t.Fatal(err)
	}
	writeFile(e.t, filepath.Join(dir, core.LaneScopeFile), string(scope))
	paused, err := core.PausedLaneScopes(e.repo.Dir, e.evolveDir, core.ResumeOptions{})
	if err != nil || len(paused) != 1 || paused[0].Cycle != cycle {
		e.t.Fatalf("fixture error: cycle %d is not seen as paused: %+v, %v", cycle, paused, err)
	}
}

func (e *sealEnv) assertLaneKept(cycle int, wantFile, wantBody string) {
	e.t.Helper()
	wt := filepath.Join(e.base, e.leaf(cycle))
	raw, err := os.ReadFile(filepath.Join(wt, wantFile))
	if err != nil {
		e.t.Errorf("lane of cycle %d lost %s: %v", cycle, wantFile, err)
	} else if string(raw) != wantBody {
		e.t.Errorf("lane of cycle %d: %s = %q, want %q", cycle, wantFile, raw, wantBody)
	}
	if !gittest.BranchExists(e.t, e.repo.Dir, e.leaf(cycle)) {
		e.t.Errorf("branch %s of cycle %d was deleted", e.leaf(cycle), cycle)
	}
}

func (e *sealEnv) bundleHolding(sha, branch string) string {
	e.t.Helper()
	var found string
	_ = filepath.WalkDir(OperatorSalvageDir(e.evolveDir), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != salvageBundleFile {
			return nil
		}
		heads, herr := e.opts().git(e.repo.Dir, "bundle", "list-heads", p)
		if herr == nil && strings.Contains(heads, sha+" refs/heads/"+branch) {
			found = p
		}
		return nil
	})
	return found
}

func TestSweepSealedCycle_ASealedDirtyLaneWithAnOrphanIsCleanedAtOnce(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	other := e.lane(801, true)
	e.orphan(20, filepath.Join(sealed, "go"))
	e.orphan(21, other)
	e.orphan(22, filepath.Join(e.repo.Dir, "console"))
	e.attached(23, 4242, sealed)

	var rep SealedCycleReport = e.sweep(800)

	if rep.Held != "" || len(rep.Errors) != 0 {
		t.Fatalf("a sealed cycle with no pause, binding or live run must be cleaned: held=%q errors=%v", rep.Held, rep.Errors)
	}
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]: only the orphan inside the lane of the sealed cycle", e.signaled)
	}
	if len(rep.Reaped) != 1 || rep.Reaped[0].Pid != 20 {
		t.Errorf("Reaped = %+v, want pid 20", rep.Reaped)
	}
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the lane of the sealed cycle must be released at once, stat err=%v", err)
	}
	if gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
		t.Errorf("branch %s of the sealed cycle must be released", e.leaf(800))
	}
	if strings.Contains(e.repo.Git("worktree", "list", "--porcelain"), sealed) {
		t.Errorf("git still lists the released lane %s", sealed)
	}
	e.assertLaneKept(801, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_TheLaneIsSavedAndTheBundleVerifiedBeforeTheLaneAndBranchGo(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	tip := strings.TrimSpace(e.repo.Git("rev-parse", e.leaf(800)))

	rep := e.sweep(800)

	if len(rep.Errors) != 0 {
		t.Fatalf("errors: %v", rep.Errors)
	}
	bundle := e.bundleHolding(tip, e.leaf(800))
	if bundle == "" {
		t.Fatalf("no refs.bundle under %s holds %s@%s: the unmerged lane commit is lost", OperatorSalvageDir(e.evolveDir), e.leaf(800), tip)
	}
	if _, err := e.opts().git(e.repo.Dir, "bundle", "verify", bundle); err != nil {
		t.Errorf("the saved bundle does not verify: %v", err)
	}
	leafDir := filepath.Join(OperatorSalvageDir(e.evolveDir), filepath.Base(sealed))
	patch, err := os.ReadFile(filepath.Join(leafDir, salvagePatchFile))
	if err != nil || !strings.Contains(string(patch), "uncommitted edit") {
		t.Errorf("the uncommitted edit is not saved: err=%v patch=%q", err, patch)
	}
	if _, err := os.Stat(filepath.Join(leafDir, salvageUntrackedFile)); err != nil {
		t.Errorf("the untracked file is not saved: %v", err)
	}
}

func TestSweepSealedCycle_ARejectedBundleKeepsTheLaneAndTheBranch(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	gittest.FaultOn(t, "bundle")

	rep := e.sweep(800)

	if len(rep.Errors) == 0 {
		t.Errorf("a failed bundle must be reported")
	}
	if _, err := os.Stat(sealed); err != nil {
		t.Errorf("the lane must stay when its bundle did not verify: %v", err)
	}
	if !gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
		t.Errorf("the branch must stay when its bundle did not verify")
	}
}

func TestSweepSealedCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses(t *testing.T) {
	e := newSealEnv(t)
	lane := e.lane(800, true)
	e.pauseRun(800, lane)
	e.orphan(20, lane)

	rep := e.sweep(800)

	if rep.Held == "" {
		t.Errorf("a paused cycle must report why it was held")
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: a paused cycle keeps its processes for the resume", e.signaled)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_ACycleThatSealsIntoAContinuationKeepsItsLaneBranchAndProcesses(t *testing.T) {
	e := newSealEnv(t)
	lane := e.lane(800, true)
	if err := continuation.WriteRegistryEntry(e.repo.Dir, "item-1", continuation.Continuation{Branch: e.leaf(800), Cycle: 800, Worktree: lane}); err != nil {
		t.Fatal(err)
	}
	e.orphan(20, lane)

	rep := e.sweep(800)

	if rep.Held == "" {
		t.Errorf("a cycle bound to a continuation must report why it was held")
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: a continuation keeps the processes of its lane", e.signaled)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_ACycleWithALiveRunKeepsItsLane(t *testing.T) {
	e := newSealEnv(t)
	lane := e.lane(800, true)
	e.leaseRun(800)
	e.orphan(20, lane)

	rep := e.sweep(800)

	if rep.Held == "" {
		t.Errorf("a cycle whose run holds a live lease must report why it was held")
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: the run of the cycle is live", e.signaled)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_TheLiveLaneOfAnotherCycleIsNeverTouched(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	live := e.lane(801, true)
	e.leaseRun(801)
	e.orphan(20, sealed)
	e.orphan(21, live)

	rep := e.sweep(800)

	if rep.Held != "" || len(rep.Errors) != 0 {
		t.Fatalf("the live lane of cycle 801 must not hold cycle 800: held=%q errors=%v", rep.Held, rep.Errors)
	}
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]: pid 21 sits in the live lane of cycle 801", e.signaled)
	}
	e.assertLaneKept(801, "f.txt", "base\nuncommitted edit\n")
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the sealed lane must still be released: %v", err)
	}
}

func TestSweepSealedCycle_ABranchWhoseLaneIsGoneIsReleasedAndNoOtherBranchIs(t *testing.T) {
	e := newSealEnv(t)
	e.repo.Git("branch", e.leaf(810))
	e.repo.Git("checkout", "-q", "-b", e.leaf(811))
	e.repo.Write("n.txt", "n\n")
	e.repo.Git("add", "n.txt")
	e.repo.Git("commit", "-q", "-m", "unmerged work of cycle 811")
	e.repo.Git("checkout", "-q", "main")
	e.repo.Git("branch", "feat/x", e.leaf(811))
	unmerged := e.leaf(812)
	e.repo.Git("branch", unmerged, e.leaf(811))
	tip := strings.TrimSpace(e.repo.Git("rev-parse", unmerged))

	rep := e.sweep(812)

	if len(rep.Errors) != 0 {
		t.Fatalf("errors: %v", rep.Errors)
	}
	if gittest.BranchExists(t, e.repo.Dir, unmerged) {
		t.Errorf("branch %s of the sealed cycle must be released", unmerged)
	}
	if e.bundleHolding(tip, unmerged) == "" {
		t.Errorf("the unmerged branch was deleted with no verified bundle holding %s", tip)
	}
	for _, kept := range []string{e.leaf(810), e.leaf(811), "feat/x", "main"} {
		if !gittest.BranchExists(t, e.repo.Dir, kept) {
			t.Errorf("branch %s is not a branch of cycle 812 and must stay", kept)
		}
	}
}

func TestSweepSealedCycle_ACycleWithNoLaneAndNoBranchIsAQuietNoop(t *testing.T) {
	e := newSealEnv(t)
	other := e.lane(801, false)
	e.orphan(21, other)

	rep := e.sweep(900)

	if rep.Held != "" || len(rep.Errors) != 0 || len(rep.Items) != 0 || len(rep.Reaped) != 0 {
		t.Errorf("a cycle with nothing left must report nothing: %+v", rep)
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v", e.signaled)
	}
	e.assertLaneKept(801, "lane.txt", "lane work\n")
}

func TestSweepSealedCycle_AFailedProcessListingStillReleasesTheLaneAndIsReported(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, false)
	o := e.opts()
	real := o.Exec
	o.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errOut io.Writer) (int, error) {
		if name == "lsof" {
			return 0, fmt.Errorf("lsof: not found")
		}
		return real(ctx, name, dir, args, env, in, out, errOut)
	}

	rep := SweepSealedCycle(context.Background(), o, 800, e.kill)

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "lsof: not found") {
		t.Errorf("Errors = %v, want the listing failure surfaced", rep.Errors)
	}
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("a failed listing must not keep a clean sealed lane: %v", err)
	}
}

func (e *sealEnv) planeState(state map[string]any) {
	e.t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		e.t.Fatal(err)
	}
	writeFile(e.t, filepath.Join(e.evolveDir, core.CycleStateFile), string(raw))
}

func (e *sealEnv) runPointer(cycle int, worktree string) {
	e.t.Helper()
	writeFile(e.t, filepath.Join(e.evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle), "run.json"), `{"active_worktree":"`+worktree+`"}`)
}

func TestSweepSealedCycle_TheStalePointersOfTheSealedRunDoNotHoldItsLane(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	e.planeState(map[string]any{"cycle_id": 800, "phase": "end", "active_worktree": sealed})
	e.runPointer(800, sealed)
	e.orphan(20, sealed)

	rep := e.sweep(800)

	if rep.Held != "" || len(rep.Errors) != 0 {
		t.Fatalf("the plane marker and run.json of the sealed cycle still name its lane; the seal proves the end: held=%q errors=%v", rep.Held, rep.Errors)
	}
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the lane of the sealed cycle must be released: %v", err)
	}
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]", e.signaled)
	}
}

func TestSweepSealedCycle_ASequentialCycleThatSealsIntoAPauseKeepsItsLane(t *testing.T) {
	e := newSealEnv(t)
	lane := e.lane(800, true)
	e.planeState(map[string]any{"cycle_id": 800, "phase": "build", "active_worktree": lane, "checkpoint": map[string]any{
		"enabled": true, "reason": "quota-likely", "resumeFromPhase": "build", "worktreePath": lane,
		"savedAt": e.now.UTC().Format(time.RFC3339), "autoResumeAttempts": 0, "autoResumeMaxAttempts": 3,
	}})
	e.orphan(20, lane)

	rep := e.sweep(800)

	if rep.Held == "" {
		t.Errorf("a cycle paused in the plane marker must report why it was held")
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: a paused cycle keeps its processes for the resume", e.signaled)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_TheLanePointedAtByThePlaneMarkerOfAnotherCycleIsNeverTouched(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	running := e.lane(801, true)
	e.planeState(map[string]any{"cycle_id": 801, "phase": "build", "active_worktree": running})
	e.orphan(21, running)

	rep := e.sweep(800)

	if rep.Held != "" || len(rep.Errors) != 0 {
		t.Fatalf("the running cycle 801 must not hold cycle 800: held=%q errors=%v", rep.Held, rep.Errors)
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: pid 21 sits in the lane of the running cycle 801", e.signaled)
	}
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the sealed lane must be released: %v", err)
	}
	e.assertLaneKept(801, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_TheFreshLeaseOfTheFinishedRunOwnedByTheSweepingProcessDoesNotHoldItsLane(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	e.leaseRunOwnedBy(800, os.Getpid())
	e.orphan(20, sealed)

	rep := e.sweep(800)

	if rep.Held != "" || len(rep.Errors) != 0 {
		t.Fatalf("the run of cycle 800 returned into this process, so its own live lease ends with it: held=%q errors=%v", rep.Held, rep.Errors)
	}
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the lane of the sealed cycle must be released: %v", err)
	}
	if gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
		t.Errorf("branch %s of the sealed cycle must be released", e.leaf(800))
	}
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]", e.signaled)
	}
}

func TestSweepSealedCycle_TheLeaseOfAnotherLiveProcessStillHoldsTheLaneWhileThisProcessIsAlsoAlive(t *testing.T) {
	e := newSealEnv(t)
	lane := e.lane(800, true)
	e.leaseRunOwnedBy(800, os.Getppid())
	e.orphan(20, lane)

	rep := e.sweep(800)

	if rep.Held == "" {
		t.Errorf("a fresh lease owned by another live process must hold the cycle")
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: another process still runs the cycle", e.signaled)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_TheLeaseOwnedByTheSweepingProcessOfAPausedCycleStillHoldsItsLane(t *testing.T) {
	e := newSealEnv(t)
	lane := e.lane(800, true)
	e.pauseRun(800, lane)
	e.leaseRunOwnedBy(800, os.Getpid())
	e.orphan(20, lane)

	rep := e.sweep(800)

	if rep.Held == "" {
		t.Errorf("a paused cycle must stay held whoever owns its lease")
	}
	if len(e.signaled) != 0 {
		t.Errorf("signaled %v: a paused cycle keeps its processes", e.signaled)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}
