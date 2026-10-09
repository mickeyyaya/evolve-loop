package gc

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type worktreeFixture struct {
	path     string
	branch   string
	detached bool
}

type scriptedGit struct {
	mu sync.Mutex

	porcelain       string
	mergedBranches  map[string]bool
	backlogBranches []string
	dirtyDirs       map[string]bool

	calls []string
}

func newScriptedGit() *scriptedGit {
	return &scriptedGit{mergedBranches: map[string]bool{}, dirtyDirs: map[string]bool{}}
}

func (g *scriptedGit) run(_ context.Context, name, dir string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	g.mu.Lock()
	g.calls = append(g.calls, strings.TrimSpace(dir+"|"+name+" "+strings.Join(args, " ")))
	g.mu.Unlock()
	if name != "git" || len(args) == 0 {
		return 0, nil
	}
	switch args[0] {
	case "worktree":
		if len(args) >= 2 && args[1] == "list" {
			io.WriteString(stdout, g.porcelain)
		}
	case "branch":
		if len(args) >= 2 && args[1] == "--merged" {
			for b, merged := range g.mergedBranches {
				if merged {
					io.WriteString(stdout, "  "+b+"\n")
				}
			}
		}
		if len(args) >= 2 && args[1] == "--list" {
			for _, b := range g.backlogBranches {
				io.WriteString(stdout, "  "+b+"\n")
			}
		}
	case "status":
		if g.dirtyDirs[dir] {
			io.WriteString(stdout, " M some-file.go\n")
		}
	}
	return 0, nil
}

func (g *scriptedGit) callCount(substr string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.calls {
		if strings.Contains(c, substr) {
			n++
		}
	}
	return n
}

func buildPorcelain(entries []worktreeFixture) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "worktree %s\n", e.path)
		fmt.Fprintf(&b, "HEAD 0000000000000000000000000000000000000000\n")
		if e.detached || e.branch == "" {
			b.WriteString("detached\n")
		} else {
			fmt.Fprintf(&b, "branch refs/heads/%s\n", e.branch)
		}
		b.WriteString("\n")
	}
	return b.String()
}

type worktreesTestEnv struct {
	t            *testing.T
	projectRoot  string
	worktreeBase string
	evolveDir    string
	git          *scriptedGit
	now          time.Time
}

func newWorktreesTestEnv(t *testing.T) *worktreesTestEnv {
	t.Helper()
	root := t.TempDir()
	e := &worktreesTestEnv{
		t:            t,
		projectRoot:  root,
		worktreeBase: filepath.Join(root, ".evolve", "worktrees"),
		evolveDir:    filepath.Join(root, ".evolve"),
		git:          newScriptedGit(),
		now:          time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
	}
	if err := os.MkdirAll(e.worktreeBase, 0o755); err != nil {
		t.Fatalf("mkdir worktree base: %v", err)
	}
	if err := os.MkdirAll(e.evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir evolve dir: %v", err)
	}
	return e
}

func (e *worktreesTestEnv) addWorktree(leaf, branch string, age time.Duration, dirty, merged bool) string {
	e.t.Helper()
	path := filepath.Join(e.worktreeBase, leaf)
	if err := os.MkdirAll(path, 0o755); err != nil {
		e.t.Fatalf("mkdir worktree %s: %v", leaf, err)
	}
	mtime := e.now.Add(-age)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		e.t.Fatalf("chtimes %s: %v", leaf, err)
	}
	entries := parsePorcelainForTest(e.git.porcelain)
	entries = append(entries, worktreeFixture{path: path, branch: branch})
	e.git.porcelain = buildPorcelain(entries)
	e.git.mergedBranches[branch] = merged
	e.git.dirtyDirs[path] = dirty
	return path
}

func parsePorcelainForTest(porcelain string) []worktreeFixture {
	var out []worktreeFixture
	var cur worktreeFixture
	for _, line := range strings.Split(porcelain, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = worktreeFixture{path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "" && cur.path != "":
			out = append(out, cur)
			cur = worktreeFixture{}
		}
	}
	return out
}

