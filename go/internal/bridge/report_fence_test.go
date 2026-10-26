package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
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
	clock.deps(&stderr).reportReadViolations(context.Background(), cfg, time.Unix(990, 0))
	kinds, source := openedKinds(t, c.root)
	if strings.Join(kinds, ",") != "scout-report" || source != inputindex.AccessSourceSandbox {
		t.Fatalf("opens %v source %q: a report that lags the end of the cell must still reach the access log", kinds, source)
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
	clock.deps(&stderr).reportReadViolations(ctx, &Config{Agent: "build", Cycle: 7, Workspace: c.ws, ProjectRoot: c.root, Worktree: "/wt", DispatchID: "run1/7/build/p1n7"}, time.Unix(990, 0))
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
}

func TestFenced_FindsOnlyTheFenceOfTheTagInTheEventMessages(t *testing.T) {
	t.Parallel()
	raw := []byte(fenceReport("other") + "not json\n" + grantReport("d1", "/x"))
	if fenced(raw, "d1") || !fenced(raw, "other") || !fenced([]byte(fenceReport("d1")), "d1") {
		t.Fatal("a fence counts only for its own tag")
	}
}
