//go:build acs

package cycle2376

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baseCommit      = "5da3551a32ee8caacaa852c5a6517f98bbda7744"
	childModeKey    = "ACS_C2376_TMUXTEST_CHILD_MODE"
	endPollAttempts = 150
	endPollStep     = 100 * time.Millisecond
	victimSession   = "victim"
	bridgeNote      = "docs/architecture/packages/internal-bridge.md"
	gcNote          = "docs/architecture/packages/internal-gc.md"
)

var (
	builtEvolveDir string
	buildEvolveErr error
)

type endingRun func() int

func (r endingRun) Run() int { return r() }

func TestMain(m *testing.M) {
	if mode := os.Getenv(childModeKey); mode != "" {
		os.Exit(tmuxtest.Main(endingRun(func() int { return endTheChild(mode) })))
	}
	code := m.Run()
	if builtEvolveDir != "" {
		_ = os.RemoveAll(builtEvolveDir)
	}
	os.Exit(code)
}

func endTheChild(mode string) int {
	if err := tmuxtest.StartServer(os.Getenv(bridge.TmuxSocketEnv) + "-own"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 9
	}
	switch mode {
	case "exits":
		os.Exit(0)
	case "killed":
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		select {}
	case "panics":
		panic("the test binary panics")
	}
	return 0
}

func privateTmuxDir(t *testing.T) string {
	t.Helper()
	testhost.Tool(t, "tmux")
	dir, err := os.MkdirTemp("/tmp", "c2376")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		socketsIn, _ := filepath.Glob(filepath.Join(socketDirOf(dir), "*"))
		for _, s := range socketsIn {
			_ = exec.Command("tmux", "-S", s, "kill-server").Run()
		}
		_ = os.RemoveAll(dir)
	})
	return dir
}

func socketDirOf(tmuxTmpdir string) string {
	return filepath.Join(tmuxTmpdir, "tmux-"+strconv.Itoa(os.Getuid()))
}

func tmuxIn(tmuxTmpdir string, args ...string) *exec.Cmd {
	cmd := exec.Command("tmux", args...)
	cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+tmuxTmpdir)
	return cmd
}

func startServerOn(t *testing.T, tmuxTmpdir, socket string) {
	t.Helper()
	cmd := tmuxIn(tmuxTmpdir, "-L", socket, "-f", os.DevNull, "new-session", "-d", "-s", victimSession, "sleep 600")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("start a tmux server on %s: %v: %s", socket, err, out)
	}
}

func serverAnswers(tmuxTmpdir, socket string) bool {
	return tmuxIn(tmuxTmpdir, "-L", socket, "display-message", "-p", "ok").Run() == nil
}

func socketFileExists(tmuxTmpdir, socket string) bool {
	_, err := os.Lstat(filepath.Join(socketDirOf(tmuxTmpdir), socket))
	return err == nil
}

func deadPID(t *testing.T) int {
	t.Helper()
	out, _, code, err := acsassert.SubprocessOutput("sh", "-c", "echo $$")
	if err != nil || code != 0 {
		t.Fatalf("run a short shell to learn an exited pid: exit=%d: %v", code, err)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		t.Fatalf("read the shell pid from %q: %v", out, convErr)
	}
	return pid
}

func runEndingChild(t *testing.T, mode string) (tmuxTmpdir string, pid int) {
	t.Helper()
	tmuxTmpdir = privateTmuxDir(t)
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.timeout=120s")
	cmd.Env = append(os.Environ(), childModeKey+"="+mode, "TMUX_TMPDIR="+tmuxTmpdir)
	cmd.WaitDelay = 30 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the %s child: %v", mode, err)
	}
	_ = cmd.Wait()
	return tmuxTmpdir, cmd.Process.Pid
}