func (e *worktreesTestEnv) writeLease(n int, l runlease.Lease, heartbeatAge time.Duration) {
	e.t.Helper()
	dir := filepath.Join(e.evolveDir, "runs", fmt.Sprintf("cycle-%d", n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatalf("mkdir run dir: %v", err)
	}
	if err := runlease.Write(dir, l, e.now.Add(-heartbeatAge)); err != nil {
		e.t.Fatalf("write lease: %v", err)
	}
}

func (e *worktreesTestEnv) opts() WorktreeOptions {
	return WorktreeOptions{
		ProjectRoot:  e.projectRoot,
		WorktreeBase: e.worktreeBase,
		EvolveDir:    e.evolveDir,
		Policy:       WorktreesPolicy{KeepRecent: 0, MinAgeMinutes: 0},
		Now:          func() time.Time { return e.now },
		Exec:         e.git.run,
		LeaseTTL:     runlease.DefaultTTL,
	}
}

func itemsWithAction(items []WorktreeItem, action WorktreeAction) []WorktreeItem {
	var out []WorktreeItem
	for _, it := range items {
		if it.Action == action {
			out = append(out, it)
		}
	}
	return out
}

func TestPlanWorktrees_MergedCleanDeadIsCollected(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-aaa1111-570", "cycle-aaa1111-570", 20*time.Hour, false, true)

	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}

	removes := itemsWithAction(m.Items, WorktreeActionRemove)
	deletes := itemsWithAction(m.Items, WorktreeActionDeleteBranch)
	if len(removes) != 1 || removes[0].Path != wt {
		t.Fatalf("want exactly one remove-worktree item for %s, got %+v", wt, m.Items)
	}
	if len(deletes) != 1 || deletes[0].Branch != "cycle-aaa1111-570" {
		t.Fatalf("want exactly one delete-branch item for cycle-aaa1111-570, got %+v", m.Items)
	}
	if flagged := itemsWithAction(m.Items, WorktreeActionFlagDirty); len(flagged) != 0 {
		t.Errorf("a clean worktree must never be flagged dirty: %+v", flagged)
	}
}

func TestPlanWorktrees_DirtyIsFlaggedNeverRemoved(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-bbb2222-571", "cycle-bbb2222-571", 20*time.Hour, true, true)

	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}

	flagged := itemsWithAction(m.Items, WorktreeActionFlagDirty)
	if len(flagged) != 1 || flagged[0].Path != wt {
		t.Fatalf("want exactly one flag-dirty item for %s, got %+v", wt, m.Items)
	}
	if removes := itemsWithAction(m.Items, WorktreeActionRemove); len(removes) != 0 {
		t.Errorf("a dirty worktree must NEVER be removed (preserved ship-fail evidence): %+v", removes)
	}
	if deletes := itemsWithAction(m.Items, WorktreeActionDeleteBranch); len(deletes) != 0 {
		t.Errorf("a dirty worktree's branch must not be deleted either: %+v", deletes)
	}
}

func TestPlanWorktrees_UnmergedBranchKept(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-ccc3333-572", "cycle-ccc3333-572", 20*time.Hour, false, false)

	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}

	flagged := itemsWithAction(m.Items, WorktreeActionFlagUnmerged)
	if len(flagged) != 1 || flagged[0].Path != wt {
		t.Fatalf("want exactly one flag-unmerged item for %s, got %+v", wt, m.Items)
	}
	if removes := itemsWithAction(m.Items, WorktreeActionRemove); len(removes) != 0 {
		t.Errorf("an unmerged worktree must never be removed: %+v", removes)
	}
	if deletes := itemsWithAction(m.Items, WorktreeActionDeleteBranch); len(deletes) != 0 {
		t.Errorf("an unmerged branch must never be deleted (git's own merged-check is the safety net): %+v", deletes)
	}
}

func TestPlanWorktrees_LiveLeaseExcluded(t *testing.T) {
	e := newWorktreesTestEnv(t)
	e.addWorktree("cycle-ddd4444-573", "cycle-ddd4444-573", 20*time.Hour, false, true)
	e.writeLease(573, runlease.Lease{RunID: "run-573"}, 1*time.Minute)

	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if len(m.Items) != 0 {
		t.Fatalf("a worktree with a fresh lease must be entirely untouched: %+v", m.Items)
	}
}

func TestPlanWorktrees_DeadPidFreshLeaseCollected(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-eee5555-574", "cycle-eee5555-574", 20*time.Hour, false, true)
	e.writeLease(574, runlease.Lease{RunID: "run-574", OwnerPID: 4242}, 1*time.Minute)
	opts := e.opts()
	opts.PidAlive = func(pid int) bool { return pid != 4242 }

	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if len(itemsWithAction(m.Items, WorktreeActionRemove)) != 1 {
		t.Fatalf("a fresh-heartbeat-but-dead-pid worktree must be collected like any other dead one: %+v (path=%s)", m.Items, wt)
	}
}

func TestPlanWorktrees_KeepRecentAndMinAge(t *testing.T) {
	e := newWorktreesTestEnv(t)
	oldest := e.addWorktree("cycle-fff0001-601", "cycle-fff0001-601", 30*24*time.Hour, false, true)
	older := e.addWorktree("cycle-fff0002-602", "cycle-fff0002-602", 20*24*time.Hour, false, true)
	recent1 := e.addWorktree("cycle-fff0003-603", "cycle-fff0003-603", 10*24*time.Hour, false, true)
	recent2 := e.addWorktree("cycle-fff0004-604", "cycle-fff0004-604", 5*24*time.Hour, false, true)
	tooYoung := e.addWorktree("cycle-fff0005-605", "cycle-fff0005-605", 5*time.Minute, false, true)

	opts := e.opts()
	opts.Policy = WorktreesPolicy{KeepRecent: 2, MinAgeMinutes: 15}

	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	removedPaths := map[string]bool{}
	for _, it := range itemsWithAction(m.Items, WorktreeActionRemove) {
		removedPaths[it.Path] = true
	}
	if !removedPaths[oldest] || !removedPaths[older] {
		t.Errorf("the two oldest eligible worktrees must be collected: got removed=%v", removedPaths)
	}
	for _, kept := range []string{recent1, recent2, tooYoung} {
		if removedPaths[kept] {
			t.Errorf("path %s must be kept (KeepRecent=2 / MinAgeMinutes grace), but was removed: %+v", kept, m.Items)
		}
	}
}

