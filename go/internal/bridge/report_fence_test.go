package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func fenceReport(tag string) string {
	return `{"eventMessage":"Sandbox: cat(1900) allow file-read-data \/dev\/null\nevolve-cell=` + tag + ` rule=cell.fence"}` + "\n"
}

type lagClock struct {
	now    time.Time
	slept  []time.Duration
	reads  int
	reply  func(read int) string
	fenced []string
}

func (c *lagClock) deps(stderr *bytes.Buffer) Deps {
	return Deps{hostOS: "darwin", ReadStage: "shadow", Env: map[string]string{}, Stderr: stderr, Signals: signalcenter.New(),
		Now:   func() time.Time { return c.now },
		Sleep: func(d time.Duration) { c.slept = append(c.slept, d); c.now = c.now.Add(d) },
		SandboxFence: func(_ context.Context, tag string) error {
			c.fenced = append(c.fenced, tag)
			return nil
		},
		SandboxReports: func(context.Context, string, time.Time) ([]byte, error) {
			c.reads++
			return []byte(c.reply(c.reads)), nil
		}}
}

func unmeasuredEvents(center *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == isolation.CodeReadUnmeasured {
			out = append(out, e)
		}
	}
	return out
}

func assertOneUnmeasured(t *testing.T, center *signalcenter.Center, cause string, severity signalcenter.Severity) signalcenter.Event {
	t.Helper()
	got := unmeasuredEvents(center)
	if len(got) != 1 || got[0].Fields["cause"] != cause || got[0].Severity != severity || got[0].Kind != signalcenter.KindIsolationContext || got[0].Fields["cell"] != "build" {
		t.Fatalf("unmeasured events = %+v, want exactly one with cause %q and severity %v", got, cause, severity)
	}
	return got[0]
}

func openedKinds(t *testing.T, root string) ([]string, string) {
	t.Helper()
	records, err := inputindex.ReadAccess(root)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	source := ""
	for _, r := range records {
		if r.Type == inputindex.AccessOpen {
			kinds = append(kinds, r.Kind)
		} else {
			source = r.Source
		}
	}
	return kinds, source
}

func TestReportReadViolations_PollsTheLogUntilTheFenceOfTheCellArrives(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	tag := "run1/7/build/p12n3"
	scout := filepath.Join(c.ws, "scout-report.md")
	clock := &lagClock{now: time.Unix(1000, 0), reply: func(read int) string {
		if read < 3 {
			return ""
		}
		return grantReport(tag, scout) + fenceReport(tag)
	}}
	var stderr bytes.Buffer
	cfg := &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: tag}
	deps := clock.deps(&stderr)
	deps.reportReadViolations(context.Background(), cfg, time.Unix(990, 0))
	kinds, source := openedKinds(t, c.root)
	if strings.Join(kinds, ",") != "scout-report" || source != inputindex.AccessSourceSandbox {
		t.Fatalf("opens %v source %q: a report that lags the end of the cell must still reach the access log", kinds, source)
	}
	if got := unmeasuredEvents(deps.Signals); len(got) != 0 {
		t.Fatalf("a complete cell emits no unmeasured event: %+v", got)
	}
	if clock.reads != 3 || len(clock.slept) != 2 || len(clock.fenced) != 1 || clock.fenced[0] != tag || stderr.Len() != 0 {
		t.Fatalf("reads %d slept %v fenced %v stderr %q: the reader polls until the fence and writes the fence once", clock.reads, clock.slept, clock.fenced, stderr.String())
	}
}

func TestReportReadViolations_AFenceThatNeverArrivesIsLoudAndTheCellCountsAsUnmeasured(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	tag := "run1/7/build/p12n4"
	scout := filepath.Join(c.ws, "scout-report.md")
	clock := &lagClock{now: time.Unix(1000, 0), reply: func(int) string { return grantReport(tag, scout) }}
	var stderr bytes.Buffer
	deps := clock.deps(&stderr)
	deps.ReportFenceDeadline = 3 * time.Second
	deps.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: tag}, time.Unix(990, 0))
	_, source := openedKinds(t, c.root)
	if source != inputindex.AccessSourceNone || !strings.Contains(stderr.String(), "WARN isolation read reports: no fence of the cell after 3s") {
		t.Fatalf("source %q stderr %q: an unfenced read is incomplete, so no reading may count the cell", source, stderr.String())
	}
	if e := assertOneUnmeasured(t, deps.Signals, "fence-deadline", signalcenter.SeverityWarn); e.Fields["elapsed"] != "3s" || e.Fields["stage"] != "shadow" || e.Origin != "bridge.reportReadViolations" {
		t.Fatalf("event = %+v, want elapsed 3s, stage shadow, origin reportReadViolations", e)
	}
	if clock.now.Sub(time.Unix(1000, 0)) < 3*time.Second {
		t.Fatalf("the reader stopped before the deadline: %v", clock.now)
	}
}

func TestReportReadViolations_AFenceThatCannotBeWrittenFallsBackToOneUnmeasuredRead(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	tag := "run1/7/build/p12n5"
	clock := &lagClock{now: time.Unix(1000, 0), reply: func(int) string { return "" }}
	var stderr bytes.Buffer
	deps := clock.deps(&stderr)
	deps.SandboxFence = func(context.Context, string) error { return errors.New("sandbox-exec: not found") }
	deps.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: tag}, time.Unix(990, 0))
	_, source := openedKinds(t, c.root)
	if clock.reads != 1 || source != inputindex.AccessSourceNone || !strings.Contains(stderr.String(), "WARN isolation read reports: write the fence: sandbox-exec: not found") {
		t.Fatalf("reads %d source %q stderr %q", clock.reads, source, stderr.String())
	}
	if e := assertOneUnmeasured(t, deps.Signals, "fence-write", signalcenter.SeverityWarn); e.Fields["elapsed"] != "" {
		t.Fatalf("a fence-write gap has no elapsed field: %+v", e)
	}
}