func leftOnItsSockets(tmuxTmpdir string, pid int) []string {
	var left []string
	for _, socket := range []string{bridge.DeriveTestSocket(pid), bridge.DeriveTestSocket(pid) + "-own"} {
		if socketFileExists(tmuxTmpdir, socket) || serverAnswers(tmuxTmpdir, socket) {
			left = append(left, socket)
		}
	}
	return left
}

func waitUntilNothingIsLeft(tmuxTmpdir string, pid int) []string {
	left := leftOnItsSockets(tmuxTmpdir, pid)
	for attempt := 0; len(left) > 0 && attempt < endPollAttempts; attempt++ {
		time.Sleep(endPollStep)
		left = leftOnItsSockets(tmuxTmpdir, pid)
	}
	return left
}

func TestC2376_001_AnEndedTmuxtestMainProcessLeavesNothingOnItsSockets(t *testing.T) {
	for _, mode := range []string{"exits", "killed", "panics"} {
		t.Run(mode, func(t *testing.T) {
			tmuxTmpdir, pid := runEndingChild(t, mode)

			left := waitUntilNothingIsLeft(tmuxTmpdir, pid)

			if len(left) > 0 {
				t.Errorf("a tmuxtest.Main process that %s left %v on its sockets in %s", mode, left, socketDirOf(tmuxTmpdir))
			}
		})
	}
}

func TestC2376_002_ATmuxtestMainProcessThatReturnsStillLeavesNothingOnItsSockets(t *testing.T) {
	tmuxTmpdir, pid := runEndingChild(t, "returns")

	left := waitUntilNothingIsLeft(tmuxTmpdir, pid)

	if len(left) > 0 {
		t.Errorf("a tmuxtest.Main process that returned left %v on its sockets", left)
	}
}

type ownerSockets struct {
	deadOwner, liveOwner []string
}

func seedOwnerSockets(t *testing.T, tmuxTmpdir string) ownerSockets {
	t.Helper()
	dead, live := bridge.DeriveTestSocket(deadPID(t)), bridge.DeriveTestSocket(os.Getpid())
	seeded := ownerSockets{
		deadOwner: []string{dead, dead + "-own"},
		liveOwner: []string{live, live + "-own"},
	}
	for _, socket := range append(append([]string{}, seeded.deadOwner...), seeded.liveOwner...) {
		startServerOn(t, tmuxTmpdir, socket)
	}
	return seeded
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	if builtEvolveDir == "" && buildEvolveErr == nil {
		builtEvolveDir, buildEvolveErr = buildEvolve(moduleDir(t))
	}
	if buildEvolveErr != nil {
		t.Fatalf("build the evolve binary from this tree: %v", buildEvolveErr)
	}
	return filepath.Join(builtEvolveDir, "evolve")
}

func buildEvolve(module string) (string, error) {
	dir, err := os.MkdirTemp("", "acs-c2376-bin-")
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "evolve"), "./cmd/evolve")
	cmd.Dir = module
	if out, err := cmd.CombinedOutput(); err != nil {
		return dir, fmt.Errorf("go build ./cmd/evolve: %w: %s", err, acsassert.OutputTail(string(out), 15))
	}
	return dir, nil
}

func moduleDir(t *testing.T) string {
	t.Helper()
	module, err := acsassert.ModuleDir(acsassert.RepoRoot(t))
	if err != nil {
		module, err = acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	}
	if err != nil {
		t.Fatalf("find the go module: %v", err)
	}
	return module
}

var socketLine = regexp.MustCompile(`(?m)^\s+socket (evolve-bridge-\S+)$`)

