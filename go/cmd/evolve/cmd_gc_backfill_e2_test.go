package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

const deadE2PID = 99999999

func e2GCRunWithReapers(dryRun bool, sessions swarm.OrphanReapReport, sockets swarm.OrphanSocketReport, stdout, stderr io.Writer) gcRun {
	r := newGCRun(context.Background(), dryRun, stdout, stderr)
	r.reapers = gcReapers{
		sessions: func(context.Context) swarm.OrphanReapReport { return sessions },
		sockets:  func(context.Context) swarm.OrphanSocketReport { return sockets },
	}
	return r
}

func TestGCSessions_ReportsEveryReapedSessionAndFailsOnAnError(t *testing.T) {
	cases := []struct {
		name       string
		rep        swarm.OrphanReapReport
		wantFailed bool
		wantOut    string
		wantErr    string
	}{
		{
			name:    "nothing to reap",
			rep:     swarm.OrphanReapReport{},
			wantOut: "evolve gc: reaped 0 orphan session(s)\nskipped: live=0 foreign=0 no-pid=0; errors=0\n",
		},
		{
			name: "two reaped and counts",
			rep:  swarm.OrphanReapReport{Killed: []string{"evolve-bridge-a-pid11", "evolve-bridge-b-pid12"}, SkippedLive: 1, SkippedForeign: 2, SkippedUnparseable: 3},
			wantOut: "evolve gc: reaped 2 orphan session(s)\n  reaped evolve-bridge-a-pid11\n  reaped evolve-bridge-b-pid12\n" +
				"skipped: live=1 foreign=2 no-pid=3; errors=0\n",
		},
		{
			name:       "an error fails the step",
			rep:        swarm.OrphanReapReport{Errors: []string{"kill evolve-bridge-c: boom", "list: gone"}},
			wantFailed: true,
			wantOut:    "evolve gc: reaped 0 orphan session(s)\nskipped: live=0 foreign=0 no-pid=0; errors=2\n",
			wantErr:    "evolve gc: error: kill evolve-bridge-c: boom\nevolve gc: error: list: gone\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			failed := e2GCRunWithReapers(false, tc.rep, swarm.OrphanSocketReport{}, &stdout, &stderr).sessions()

			if failed != tc.wantFailed || stdout.String() != tc.wantOut || stderr.String() != tc.wantErr {
				t.Fatalf("failed=%v stdout=%q stderr=%q\nwant %v %q %q", failed, stdout.String(), stderr.String(), tc.wantFailed, tc.wantOut, tc.wantErr)
			}
		})
	}
}

func TestGCSockets_ReportsEveryReapedSocketAndFailsOnAnError(t *testing.T) {
	cases := []struct {
		name       string
		rep        swarm.OrphanSocketReport
		wantFailed bool
		wantOut    string
		wantErr    string
	}{
		{"nothing", swarm.OrphanSocketReport{}, false, "evolve gc: reaped 0 dead per-run socket(s)\n", ""},
		{"one reaped", swarm.OrphanSocketReport{Killed: []string{"evolve-bridge-p77"}}, false, "evolve gc: reaped 1 dead per-run socket(s)\n  socket evolve-bridge-p77\n", ""},
		{"an error", swarm.OrphanSocketReport{Killed: []string{"evolve-bridge-p78"}, Errors: []string{"remove: denied"}}, true, "evolve gc: reaped 1 dead per-run socket(s)\n  socket evolve-bridge-p78\n", "evolve gc: socket error: remove: denied\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			failed := e2GCRunWithReapers(false, swarm.OrphanReapReport{}, tc.rep, &stdout, &stderr).sockets()

			if failed != tc.wantFailed || stdout.String() != tc.wantOut || stderr.String() != tc.wantErr {
				t.Fatalf("failed=%v stdout=%q stderr=%q", failed, stdout.String(), stderr.String())
			}
		})
	}
}

func TestGCSockets_ADryRunNamesADeadSocketAndLeavesItInPlace(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := bridge.TmuxSocketDir()
	dead := bridge.DeriveRunSocket(deadE2PID)
	live := bridge.DeriveRunSocket(os.Getpid())
	writeE2File(t, filepath.Join(dir, dead), "")
	writeE2File(t, filepath.Join(dir, live), "")
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), true, &stdout, &stderr).sockets()

	want := "evolve gc --dry-run: 1 dead per-run socket(s) would be reaped\n  socket " + dead + "\n"
	if failed || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("failed=%v stdout=%q stderr=%q, want %q", failed, stdout.String(), stderr.String(), want)
	}
	if _, err := os.Stat(filepath.Join(dir, dead)); err != nil {
		t.Fatalf("a dry run removed the dead socket: %v", err)
	}
}

