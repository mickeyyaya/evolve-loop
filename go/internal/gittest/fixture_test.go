package gittest

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// recordingTB captures Errorf so a teardown failure can be asserted on without
// failing the test that provokes it.
type recordingTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func untilFatal(body func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		body()
	}()
	<-done
}

type gitExecs struct {
	cmds []*exec.Cmd
}

func (g *gitExecs) running(subcommand string) []*exec.Cmd {
	var matched []*exec.Cmd
	for _, cmd := range g.cmds {
		if slices.Contains(cmd.Args, subcommand) {
			matched = append(matched, cmd)
		}
	}
	return matched
}

func failGitExecs(t *testing.T, faults ...error) *gitExecs {
	t.Helper()
	execs := &gitExecs{}
	realCombinedOutput := combinedOutput
	t.Cleanup(func() { combinedOutput = realCombinedOutput })
	combinedOutput = func(cmd *exec.Cmd) ([]byte, error) {
		execs.cmds = append(execs.cmds, cmd)
		if n := len(execs.cmds); n <= len(faults) {
			return nil, faults[n-1]
		}
		return realCombinedOutput(cmd)
	}
	return execs
}

func forkExecGitError(errno syscall.Errno) error {
	return &os.PathError{Op: "fork/exec", Path: "/usr/bin/git", Err: errno}
}

// lockedDir returns a directory RemoveAll cannot finish: a read-only
// subdirectory holding a file. unlock makes it removable again.
func lockedDir(t *testing.T) (dir string, unlock func()) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir = filepath.Join(t.TempDir(), "repo")
	sub := filepath.Join(dir, "held")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	unlock = func() { _ = os.Chmod(sub, 0o755) }
	t.Cleanup(unlock)
	return dir, unlock
}

func TestRemoveWithRetry_OutlastsATransientHold(t *testing.T) {
	dir, unlock := lockedDir(t)
	go func() {
		time.Sleep(3 * teardownBackoff)
		unlock()
	}()
	rec := &recordingTB{TB: t}
	removeWithRetry(rec, dir, teardownAttempts)
	if len(rec.errs) != 0 {
		t.Fatalf("a hold released mid-retry still failed the teardown: %v", rec.errs)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("%s survived the retried removal (Lstat err=%v)", dir, err)
	}
}

func TestRemoveWithRetry_ExhaustionFailsWithPathAndDiagnostic(t *testing.T) {
	dir, _ := lockedDir(t)
	t.Setenv("PATH", t.TempDir()) // no lsof: the fallback must be stated
	rec := &recordingTB{TB: t}
	removeWithRetry(rec, dir, 2)
	if len(rec.errs) != 1 {
		t.Fatalf("exhausted teardown reported %d errors, want 1: %v", len(rec.errs), rec.errs)
	}
	msg := rec.errs[0]
	for _, want := range []string{dir, "after 2 attempts", "diagnostic unavailable"} {
		if !strings.Contains(msg, want) {
			t.Errorf("teardown failure lacks %q:\n%s", want, msg)
		}
	}
}

func TestParseLsof_DistinctCommandAndPID(t *testing.T) {
	out := "COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME\n" +
		"sleep   4242 me    cwd    DIR   1,16       64    9 /tmp/x/repo/held\n" +
		"git     7 me      3r   REG   1,16       10    8 /tmp/x/repo/.git/index\n" +
		"git     7 me      4r   REG   1,16       10    8 /tmp/x/repo/.git/HEAD\n"
	want := []string{"sleep (PID 4242)", "git (PID 7)"}
	if got := parseLsof(out); !reflect.DeepEqual(got, want) {
		t.Errorf("parseLsof = %v, want %v", got, want)
	}
	if got := parseLsof(""); len(got) != 0 {
		t.Errorf("parseLsof(\"\") = %v, want none", got)
	}
}

func TestFixture_RetriesAGitInitWhoseFirstExecFailsTransiently(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transient error
	}{
		{"fork/exec EBADF", forkExecGitError(syscall.EBADF)},
		{"pipe read EBADF", &os.PathError{Op: "read", Path: "|0", Err: syscall.EBADF}},
		{"closed pipe", io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			execs := failGitExecs(t, tc.transient)
			r := Fixture(t)

			inits := execs.running("init")
			if len(inits) != 2 {
				t.Fatalf("git init ran %d times after a %s on its first exec, want 2: the failed exec and one retry", len(inits), tc.name)
			}
			failed, retry := inits[0], inits[1]
			if failed == retry {
				t.Errorf("the retry reused the failed *exec.Cmd; an exec.Cmd runs once, so each attempt needs a fresh one")
			}
			if !slices.Equal(failed.Args, retry.Args) || failed.Dir != retry.Dir {
				t.Errorf("the retry ran %v in %q, want the failed %v in %q again", retry.Args, retry.Dir, failed.Args, failed.Dir)
			}
			if retry.ProcessState == nil || !retry.ProcessState.Success() {
				t.Errorf("the retried git init did not run to success: %v", retry.ProcessState)
			}
			if got := r.Git("symbolic-ref", "--short", "HEAD"); got != "main" {
				t.Errorf("branch = %q after the retried init, want main", got)
			}
			assertPersistsMaintenanceConfig(t, r.Dir)
		})
	}
}

func TestFixture_PersistentEBADFFailsTheTestNamingTheError(t *testing.T) {
	execs := failGitExecs(t, forkExecGitError(syscall.EBADF), forkExecGitError(syscall.EBADF))
	rec := &recordingTB{TB: t}
	untilFatal(func() { Fixture(rec) })

	if inits := execs.running("init"); len(inits) != 2 {
		t.Errorf("git init ran %d times under a persistent EBADF, want 2: one exec and exactly one retry", len(inits))
	}
	if len(rec.errs) != 1 {
		t.Fatalf("a persistent EBADF reported %d failures, want 1: %q", len(rec.errs), rec.errs)
	}
	for _, want := range []string{"git init -q -b main", "fork/exec /usr/bin/git: bad file descriptor"} {
		if !strings.Contains(rec.errs[0], want) {
			t.Errorf("the failure lacks %q:\n%s", want, rec.errs[0])
		}
	}
}

func TestRepoGit_NeverRetriesANonEBADFFailure(t *testing.T) {
	succeedsWhenRun := []string{"rev-parse", "--is-inside-work-tree"}
	for _, tc := range []struct {
		name      string
		faults    []error
		args      []string
		wantFatal string
	}{
		{"git exits non-zero", nil, []string{"rev-parse", "--verify", "refs/heads/absent"}, "exit status"},
		{"fork/exec EMFILE", []error{forkExecGitError(syscall.EMFILE)}, succeedsWhenRun, "too many open files"},
		{"broken pipe", []error{forkExecGitError(syscall.EPIPE)}, succeedsWhenRun, "broken pipe"},
		{"file already closed", []error{os.ErrClosed}, succeedsWhenRun, "file already closed"},
		{"git not on PATH", []error{&exec.Error{Name: "git", Err: exec.ErrNotFound}}, succeedsWhenRun, "executable file not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := Fixture(t).Dir
			execs := failGitExecs(t, tc.faults...)
			rec := &recordingTB{TB: t}
			untilFatal(func() { (&Repo{Dir: dir, tb: rec}).Git(tc.args...) })

			if len(execs.cmds) != 1 {
				t.Errorf("git %v ran %d times after a %s, want 1: a non-EBADF failure is never retried", tc.args, len(execs.cmds), tc.name)
			}
			if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], tc.wantFatal) {
				t.Errorf("failures = %q, want exactly one naming %q", rec.errs, tc.wantFatal)
			}
		})
	}
}
