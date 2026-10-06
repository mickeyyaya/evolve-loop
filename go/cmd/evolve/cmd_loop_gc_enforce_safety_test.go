package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func checkedInGCPolicy(t *testing.T) gc.Policy {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	checkedIn := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "policy.json")
	pol, err := policy.Load(checkedIn)
	if err != nil {
		t.Fatalf("load %s: %v", checkedIn, err)
	}
	if pol.GC == nil {
		t.Fatalf("%s has no gc block", checkedIn)
	}
	return *pol.GC
}

func TestTheCheckedInPolicyRunsTheWaveEndGCInEnforce(t *testing.T) {
	if mode := checkedInGCPolicy(t).Mode; mode != "enforce" {
		t.Fatalf("checked-in gc.mode = %q, want enforce: in shadow the loop's batch-start and batch-end GC only reports what it would release", mode)
	}
}

type enforceSafetyFixture struct {
	t                       *testing.T
	projectRoot, evolveDir  string
	devUnlanded, devMerged  string
	liveLane, snapshotLane  string
	snapshotSHA, registry   string
	unmergedBranchHead      string
	devUnlandedHead         string
	liveLaneHead            string
	registryBefore          []byte
	unlandedEdit, laneEdit  string
	workspace               string
	closedOutLongAgo        time.Time
	unmergedBranch, devHead string
	checkpoints             map[string]string
}