func e2FailingGCExec(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return -1, errors.New("exec refused")
}

func TestGCCycleProcesses_AListFailureIsAnError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), false, &stdout, &stderr).cycleProcesses(gc.WorktreeOptions{ProjectRoot: t.TempDir(), Exec: e2FailingGCExec})

	if !failed || stderr.String() != "evolve gc: process error: gc: list process cwds: exec refused\n" {
		t.Fatalf("failed=%v stderr=%q", failed, stderr.String())
	}
	if stdout.String() != "evolve gc: terminated 0 finished-cycle orphan process(es)\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestGCCycleProcesses_ASignalFailureIsAnErrorAndTheProcessIsNotCountedReaped(t *testing.T) {
	root := t.TempDir()
	tree := gcClosedOutCycleTree(t, root, 1700)
	opts := worktreeGCOptions(root, filepath.Join(root, ".evolve"), gc.WorktreesPolicy{})
	opts.Exec = func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		_, err := io.WriteString(out, "p4242\nR1\nn"+tree+"\n")
		return 0, err
	}
	r := newGCRun(context.Background(), false, io.Discard, io.Discard)
	var signalled []int
	r.kill = func(pid int) error {
		signalled = append(signalled, pid)
		return errors.New("EPERM")
	}
	var stdout, stderr bytes.Buffer
	r.stdout, r.stderr = &stdout, &stderr

	failed := r.cycleProcesses(opts)

	if !failed || len(signalled) != 1 || signalled[0] != 4242 {
		t.Fatalf("failed=%v signalled=%v, want one signal to 4242 that failed", failed, signalled)
	}
	if stderr.String() != "evolve gc: process error: signal pid 4242 (cwd "+tree+"): EPERM\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if strings.Contains(stdout.String(), "terminated pid=") {
		t.Fatalf("a failed signal was reported as a termination: %q", stdout.String())
	}
}

func TestGCWorktrees_APlanFailureFailsTheStep(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		var stdout, stderr bytes.Buffer

		failed := newGCRun(context.Background(), dryRun, &stdout, &stderr).worktrees(gc.WorktreeOptions{ProjectRoot: t.TempDir()})

		if !failed || stderr.String() != "evolve gc: workspace sweep plan failed: gc: PlanWorktrees requires Exec\n" || stdout.Len() != 0 {
			t.Fatalf("dryRun=%v failed=%v stdout=%q stderr=%q", dryRun, failed, stdout.String(), stderr.String())
		}
	}
}

func TestGCWorktrees_AGitFailureFailsThePlan(t *testing.T) {
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), false, &stdout, &stderr).worktrees(gc.WorktreeOptions{ProjectRoot: t.TempDir(), Exec: e2FailingGCExec})

	if !failed || !strings.HasPrefix(stderr.String(), "evolve gc: workspace sweep plan failed: gc: git worktree list --porcelain:") {
		t.Fatalf("failed=%v stderr=%q", failed, stderr.String())
	}
}

func TestGCGoCache_ARemoveFailureIsReportedAndFailsTheStep(t *testing.T) {
	stale, fresh := gcFakeGoCache(t)
	r := newGCRun(context.Background(), false, io.Discard, io.Discard)
	r.remove = func(p string) error { return errors.New("read-only " + filepath.Base(p)) }
	var stdout, stderr bytes.Buffer
	r.stdout, r.stderr = &stdout, &stderr

	failed := r.goCache(gc.Policy{GoCacheTTLHours: 24})

	if !failed || !strings.Contains(stderr.String(), "evolve gc: go build cache error: ") || !strings.Contains(stderr.String(), "read-only 0a1b2c-d") {
		t.Fatalf("failed=%v stderr=%q", failed, stderr.String())
	}
	for _, p := range []string{stale, fresh} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s vanished though every remove failed: %v", p, err)
		}
	}
}

