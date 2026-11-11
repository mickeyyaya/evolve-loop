//go:build acs

package cycle2280

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cycleNumber     = "2280"
	gcPackageRel    = "internal/gc"
	flaggedBranchA  = "cycle-flaga"
	flaggedBranchB  = "cycle-flagb"
	decisionID      = "D11"
	componentID     = "X7"
	sweepStepFailed = 1
)

var evolveBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cycle2280-bin-")
	if err != nil {
		os.Exit(2)
	}
	code := 2
	if bin, ok := buildEvolve(dir); ok {
		evolveBinary = bin
		code = m.Run()
	}
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func buildEvolve(dir string) (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	moduleDir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		return "", false
	}
	bin := filepath.Join(dir, "evolve")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = moduleDir
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Stderr.Write(out)
		return "", false
	}
	return bin, true
}

type hub struct {
	repo  *gittest.Repo
	t     *testing.T
	scrub string
}

func newHub(t *testing.T) *hub {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("README.md", "hub\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-q", "-m", "base")
	runDir := filepath.Join(repo.Dir, ".evolve", "runs", "cycle-9001")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	now := time.Now()
	lease := runlease.Lease{RunID: "cycle-9001", OwnerPID: os.Getpid(), HeartbeatAt: now.UTC().Format(time.RFC3339)}
	if err := runlease.Write(runDir, lease, now); err != nil {
		t.Fatalf("write live lease: %v", err)
	}
	return &hub{repo: repo, t: t, scrub: t.TempDir()}
}

func (h *hub) flagUnmergedBranch(name string) {
	h.t.Helper()
	h.repo.Git("checkout", "-q", "-b", name)
	h.repo.Write(name+".txt", name+"\n")
	h.repo.Git("add", name+".txt")
	h.repo.Git("commit", "-q", "-m", "unmerged work on "+name)
	h.repo.Git("checkout", "-q", "main")
}

func (h *hub) env() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") || strings.HasPrefix(kv, "TMPDIR=") || strings.HasPrefix(kv, "TMUX_TMPDIR=") || strings.HasPrefix(kv, "HOME=") {
			continue
		}
		env = append(env, kv)
	}
	cache := filepath.Join(h.scrub, "gocache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		h.t.Fatalf("create go cache dir: %v", err)
	}
	return append(env, "TMPDIR="+h.scrub, "TMUX_TMPDIR="+h.scrub, "HOME="+h.scrub, "GOCACHE="+cache)
}

func (h *hub) gc(args ...string) (string, int) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBinary, append([]string{"gc", "--project-root", h.repo.Dir}, args...)...)
	cmd.Dir = h.repo.Dir
	cmd.Env = h.env()
	cmd.WaitDelay = 10 * time.Second
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		h.t.Fatalf("run evolve gc: %v", err)
	}
	return out.String(), code
}

func (h *hub) run(label string, args ...string) string {
	h.t.Helper()
	out, code := h.gc(args...)
	if code != 0 && code != sweepStepFailed {
		h.t.Fatalf("%s: evolve gc exit=%d\n%s", label, code, acsassert.OutputTail(out, 30))
	}
	return out
}

func (h *hub) inboxItems() []string {
	h.t.Helper()
	files, err := filepath.Glob(filepath.Join(h.repo.Dir, ".evolve", "inbox", "*.json"))
	if err != nil {
		h.t.Fatalf("glob inbox: %v", err)
	}
	return files
}

func flagLine(out, branch string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, branch) && strings.Contains(strings.ToUpper(line), "FLAG-UNMERGED") {
			return line
		}
	}
	return ""
}

func requireFlagReported(t *testing.T, label, out, branch string) string {
	t.Helper()
	line := flagLine(out, branch)
	if line == "" {
		t.Fatalf("%s: the gc report has no FLAG-UNMERGED line for %s (fixture or report broken)\n%s", label, branch, acsassert.OutputTail(out, 30))
	}
	return line
}

func requireCount(t *testing.T, label, out, branch, count string) {
	t.Helper()
	line := requireFlagReported(t, label, out, branch)
	if !strings.Contains(line, "count="+count) {
		t.Errorf("%s: flag line for %s lacks %q: %q", label, branch, "count="+count, line)
	}
}

func TestC2280_001_FlagRepeatedOnTwoRunsFilesOneItemNotTwo(t *testing.T) {
	h := newHub(t)
	h.flagUnmergedBranch(flaggedBranchA)
	first := h.run("run 1")
	requireFlagReported(t, "run 1", first, flaggedBranchA)
	if n := len(h.inboxItems()); n != 0 {
		t.Errorf("a flag seen on one run filed %d inbox item(s), want 0", n)
	}
	second := h.run("run 2")
	requireFlagReported(t, "run 2", second, flaggedBranchA)
	items := h.inboxItems()
	if len(items) != 1 {
		t.Fatalf("a flag repeated on two runs filed %d inbox item(s), want exactly 1: %v", len(items), items)
	}
	body, err := os.ReadFile(items[0])
	if err != nil {
		t.Fatalf("read filed item: %v", err)
	}
	if !strings.Contains(string(body), flaggedBranchA) {
		t.Errorf("the filed item does not name the flagged branch %s:\n%s", flaggedBranchA, body)
	}
}