func TestPlanWorktrees_BranchBacklogSweep_NotCheckedOutMergedOnly(t *testing.T) {
	e := newWorktreesTestEnv(t)
	e.git.porcelain = ""
	e.git.backlogBranches = []string{"cycle-legacyA-7", "cycle-legacyB-8-integration", "cycle-legacyC-9-w0"}
	e.git.mergedBranches["cycle-legacyA-7"] = true
	e.git.mergedBranches["cycle-legacyB-8-integration"] = false
	e.git.mergedBranches["cycle-legacyC-9-w0"] = true

	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}

	deletes := map[string]bool{}
	for _, it := range itemsWithAction(m.Items, WorktreeActionDeleteBranch) {
		deletes[it.Branch] = true
	}
	if !deletes["cycle-legacyA-7"] || !deletes["cycle-legacyC-9-w0"] {
		t.Errorf("both merged orphan branches must be swept for delete-branch: %+v", m.Items)
	}
	if deletes["cycle-legacyB-8-integration"] {
		t.Errorf("the unmerged orphan branch must never be deleted: %+v", m.Items)
	}
	flagged := map[string]bool{}
	for _, it := range itemsWithAction(m.Items, WorktreeActionFlagUnmerged) {
		flagged[it.Branch] = true
	}
	if !flagged["cycle-legacyB-8-integration"] {
		t.Errorf("the unmerged orphan branch must be flagged: %+v", m.Items)
	}
	if removes := itemsWithAction(m.Items, WorktreeActionRemove); len(removes) != 0 {
		t.Errorf("a branch-only backlog entry has no worktree dir to remove: %+v", removes)
	}
}

func TestApplyWorktrees_TOCTOURecheckRefusesNewlyLiveOrDirty(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-aaa9999-700", "cycle-aaa9999-700", 20*time.Hour, false, true)

	opts := e.opts()
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if len(itemsWithAction(m.Items, WorktreeActionRemove)) != 1 {
		t.Fatalf("setup: expected the worktree to be planned for removal: %+v", m.Items)
	}

	e.git.mu.Lock()
	e.git.dirtyDirs[wt] = true
	e.git.mu.Unlock()

	if err := ApplyWorktrees(opts, m); err == nil {
		t.Fatal("ApplyWorktrees must report an error when the TOCTOU re-check finds a now-dirty target")
	}
	if n := e.git.callCount("worktree remove"); n != 0 {
		t.Errorf("a newly-dirty target must NEVER be removed: saw %d 'worktree remove' calls (calls=%v)", n, e.git.calls)
	}
	if n := e.git.callCount("branch -d " + filepath.Base(wt)); n != 0 {
		t.Errorf("a newly-dirty target's branch must NEVER be deleted: calls=%v", e.git.calls)
	}
}

func TestApplyWorktrees_NonForceRemoveAndPrune(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-bbb8888-701", "cycle-bbb8888-701", 20*time.Hour, false, true)

	opts := e.opts()
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}

	if err := ApplyWorktrees(opts, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}

	e.git.mu.Lock()
	defer e.git.mu.Unlock()
	var sawRemove, sawForceRemove, sawBranchD, sawBranchCapD, sawPrune int
	for _, c := range e.git.calls {
		switch {
		case strings.Contains(c, "worktree remove --force"):
			sawForceRemove++
		case strings.Contains(c, "worktree remove "+wt):
			sawRemove++
		case strings.Contains(c, "branch -D"):
			sawBranchCapD++
		case strings.Contains(c, "branch -d cycle-bbb8888-701"):
			sawBranchD++
		case strings.Contains(c, "worktree prune"):
			sawPrune++
		}
	}
	if sawForceRemove != 0 {
		t.Errorf("worktree remove must NEVER pass --force (git's own dirty-check is the safety net): calls=%v", e.git.calls)
	}
	if sawRemove != 1 {
		t.Errorf("want exactly one non-force 'worktree remove %s': got %d, calls=%v", wt, sawRemove, e.git.calls)
	}
	if sawBranchCapD != 0 {
		t.Errorf("branch delete must NEVER escalate to -D: calls=%v", e.git.calls)
	}
	if sawBranchD != 1 {
		t.Errorf("want exactly one 'branch -d cycle-bbb8888-701': got %d, calls=%v", sawBranchD, e.git.calls)
	}
	if sawPrune != 1 {
		t.Errorf("want exactly one trailing 'worktree prune' for the whole batch: got %d, calls=%v", sawPrune, e.git.calls)
	}
}