func TestGCPipelineTemp_ARemoveFailureIsReportedAndFailsTheStep(t *testing.T) {
	tmp := t.TempDir()
	stale := filepath.Join(tmp, "acs-cycle1515-bin-2718281828")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	r := newGCRun(context.Background(), false, io.Discard, io.Discard)
	r.removeAll = func(string) error { return errors.New("busy") }
	var stdout, stderr bytes.Buffer
	r.stdout, r.stderr = &stdout, &stderr

	failed := r.pipelineTemp(24)

	if !failed || !strings.Contains(stderr.String(), "evolve gc: temp error: ") || !strings.Contains(stderr.String(), "busy") {
		t.Fatalf("failed=%v stdout=%q stderr=%q", failed, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("the stale entry vanished though removal failed: %v", err)
	}
}

func TestGCPipelineTemp_ZeroOrNegativeTTLIsOff(t *testing.T) {
	for _, ttl := range []int{0, -1} {
		var stdout, stderr bytes.Buffer
		r := newGCRun(context.Background(), false, &stdout, &stderr)
		r.removeAll = func(string) error { t.Fatal("an off sweep removed something"); return nil }

		if r.pipelineTemp(ttl) || stdout.String() != "evolve gc: pipeline temp sweep off (gc.temp_ttl_hours unset)\n" {
			t.Fatalf("ttl=%d stdout=%q", ttl, stdout.String())
		}
	}
}

func TestGCReportDiskFree_ReportsTheNetChangeOrAnUnreadablePath(t *testing.T) {
	t.Run("readable", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		newGCRun(context.Background(), false, &stdout, &stderr).reportDiskFree(t.TempDir(), 0)
		if !strings.HasPrefix(stdout.String(), "evolve gc: disk free 0.00 GB → ") || !strings.Contains(stdout.String(), "(net released ") || stderr.Len() != 0 {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		newGCRun(context.Background(), false, &stdout, &stderr).reportDiskFree(filepath.Join(t.TempDir(), "absent"), 5)
		if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "evolve gc: disk free after the run unreadable: ") {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	})
}

func e2FakeGoOnPath(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	fakeclitest.Install(t, filepath.Join(dir, "go"), "#!/bin/sh\n"+body)
	t.Setenv("PATH", dir)
}

func TestGCGoCacheDir_ResolvesOrRefusesTheCache(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr string
	}{
		{"a cache dir", "echo /cache/dir\n", "/cache/dir", ""},
		{"a padded cache dir", "printf '  /c d  \\n\\n'\n", "/c d", ""},
		{"a failing go", "exit 3\n", "", "go env GOCACHE: exit 3"},
		{"an empty answer", "echo\n", "", `GOCACHE is ""`},
		{"the cache is off", "echo off\n", "", `GOCACHE is "off"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e2FakeGoOnPath(t, tc.body)

			got, err := gcGoCacheDir(context.Background())

			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr || got != "" {
					t.Fatalf("got %q err=%v, want error %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q err=%v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestGCGoCacheDir_ACancelledContextIsAnError(t *testing.T) {
	e2FakeGoOnPath(t, "sleep 5\necho /late\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := gcGoCacheDir(ctx)

	if err == nil || got != "" {
		t.Fatalf("got %q err=%v, want a cancelled run to fail", got, err)
	}
}

func TestResolveGCProjectRoot_AnUnreadableCwdSkipsTheDryRunSweep(t *testing.T) {
	e2StubProcessGetwd(t, errors.New("stale handle"))
	var stderr bytes.Buffer

	root, code, ok := resolveGCProjectRoot("", true, &stderr)

	if ok || code != 1 || root != "" {
		t.Fatalf("root=%q code=%d ok=%v, want a refusal with code 1", root, code, ok)
	}
	if stderr.String() != "evolve gc: workspace sweep skipped: no --project-root and cwd is unreadable: stale handle\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestGCSize_RendersGigabytesWithTwoDecimals(t *testing.T) {
	for b, want := range map[int64]string{0: "0.00 GB", 1e9: "1.00 GB", 1_234_567_890: "1.23 GB", -5e8: "-0.50 GB"} {
		if got := gcSize(b); got != want {
			t.Errorf("gcSize(%d) = %q, want %q", b, got, want)
		}
	}
}

func TestGCCycleProcesses_NamesTheOrphanAndSignalsItOnlyOnARealRun(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		root := t.TempDir()
		tree := gcClosedOutCycleTree(t, root, 1701)
		opts := worktreeGCOptions(root, filepath.Join(root, ".evolve"), gc.WorktreesPolicy{})
		opts.Exec = func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
			_, err := io.WriteString(out, "p4343\nR1\nn"+tree+"\n")
			return 0, err
		}
		var stdout, stderr bytes.Buffer
		r := newGCRun(context.Background(), dryRun, &stdout, &stderr)
		var signalled []int
		if !dryRun {
			r.kill = func(pid int) error { signalled = append(signalled, pid); return nil }
		}

		failed := r.cycleProcesses(opts)

		want := "evolve gc: terminated 1 finished-cycle orphan process(es)\n  terminated pid=4343 cwd=" + tree + "\n"
		wantSignals := 1
		if dryRun {
			want = "evolve gc --dry-run: 1 finished-cycle orphan process(es) would be terminated\n  WOULD-TERMINATE pid=4343 cwd=" + tree + "\n"
			wantSignals = 0
		}
		if failed || stdout.String() != want || stderr.Len() != 0 || len(signalled) != wantSignals {
			t.Fatalf("dryRun=%v failed=%v signalled=%v stdout=%q, want %q", dryRun, failed, signalled, stdout.String(), want)
		}
	}
}
