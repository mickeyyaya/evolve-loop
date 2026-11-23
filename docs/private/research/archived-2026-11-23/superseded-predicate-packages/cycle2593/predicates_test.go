//go:build acs

package cycle2593

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	laneCycle       = 2701
	otherCycle      = 2702
	uncommittedLine = "uncommitted edit of the ended lane"
	untrackedBody   = "untracked\n"
	archiveRefRoot  = "refs/archive/cycles/"
	manifestFile    = "manifest.json"
	manifestHash    = "manifest.sha256"
)

var evolveBuild struct {
	once sync.Once
	dir  string
	path string
	fail string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle2593: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	evolveBuild.once.Do(func() {
		dir, err := os.MkdirTemp("", "cycle2593-evolve-")
		if err != nil {
			evolveBuild.fail = err.Error()
			return
		}
		evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
		_, stderr, code, runErr := acsassert.SubprocessOutput("go", "build", "-C", acsassert.GoModuleDir(t), "-o", evolveBuild.path, "./cmd/evolve")
		if runErr != nil || code != 0 {
			evolveBuild.fail = fmt.Sprintf("exit %d: %v\n%s", code, runErr, stderr)
		}
	})
	if evolveBuild.fail != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.fail)
	}
	return evolveBuild.path
}

type lane struct {
	t      *testing.T
	repo   *gittest.Repo
	cycle  int
	leaf   string
	path   string
	branch string
	tip    string
	opts   gc.WorktreeOptions
}

