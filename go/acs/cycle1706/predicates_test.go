//go:build acs

package cycle1706

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var _ func(testing.TB) *gittest.Repo = gittest.Fixture

const (
	corePkg        = "./internal/core"
	coreRun        = "^(TestDefaultBuildFloorChecks_IncludesPersonaBudgetCheck|TestLaneStartRef_IntegrationHeadAuthority)$"
	routingtestPkg = "./internal/routingtest"
	routingtestRun = "^TestEngine_RunAllExecutesPureAndCycleSpecs$"
	gittestPkg     = "./internal/gittest"
)

type fakeTB struct {
	testing.TB
	mu       sync.Mutex
	failed   bool
	skipped  bool
	lines    []string
	cleanups []func()
	tempDir  func() string
}

func newFakeTB(t *testing.T) *fakeTB { return &fakeTB{TB: t} }

func (f *fakeTB) note(fail, skip bool, s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, s)
	f.failed = f.failed || fail
	f.skipped = f.skipped || skip
}

func (f *fakeTB) Helper()                         {}
func (f *fakeTB) Log(args ...any)                 { f.note(false, false, fmt.Sprintln(args...)) }
func (f *fakeTB) Logf(format string, args ...any) { f.note(false, false, fmt.Sprintf(format, args...)) }
func (f *fakeTB) Error(args ...any)               { f.note(true, false, fmt.Sprintln(args...)) }
func (f *fakeTB) Errorf(format string, args ...any) {
	f.note(true, false, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Fail()                             { f.note(true, false, "") }
func (f *fakeTB) FailNow()                          { f.Fail(); runtime.Goexit() }
func (f *fakeTB) Fatal(args ...any)                 { f.Error(args...); runtime.Goexit() }
func (f *fakeTB) Fatalf(format string, args ...any) { f.Errorf(format, args...); runtime.Goexit() }
func (f *fakeTB) Skip(args ...any)                  { f.note(false, true, fmt.Sprintln(args...)); runtime.Goexit() }
func (f *fakeTB) Skipf(format string, args ...any) {
	f.note(false, true, fmt.Sprintf(format, args...))
	runtime.Goexit()
}
func (f *fakeTB) SkipNow() { f.note(false, true, ""); runtime.Goexit() }

func (f *fakeTB) Failed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failed
}

func (f *fakeTB) Skipped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.skipped
}

func (f *fakeTB) Cleanup(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups = append(f.cleanups, fn)
}

func (f *fakeTB) TempDir() string {
	if f.tempDir != nil {
		return f.tempDir()
	}
	return f.TB.TempDir()
}

func (f *fakeTB) transcript() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.lines, "\n")
}

func (f *fakeTB) runCleanups() {
	f.mu.Lock()
	fns := f.cleanups
	f.cleanups = nil
	f.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		within(fns[i])
	}
}

func within(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
}

func isolateGitConfig(t *testing.T, global string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

const hostileMaintenance = "[maintenance]\n\tauto = true\n\tautoDetach = true\n[gc]\n\tautoDetach = true\n"

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions; the irremovable-fixture shape cannot be built")
	}
}

func rawGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", dir}, args...)...)
	if code != 0 {
		t.Fatalf("git -C %s %v exited %d (err=%v): %s", dir, args, code, err, errOut)
	}
	return strings.TrimSpace(out)
}

func gitBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false", "no", "off", "0", "":
		return false
	}
	return true
}

func detachesMaintenance(configList string) (bool, string) {
	last := map[string]string{}
	for _, line := range strings.Split(configList, "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		k, v, hasValue := strings.Cut(line, "=")
		if !hasValue {
			v = "true"
		}
		last[strings.ToLower(k)] = v
	}
	if v, ok := last["maintenance.auto"]; ok && !gitBool(v) {
		return false, "maintenance.auto=" + v
	}
	for _, key := range []string{"maintenance.autodetach", "gc.autodetach"} {
		if v, ok := last[key]; ok {
			return gitBool(v), key + "=" + v
		}
	}
	return true, "maintenance.auto, maintenance.autoDetach and gc.autoDetach all default (detached child)"
}

func mentionsPath(msg, path string) bool {
	if strings.Contains(msg, path) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && strings.Contains(msg, resolved)
}

func irremovable(t *testing.T, repoDir string) string {
	t.Helper()
	locked := filepath.Join(repoDir, "c1706-held")
	pinned := filepath.Join(locked, "pinned")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pinned, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	chflags := ""
	if runtime.GOOS == "darwin" {
		var err error
		if chflags, err = exec.LookPath("chflags"); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(chflags, "uchg", pinned).CombinedOutput(); err != nil {
			t.Fatalf("chflags uchg: %v: %s", err, out)
		}
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(locked, 0o755)
		if chflags != "" {
			_ = exec.Command(chflags, "nouchg", pinned).Run()
		}
	})
	return locked
}