func (f *enforceSafetyFixture) closeOut(cycle int) {
	f.t.Helper()
	dir := dossier.CyclesDir(f.projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(dir, "cycle-"+strconv.Itoa(cycle)+".json")
	if err := os.WriteFile(p, []byte(`{}`), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chtimes(p, f.closedOutLongAgo, f.closedOutLongAgo); err != nil {
		f.t.Fatal(err)
	}
}

func (f *enforceSafetyFixture) write(path, body string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *enforceSafetyFixture) commitIn(dir, name, body string) string {
	f.t.Helper()
	f.write(filepath.Join(dir, name), body)
	gcGit(f.t, dir, "add", name)
	gcGit(f.t, dir, "commit", "-q", "-m", "work on "+name)
	return strings.TrimSpace(gcGit(f.t, dir, "rev-parse", "HEAD"))
}

func (f *enforceSafetyFixture) addWorktree(dir, branch string) {
	f.t.Helper()
	gcGit(f.t, f.projectRoot, "worktree", "add", "-q", "-b", branch, dir, "main")
}

func newEnforceSafetyFixture(t *testing.T, gcPol gc.Policy) *enforceSafetyFixture {
	t.Helper()
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("GOCACHE", t.TempDir())
	repo := gittest.Fixture(t)
	hub := filepath.Dir(repo.Dir)
	f := &enforceSafetyFixture{t: t, projectRoot: repo.Dir, workspace: t.TempDir(), closedOutLongAgo: time.Now().Add(-72 * time.Hour)}
	f.evolveDir = filepath.Join(f.projectRoot, ".evolve")
	if err := os.MkdirAll(filepath.Join(f.evolveDir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.commitIn(f.projectRoot, "seed.txt", "seed\n")
	raw, err := json.Marshal(map[string]gc.Policy{"gc": gcPol})
	if err != nil {
		t.Fatal(err)
	}
	f.write(filepath.Join(f.evolveDir, "policy.json"), string(raw))

	f.devUnlanded = filepath.Join(hub, "dev", "cycle-console-77")
	f.addWorktree(f.devUnlanded, "cycle-console-77")
	f.devUnlandedHead = f.commitIn(f.devUnlanded, "console.txt", "console work\n")
	f.unlandedEdit = filepath.Join(f.devUnlanded, "seed.txt")
	f.write(f.unlandedEdit, "uncommitted console edit\n")
	f.closeOut(77)

	f.devMerged = filepath.Join(hub, "dev", "cycle-console-78")
	f.addWorktree(f.devMerged, "cycle-console-78")
	f.devHead = strings.TrimSpace(gcGit(t, f.devMerged, "rev-parse", "HEAD"))
	f.closeOut(78)

	f.liveLane = filepath.Join(f.evolveDir, "worktrees", "cycle-abc-501")
	f.addWorktree(f.liveLane, "cycle-abc-501")
	f.liveLaneHead = f.commitIn(f.liveLane, "lane.txt", "lane work\n")
	f.laneEdit = filepath.Join(f.liveLane, "seed.txt")
	f.write(f.laneEdit, "in-flight lane edit\n")
	f.closeOut(501)
	runDir := filepath.Join(f.evolveDir, "runs", "cycle-501")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "cycle-501", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	f.unmergedBranch = "cycle-abc-600"
	gcGit(t, f.projectRoot, "switch", "-q", "-c", f.unmergedBranch)
	f.unmergedBranchHead = f.commitIn(f.projectRoot, "orphan.txt", "a failed cycle's work\n")
	gcGit(t, f.projectRoot, "switch", "-q", "main")

	f.snapshotLane = filepath.Join(f.evolveDir, "worktrees", "cycle-abc-502")
	f.addWorktree(f.snapshotLane, "cycle-abc-502")
	base := strings.TrimSpace(gcGit(t, f.snapshotLane, "rev-parse", "HEAD"))
	f.snapshotSHA = f.commitIn(f.snapshotLane, "resume.txt", "preserved work to resume\n")
	f.write(filepath.Join(f.snapshotLane, "after-snapshot.txt"), "written after the snapshot\n")
	f.closeOut(502)
	if err := continuation.WriteRegistryEntry(f.projectRoot, "todo-resume", continuation.Continuation{
		Worktree: f.snapshotLane, Branch: "cycle-abc-502", SnapshotSHA: f.snapshotSHA, BaseSHA: base, Cycle: 502,
	}); err != nil {
		t.Fatal(err)
	}
	f.checkpoints = map[string]string{}
	for _, wt := range []string{f.snapshotLane, f.devUnlanded, f.liveLane} {
		f.checkpoint(wt)
	}
	f.registry = continuation.RegistryPath(f.projectRoot)
	if f.registryBefore, err = os.ReadFile(f.registry); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *enforceSafetyFixture) checkpoint(worktree string) {
	f.t.Helper()
	tree := strings.TrimSpace(gcGit(f.t, worktree, "write-tree"))
	head := strings.TrimSpace(gcGit(f.t, worktree, "rev-parse", "HEAD"))
	full := strings.TrimSpace(gcGit(f.t, worktree, "commit-tree", tree, "-p", head, "-m", "checkpoint"))
	ref := "refs/checkpoints/" + filepath.Base(worktree) + "/20261006T000000Z"
	gcGit(f.t, f.projectRoot, "update-ref", ref, full)
	f.checkpoints[ref] = full
}

func (f *enforceSafetyFixture) branchHead(branch string) string {
	f.t.Helper()
	cmd := exec.Command("git", "-C", f.projectRoot, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (f *enforceSafetyFixture) gitOK(args ...string) bool {
	return exec.Command("git", append([]string{"-C", f.projectRoot}, args...)...).Run() == nil
}

func (f *enforceSafetyFixture) readFile(path string) string {
	f.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(b)
}

func TestRunGCHook_TheCheckedInEnforcePolicyNeverTouchesADevWorktreeALiveLaneAnUnmergedBranchOrAContinuationSnapshot(t *testing.T) {
	gcPol := checkedInGCPolicy(t)
	if gcPol.Worktrees.SalvageAfterHours <= 0 || gcPol.Worktrees.SalvageAfterHours >= 72 {
		t.Fatalf("checked-in gc.worktrees.salvage_after_hours = %d: this fixture closes cycles out 72h ago so the salvage-remove is live", gcPol.Worktrees.SalvageAfterHours)
	}
	f := newEnforceSafetyFixture(t, gcPol)

	var stderr bytes.Buffer
	runGCHook(loopConfig{EvolveDir: f.evolveDir, ProjectRoot: f.projectRoot}, f.workspace, &stderr)

	if !strings.Contains(stderr.String(), "[gc] worktree enforce: applied") {
		t.Fatalf("the hook did not run its worktree sweep in enforce:\n%s", stderr.String())
	}
	if got := f.readFile(f.unlandedEdit); got != "uncommitted console edit\n" {
		t.Errorf("dev worktree %s lost its uncommitted edit: %q", f.devUnlanded, got)
	}
	if got := f.branchHead("cycle-console-77"); got != f.devUnlandedHead {
		t.Errorf("dev branch cycle-console-77 = %q, want its unlanded head %s", got, f.devUnlandedHead)
	}
	if _, err := os.Stat(filepath.Join(f.devMerged, "seed.txt")); err != nil {
		t.Errorf("merged, clean dev worktree %s was removed: %v", f.devMerged, err)
	}
	if got := f.branchHead("cycle-console-78"); got != f.devHead {
		t.Errorf("a dev worktree's checked-out cycle-* branch was deleted: cycle-console-78 = %q, want %s", got, f.devHead)
	}
	if got := f.readFile(f.laneEdit); got != "in-flight lane edit\n" {
		t.Errorf("live lane %s lost its in-flight edit: %q", f.liveLane, got)
	}
	if got := f.branchHead("cycle-abc-501"); got != f.liveLaneHead {
		t.Errorf("live lane branch cycle-abc-501 = %q, want %s", got, f.liveLaneHead)
	}
	if got := f.branchHead(f.unmergedBranch); got != f.unmergedBranchHead {
		t.Errorf("unmerged branch %s = %q, want %s", f.unmergedBranch, got, f.unmergedBranchHead)
	}
	if !f.gitOK("cat-file", "-e", f.snapshotSHA+"^{commit}") || !f.gitOK("merge-base", "--is-ancestor", f.snapshotSHA, "refs/heads/cycle-abc-502") {
		t.Errorf("continuation snapshot %s is no longer a commit on the kept branch cycle-abc-502", f.snapshotSHA)
	}
	if after, err := os.ReadFile(f.registry); err != nil || !bytes.Equal(after, f.registryBefore) {
		t.Errorf("the continuation registry changed (err=%v):\nbefore=%s\nafter=%s", err, f.registryBefore, after)
	}
	for ref, want := range f.checkpoints {
		got, err := exec.Command("git", "-C", f.projectRoot, "rev-parse", "--verify", "--quiet", ref).Output()
		if err != nil || strings.TrimSpace(string(got)) != want || !f.gitOK("cat-file", "-e", want+"^{commit}") {
			t.Errorf("checkpoint %s = %q (err=%v), want %s with its commit still in the store", ref, strings.TrimSpace(string(got)), err, want)
		}
	}
	salvaged := filepath.Join(gc.OperatorSalvageDir(f.evolveDir), filepath.Base(f.snapshotLane))
	if _, err := os.Stat(filepath.Join(salvaged, "untracked.tgz")); err != nil {
		t.Errorf("the finished snapshot lane's post-snapshot file was not salvaged before its tree was removed: %v", err)
	}
	if _, err := os.Stat(f.snapshotLane); !os.IsNotExist(err) {
		t.Errorf("control: the finished, unmerged snapshot lane past its salvage age must be salvaged and removed under enforce, stat err=%v\n%s", err, stderr.String())
	}
}
