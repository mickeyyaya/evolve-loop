package bridge

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fakeTmuxOnPath(t *testing.T, newSessionOut string, newSessionRC int, consume bool) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	pane := ""
	if consume {
		pane = "for a; do case \"$a\" in *cellenv*) f=\"${a##* }\"; /bin/rm -rf \"${f%/*}\";; esac; done; "
	}
	script := "#!/bin/sh\necho \"$*\" >> " + shellQuotePOSIX(log) + "\ncase \"$*\" in *new-session*) " + pane + "printf '%s' " +
		shellQuotePOSIX(newSessionOut) + "; exit " + strconv.Itoa(newSessionRC) + ";; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("TMPDIR", t.TempDir())
	return log
}

func cellEnvLeftovers(t *testing.T) []string {
	t.Helper()
	got, _ := filepath.Glob(filepath.Join(os.TempDir(), cellEnvPrefix+"*"))
	return got
}

func TestNewSessionEnv_APaneThatNeverReadsItsFileFailsTheStartRemovesTheFileAndKillsOnlyItsOwnSessionID(t *testing.T) {
	log := fakeTmuxOnPath(t, "$7\n", 0, false)
	defer func(old time.Duration) { cellEnvConsumeTimeout = old }(cellEnvConsumeTimeout)
	cellEnvConsumeTimeout = 20 * time.Millisecond
	err := (execTmux{}).NewSessionEnv(context.Background(), "mine", 80, 24, "", []string{"PATH=/bin", "SECRET=fake"})
	if err == nil || !strings.Contains(err.Error(), "did not read its cell env file") {
		t.Fatalf("a pane that never reads its env file must fail the start; got %v", err)
	}
	if left := cellEnvLeftovers(t); len(left) != 0 {
		t.Fatalf("the failed start left its env file: %v", left)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "kill-session -t $7") || strings.Contains(string(calls), "kill-session -t =mine:") {
		t.Fatalf("the start must kill the session by the id that it created, never by name; calls:\n%s", calls)
	}
}

func TestNewSessionEnv_AStartThatPrintsNoSessionIDKillsNothing(t *testing.T) {
	log := fakeTmuxOnPath(t, "duplicate session: theirs\n", 1, false)
	err := (execTmux{}).NewSessionEnv(context.Background(), "theirs", 80, 24, "", []string{"PATH=/bin"})
	if err == nil {
		t.Fatal("a failed new-session must fail the start")
	}
	if left := cellEnvLeftovers(t); len(left) != 0 {
		t.Fatalf("the failed start left its env file: %v", left)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "kill-session") {
		t.Fatalf("a start that created no session must kill nothing; calls:\n%s", calls)
	}
}

func TestNewSessionEnv_AFailedStartNamesTheOutputOfTmux(t *testing.T) {
	fakeTmuxOnPath(t, "server exited unexpectedly\n", 1, false)
	err := (execTmux{}).NewSessionEnv(context.Background(), "early-1", 80, 24, "", []string{"PATH=/bin"})
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("the start must keep the exit error of tmux; got %v", err)
	}
	if got := err.Error(); !strings.HasPrefix(got, "tmux new-session: exit status 1: ") || !strings.HasSuffix(got, "server exited unexpectedly") {
		t.Fatalf("the start error must name the command and the output of tmux; got %q", got)
	}
}

func TestWithTmuxOutput_WrapsOnlyAFailure(t *testing.T) {
	t.Parallel()
	if err := withTmuxOutput(nil, []string{"kill-session"}, "noise\n"); err != nil {
		t.Fatalf("a success must stay nil; got %v", err)
	}
	cause := errors.New("exit status 1")
	err := withTmuxOutput(cause, []string{"kill-session", "-t", "=x:"}, "can't find session: x\n")
	if !errors.Is(err, cause) || err.Error() != "tmux kill-session: exit status 1: can't find session: x" {
		t.Fatalf("a failure must keep its cause and add the output of tmux; got %v", err)
	}
	if err := withTmuxOutput(cause, []string{"has-session"}, " \n"); !errors.Is(err, cause) || err.Error() != "tmux has-session: exit status 1" {
		t.Fatalf("a failure with no output must end at its cause; got %v", err)
	}
}

func TestCreatedSessionID_ReadsOnlyATmuxSessionIDLine(t *testing.T) {
	t.Parallel()
	for out, want := range map[string]string{
		"$12\n":                        "$12",
		"warning\n  $3  \n":            "$3",
		"duplicate session: dup\n":     "",
		"$abc\n":                       "",
		"":                             "",
		"$1; kill-server\n":            "",
		"no current client\n$4\nextra": "$4",
	} {
		if got := createdSessionID(out); got != want {
			t.Errorf("createdSessionID(%q) = %q; want %q", out, got, want)
		}
	}
}