func newLane(t *testing.T, cycle int) *lane {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("f.txt", "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := filepath.Join(repo.Dir, ".evolve", "worktrees")
	leaf := fmt.Sprintf("cycle-cd3ae73e-%d", cycle)
	l := &lane{t: t, repo: repo, cycle: cycle, leaf: leaf, path: filepath.Join(base, leaf), branch: leaf}
	repo.Git("worktree", "add", "-q", l.path, "-b", l.branch)
	repo.Write(filepath.Join(".evolve", "worktrees", leaf, "f.txt"), "base\nlane edit\n")
	repo.Git("-C", l.path, "commit", "-q", "-am", "lane work")
	l.tip = repo.Git("rev-parse", l.branch)
	repo.Write(filepath.Join(".evolve", "worktrees", leaf, "f.txt"), "base\nlane edit\n"+uncommittedLine+"\n")
	repo.Write(filepath.Join(".evolve", "worktrees", leaf, "sub", "new.txt"), untrackedBody)
	if err := os.Symlink("new.txt", filepath.Join(l.path, "sub", "link")); err != nil {
		t.Fatal(err)
	}
	aged := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(l.path, aged, aged); err != nil {
		t.Fatal(err)
	}
	l.opts = gc.WorktreeOptions{
		ProjectRoot:  repo.Dir,
		WorktreeBase: base,
		EvolveDir:    filepath.Join(repo.Dir, ".evolve"),
		Policy:       gc.WorktreesPolicy{SalvageAfterHours: 24},
		Exec:         sysexec.DefaultRunner,
		PidAlive:     func(int) bool { return true },
		LeaseTTL:     time.Hour,
	}
	return l
}

func (l *lane) plan() gc.WorktreeManifest {
	l.t.Helper()
	m, err := gc.PlanWorktrees(l.opts)
	if err != nil {
		l.t.Fatalf("PlanWorktrees: %v", err)
	}
	return m
}

func (l *lane) plannedItem(m gc.WorktreeManifest) gc.WorktreeItem {
	l.t.Helper()
	for _, it := range m.Items {
		if it.Path == l.path {
			if it.Action != gc.WorktreeActionSalvageRemove {
				l.t.Fatalf("fixture error: lane planned %q, want %q", it.Action, gc.WorktreeActionSalvageRemove)
			}
			return it
		}
	}
	l.t.Fatalf("fixture error: the plan holds no item for %s", l.path)
	return gc.WorktreeItem{}
}

func (l *lane) salvage() {
	l.t.Helper()
	m := l.plan()
	l.plannedItem(m)
	if err := gc.ApplyWorktrees(l.opts, m); err != nil {
		l.t.Fatalf("ApplyWorktrees: %v", err)
	}
}

func (l *lane) leafDir() string {
	return filepath.Join(gc.OperatorSalvageDir(l.opts.EvolveDir), l.leaf)
}

func (l *lane) archiveRef() string { return archiveRefRoot + l.leaf }

func (l *lane) refValue(ref string) (string, bool) {
	stdout, _, code, _ := acsassert.SubprocessOutput("git", "-C", l.repo.Dir, "rev-parse", "--verify", "--quiet", ref)
	return strings.TrimSpace(stdout), code == 0
}

func (l *lane) requireBranchMovedToArchive() {
	l.t.Helper()
	if _, ok := l.refValue("refs/heads/" + l.branch); ok {
		l.t.Fatalf("RED: branch %s survived the salvage: a failed cycle moves its branch to %s", l.branch, l.archiveRef())
	}
}

func (l *lane) restore(cycle int, worktree string) (stdout, stderr string, code int) {
	l.t.Helper()
	stdout, stderr, code, _ = acsassert.SubprocessOutput(evolveBinary(l.t), "archive", "restore", fmt.Sprint(cycle),
		"--project-root", l.repo.Dir, "--worktree", worktree)
	return stdout, stderr, code
}

func (l *lane) requireRestoredRoundTrip(worktree string) {
	l.t.Helper()
	if got, ok := l.refValue("refs/heads/" + l.branch); !ok || got != l.tip {
		l.t.Errorf("RED: restored branch %s = %q (exists=%v), want the archived tip %s", l.branch, got, ok, l.tip)
	}
	if got := l.repo.Git("-C", worktree, "symbolic-ref", "--short", "HEAD"); got != l.branch {
		l.t.Errorf("RED: restored worktree is on %q, want branch %q", got, l.branch)
	}
	if got := l.repo.Git("-C", worktree, "rev-parse", "HEAD"); got != l.tip {
		l.t.Errorf("RED: restored worktree HEAD = %s, want %s", got, l.tip)
	}
	body, err := os.ReadFile(filepath.Join(worktree, "f.txt"))
	if err != nil || string(body) != "base\nlane edit\n"+uncommittedLine+"\n" {
		l.t.Errorf("RED: restored f.txt = %q (%v), want the committed edit plus the uncommitted edit", body, err)
	}
	untracked, err := os.ReadFile(filepath.Join(worktree, "sub", "new.txt"))
	if err != nil || string(untracked) != untrackedBody {
		l.t.Errorf("RED: restored sub/new.txt = %q (%v), want %q", untracked, err, untrackedBody)
	}
	if target, err := os.Readlink(filepath.Join(worktree, "sub", "link")); err != nil || target != "new.txt" {
		l.t.Errorf("RED: restored sub/link target = %q (%v), want the link itself: new.txt", target, err)
	}
	status := l.repo.Git("-C", worktree, "status", "--porcelain")
	if !strings.Contains(status, " M f.txt") || !strings.Contains(status, "?? sub/") {
		l.t.Errorf("RED: restored status %q, want f.txt modified and sub/ untracked: the patch and the untracked files stay uncommitted", status)
	}
}

func requireRefusal(t *testing.T, stdout, stderr string, code int, subject string) {
	t.Helper()
	if code == 0 {
		t.Errorf("RED: restore exited 0, want a refusal naming %q\n%s%s", subject, stdout, stderr)
		return
	}
	if strings.Contains(stderr, "unknown command") {
		first, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
		t.Errorf("RED: the archive verb does not exist: %s", first)
		return
	}
	if !strings.Contains(stderr, subject) {
		first, _, _ := strings.Cut(strings.TrimSpace(stderr), "\n")
		t.Errorf("RED: the refusal does not name %q: %s", subject, first)
	}
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestC2593_001_ARoundTripThroughTheArchiveRefRestoresBranchPatchAndUntrackedFiles(t *testing.T) {
	l := newLane(t, laneCycle)
	l.salvage()

	if _, ok := l.refValue("refs/heads/" + l.branch); ok {
		t.Errorf("RED: branch %s survived the salvage: a failed cycle moves its branch to %s", l.branch, l.archiveRef())
	}
	if got, ok := l.refValue(l.archiveRef()); !ok || got != l.tip {
		t.Fatalf("RED: %s = %q (exists=%v), want the lane tip %s: the ref is the one copy of the commits", l.archiveRef(), got, ok, l.tip)
	}
	if _, err := os.Stat(l.path); err == nil {
		t.Errorf("RED: the lane worktree %s is still on disk after the salvage", l.path)
	}
	restored := filepath.Join(t.TempDir(), "restored")

	stdout, stderr, code := l.restore(l.cycle, restored)

	if code != 0 {
		t.Fatalf("RED: evolve archive restore exit %d\n--- stdout ---\n%s--- stderr ---\n%s", code, stdout, stderr)
	}
	l.requireRestoredRoundTrip(restored)
}

func TestC2593_002_ARoundTripThroughTheLeafBundleRestoresWhenTheArchiveRefAndItsObjectsAreGone(t *testing.T) {
	l := newLane(t, laneCycle)
	l.salvage()
	l.requireBranchMovedToArchive()
	if _, err := os.Stat(filepath.Join(l.leafDir(), "refs.bundle")); err != nil {
		t.Fatalf("RED: the leaf holds no refs.bundle to restore from: %v", err)
	}
	l.repo.Git("update-ref", "-d", l.archiveRef())
	l.repo.Git("reflog", "expire", "--expire=now", "--all")
	l.repo.Git("gc", "-q", "--prune=now")
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", l.repo.Dir, "cat-file", "-e", l.tip+"^{commit}"); code == 0 {
		t.Fatalf("fixture error: commit %s is still in the object store, the bundle would not be what restores it", l.tip)
	}
	restored := filepath.Join(t.TempDir(), "restored")

	stdout, stderr, code := l.restore(l.cycle, restored)

	if code != 0 {
		t.Fatalf("RED: evolve archive restore exit %d with the ref gone\n--- stdout ---\n%s--- stderr ---\n%s", code, stdout, stderr)
	}
	l.requireRestoredRoundTrip(restored)
}

func TestC2593_003_RestoreOfACycleWithNoLeafFailsAndCreatesNothing(t *testing.T) {
	l := newLane(t, laneCycle)
	l.salvage()
	restored := filepath.Join(t.TempDir(), "restored")

	stdout, stderr, code := l.restore(otherCycle, restored)

	requireRefusal(t, stdout, stderr, code, fmt.Sprint(otherCycle))
	if _, err := os.Stat(restored); err == nil {
		t.Errorf("RED: restore of an unknown cycle created %s", restored)
	}
	if _, ok := l.refValue("refs/heads/" + l.branch); ok {
		t.Errorf("RED: restore of an unknown cycle created branch %s", l.branch)
	}
}

func TestC2593_004_RestoreNeverOverwritesABranchThatAlreadyExists(t *testing.T) {
	l := newLane(t, laneCycle)
	baseTip := l.repo.Git("rev-parse", "main")
	l.salvage()
	l.requireBranchMovedToArchive()
	l.repo.Git("branch", l.branch, baseTip)
	restored := filepath.Join(t.TempDir(), "restored")

	stdout, stderr, code := l.restore(l.cycle, restored)

	requireRefusal(t, stdout, stderr, code, l.branch)
	if got, _ := l.refValue("refs/heads/" + l.branch); got != baseTip {
		t.Errorf("RED: branch %s = %q after a refused restore, want it untouched at %s", l.branch, got, baseTip)
	}
	if _, err := os.Stat(restored); err == nil {
		t.Errorf("RED: a refused restore still created %s", restored)
	}
}

func TestC2593_005_ABranchThatMovedAfterTheScanIsNotDeleted(t *testing.T) {
	l := newLane(t, laneCycle)
	scanned := l.plannedItem(l.plan())
	l.repo.Git("-C", l.path, "commit", "--allow-empty", "-q", "-m", "moved after the scan")
	moved := l.repo.Git("rev-parse", l.branch)
	if moved == l.tip {
		t.Fatalf("fixture error: the branch did not move off %s", l.tip)
	}
	stale := gc.WorktreeItem{Path: l.path, Branch: l.branch, Head: l.tip, Action: scanned.Action, Reason: scanned.Reason}

	err := gc.ApplyWorktrees(l.opts, gc.WorktreeManifest{Items: []gc.WorktreeItem{stale}})

	if err == nil {
		t.Errorf("RED: ApplyWorktrees returned nil although %s moved from the scanned %s to %s", l.branch, l.tip, moved)
	}
	if got, ok := l.refValue("refs/heads/" + l.branch); !ok || got != moved {
		t.Errorf("RED: branch %s = %q (exists=%v), want it kept at the moved tip %s: compare-and-delete must refuse a moved ref", l.branch, got, ok, moved)
	}
}

func TestC2593_006_ThePlanRecordsTheScannedTipOfASalvageItemForTheCompareAndDelete(t *testing.T) {
	l := newLane(t, laneCycle)

	item := l.plannedItem(l.plan())

	if item.Head != l.tip {
		t.Errorf("RED: the salvage-remove item carries Head %q, want the scanned tip %s: the delete needs an expected value", item.Head, l.tip)
	}
}

type leafManifest struct {
	Cycle  int               `json:"cycle"`
	Branch string            `json:"branch"`
	Head   string            `json:"head"`
	Ref    string            `json:"ref"`
	Files  map[string]string `json:"files"`
}

func TestC2593_007_TheLeafManifestHashVerifiesItsFilesAndItself(t *testing.T) {
	l := newLane(t, laneCycle)
	l.salvage()
	raw, err := os.ReadFile(filepath.Join(l.leafDir(), manifestFile))
	if err != nil {
		t.Fatalf("RED: the leaf holds no %s: %v", manifestFile, err)
	}
	var m leafManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("RED: %s is not JSON: %v\n%s", manifestFile, err, raw)
	}

	if m.Head != l.tip || m.Branch != l.branch || m.Ref != l.archiveRef() || m.Cycle != l.cycle {
		t.Errorf("RED: manifest identity = {cycle %d, branch %q, head %q, ref %q}, want {%d, %q, %q, %q}",
			m.Cycle, m.Branch, m.Head, m.Ref, l.cycle, l.branch, l.tip, l.archiveRef())
	}
	for _, name := range []string{"HEAD", "uncommitted.patch", "untracked.tgz"} {
		want, listed := m.Files[name]
		if !listed {
			t.Errorf("RED: manifest files lists no entry for %s", name)
			continue
		}
		if got := fileSHA256(t, filepath.Join(l.leafDir(), name)); got != want {
			t.Errorf("RED: manifest sha256 of %s = %s, the file hashes to %s", name, want, got)
		}
	}
	recorded, err := os.ReadFile(filepath.Join(l.leafDir(), manifestHash))
	if err != nil {
		t.Fatalf("RED: the leaf holds no %s: %v", manifestHash, err)
	}
	if got := fileSHA256(t, filepath.Join(l.leafDir(), manifestFile)); strings.TrimSpace(string(recorded)) != got {
		t.Errorf("RED: %s = %q, the manifest hashes to %s", manifestHash, strings.TrimSpace(string(recorded)), got)
	}
}