func TestC2376_003_EvolveGcListsTheServersOfADeadTestProcessAndNeverALiveOnes(t *testing.T) {
	tmuxTmpdir := privateTmuxDir(t)
	seeded := seedOwnerSockets(t, tmuxTmpdir)
	cmd := exec.Command(evolveBinary(t), "gc", "--dry-run", "--project-root", t.TempDir())
	cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+tmuxTmpdir)

	out, _ := cmd.Output()

	listed := map[string]bool{}
	for _, m := range socketLine.FindAllStringSubmatch(string(out), -1) {
		listed[m[1]] = true
	}
	for _, socket := range seeded.deadOwner {
		if !listed[socket] {
			t.Errorf("evolve gc --dry-run does not list %s, the server of a test process that is dead; listed %v", socket, listed)
		}
	}
	for _, socket := range seeded.liveOwner {
		if listed[socket] {
			t.Errorf("evolve gc --dry-run lists %s, the server of a test process that is alive", socket)
		}
	}
	if !acsassert.FileContains(t, filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve", "cmd_gc.go"), "sockets: swarm.ExecReapOrphanSockets") {
		t.Errorf("evolve gc no longer runs the socket reaper")
	}
	for _, socket := range seeded.deadOwner {
		if !serverAnswers(tmuxTmpdir, socket) {
			t.Errorf("--dry-run stopped %s; a dry run must mutate nothing", socket)
		}
	}
}

func TestC2376_004_TheGcReaperStopsTheServersOfADeadTestProcessAndSparesLiveOnes(t *testing.T) {
	tmuxTmpdir := privateTmuxDir(t)
	seeded := seedOwnerSockets(t, tmuxTmpdir)
	t.Setenv("TMUX_TMPDIR", tmuxTmpdir)

	swarm.ExecReapOrphanSockets(context.Background())

	for _, socket := range seeded.deadOwner {
		if serverAnswers(tmuxTmpdir, socket) || socketFileExists(tmuxTmpdir, socket) {
			t.Errorf("the gc reaper left %s, the server of a test process that is dead", socket)
		}
	}
	for _, socket := range seeded.liveOwner {
		if !serverAnswers(tmuxTmpdir, socket) {
			t.Errorf("the gc reaper stopped %s, the server of a test process that is alive", socket)
		}
	}
}

var diffHunk = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)

func gitLines(t *testing.T, root string, args ...string) []string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root}, args...)...)
	if err != nil || code != 0 {
		t.Fatalf("git %v: code=%d err=%v: %s", args, code, err, errOut)
	}
	return strings.FieldsFunc(out, func(r rune) bool { return r == '\n' })
}

type changeSet struct {
	root      string
	files     []string
	untracked map[string]bool
}

func readChangeSet(t *testing.T) changeSet {
	t.Helper()
	root := acsassert.RepoRoot(t)
	set := changeSet{root: root, untracked: map[string]bool{}}
	for _, f := range gitLines(t, root, "diff", "--name-only", "--diff-filter=AMR", baseCommit) {
		set.files = append(set.files, f)
	}
	for _, f := range gitLines(t, root, "ls-files", "--others", "--exclude-standard") {
		set.untracked[f] = true
		set.files = append(set.files, f)
	}
	return set
}

func (c changeSet) goSources(withTests bool) []string {
	var kept []string
	for _, f := range c.files {
		isTest := strings.HasSuffix(f, "_test.go")
		if strings.HasPrefix(f, "go/") && strings.HasSuffix(f, ".go") && !strings.HasPrefix(f, "go/acs/") && (withTests || !isTest) {
			kept = append(kept, f)
		}
	}
	return kept
}

func (c changeSet) addedLines(t *testing.T, rel string, total int) map[int]bool {
	t.Helper()
	added := map[int]bool{}
	if c.untracked[rel] {
		for line := 1; line <= total; line++ {
			added[line] = true
		}
		return added
	}
	for _, row := range gitLines(t, c.root, "diff", "-U0", baseCommit, "--", rel) {
		m := diffHunk.FindStringSubmatch(row)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for line := start; line < start+count; line++ {
			added[line] = true
		}
	}
	return added
}

func parseSource(t *testing.T, root, rel string) (*token.FileSet, *ast.File, int) {
	t.Helper()
	path := filepath.Join(root, rel)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	return fset, file, fset.File(file.Pos()).LineCount()
}

type changedFunc struct {
	pkg, name string
}