func TestAwaitCellEnvConsumed_ReturnsWhenThePaneRemovesTheDirAndFailsAtTheDeadline(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "cell")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := awaitCellEnvConsumed(context.Background(), dir, 10*time.Millisecond); err == nil {
		t.Fatal("a dir that stays must fail the wait at the deadline")
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = os.Remove(dir)
	}()
	if err := awaitCellEnvConsumed(context.Background(), dir, 5*time.Second); err != nil {
		t.Fatalf("the wait must end when the pane removes the dir: %v", err)
	}
}

func staleCellEnvDir(t *testing.T, tmp, name, body string) string {
	t.Helper()
	dir := filepath.Join(tmp, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if body == "" {
		body = cellEnvHeader(dir) + "SECRET=fake\n"
	}
	if err := os.WriteFile(filepath.Join(dir, cellEnvFileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSweepStaleCellEnv_RemovesOnlyOldDirsThatCarryTheirOwnBridgeHeader(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	now := time.Now().Add(2 * cellEnvStaleAfter)
	stale := staleCellEnvDir(t, tmp, cellEnvPrefix+"stale", "")
	foreignHeader := staleCellEnvDir(t, tmp, cellEnvPrefix+"foreign", cellEnvHeader("/elsewhere")+"X=1\n")
	short := staleCellEnvDir(t, tmp, cellEnvPrefix+"short", "/bin")
	extra := staleCellEnvDir(t, tmp, cellEnvPrefix+"extra", "")
	if err := os.WriteFile(filepath.Join(extra, "other"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	notOurs := staleCellEnvDir(t, tmp, "other-prefix", "")
	subdir := filepath.Join(tmp, cellEnvPrefix+"subdir")
	if err := os.MkdirAll(filepath.Join(subdir, cellEnvFileName), 0o700); err != nil {
		t.Fatal(err)
	}
	unreadable := staleCellEnvDir(t, tmp, cellEnvPrefix+"unreadable", "")
	if err := os.Chmod(filepath.Join(unreadable, cellEnvFileName), 0); err != nil {
		t.Fatal(err)
	}
	locked := staleCellEnvDir(t, tmp, cellEnvPrefix+"locked", "")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	file := filepath.Join(tmp, cellEnvPrefix+"file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, cellEnvPrefix+"link")
	if err := os.Symlink(stale, link); err != nil {
		t.Fatal(err)
	}

	sweepStaleCellEnv(tmp, now)

	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Errorf("an old dir with its own bridge header must go; stat err = %v", err)
	}
	for _, kept := range []string{foreignHeader, short, extra, notOurs, subdir, locked, file, link} {
		if _, err := os.Lstat(kept); err != nil {
			t.Errorf("%s is not a bridge env dir and must stay: %v", filepath.Base(kept), err)
		}
	}
	if os.Getuid() != 0 {
		if _, err := os.Lstat(unreadable); err != nil {
			t.Errorf("an env file that cannot be read cannot prove its header and must stay: %v", err)
		}
	}
}

func TestSweepStaleCellEnv_KeepsAFreshDirAndToleratesAMissingTempDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	fresh := staleCellEnvDir(t, tmp, cellEnvPrefix+"fresh", "")
	sweepStaleCellEnv(tmp, time.Now())
	if _, err := os.Lstat(fresh); err != nil {
		t.Fatalf("a dir younger than the stale age may belong to a start in progress: %v", err)
	}
	sweepStaleCellEnv(filepath.Join(tmp, "missing"), time.Now())
}

func TestNewSessionEnv_SweepsAStaleEnvDirBeforeItStarts(t *testing.T) {
	fakeTmuxOnPath(t, "dup\n", 1, false)
	stale := staleCellEnvDir(t, os.TempDir(), cellEnvPrefix+"crashed", "")
	old := time.Now().Add(-2 * cellEnvStaleAfter)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	_ = (execTmux{}).NewSessionEnv(context.Background(), "s", 80, 24, "", []string{"PATH=/bin"})
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("the env dir of a crashed start must be swept; stat err = %v", err)
	}
}

func TestNewSessionEnv_ReturnsOnlyAfterThePaneHasRemovedItsFile(t *testing.T) {
	log := fakeTmuxOnPath(t, "$9\n", 0, true)
	if err := (execTmux{}).NewSessionEnv(context.Background(), "ok", 80, 24, "", []string{"PATH=/bin", "SECRET=fake"}); err != nil {
		t.Fatalf("a pane that reads its file must start: %v", err)
	}
	if left := cellEnvLeftovers(t); len(left) != 0 {
		t.Fatalf("a started session must have no env file left: %v", left)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "kill-session") {
		t.Fatalf("a good start kills nothing; calls:\n%s", calls)
	}
}

func TestNewSessionEnv_AnUnwritableTempDirFailsBeforeTmuxRuns(t *testing.T) {
	log := fakeTmuxOnPath(t, "$1\n", 0, true)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if err := (execTmux{}).NewSessionEnv(context.Background(), "s", 80, 24, "", []string{"PATH=/bin"}); err == nil {
		t.Fatal("a start that cannot write its env file must fail")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("tmux must not run without the env file; stat err = %v", err)
	}
}
