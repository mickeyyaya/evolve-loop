package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/pruneephemeral"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

func breakWorkingDirectory(t *testing.T) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }
}

func TestRunBoundaryWith_AnUnreadableWorkingDirectoryIsAnIOError(t *testing.T) {
	goal := filepath.Join(t.TempDir(), "goal.md")
	if err := os.WriteFile(goal, []byte("ship it"), 0o644); err != nil {
		t.Fatal(err)
	}
	breakWorkingDirectory(t)
	dispatched := 0
	dispatch := func(string, []string, io.Writer, io.Writer) int { dispatched++; return 0 }
	var stdout, stderr bytes.Buffer

	code := runBoundaryWith(dispatch, []string{"run", "--goal-text-file", goal}, &stdout, &stderr)

	if code != exitIO || stderr.String() != boundaryPrefix+"getwd: no such file or directory\n" {
		t.Errorf("code=%d stderr=%q, want %d and the getwd failure", code, stderr.String(), exitIO)
	}
	if dispatched != 0 {
		t.Errorf("dispatched %d step(s), want none", dispatched)
	}
}

func TestRunBoundaryLog_AnUnreadableWorkingDirectoryIsAnIOError(t *testing.T) {
	breakWorkingDirectory(t)
	var stdout, stderr bytes.Buffer

	code := runBoundaryLog([]string{"--run-id", gcpolicy.LogRunID(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))}, nil, &stdout, &stderr)

	if code != exitIO || stderr.String() != boundaryLogPrefix+"getwd: no such file or directory\n" || stdout.Len() != 0 {
		t.Errorf("code=%d stdout=%q stderr=%q, want %d and only the getwd failure", code, stdout.String(), stderr.String(), exitIO)
	}
}

func TestParseRoutingWriteFlags_AnUnreadableWorkingDirectoryRefuses(t *testing.T) {
	breakWorkingDirectory(t)
	var stderr bytes.Buffer

	_, rest, ok := parseRoutingWriteFlags("set", []string{"tiers.deep", "claude"}, &stderr)

	if ok || rest != nil || !strings.Contains(stderr.String(), "evolve cli-routing set: cannot read the working directory") {
		t.Errorf("ok=%v rest=%v stderr=%q, want a refusal naming the working directory", ok, rest, stderr.String())
	}
}

func TestParseRoutingWriteFlags_AFlaggedRootNeverReadsTheWorkingDirectory(t *testing.T) {
	breakWorkingDirectory(t)
	root := t.TempDir()
	var stderr bytes.Buffer

	f, rest, ok := parseRoutingWriteFlags("set", []string{"--project-root", root, "--dry-run", "tiers.deep", "claude"}, &stderr)

	if !ok || f.root != root || !f.dryRun || len(rest) != 2 || rest[0] != "tiers.deep" {
		t.Errorf("= (%+v, %v, %v) stderr=%q, want the flagged root and both operands", f, rest, ok, stderr.String())
	}
}

func TestDefaultShipRepinProvenance_VerifiesAncestryAgainstHEAD(t *testing.T) {
	r := gittest.Fixture(t)
	for _, body := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(r.Dir, "f.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		r.Git("add", "f.txt")
		r.Git("commit", "-q", "-m", body)
	}
	parent := r.Git("rev-parse", "HEAD~1")
	r.Git("checkout", "-q", "-b", "side", parent)
	if err := os.WriteFile(filepath.Join(r.Dir, "g.txt"), []byte("side"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "g.txt")
	r.Git("commit", "-q", "-m", "side")
	sideTip := r.Git("rev-parse", "HEAD")
	r.Git("checkout", "-q", "main")

	commit, verified := defaultShipRepinProvenance(r.Dir)

	if commit != version.Commit() {
		t.Errorf("commit = %q, want %q", commit, version.Commit())
	}
	for sha, want := range map[string]bool{"": false, parent: true, "HEAD": true, sideTip: false, "0123456789abcdef0123456789abcdef01234567": false} {
		if got := verified(sha); got != want {
			t.Errorf("verified(%q) = %v, want %v", sha, got, want)
		}
	}
}

func TestDefaultLoopDetachCommand(t *testing.T) {
	t.Run("an unknown executable is an error", func(t *testing.T) {
		prev := loopDetachExecutable
		t.Cleanup(func() { loopDetachExecutable = prev })
		loopDetachExecutable = func() (string, error) { return "", errors.New("executable unknown") }

		cmd, err := defaultLoopDetachCommand([]string{"--goal-text", "g"})

		if cmd != nil || err == nil || err.Error() != "executable unknown" {
			t.Errorf("= (%v, %v), want (nil, executable unknown)", cmd, err)
		}
	})
	t.Run("the child re-runs this binary as loop with the same argv", func(t *testing.T) {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}

		cmd, err := defaultLoopDetachCommand([]string{"--goal-text", "g", "--max-cycles", "2"})

		if err != nil {
			t.Fatal(err)
		}
		want := []string{self, "loop", "--goal-text", "g", "--max-cycles", "2"}
		if cmd.Path != self || strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
			t.Errorf("cmd = %s %v, want %v", cmd.Path, cmd.Args, want)
		}
	})
}