func (c changeSet) changedFuncs(t *testing.T) []changedFunc {
	t.Helper()
	var funcs []changedFunc
	for _, rel := range c.goSources(false) {
		fset, file, lines := parseSource(t, c.root, rel)
		added := c.addedLines(t, rel, lines)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			for line := fset.Position(fn.Pos()).Line; line <= fset.Position(fn.End()).Line; line++ {
				if added[line] {
					funcs = append(funcs, changedFunc{pkg: "./" + strings.TrimPrefix(filepath.Dir(rel), "go/"), name: fn.Name.Name})
					break
				}
			}
		}
	}
	return funcs
}

func (c changeSet) has(rel string) bool {
	for _, f := range c.files {
		if f == rel {
			return true
		}
	}
	return false
}

var packagesTooSlowForAPredicate = map[string]bool{"./cmd/evolve": true, "./internal/core": true}

func TestC2376_005_EveryChangedFunctionIsFullyCovered(t *testing.T) {
	set := readChangeSet(t)
	funcs := set.changedFuncs(t)
	if len(funcs) == 0 {
		t.Fatalf("no non-test Go function changed since %s; the fix changes production code", baseCommit)
	}
	module := moduleDir(t)
	byPackage := map[string][]string{}
	for _, f := range funcs {
		if packagesTooSlowForAPredicate[f.pkg] {
			t.Logf("%s is covered by its own suite, not by this predicate", f.pkg)
			continue
		}
		byPackage[f.pkg] = append(byPackage[f.pkg], f.name)
	}
	for pkg, names := range byPackage {
		profile := filepath.Join(t.TempDir(), "cover.out")
		test := exec.Command("go", "test", "-count=1", "-coverprofile="+profile, pkg)
		test.Dir = module
		if out, err := test.CombinedOutput(); err != nil {
			t.Errorf("go test %s: %v: %s", pkg, err, acsassert.OutputTail(string(out), 20))
			continue
		}
		report := exec.Command("go", "tool", "cover", "-func="+profile)
		report.Dir = module
		out, err := report.Output()
		if err != nil {
			t.Errorf("go tool cover %s: %v", pkg, err)
			continue
		}
		for _, name := range names {
			pct, err := acsassert.FuncCoverage(string(out), name)
			if err != nil || pct < 100 {
				t.Errorf("%s.%s is covered %.1f%% (%v); each changed function needs 100%%", pkg, name, pct, err)
			}
		}
	}
}

var directiveComment = regexp.MustCompile(`^//(go:|nolint|line |export |\s?acs-predicate:|\s?\+build)`)

func TestC2376_006_NoGoFileTheChangeTouchesGainsAComment(t *testing.T) {
	set := readChangeSet(t)
	sources := set.goSources(true)
	if len(sources) == 0 {
		t.Fatalf("no Go file changed since %s; the fix changes production code", baseCommit)
	}
	for _, rel := range sources {
		fset, file, lines := parseSource(t, set.root, rel)
		added := set.addedLines(t, rel, lines)
		for _, group := range file.Comments {
			for _, c := range group.List {
				line := fset.Position(c.Pos()).Line
				if added[line] && !directiveComment.MatchString(c.Text) {
					t.Errorf("%s:%d adds a code comment: %s", rel, line, c.Text)
				}
			}
		}
	}
}

func TestC2376_007_TheBridgeAndGcPackageNotesChangeAndPassTheSteLint(t *testing.T) {
	set := readChangeSet(t)
	for _, note := range []string{bridgeNote, gcNote} {
		if !set.has(note) {
			t.Errorf("%s is unchanged since %s", note, baseCommit)
		}
	}
	cmd := exec.Command(evolveBinary(t), "docs", "ste-lint", "--strict", "--changed", baseCommit, "--added", "--project-root", set.root)
	cmd.Dir = set.root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("evolve docs ste-lint --strict reports findings in the added text: %v: %s", err, acsassert.OutputTail(string(out), 25))
	}
}