func TestC2593_008_RestoreRefusesALeafWhoseFileNoLongerMatchesTheManifest(t *testing.T) {
	l := newLane(t, laneCycle)
	l.salvage()
	patch := filepath.Join(l.leafDir(), "uncommitted.patch")
	body, err := os.ReadFile(patch)
	if err != nil {
		t.Fatalf("fixture error: %v", err)
	}
	if err := os.WriteFile(patch, append(body, []byte("\n# tampered\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(t.TempDir(), "restored")

	stdout, stderr, code := l.restore(l.cycle, restored)

	requireRefusal(t, stdout, stderr, code, "uncommitted.patch")
	if _, err := os.Stat(restored); err == nil {
		t.Errorf("RED: a refused restore still created %s", restored)
	}
	if _, ok := l.refValue("refs/heads/" + l.branch); ok {
		t.Errorf("RED: a refused restore still created branch %s", l.branch)
	}
}

func TestC2593_009_RestoreRefusesAManifestWhoseHashNoLongerMatches(t *testing.T) {
	l := newLane(t, laneCycle)
	l.salvage()
	manifest := filepath.Join(l.leafDir(), manifestFile)
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("RED: the leaf holds no %s: %v", manifestFile, err)
	}
	if err := os.WriteFile(manifest, append(body, '\n', ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(t.TempDir(), "restored")

	stdout, stderr, code := l.restore(l.cycle, restored)

	requireRefusal(t, stdout, stderr, code, manifestFile)
	if _, err := os.Stat(restored); err == nil {
		t.Errorf("RED: a refused restore still created %s", restored)
	}
}