func fixtureVia(t *testing.T, f *fakeTB) *gittest.Repo {
	t.Helper()
	var repo *gittest.Repo
	within(func() { repo = gittest.Fixture(f) })
	if repo == nil || f.Failed() || f.Skipped() {
		t.Fatalf("gittest.Fixture setup did not produce a repo (failed=%v skipped=%v):\n%s", f.Failed(), f.Skipped(), f.transcript())
	}
	return repo
}

func TestC1706_001_FixtureBuildsACommittableRepoGoneAtTestEnd(t *testing.T) {
	isolateGitConfig(t, "")
	var dirs []string
	ok := t.Run("fixture", func(st *testing.T) {
		a, b := gittest.Fixture(st), gittest.Fixture(st)
		if a == nil || b == nil {
			st.Fatalf("Fixture returned nil (a=%v b=%v)", a, b)
		}
		dirs = []string{a.Dir, b.Dir}
		for _, r := range []*gittest.Repo{a, b} {
			if !filepath.IsAbs(r.Dir) {
				st.Errorf("Repo.Dir %q is not absolute", r.Dir)
			}
			if got := r.Git("rev-parse", "--is-inside-work-tree"); got != "true" {
				st.Errorf("Repo.Git(rev-parse --is-inside-work-tree) = %q, want \"true\" (trimmed)", got)
			}
			top, _ := filepath.EvalSymlinks(rawGit(st, r.Dir, "rev-parse", "--show-toplevel"))
			if dir, _ := filepath.EvalSymlinks(r.Dir); top != dir {
				st.Errorf("Repo.Dir %q is not the work-tree root (git says %q)", r.Dir, top)
			}
			r.Git("commit", "--allow-empty", "-q", "-m", "c1706 identity probe")
			if got := rawGit(st, r.Dir, "log", "-1", "--format=%s"); got != "c1706 identity probe" {
				st.Errorf("commit through Repo.Git did not land: HEAD subject %q", got)
			}
		}
		if rel, err := filepath.Rel(a.Dir, b.Dir); a.Dir == b.Dir || (err == nil && !strings.HasPrefix(rel, "..")) {
			st.Errorf("two fixtures share or nest a tree: %q and %q", a.Dir, b.Dir)
		}
	})
	if !ok {
		t.Fatal("the fixture subtest failed (see above)")
	}
	for _, dir := range dirs {
		if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("fixture tree %s survived the test that built it (Lstat err=%v)", dir, err)
		}
	}
}

