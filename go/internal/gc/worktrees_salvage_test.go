package gc

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func closeOutCycle(t *testing.T, projectRoot string, cycle int, at time.Time) {
	t.Helper()
	p := filepath.Join(dossier.CyclesDir(projectRoot), fmt.Sprintf("cycle-%d.json", cycle))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

func actionFor(items []WorktreeItem, path string) WorktreeAction {
	for _, it := range items {
		if it.Path == path {
			return it.Action
		}
	}
	return ""
}

func TestPlanWorktrees_AFinishedDirtyOrUnmergedTreePastTheSalvageAgeIsSalvageRemoved(t *testing.T) {
	e := newWorktreesTestEnv(t)
	unmerged := e.addWorktree("cycle-cd3ae73e-1677", "cycle-cd3ae73e-1677", 48*time.Hour, false, false)
	dirty := e.addWorktree("cycle-cd3ae73e-1686", "cycle-cd3ae73e-1686", 48*time.Hour, true, true)
	young := e.addWorktree("cycle-cd3ae73e-1761", "cycle-cd3ae73e-1761", 48*time.Hour, false, false)
	open := e.addWorktree("cycle-cd3ae73e-1762", "cycle-cd3ae73e-1762", 48*time.Hour, true, false)
	leased := e.addWorktree("cycle-cd3ae73e-1699", "cycle-cd3ae73e-1699", 48*time.Hour, true, false)
	for _, c := range []int{1677, 1686, 1699} {
		closeOutCycle(t, e.projectRoot, c, e.now.Add(-30*time.Hour))
	}
	closeOutCycle(t, e.projectRoot, 1761, e.now.Add(-2*time.Hour))
	e.writeLease(1699, runlease.Lease{RunID: "cycle-1699"}, time.Minute)
	o := e.opts()
	o.Policy.SalvageAfterHours = 24

	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]WorktreeAction{
		unmerged: WorktreeActionSalvageRemove,
		dirty:    WorktreeActionSalvageRemove,
		young:    WorktreeActionFlagUnmerged,
		open:     WorktreeActionFlagDirty,
		leased:   "",
	} {
		if got := actionFor(m.Items, path); got != want {
			t.Errorf("%s: action %q, want %q", filepath.Base(path), got, want)
		}
	}
}

func TestPlanWorktrees_ZeroSalvageAgeKeepsFinishedTreesFlaggedOnly(t *testing.T) {
	e := newWorktreesTestEnv(t)
	unmerged := e.addWorktree("cycle-cd3ae73e-1677", "cycle-cd3ae73e-1677", 48*time.Hour, false, false)
	closeOutCycle(t, e.projectRoot, 1677, e.now.Add(-300*time.Hour))

	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatal(err)
	}

	if got := actionFor(m.Items, unmerged); got != WorktreeActionFlagUnmerged {
		t.Errorf("action %q, want %q: salvage_after_hours 0 means never", got, WorktreeActionFlagUnmerged)
	}
}

func TestApplyWorktrees_ASalvageTargetThatTurnedLiveIsRefusedUntouched(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-cd3ae73e-1677", "cycle-cd3ae73e-1677", 48*time.Hour, true, false)
	closeOutCycle(t, e.projectRoot, 1677, e.now.Add(-30*time.Hour))
	o := e.opts()
	o.Policy.SalvageAfterHours = 24
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	e.writeLease(1677, runlease.Lease{RunID: "cycle-1677"}, time.Minute)

	err = ApplyWorktrees(o, m)

	if err == nil || !strings.Contains(err.Error(), "became live") {
		t.Errorf("ApplyWorktrees err = %v, want a became-live refusal", err)
	}
	if n := e.git.callCount("worktree remove"); n != 0 {
		t.Errorf("%d worktree removals ran for %s after it turned live", n, filepath.Base(wt))
	}
}

func TestApplyWorktrees_SalvageRemoveKeepsARestorableCopyAndTheBranch(t *testing.T) {
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := filepath.Join(repo.Dir, ".evolve", "worktrees")
	wt := filepath.Join(base, "cycle-cd3ae73e-1677")
	repo.Git("worktree", "add", "-q", wt, "-b", "cycle-cd3ae73e-1677")
	writeFile(t, filepath.Join(wt, "f.txt"), "base\nlane edit\n")
	repo.Git("-C", wt, "commit", "-q", "-am", "lane work")
	writeFile(t, filepath.Join(wt, "f.txt"), "base\nlane edit\nuncommitted edit\n")
	writeFile(t, filepath.Join(wt, "sub", "new.txt"), "untracked\n")
	tip := repo.Git("rev-parse", "cycle-cd3ae73e-1677")
	now := time.Now()
	closeOutCycle(t, repo.Dir, 1677, now.Add(-30*time.Hour))
	o := WorktreeOptions{ProjectRoot: repo.Dir, WorktreeBase: base, EvolveDir: filepath.Join(repo.Dir, ".evolve"),
		Policy: WorktreesPolicy{SalvageAfterHours: 24}, Now: func() time.Time { return now }, Exec: sysexec.DefaultRunner}

	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	if got := actionFor(m.Items, wt); got != WorktreeActionSalvageRemove {
		t.Fatalf("planned %q, want %q", got, WorktreeActionSalvageRemove)
	}
	if err := ApplyWorktrees(o, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}

	if _, err := os.Stat(wt); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree still on disk: %v", err)
	}
	if got := repo.Git("rev-parse", "cycle-cd3ae73e-1677"); got != tip {
		t.Fatalf("branch tip %q, want %q: the lane's commits must survive on the kept branch", got, tip)
	}
	salvage := filepath.Join(o.EvolveDir, "operator-salvage", "cycle-cd3ae73e-1677")
	if head := readFile(t, filepath.Join(salvage, "HEAD")); !strings.HasPrefix(head, tip) {
		t.Errorf("salvage HEAD %q does not record the tip %s", head, tip)
	}
	restore := filepath.Join(t.TempDir(), "restore")
	repo.Git("worktree", "add", "-q", "--detach", restore, tip)
	repo.Git("-C", restore, "apply", filepath.Join(salvage, "uncommitted.patch"))
	if got := readFile(t, filepath.Join(restore, "f.txt")); got != "base\nlane edit\nuncommitted edit\n" {
		t.Errorf("patch restores f.txt as %q", got)
	}
	if got := untarFile(t, filepath.Join(salvage, "untracked.tgz"), "sub/new.txt"); got != "untracked\n" {
		t.Errorf("untracked archive holds sub/new.txt = %q", got)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func untarFile(t *testing.T, archive, name string) string {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == name {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
}

func TestApplyWorktrees_AFailedSalvageKeepsTheTree(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-cd3ae73e-1677", "cycle-cd3ae73e-1677", 48*time.Hour, true, false)
	closeOutCycle(t, e.projectRoot, 1677, e.now.Add(-30*time.Hour))
	writeFile(t, filepath.Join(e.evolveDir, "operator-salvage"), "a file where the salvage dir must go")
	o := e.opts()
	o.Policy.SalvageAfterHours = 24
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}

	err = ApplyWorktrees(o, m)

	if err == nil || !strings.Contains(err.Error(), "salvage failed") {
		t.Errorf("ApplyWorktrees err = %v, want the salvage failure named", err)
	}
	if n := e.git.callCount("worktree remove"); n != 0 {
		t.Errorf("%d worktree removals ran for %s although its salvage failed", n, filepath.Base(wt))
	}
}
