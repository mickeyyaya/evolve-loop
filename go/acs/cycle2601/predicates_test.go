//go:build acs

// Package cycle2601 encodes the acceptance of the gc check registry, first for the worktree sweep, with the fetched origin/main as the landing target.
package cycle2601

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	landedBranch   = "cycle-landed-100"
	unpushedBranch = "cycle-unpushed-101"
	dirtyBranch    = "cycle-dirty-102"
	aheadBranch    = "cycle-ahead-103"
	baselineCommit = "f34d4bc1d52cc4b4b7d5eb13d2306889ade61b33"
)

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2601-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for the evolve build: %v", err)
		return m.Run()
	}
	defer os.RemoveAll(dir)
	mod, err := acsassert.ModuleDir(".")
	if err != nil {
		buildErr = err.Error()
		return m.Run()
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := sysexec.Command(context.Background(), "go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(mod, "cmd", "evolve")
	if out, berr := cmd.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build in %s: %v\n%s", mod, berr, out)
	} else {
		evolveBin = bin
	}
	return m.Run()
}

type reportItem struct {
	Subject  string `json:"subject"`
	Action   string `json:"action"`
	Rule     string `json:"rule"`
	Evidence string `json:"evidence"`
}

type reportDoc struct {
	Checks []struct {
		Name  string       `json:"name"`
		Items []reportItem `json:"items"`
	} `json:"checks"`
}

type sweepTree struct {
	hub    *gittest.Repo
	origin *gittest.Repo
	base   string
}