func TestC1706_002_FixtureAndGitFailLoudly(t *testing.T) {
	isolateGitConfig(t, "")
	skipIfRoot(t)

	t.Run("unwritable root", func(t *testing.T) {
		ro := t.TempDir()
		if err := os.Chmod(ro, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
		f := newFakeTB(t)
		f.tempDir = func() string { return ro }
		var repo *gittest.Repo
		within(func() { repo = gittest.Fixture(f) })
		defer f.runCleanups()
		switch {
		case f.Skipped():
			t.Errorf("Fixture SKIPPED over an unwritable root — a skip hides the broken fixture:\n%s", f.transcript())
		case !f.Failed():
			t.Errorf("Fixture over an unwritable root (tb.TempDir()=%s) did not fail the test; returned %+v", ro, repo)
		case strings.TrimSpace(f.transcript()) == "":
			t.Errorf("Fixture failed with an empty message")
		}
	})

	t.Run("failing git command", func(t *testing.T) {
		f := newFakeTB(t)
		repo := fixtureVia(t, f)
		defer f.runCleanups()
		within(func() { repo.Git("rev-parse", "--verify", "refs/heads/c1706-no-such-branch") })
		if !f.Failed() {
			t.Fatalf("Repo.Git of a failing git command did not fail the test:\n%s", f.transcript())
		}
		if !strings.Contains(f.transcript(), "c1706-no-such-branch") {
			t.Errorf("failure lacks the failing git arguments (no context to debug from):\n%s", f.transcript())
		}
	})
}

var detachedChildRE = regexp.MustCompile(`(?m)run_command:.*\bmaintenance run\b.*\s--detach(\s|$)`)

func TestC1706_003_FixtureReposNeverDetachBackgroundMaintenance(t *testing.T) {
	isolateGitConfig(t, hostileMaintenance)
	ok := t.Run("fixture", func(st *testing.T) {
		repo := gittest.Fixture(st)
		if d, why := detachesMaintenance(repo.Git("config", "--list")); d {
			st.Errorf("through the helper, a commit would detach a maintenance child (%s)", why)
		}
		if d, why := detachesMaintenance(rawGit(st, repo.Dir, "config", "--list")); d {
			st.Errorf("a raw git in the fixture (production code's path) would detach a maintenance child (%s) — persist the setting in the repo's own config", why)
		}
		trace := filepath.Join(st.TempDir(), "git-trace")
		st.Setenv("GIT_TRACE", trace)
		if err := os.WriteFile(filepath.Join(repo.Dir, "c1706.txt"), []byte("probe\n"), 0o644); err != nil {
			st.Fatal(err)
		}
		repo.Git("add", "c1706.txt")
		repo.Git("commit", "-q", "-m", "c1706 trace probe")
		data, err := os.ReadFile(trace)
		switch {
		case err != nil || len(data) == 0:
			st.Logf("GIT_TRACE did not reach the helper's git (env scrubbed?) — (a)/(b) carry the verdict")
		case detachedChildRE.Match(data):
			st.Errorf("a commit through the fixture spawned a detached maintenance child:\n%s", detachedChildRE.Find(data))
		}
	})
	if !ok {
		t.Fatal("the fixture subtest failed (see above)")
	}
}

const gitShim = `#!/bin/bash
real='%s'
log="${EVOLVE_C1706_GITLOG:-}"
args=("$@")
globals=()
sub=""
k=0
while [ "$k" -lt "${#args[@]}" ]; do
  a="${args[$k]}"
  case "$a" in
    -C|-c) globals+=("$a" "${args[$((k+1))]}"); k=$((k+2)) ;;
    -*) globals+=("$a"); k=$((k+1)) ;;
    *) sub="$a"; break ;;
  esac
done
if [ -n "$log" ]; then
  case "$sub" in
    commit|fetch)
      get() { "$real" "${globals[@]}" config --type=bool --get "$1" 2>/dev/null || echo unset; }
      printf '%%s\tmaintenance.auto=%%s\tmaintenance.autodetach=%%s\tgc.autodetach=%%s\tcwd=%%s\n' \
        "$sub" "$(get maintenance.auto)" "$(get maintenance.autodetach)" "$(get gc.autodetach)" "$PWD" >> "$log"
      ;;
  esac
fi
exec "$real" "$@"
`

func shimRecordDetaches(line string) (bool, string) {
	var list []string
	for _, field := range strings.Split(line, "\t")[1:] {
		if k, v, _ := strings.Cut(field, "="); k != "cwd" && v != "unset" {
			list = append(list, field)
		}
	}
	return detachesMaintenance(strings.Join(list, "\n"))
}

func TestC1706_004_NamedTestsBuildTheirReposThroughTheHelper(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, rel := range []string{
		"go/internal/core/build_persona_budget_check_test.go",
		"go/internal/core/worktree_startref_test.go",
		"go/internal/routingtest/engine.go",
	} {
		acsassert.FileContains(t, filepath.Join(root, rel), "gittest.")
	}
	realGit, err := exec.LookPath("git")
	if err != nil || strings.Contains(realGit, "'") {
		t.Fatalf("need a real git on PATH (got %q, err=%v)", realGit, err)
	}
	shimDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(fmt.Sprintf(gitShim, realGit)), 0o755); err != nil {
		t.Fatal(err)
	}
	isolateGitConfig(t, hostileMaintenance+"[user]\n\tname = c1706\n\temail = c1706@example.com\n")
	t.Run(corePkg, func(t *testing.T) { shimmedGoTest(t, goDir, shimDir, coreRun, corePkg) })
	t.Run(routingtestPkg, func(t *testing.T) { shimmedGoTest(t, goDir, shimDir, routingtestRun, routingtestPkg) })
}

func shimmedGoTest(t *testing.T, goDir, shimDir, run, pkg string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "git-calls.tsv")
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", run, pkg)
	cmd.Dir = goDir
	cmd.Env = append(os.Environ(), "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"), "EVOLVE_C1706_GITLOG="+logPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -run %q %s under the git shim failed: %v\n%s", run, pkg, err, out)
	}
	assertNamedPasses(t, string(out), run, 1)
	assertQuietRepos(t, logPath)
}

func assertNamedPasses(t *testing.T, out, run string, want int) {
	t.Helper()
	for _, name := range strings.Split(strings.Trim(run, "^$()"), "|") {
		if n := strings.Count(out, "--- PASS: "+name+" "); n < want {
			t.Errorf("%s: %d PASS line(s), want >= %d (renamed, skipped, or failing?)", name, n, want)
		}
	}
}

