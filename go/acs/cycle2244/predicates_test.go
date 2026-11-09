//go:build acs

package cycle2244

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	landedBranch = "feat/x"
	endedBranch  = "feat/old"
	freshBranch  = "feat/y"
	cycleBranch  = "cycle-777"
)

var idleSince = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
	binDir    string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		goDir := filepath.Join(acsassert.RepoRoot(t), "go")
		if binDir, buildErr = os.MkdirTemp("", "cycle2244-bin-"); buildErr != nil {
			return
		}
		builtBin = filepath.Join(binDir, "evolve")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", builtBin, "./cmd/evolve")
		cmd.Dir = goDir
		cmd.WaitDelay = 10 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildFailure{err: err, out: string(out)}
		}
	})
	if buildErr != nil {
		t.Fatalf("RED: go build ./cmd/evolve: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct {
	err error
	out string
}

func (b *buildFailure) Error() string { return b.err.Error() + "\n" + b.out }

type project struct {
	repo *gittest.Repo
	t    *testing.T
}

func newProject(t *testing.T) project {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("seed.txt", "seed\n")
	repo.Git("add", "seed.txt")
	repo.Git("commit", "-m", "seed")
	repo.Git("branch", cycleBranch)
	repo.Write(".evolve/policy.json", `{"gc":{}}`+"\n")
	liveRun := filepath.Join(repo.Dir, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(liveRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(liveRun, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return project{repo: repo, t: t}
}

func (p project) commitBranch(branch string) string {
	p.t.Helper()
	p.repo.Git("checkout", "-b", branch)
	p.repo.Write(strings.ReplaceAll(branch, "/", "-")+".txt", branch+"\n")
	p.repo.Git("add", "-A")
	p.repo.Git("commit", "-m", "work on "+branch)
	tip := p.repo.Git("rev-parse", "HEAD")
	p.repo.Git("checkout", "main")
	return tip
}

func (p project) landBranch(branch string) string {
	p.t.Helper()
	tip := p.commitBranch(branch)
	p.repo.Git("merge", "--no-ff", "-m", "land "+branch, branch)
	return tip
}

func (p project) endBranch(branch string) string {
	p.t.Helper()
	stamp := idleSince.Format(time.RFC3339)
	p.t.Setenv("GIT_AUTHOR_DATE", stamp)
	p.t.Setenv("GIT_COMMITTER_DATE", stamp)
	tip := p.commitBranch(branch)
	os.Unsetenv("GIT_AUTHOR_DATE")
	os.Unsetenv("GIT_COMMITTER_DATE")
	return tip
}

func (p project) branchTip(branch string) string {
	p.t.Helper()
	out, err := exec.Command("git", "-C", p.repo.Dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (p project) bundleHolds(branch, tip string) bool {
	p.t.Helper()
	bundles, err := filepath.Glob(filepath.Join(p.repo.Dir, ".evolve", "operator-salvage", "*", "refs.bundle"))
	if err != nil {
		p.t.Fatal(err)
	}
	for _, b := range bundles {
		if exec.Command("git", "-C", p.repo.Dir, "bundle", "verify", b).Run() != nil {
			continue
		}
		heads, err := exec.Command("git", "-C", p.repo.Dir, "bundle", "list-heads", b).Output()
		if err == nil && strings.Contains(string(heads), tip+" refs/heads/"+branch) {
			return true
		}
	}
	return false
}

func (p project) bundleCount() int {
	bundles, _ := filepath.Glob(filepath.Join(p.repo.Dir, ".evolve", "operator-salvage", "*", "refs.bundle"))
	return len(bundles)
}

func (p project) operatorRun(args ...string) string {
	p.t.Helper()
	bin := evolveBinary(p.t)
	home := p.t.TempDir()
	cmd := exec.Command(bin, append([]string{"gc", "--project-root", p.repo.Dir}, args...)...)
	cmd.Dir = p.repo.Dir
	cmd.Env = hermeticEnv(home)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func hermeticEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		env = append(env, kv)
	}
	goCache := filepath.Join(home, "gocache")
	_ = os.MkdirAll(goCache, 0o755)
	return append(env,
		"HOME="+home, "TMPDIR="+home, "GOCACHE="+goCache,
		"TMUX_TMPDIR="+home, "GIT_TERMINAL_PROMPT=0")
}

func TestC2244_001_BatchHookScopeNeverPlansANonCycleBranch(t *testing.T) {
	p := newProject(t)
	p.landBranch(landedBranch)
	p.endBranch(endedBranch)
	opts := gc.WorktreeOptions{
		ProjectRoot:  p.repo.Dir,
		WorktreeBase: filepath.Join(p.repo.Dir, ".evolve", "worktrees"),
		EvolveDir:    filepath.Join(p.repo.Dir, ".evolve"),
		Exec:         sysexec.DefaultRunner,
	}
	m, err := gc.PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	cycleDeleted := false
	for _, it := range m.Items {
		if it.Action != gc.WorktreeActionDeleteBranch {
			continue
		}
		switch it.Branch {
		case cycleBranch:
			cycleDeleted = true
		case landedBranch, endedBranch:
			t.Errorf("RED: the hook-scope plan deletes %s: %+v", it.Branch, it)
		}
	}
	if !cycleDeleted {
		t.Errorf("the hook-scope plan no longer sweeps %s: %+v", cycleBranch, m.Items)
	}
}

func TestC2244_002_OperatorDryRunPlansTheLandedBranchDeleteAndMutatesNothing(t *testing.T) {
	p := newProject(t)
	tip := p.landBranch(landedBranch)
	out := p.operatorRun("--dry-run")
	if !strings.Contains(out, "WOULD-DELETE-BRANCH branch="+landedBranch) {
		t.Errorf("RED: operator --dry-run does not plan to delete the landed branch %s:\n%s", landedBranch, acsassert.OutputTail(out, 25))
	}
	if got := p.branchTip(landedBranch); got != tip {
		t.Errorf("--dry-run changed %s: tip %q, want %s", landedBranch, got, tip)
	}
	if n := p.bundleCount(); n != 0 {
		t.Errorf("--dry-run wrote %d bundle(s)", n)
	}
}

func TestC2244_003_OperatorDryRunPlansTheEndedBranchDeleteButNotTheFreshOne(t *testing.T) {
	p := newProject(t)
	p.endBranch(endedBranch)
	p.commitBranch(freshBranch)
	out := p.operatorRun("--dry-run")
	if !strings.Contains(out, "WOULD-DELETE-BRANCH branch="+endedBranch) {
		t.Errorf("RED: operator --dry-run does not plan to delete the ended branch %s:\n%s", endedBranch, acsassert.OutputTail(out, 25))
	}
	if strings.Contains(out, "WOULD-DELETE-BRANCH branch="+freshBranch) {
		t.Errorf("operator --dry-run plans to delete the fresh unlanded branch %s", freshBranch)
	}
}

func TestC2244_004_OperatorRunDeletesALandedBranchAfterAVerifiedBundle(t *testing.T) {
	p := newProject(t)
	tip := p.landBranch(landedBranch)
	out := p.operatorRun()
	if got := p.branchTip(landedBranch); got != "" {
		t.Errorf("RED: the operator run left the landed branch %s at %s:\n%s", landedBranch, got, acsassert.OutputTail(out, 25))
	}
	if !p.bundleHolds(landedBranch, tip) {
		t.Errorf("RED: no verified refs.bundle under .evolve/operator-salvage holds %s@%s", landedBranch, tip)
	}
	if p.branchTip("main") == "" {
		t.Error("the operator run deleted main")
	}
}

func TestC2244_005_OperatorRunDeletesAnEndedBranchAfterAVerifiedBundle(t *testing.T) {
	p := newProject(t)
	tip := p.endBranch(endedBranch)
	out := p.operatorRun()
	if got := p.branchTip(endedBranch); got != "" {
		t.Errorf("RED: the operator run left the ended branch %s at %s:\n%s", endedBranch, got, acsassert.OutputTail(out, 25))
	}
	if !p.bundleHolds(endedBranch, tip) {
		t.Errorf("RED: no verified refs.bundle under .evolve/operator-salvage holds %s@%s", endedBranch, tip)
	}
}

func TestC2244_006_OperatorRunKeepsBranchesWhenTheBundleCannotBeWritten(t *testing.T) {
	p := newProject(t)
	landedTip := p.landBranch(landedBranch)
	endedTip := p.endBranch(endedBranch)
	p.repo.Write(".evolve/operator-salvage", "not a directory\n")
	p.operatorRun()
	for branch, tip := range map[string]string{landedBranch: landedTip, endedBranch: endedTip} {
		if got := p.branchTip(branch); got != tip {
			t.Errorf("%s was deleted or moved although no bundle could be written: tip %q, want %s", branch, got, tip)
		}
	}
}

func TestC2244_007_OperatorRunKeepsAFreshUnlandedBranchAndMain(t *testing.T) {
	p := newProject(t)
	freshTip := p.commitBranch(freshBranch)
	mainTip := p.branchTip("main")
	p.operatorRun()
	if got := p.branchTip(freshBranch); got != freshTip {
		t.Errorf("the fresh unlanded branch %s changed: tip %q, want %s", freshBranch, got, freshTip)
	}
	if got := p.branchTip("main"); got != mainTip {
		t.Errorf("main changed: tip %q, want %s", got, mainTip)
	}
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func changedLines(t *testing.T, root, rel string) map[int]bool {
	t.Helper()
	lines := map[int]bool{}
	tracked, _ := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", rel).Output()
	if len(tracked) == 0 {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= strings.Count(string(body), "\n")+1; i++ {
			lines[i] = true
		}
		return lines
	}
	diff, err := exec.Command("git", "-C", root, "diff", "-U0", "refs/remotes/origin/main", "--", rel).Output()
	if err != nil {
		t.Fatalf("git diff %s: %v", rel, err)
	}
	for _, l := range strings.Split(string(diff), "\n") {
		m := hunkHeader.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2])
		}
		for i := start; i < start+n; i++ {
			lines[i] = true
		}
	}
	return lines
}

func changedGCFunctions(t *testing.T, root string) []string {
	t.Helper()
	listed, err := exec.Command("git", "-C", root, "ls-files", "--modified", "--others", "--exclude-standard", "--", "go/internal/gc").Output()
	if err != nil {
		t.Fatal(err)
	}
	committed, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=AM", "refs/remotes/origin/main", "--", "go/internal/gc").Output()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var funcs []string
	for _, rel := range strings.Fields(string(listed) + "\n" + string(committed)) {
		if seen[rel] || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || filepath.Dir(rel) != "go/internal/gc" {
			continue
		}
		seen[rel] = true
		changed := changedLines(t, root, rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			for l := fset.Position(fn.Pos()).Line; l <= fset.Position(fn.End()).Line; l++ {
				if changed[l] {
					funcs = append(funcs, fn.Name.Name)
					break
				}
			}
		}
	}
	return funcs
}

func TestC2244_008_EveryChangedGCFunctionHasFullCoverageFromTheBranchSweepTests(t *testing.T) {
	root := acsassert.RepoRoot(t)
	funcs := changedGCFunctions(t, root)
	if len(funcs) == 0 {
		t.Fatal("RED: no function in go/internal/gc changed against origin/main — the branch sweep is not built")
	}
	dir := filepath.Join(root, "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-run", "BranchSweep", "-coverprofile="+profile, "./internal/gc")
	test.Dir = dir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("RED: the BranchSweep tests of go/internal/gc failed: %v\n%s", err, acsassert.OutputTail(string(out), 15))
	}
	cover := exec.Command("go", "tool", "cover", "-func="+profile)
	cover.Dir = dir
	out, err := cover.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, acsassert.OutputTail(string(out), 15))
	}
	for _, fn := range funcs {
		pct, err := acsassert.FuncCoverage(string(out), fn)
		if err != nil {
			t.Errorf("RED: %v", err)
			continue
		}
		if pct < 100.0 {
			t.Errorf("RED: %s coverage %.1f%% under -run BranchSweep, want 100.0%%", fn, pct)
		}
	}
}