func newSweepTree(t *testing.T) sweepTree {
	t.Helper()
	hub := gittest.Fixture(t)
	hub.Write("README.md", "hub\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "init")
	origin := gittest.Bare(t)
	hub.Git("remote", "add", "origin", origin.Dir)
	hub.Git("push", "-q", "origin", "main")
	s := sweepTree{hub: hub, origin: origin, base: filepath.Join(hub.Dir, ".evolve", "worktrees")}
	for _, name := range []string{landedBranch, unpushedBranch, dirtyBranch, aheadBranch} {
		hub.Git("worktree", "add", "-q", "-b", name, filepath.Join(s.base, name), "main")
	}
	s.commitIn(t, landedBranch)
	hub.Git("merge", "-q", "--no-ff", landedBranch, "-m", "land "+landedBranch)
	hub.Git("push", "-q", "origin", "main")
	s.commitIn(t, unpushedBranch)
	hub.Git("merge", "-q", "--no-ff", unpushedBranch, "-m", "integrate "+unpushedBranch)
	s.commitIn(t, aheadBranch)
	if err := os.WriteFile(filepath.Join(s.base, dirtyBranch, "scratch.txt"), []byte("uncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return s
}

func (s sweepTree) commitIn(t *testing.T, name string) {
	t.Helper()
	wt := filepath.Join(s.base, name)
	if err := os.WriteFile(filepath.Join(wt, name+".txt"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.hub.Git("-C", wt, "add", "-A")
	s.hub.Git("-C", wt, "commit", "-q", "-m", "work "+name)
}

func (s sweepTree) breakOrigin(t *testing.T) {
	t.Helper()
	s.hub.Git("remote", "set-url", "origin", filepath.Join(t.TempDir(), "unreachable.git"))
}

func (s sweepTree) options() gc.WorktreeOptions {
	return gc.WorktreeOptions{
		ProjectRoot:  s.hub.Dir,
		WorktreeBase: s.base,
		EvolveDir:    filepath.Join(s.hub.Dir, ".evolve"),
		Exec:         sysexec.DefaultRunner,
		LeaseTTL:     10 * time.Minute,
	}
}

func (s sweepTree) originMain() gc.WorktreeOptions {
	o := s.options()
	o.Landing = gc.LandingTarget{Remote: "origin", Branch: "main"}
	return o
}

func (s sweepTree) branchKept(name string) bool {
	return s.hub.Git("branch", "--list", name) != ""
}

func (s sweepTree) worktreeKept(name string) bool {
	_, err := os.Stat(filepath.Join(s.base, name))
	return err == nil
}

func hermeticEnv(t *testing.T, repoDir string) []string {
	t.Helper()
	bin := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not on PATH: %v", err)
	}
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	execPath := strings.TrimSpace(gitOutput(t, repoDir, "--exec-path"))
	return []string{"PATH=" + bin, "GIT_EXEC_PATH=" + execPath, "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(), "GIT_TERMINAL_PROMPT=0"}
}

func gitOutput(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", repoDir}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %v: exit=%d err=%v\n%s", args, code, err, stderr)
	}
	return out
}

func (s sweepTree) runCheck(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	if evolveBin == "" {
		t.Fatalf("RED: the evolve binary did not build:\n%s", buildErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := sysexec.Command(ctx, evolveBin, append([]string{"gc", "check"}, args...)...)
	cmd.Dir = s.hub.Dir
	cmd.Env = hermeticEnv(t, s.hub.Dir)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code = 0
	if err := cmd.Run(); err != nil {
		code = -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
	}
	return out.String(), errOut.String(), code
}

func (s sweepTree) worktreeReport(t *testing.T) ([]reportItem, int) {
	t.Helper()
	stdout, stderr, code := s.runCheck(t, "--project-root", s.hub.Dir, "--json")
	var doc reportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("RED: evolve gc check --json (exit=%d) printed no JSON report: %v\nstdout: %s\nstderr: %s", code, err, acsassert.OutputTail(stdout, 10), acsassert.OutputTail(stderr, 10))
	}
	for _, c := range doc.Checks {
		if c.Name == "worktrees" {
			return c.Items, code
		}
	}
	t.Fatalf("RED: the report has no check named worktrees: %s", stdout)
	return nil, code
}

func fromManifest(m gc.WorktreeManifest) []reportItem {
	var out []reportItem
	for _, it := range m.Items {
		out = append(out, reportItem{Subject: it.Path + " " + it.Branch, Action: string(it.Action), Evidence: it.Reason})
	}
	return out
}

func itemsFor(items []reportItem, name string) []reportItem {
	var out []reportItem
	for _, it := range items {
		if strings.Contains(it.Subject, name) {
			out = append(out, it)
		}
	}
	return out
}

func hasAction(items []reportItem, name, action string) (reportItem, bool) {
	for _, it := range itemsFor(items, name) {
		if it.Action == action {
			return it, true
		}
	}
	return reportItem{}, false
}

func expectEvidence(t *testing.T, items []reportItem, name, action, evidenceHas string) {
	t.Helper()
	it, ok := hasAction(items, name, action)
	if !ok {
		t.Errorf("%s: no %q item; items = %+v", name, action, itemsFor(items, name))
		return
	}
	if !strings.Contains(strings.ToLower(it.Evidence), evidenceHas) {
		t.Errorf("%s %s: evidence %q does not mention %q", name, action, it.Evidence, evidenceHas)
	}
}

func expectKept(t *testing.T, items []reportItem, name string) {
	t.Helper()
	for _, action := range []string{"remove", "delete-branch"} {
		if _, found := hasAction(items, name, action); found {
			t.Errorf("%s: planned %q although it is not landed on the fetched origin/main", name, action)
		}
	}
	if _, ok := hasAction(items, name, "flag-unmerged"); !ok {
		t.Errorf("%s: not reported as flag-unmerged; items = %+v", name, itemsFor(items, name))
	}
}

func TestC2601_001_ReportListsEachItemWithRuleAndEvidence(t *testing.T) {
	tree := newSweepTree(t)
	items, code := tree.worktreeReport(t)
	if code != 0 {
		t.Errorf("evolve gc check exit = %d, want 0", code)
	}
	for _, name := range []string{landedBranch, unpushedBranch, dirtyBranch, aheadBranch} {
		if len(itemsFor(items, name)) == 0 {
			t.Errorf("%s: the report lists no item for it", name)
		}
	}
	for _, it := range items {
		if it.Subject == "" || it.Action == "" || it.Rule == "" || it.Evidence == "" || it.Rule == it.Evidence {
			t.Errorf("item lacks its own rule or evidence: %+v", it)
		}
	}
	expectEvidence(t, items, landedBranch, "remove", "merged")
	expectEvidence(t, items, landedBranch, "delete-branch", "merged")
	expectEvidence(t, items, dirtyBranch, "flag-dirty", "dirty")
	expectEvidence(t, items, aheadBranch, "flag-unmerged", "origin/main")
}

func TestC2601_002_CheckReportsAndMutatesNothing(t *testing.T) {
	tree := newSweepTree(t)
	if _, code := tree.worktreeReport(t); code != 0 {
		t.Errorf("evolve gc check exit = %d, want 0", code)
	}
	for _, name := range []string{landedBranch, unpushedBranch, dirtyBranch, aheadBranch} {
		if !tree.branchKept(name) || !tree.worktreeKept(name) {
			t.Errorf("%s: the check changed the tree (branch kept=%v, worktree kept=%v)", name, tree.branchKept(name), tree.worktreeKept(name))
		}
	}
}

func TestC2601_003_UnknownFlagIsAUsageError(t *testing.T) {
	tree := newSweepTree(t)
	stdout, stderr, code := tree.runCheck(t, "--project-root", tree.hub.Dir, "--no-such-flag")
	if code != 10 {
		t.Errorf("exit = %d, want the usage exit 10", code)
	}
	if !strings.Contains(stderr, "no-such-flag") {
		t.Errorf("stderr does not name the bad flag: %s", acsassert.OutputTail(stderr, 5))
	}
	if strings.Contains(stdout, `"checks"`) {
		t.Errorf("a report was printed for a bad flag: %s", stdout)
	}
	if !tree.worktreeKept(landedBranch) {
		t.Errorf("a bad flag mutated the tree")
	}
}

func TestC2601_004_HubHoldingAnUnpushedMergeKeepsTheCycleBranch(t *testing.T) {
	tree := newSweepTree(t)
	tree.hub.Git("merge-base", "--is-ancestor", unpushedBranch, "HEAD")
	items, _ := tree.worktreeReport(t)
	expectKept(t, items, unpushedBranch)
	if _, ok := hasAction(items, landedBranch, "delete-branch"); !ok {
		t.Errorf("%s landed on origin/main yet is not planned for deletion: %+v", landedBranch, itemsFor(items, landedBranch))
	}
	if !tree.branchKept(unpushedBranch) {
		t.Errorf("%s was deleted", unpushedBranch)
	}
}

func TestC2601_005_ApplyRechecksTheLandingTargetBeforeDeleting(t *testing.T) {
	tree := newSweepTree(t)
	manifest, err := gc.PlanWorktrees(tree.options())
	if err != nil {
		t.Fatalf("PlanWorktrees against the hub HEAD: %v", err)
	}
	if _, ok := hasAction(fromManifest(manifest), unpushedBranch, "delete-branch"); !ok {
		t.Fatalf("setup: the HEAD-judged plan must hold the unpushed merge as deletable; items = %+v", manifest.Items)
	}
	_ = gc.ApplyWorktrees(tree.originMain(), manifest)
	if !tree.branchKept(unpushedBranch) {
		t.Errorf("%s deleted although origin/main does not hold it", unpushedBranch)
	}
	if tree.branchKept(landedBranch) {
		t.Errorf("%s landed on origin/main yet survived the apply", landedBranch)
	}
}

func TestC2601_006_DeleteItemsCarryTheTipTheyWereJudgedAt(t *testing.T) {
	tree := newSweepTree(t)
	manifest, err := gc.PlanWorktrees(tree.originMain())
	if err != nil {
		t.Fatalf("PlanWorktrees against origin/main: %v", err)
	}
	tip := strings.TrimSpace(tree.hub.Git("rev-parse", "refs/heads/"+landedBranch))
	found := false
	for _, it := range manifest.Items {
		if it.Action == gc.WorktreeActionDeleteBranch && it.Branch == landedBranch {
			found = true
			if it.Head != tip {
				t.Errorf("delete-branch head = %q, want the judged tip %q", it.Head, tip)
			}
		}
	}
	if !found {
		t.Errorf("no delete-branch item for %s: %+v", landedBranch, manifest.Items)
	}
}

var testFuncRe = regexp.MustCompile(`(?m)^func (Test\w+)\(`)

func TestC2601_007_ExistingWorktreeSweepTestsPassUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	mod := acsassert.GoModuleDir(t)
	files, err := filepath.Glob(filepath.Join(mod, "internal", "gc", "worktrees*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no worktrees*_test.go files found: %v", err)
	}
	var names []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range testFuncRe.FindAllStringSubmatch(string(raw), -1) {
			names = append(names, m[1])
		}
	}
	if len(names) == 0 {
		t.Fatal("no existing worktree sweep test found")
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "cat-file", "-e", baselineCommit+"^{commit}"); code == 0 {
		changed, _, _, _ := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "--diff-filter=MDR", baselineCommit, "--", "go/internal/gc/*_test.go", "go/cmd/evolve/*gc*_test.go")
		if strings.TrimSpace(changed) != "" {
			t.Errorf("existing test files changed since the baseline:\n%s", changed)
		}
	} else {
		t.Logf("baseline commit %s absent; the unchanged-file half is skipped", baselineCommit)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", mod, "-count=1", "-run", "^("+strings.Join(names, "|")+")$", "./internal/gc")
	if err != nil || code != 0 || !strings.Contains(stdout, "ok") {
		t.Errorf("the existing worktree sweep tests fail (exit=%d, err=%v):\n%s\n%s", code, err, acsassert.OutputTail(stdout, 30), acsassert.OutputTail(stderr, 30))
	}
}

func TestC2601_008_FailedFetchCountsAsNotLandedInTheCheck(t *testing.T) {
	tree := newSweepTree(t)
	tree.breakOrigin(t)
	items, _ := tree.worktreeReport(t)
	expectKept(t, items, landedBranch)
	expectKept(t, items, unpushedBranch)
	expectEvidence(t, items, landedBranch, "flag-unmerged", "fetch")
	if !tree.branchKept(landedBranch) {
		t.Errorf("%s was deleted after a failed fetch", landedBranch)
	}
}

func TestC2601_009_FailedFetchCountsAsNotLandedInThePlan(t *testing.T) {
	tree := newSweepTree(t)
	tree.breakOrigin(t)
	manifest, err := gc.PlanWorktrees(tree.originMain())
	if err != nil {
		t.Fatalf("PlanWorktrees must degrade to not landed on a failed fetch, got error: %v", err)
	}
	items := fromManifest(manifest)
	expectKept(t, items, landedBranch)
	expectKept(t, items, unpushedBranch)
	expectEvidence(t, items, landedBranch, "flag-unmerged", "fetch")
}