func TestC2280_002_ReportCarriesTheRepeatCount(t *testing.T) {
	h := newHub(t)
	h.flagUnmergedBranch(flaggedBranchA)
	requireCount(t, "run 1", h.run("run 1"), flaggedBranchA, "1")
	requireCount(t, "run 2", h.run("run 2"), flaggedBranchA, "2")
	requireCount(t, "run 3", h.run("run 3"), flaggedBranchA, "3")
}

func TestC2280_003_RunsAfterTheFilingAddNoSecondItem(t *testing.T) {
	h := newHub(t)
	h.flagUnmergedBranch(flaggedBranchA)
	for i := 1; i <= 4; i++ {
		h.run("run " + string(rune('0'+i)))
	}
	if n := len(h.inboxItems()); n != 1 {
		t.Errorf("four runs of one flag left %d inbox item(s), want exactly 1", n)
	}
}

func TestC2280_004_EachFlagKeyCountsAndFilesOnItsOwn(t *testing.T) {
	h := newHub(t)
	h.flagUnmergedBranch(flaggedBranchA)
	h.run("run 1")
	h.flagUnmergedBranch(flaggedBranchB)
	second := h.run("run 2")
	requireCount(t, "run 2", second, flaggedBranchA, "2")
	requireCount(t, "run 2", second, flaggedBranchB, "1")
	if n := len(h.inboxItems()); n != 1 {
		t.Fatalf("after run 2 the inbox holds %d item(s), want 1 (only %s has repeated)", n, flaggedBranchA)
	}
	third := h.run("run 3")
	requireCount(t, "run 3", third, flaggedBranchB, "2")
	items := h.inboxItems()
	if len(items) != 2 {
		t.Fatalf("after run 3 the inbox holds %d item(s), want 2 (one for each flag): %v", len(items), items)
	}
	owners := map[string]bool{}
	for _, item := range items {
		body, err := os.ReadFile(item)
		if err != nil {
			t.Fatalf("read %s: %v", item, err)
		}
		for _, branch := range []string{flaggedBranchA, flaggedBranchB} {
			if strings.Contains(string(body), branch) {
				owners[branch] = true
			}
		}
	}
	if len(owners) != 2 {
		t.Errorf("the two items do not name one flag each: %v", owners)
	}
}

func TestC2280_005_DryRunNeitherCountsNorFiles(t *testing.T) {
	h := newHub(t)
	h.flagUnmergedBranch(flaggedBranchA)
	for i := 1; i <= 3; i++ {
		out := h.run("dry run", "--dry-run")
		requireFlagReported(t, "dry run", out, flaggedBranchA)
	}
	if n := len(h.inboxItems()); n != 0 {
		t.Fatalf("three dry runs filed %d inbox item(s), want 0", n)
	}
	requireCount(t, "first real run", h.run("run 1"), flaggedBranchA, "1")
	if n := len(h.inboxItems()); n != 0 {
		t.Errorf("the first real run after dry runs filed %d inbox item(s), want 0: a dry run advanced the tally", n)
	}
}

func TestC2280_006_ChangedGcFunctionsAreFullyCovered(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	moduleDir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-tags", "integration", "-coverprofile="+profile, "./"+gcPackageRel)
	test.Dir = moduleDir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./%s failed: %v\n%s", gcPackageRel, err, acsassert.OutputTail(string(out), 30))
	}
	packages, err := covchanged.ParsePackageSet([]byte(gcPackageRel))
	if err != nil {
		t.Fatalf("package set: %v", err)
	}
	ws := covchanged.Workspace{ModuleDir: moduleDir, Run: sysexec.DefaultRunner}
	verdict, err := ws.Check(context.Background(), covchanged.Options{Base: "main", Profile: profile, Tags: covchanged.DefaultTags, Packages: packages})
	if err != nil {
		t.Fatalf("covchanged: %v", err)
	}
	if verdict.Changed == 0 {
		t.Fatalf("no function of %s changed against main: the flag tally is not built in the gc package", gcPackageRel)
	}
	if !verdict.Clean() {
		t.Errorf("changed gc functions below 100%% coverage:\n%s", strings.Join(verdict.Problems(), "\n"))
	}
}

func TestC2280_007_ExplanationNamesTheDecisionAndComponentIDs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-"+cycleNumber+"-*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("docs/explain/builds holds %d cycle-%s document(s), want exactly 1", len(matches), cycleNumber)
	}
	rel := filepath.Base(matches[0])
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	for _, id := range []string{decisionID, componentID} {
		if !strings.Contains(string(body), id) {
			t.Errorf("%s does not name %s", rel, id)
		}
	}
}