func assertQuietRepos(t *testing.T, logPath string) {
	t.Helper()
	data, err := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if err != nil || len(data) == 0 {
		t.Fatalf("the git shim recorded no commit/fetch (err=%v) — the shim was bypassed, the predicate proves nothing", err)
	}
	var loud []string
	for _, line := range lines {
		if d, why := shimRecordDetaches(line); d {
			loud = append(loud, why+"  <=  "+line)
		}
	}
	if len(loud) > 0 {
		t.Errorf("%d of %d commit/fetch calls ran in a repo that detaches a maintenance child (repo not built through gittest, or its config not persisted):\n%s",
			len(loud), len(lines), strings.Join(loud, "\n"))
	}
}

type lateWriter struct {
	cancel     context.CancelFunc
	done       chan struct{}
	overlapped atomic.Bool
	wrote      atomic.Int64
}

func startLateWriter(t *testing.T, objects string) *lateWriter {
	t.Helper()
	leaf := filepath.Join(objects, "c1706-late")
	sentinel := filepath.Join(leaf, "sentinel")
	if err := os.Mkdir(leaf, 0o755); err != nil {
		t.Fatalf("late writer: %v", err)
	}
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatalf("late writer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &lateWriter{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for i := 0; ctx.Err() == nil; i++ {
			if _, err := os.Lstat(sentinel); errors.Is(err, fs.ErrNotExist) {
				w.overlapped.Store(true)
				_ = os.Mkdir(leaf, 0o755)
				_ = os.WriteFile(filepath.Join(leaf, "tmp_obj_late"), []byte("late"), 0o644)
				return
			}
			if os.WriteFile(filepath.Join(leaf, "tmp_obj_"+strconv.Itoa(i%8)), []byte("pack"), 0o644) == nil {
				w.wrote.Add(1)
			}
			runtime.Gosched()
		}
	}()
	return w
}

func (w *lateWriter) stop() {
	w.cancel()
	<-w.done
}

func TestC1706_005_TeardownOutlastsAWriterThatOutlivesItsTest(t *testing.T) {
	isolateGitConfig(t, "")
	var dir string
	var w *lateWriter
	ok := t.Run("late writer", func(st *testing.T) {
		repo := gittest.Fixture(st)
		dir = repo.Dir
		w = startLateWriter(st, repo.Git("rev-parse", "--absolute-git-dir")+"/objects")
	})
	if w == nil {
		t.Fatal("the late writer never started (fixture setup failed above)")
	}
	w.stop()
	if !ok {
		t.Errorf("the test failed in teardown: a writer that outlived the test body was not absorbed by a retried removal (see the subtest output above)")
	}
	if !w.overlapped.Load() || w.wrote.Load() == 0 {
		t.Errorf("the race was not exercised (writer wrote %d files, saw the teardown: %v)", w.wrote.Load(), w.overlapped.Load())
	}
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("fixture tree %s survived its teardown (Lstat err=%v)", dir, err)
	}
}

func TestC1706_006_RetryExhaustionNamesTheHoldingProcess(t *testing.T) {
	isolateGitConfig(t, "")
	skipIfRoot(t)
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not installed here; 007 pins the explicit-fallback contract")
	}
	f := newFakeTB(t)
	repo := fixtureVia(t, f)
	locked := irremovable(t, repo.Dir)

	ctx, cancel := context.WithCancel(context.Background())
	holder := exec.CommandContext(ctx, "sleep", "600")
	holder.Dir = locked
	holder.WaitDelay = 5 * time.Second
	if err := holder.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = holder.Wait()
	}()

	f.runCleanups()
	msg := f.transcript()
	if !f.Failed() {
		t.Fatalf("teardown of an irremovable fixture did not fail the test — the error was swallowed:\n%s", msg)
	}
	pid := strconv.Itoa(holder.Process.Pid)
	if !regexp.MustCompile(`(^|\D)` + pid + `(\D|$)`).MatchString(msg) {
		t.Errorf("the failure names no holding process — want PID %s (sleep, cwd %s) in:\n%s", pid, locked, msg)
	}
	if !mentionsPath(msg, repo.Dir) {
		t.Errorf("the failure omits the fixture path %s:\n%s", repo.Dir, msg)
	}
}