func TestRunLoopDetached_AnUnsupportedPlatformRefusesBeforeLaunching(t *testing.T) {
	prev := loopDetachAttr
	t.Cleanup(func() { loopDetachAttr = prev })
	loopDetachAttr = func() (*syscall.SysProcAttr, error) { return nil, errors.New("--detach is unsupported on plan9") }
	launched := false
	prevCmd := loopDetachCommandFn
	t.Cleanup(func() { loopDetachCommandFn = prevCmd })
	loopDetachCommandFn = func([]string) (*exec.Cmd, error) { launched = true; return nil, errors.New("no") }
	var stdout, stderr bytes.Buffer

	code := runLoopDetached(loopConfig{EvolveDir: t.TempDir(), LogPath: filepath.Join(t.TempDir(), "loop.log")}, &stdout, &stderr)

	if code != 1 || stderr.String() != "evolve loop: --detach: --detach is unsupported on plan9\n" || launched {
		t.Errorf("code=%d stderr=%q launched=%v, want 1, the platform refusal and no launch", code, stderr.String(), launched)
	}
}

func TestRunPruneEphemeral_APruneFailureExits1(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	prev := pruneEphemeralRun
	t.Cleanup(func() { pruneEphemeralRun = prev })
	var got pruneephemeral.Options
	pruneEphemeralRun = func(o pruneephemeral.Options) (pruneephemeral.Result, error) {
		got = o
		return pruneephemeral.Result{}, errors.New("runs dir unreadable")
	}
	var stdout, stderr bytes.Buffer

	code := runPruneEphemeral([]string{"--dry-run", "--tracker-ttl-days", "3", "--dispatch-log-ttl-days", "9"}, nil, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "[prune-ephemeral] FAIL: runs dir unreadable") {
		t.Fatalf("code=%d stderr=%q, want 1 and the FAIL line", code, stderr.String())
	}
	if got.ProjectRoot != root || !got.DryRun || got.TrackerTTL != 3*24*time.Hour || got.DispatchLogTTL != 9*24*time.Hour {
		t.Errorf("options = %+v, want the flags passed through", got)
	}
}

func TestDispatchLogTTLDays_ACatalogWithoutTheDispatchCategoryFallsBackToTheLogsTTL(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"gc":{"logs_ttl_days":12}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := dispatchLogCatalog
	t.Cleanup(func() { dispatchLogCatalog = prev })
	dispatchLogCatalog = func(gcpolicy.Policy) []gcpolicy.LogCategory {
		return []gcpolicy.LogCategory{{Name: gcpolicy.LogCategoryLoop, TTLDays: 99}}
	}
	var stderr bytes.Buffer

	if got := dispatchLogTTLDays(root, &stderr); got != 12 {
		t.Errorf("dispatchLogTTLDays = %d, want the gc.logs_ttl_days fallback 12", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence for a readable policy", stderr.String())
	}
}

func TestPrintLoadWarnings(t *testing.T) {
	var stderr bytes.Buffer

	printLoadWarnings(&stderr, "inbox x", []inboxbatch.LoadWarning{
		{Text: "a.json: not JSON", Unreadable: true},
		{Text: "b.json: sanitized control characters in title"},
	})
	printLoadWarnings(&stderr, "inbox x", nil)

	want := "inbox x: WARN skipped a.json: not JSON\ninbox x: WARN b.json: sanitized control characters in title\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunInboxQuarantineList_FailurePaths(t *testing.T) {
	t.Run("a quarantine path that is a file", func(t *testing.T) {
		root := t.TempDir()
		q := filepath.Join(root, ".evolve", "inbox", "quarantine")
		if err := os.MkdirAll(filepath.Dir(q), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(q, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EVOLVE_PROJECT_ROOT", root)
		var stdout, stderr bytes.Buffer

		code := runInboxQuarantine([]string{"list"}, nil, &stdout, &stderr)

		if code != 1 || !strings.HasPrefix(stderr.String(), "inbox quarantine list: ") || stdout.Len() != 0 {
			t.Errorf("code=%d stdout=%q stderr=%q, want 1 and a scan error", code, stdout.String(), stderr.String())
		}
	})
	t.Run("a broken stdout fails the JSON encode", func(t *testing.T) {
		root := t.TempDir()
		q := filepath.Join(root, ".evolve", "inbox", "quarantine")
		if err := os.MkdirAll(q, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(q, "x.json"), []byte(`{"id":"x","title":"t"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("EVOLVE_PROJECT_ROOT", root)
		var stderr bytes.Buffer

		code := runInboxQuarantine([]string{"list", "--json"}, nil, &failingWriter{}, &stderr)

		if code != 1 || !strings.Contains(stderr.String(), "inbox quarantine list: encode: broken pipe") {
			t.Errorf("code=%d stderr=%q, want 1 and the encode failure", code, stderr.String())
		}
	})
}

func TestLoopStopRoot_AnUnresolvableRootWarnsAndKeepsTheGivenPath(t *testing.T) {
	prev := loopStopAbsoluteRoot
	t.Cleanup(func() { loopStopAbsoluteRoot = prev })
	var label string
	loopStopAbsoluteRoot = func(l, p string, warn func(string)) string {
		label = l
		warn("could not resolve --project-root \"rel\" to an absolute path (getwd failed)")
		return p
	}
	var stderr bytes.Buffer

	root, err := loopStopRoot("rel", &stderr)

	if err != nil || root != "rel" || label != "--project-root" {
		t.Fatalf("= (%q, %v) label=%q, want (rel, nil) resolved under --project-root", root, err, label)
	}
	if want := "evolve loop-stop: WARN: could not resolve --project-root \"rel\" to an absolute path (getwd failed)\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestLoopStopRoot_AResolvableRootIsAbsoluteAndSilent(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer

	root, err := loopStopRoot(dir, &stderr)

	if err != nil || root != dir || stderr.Len() != 0 {
		t.Errorf("= (%q, %v) stderr=%q, want (%q, nil) and no warning", root, err, stderr.String(), dir)
	}
}