func TestSandboxFence_RunsOneReportedReadUnderTheTagOfTheCell(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	argv := filepath.Join(dir, "argv")
	stub := filepath.Join(dir, "sandbox-exec")
	writeFixtureFile(t, stub, "#!/bin/sh\nprintf '%s\\n' \"$@\" > \""+argv+"\"\n")
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sandboxFence(stub)(context.Background(), "run1/7/build/p1n2"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	want := "-p\n" + `(version 1)(allow default)(allow file-read-data (literal "/dev/null") (with report) (with message "evolve-cell=run1/7/build/p1n2 rule=cell.fence"))` + "\n/bin/cat\n/dev/null\n"
	if string(got) != want {
		t.Fatalf("sandbox-exec argv =\n%s\nwant\n%s", got, want)
	}
	if err := sandboxFence(filepath.Join(dir, "absent"))(context.Background(), "d1"); err == nil {
		t.Fatal("a missing sandbox-exec must fail")
	}
	if NewEngine(Deps{TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }}).deps.SandboxFence == nil {
		t.Fatal("the engine defaults must write the fence through sandbox-exec")
	}
}

func TestReportReadViolations_ACancelledLaunchStopsTheWaitForTheFenceAfterOneRead(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	clock := &lagClock{now: time.Unix(1000, 0), reply: func(int) string { return "" }}
	var stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deps := clock.deps(&stderr)
	deps.reportReadViolations(ctx, &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: "run1/7/build/p1n7"}, time.Unix(990, 0))
	if e := assertOneUnmeasured(t, deps.Signals, "cancelled", signalcenter.SeverityInfo); e.Fields["elapsed"] != "0s" {
		t.Fatalf("elapsed %q, want 0s on a cancelled launch", e.Fields["elapsed"])
	}
	_, source := openedKinds(t, c.root)
	if clock.reads != 1 || len(clock.slept) != 0 || source != inputindex.AccessSourceNone || !strings.Contains(stderr.String(), "no fence of the cell after 0s") {
		t.Fatalf("reads %d slept %v source %q stderr %q: a cancelled run reads the log once and waits no deadline", clock.reads, clock.slept, source, stderr.String())
	}
}

func TestReportReadViolations_AReadErrorWhileItWaitsForTheFenceIsLoud(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	clock := &lagClock{now: time.Unix(1000, 0)}
	var stderr bytes.Buffer
	deps := clock.deps(&stderr)
	deps.SandboxReports = func(context.Context, string, time.Time) ([]byte, error) { return nil, errors.New("log: denied") }
	deps.reportReadViolations(context.Background(), &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: "run1/7/build/p1n6"}, time.Unix(990, 0))
	_, source := openedKinds(t, c.root)
	if source != inputindex.AccessSourceNone || !strings.Contains(stderr.String(), "WARN isolation read reports: log: denied") || len(clock.slept) != 0 {
		t.Fatalf("source %q stderr %q slept %v", source, stderr.String(), clock.slept)
	}
	assertOneUnmeasured(t, deps.Signals, "report-read", signalcenter.SeverityWarn)
}

func TestFenced_FindsOnlyTheFenceOfTheTagInTheEventMessages(t *testing.T) {
	t.Parallel()
	raw := []byte(fenceReport("other") + "not json\n" + grantReport("d1", "/x"))
	if fenced(raw, "d1") || !fenced(raw, "other") || !fenced([]byte(fenceReport("d1")), "d1") {
		t.Fatal("a fence counts only for its own tag")
	}
}

func TestLaunchSwept_ACellWhoseFenceNeverArrivesEmitsOneUnmeasuredEventThroughTheDeferredReader(t *testing.T) {
	t.Parallel()
	c := newReadCell(t)
	c.index(t)
	clock := &lagClock{now: time.Unix(1000, 0), reply: func(int) string { return "" }}
	var stderr bytes.Buffer
	deps := clock.deps(&stderr)
	deps.ReportFenceDeadline = 2 * time.Second
	deps.ListProcesses = func(context.Context) ([]proctree.Process, error) { return nil, nil }
	deps.ListCwds = func(context.Context) ([]proctree.Cwd, error) { return nil, nil }
	deps.SignalProcess = func(pid int, _ syscall.Signal) error {
		return fmt.Errorf("the test signals no process, got pid %d", pid)
	}
	driver := &launchCountingDriver{}
	cfg := &Config{CLI: "claude-tmux", Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt"}

	rc, err := launchSwept(context.Background(), driver, cfg, deps)

	if rc != ExitOK || err != nil || driver.launches != 1 {
		t.Fatalf("launchSwept = (%d, %v), launches %d", rc, err, driver.launches)
	}
	e := assertOneUnmeasured(t, deps.Signals, "fence-deadline", signalcenter.SeverityWarn)
	if e.Fields["elapsed"] != "2s" || len(clock.fenced) != 1 || clock.fenced[0] != cfg.DispatchID {
		t.Fatalf("event %+v fenced %v, want the fence written under the minted dispatch id %q", e, clock.fenced, cfg.DispatchID)
	}
}