func TestC1706_007_UnavailableDiagnosticIsStatedNotOmitted(t *testing.T) {
	isolateGitConfig(t, "")
	skipIfRoot(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f := newFakeTB(t)
	repo := fixtureVia(t, f)
	irremovable(t, repo.Dir)

	bin := t.TempDir()
	if err := os.Symlink(realGit, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	f.runCleanups()
	msg := f.transcript()
	if !f.Failed() {
		t.Fatalf("teardown of an irremovable fixture did not fail the test — the error was swallowed:\n%s", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "diagnostic unavailable") {
		t.Errorf("with no lsof on PATH the failure must say \"diagnostic unavailable\":\n%s", msg)
	}
	if !mentionsPath(msg, repo.Dir) {
		t.Errorf("the failure omits the fixture path %s:\n%s", repo.Dir, msg)
	}
}

func TestC1706_008_StressRunOfTheAffectedTestsIsGreen(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	t.Logf("platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	t.Run(corePkg, func(t *testing.T) {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", goDir, "-race", "-count=20", "-v", "-run", coreRun, corePkg)
		assertStressGreen(t, stdout+stderr, code, err, corePkg)
		assertNamedPasses(t, stdout+stderr, coreRun, 20)
	})
	t.Run(routingtestPkg, func(t *testing.T) {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", goDir, "-race", "-count=20", "-v", "-run", routingtestRun, routingtestPkg)
		assertStressGreen(t, stdout+stderr, code, err, routingtestPkg)
		assertNamedPasses(t, stdout+stderr, routingtestRun, 20)
	})
	t.Run(gittestPkg, func(t *testing.T) {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", goDir, "-race", "-count=20", "-run", ".", gittestPkg)
		out := stdout + stderr
		assertStressGreen(t, out, code, err, gittestPkg)
		if strings.Contains(out, "[no test files]") || !strings.Contains(out, "ok ") {
			t.Errorf("%s ran no tests — the helper needs a suite of its own:\n%s", gittestPkg, out)
		}
	})
}

func assertStressGreen(t *testing.T, out string, code int, err error, pkg string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("go test -race -count=20 %s exited %d (err=%v)\n%s", pkg, code, err, out)
	}
	for _, sig := range []string{"directory not empty", "unlinkat", "RemoveAll cleanup"} {
		if strings.Contains(out, sig) {
			t.Errorf("%s stress output carries the cleanup-race signature %q", pkg, sig)
		}
	}
}

func TestC1706_009_GittestGraduatesUnderApicover(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	pkgDir := filepath.Join(goDir, "internal", "gittest")
	enrolled := false
	if data, err := os.ReadFile(filepath.Join(goDir, ".apicover-enforce")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			enrolled = enrolled || strings.TrimSpace(line) == gittestPkg
		}
	}
	if !enrolled {
		t.Errorf("./internal/gittest is not a line of go/.apicover-enforce")
	}
	for _, rel := range []string{"go/internal/gittest/fixture.go", "go/internal/gittest/apicover_named_test.go"} {
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("%s is not git-tracked (missing, or dropped at ship)", rel)
		}
	}
	ctx := context.Background()
	syms, err := apicover.Enumerate(ctx, pkgDir)
	if err != nil {
		t.Fatalf("apicover.Enumerate(gittest): %v", err)
	}
	want := map[string]bool{"Fixture": false, "Repo": false, "Repo.Git": false}
	for _, s := range syms {
		if _, ok := want[s.Name]; ok {
			want[s.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("apicover does not enumerate %s as an export of internal/gittest", name)
		}
	}
	for _, s := range apicover.MissingDoc(syms) {
		t.Errorf("export %s has no godoc", s.Name)
	}
	cov := filepath.Join(t.TempDir(), "coverage.txt")
	if out, errOut, code, _ := acsassert.SubprocessOutput("go", "test", "-C", goDir, "-count=1", "-tags", "integration", "-coverprofile="+cov, gittestPkg); code != 0 {
		t.Fatalf("go test -coverprofile ./internal/gittest exited %d\n%s%s", code, out, errOut)
	}
	funcOut, errOut, code, _ := acsassert.SubprocessOutput("go", "tool", "-C", goDir, "cover", "-func="+cov)
	if code != 0 {
		t.Fatalf("go tool cover -func exited %d: %s", code, errOut)
	}
	funcFile := filepath.Join(t.TempDir(), "coverage.func.txt")
	if err := os.WriteFile(funcFile, []byte(funcOut), 0o644); err != nil {
		t.Fatal(err)
	}
	var report strings.Builder
	rc, err := apicover.Run(ctx, apicover.Config{Dirs: []string{pkgDir}, CoverPath: funcFile, Enforce: true, RequireDoc: true}, &report)
	if err != nil || rc != 0 {
		t.Errorf("apicover enforce over internal/gittest: rc=%d err=%v\n%s", rc, err, report.String())
	}
}
