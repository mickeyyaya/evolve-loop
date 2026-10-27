package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func eventsRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func appendEvents(t *testing.T, root, name string, events ...signalcenter.Event) {
	t.Helper()
	l, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), name, channel.Config{LockDeadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(l.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	records := make([]channel.Record, len(events))
	for i := range events {
		e := events[i]
		records[i] = channel.Record{Source: "loop", Signal: &e}
	}
	if _, err := l.Append(records); err != nil {
		t.Fatal(err)
	}
}

func loopEvent(seq uint64, kind signalcenter.Kind) signalcenter.Event {
	return signalcenter.Event{SchemaVersion: signalcenter.SchemaVersion, Seq: seq, PID: 4242, TS: "2026-10-27T01:00:0" + string(rune('0'+seq)) + "Z",
		Module: signalcenter.ModuleLoop, Origin: "test", Kind: kind, Severity: signalcenter.SeverityInfo, Reason: "r"}
}

func TestEventsChannels_ListsTheCatalogWithTheTailCursorOfEachChannel(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopExit))
	var stdout, stderr bytes.Buffer
	if rc := runEvents([]string{"channels", "--json", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	var got struct {
		Channels []eventChannelRow `json:"channels"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Channels) != len(policy.EventsDefaults().Channels) {
		t.Fatalf("channels = %d, want the %d of the catalog", len(got.Channels), len(policy.EventsDefaults().Channels))
	}
	for _, c := range got.Channels {
		if (c.Name == "loop") != (c.End > 0) {
			t.Errorf("%s: end %d; only the loop channel holds a record", c.Name, c.End)
		}
	}
	stdout.Reset()
	if rc := runEvents([]string{"channels", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK || !strings.Contains(stdout.String(), "loop") {
		t.Errorf("text form: rc = %d stdout = %q", rc, stdout.String())
	}
}

func TestEventsKinds_PrintsTheRegisteredVocabulary(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runEvents([]string{"kinds", "--json"}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	var got filter.Catalog
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || len(got.Kinds) != len(signalcenter.Kinds()) || len(got.Modules) == 0 || len(got.Codes) == 0 {
		t.Fatalf("catalog = %d kinds %d modules %d codes (%v)", len(got.Kinds), len(got.Modules), len(got.Codes), err)
	}
	stdout.Reset()
	if rc := runEvents([]string{"kinds"}, nil, &stdout, &stderr); rc != cliExitOK || !strings.Contains(stdout.String(), "kind loop.exit") || !strings.Contains(stdout.String(), "module loop") || !strings.Contains(stdout.String(), "code LOOP_LOST") {
		t.Errorf("text form: rc = %d", rc)
	}
}

func TestEventsCLI_ExitCodesMatchTheTable(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no verb", nil, cliExitUsage},
		{"an unknown verb", []string{"bogus"}, cliExitUsage},
		{"channels with an operand", []string{"channels", "x"}, cliExitUsage},
		{"kinds with an unknown flag", []string{"kinds", "--bogus"}, cliExitUsage},
		{"a watch with no selector", []string{"watch", "--project-root", root}, cliExitUsage},
		{"a watch with an operand", []string{"watch", "--channel", "loop", "x", "--project-root", root}, cliExitUsage},
		{"a bad since", []string{"watch", "--channel", "loop", "--since", "soon", "--project-root", root}, cliExitUsage},
		{"a bad filter key", []string{"watch", "--channel", "loop", "--filter", "colour=red", "--project-root", root}, cliExitUsage},
		{"an unknown kind", []string{"watch", "--channel", "loop", "--until", "kind=no.such", "--project-root", root}, cliExitInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if rc := runEvents(c.args, nil, &stdout, &stderr); rc != c.want || stderr.Len() == 0 {
				t.Errorf("rc = %d stderr = %q, want %d and a message", rc, stderr.String(), c.want)
			}
		})
	}
}

func TestEventCatalog_ARouteThatDoesNotParseIsAnError(t *testing.T) {
	t.Parallel()
	ev := policy.EventsPolicy{Channels: []policy.EventChannel{{Name: "bad", Route: "kind=no.such"}}}
	if _, err := eventCatalog(ev, filter.RegisteredCatalog()); err == nil || !strings.Contains(err.Error(), "channel bad") {
		t.Errorf("err = %v, want the bad route named", err)
	}
	got, err := eventCatalog(policy.EventsDefaults(), filter.RegisteredCatalog())
	if err != nil || len(got) != len(policy.EventsDefaults().Channels) {
		t.Errorf("defaults: %d channels, %v", len(got), err)
	}
}

func TestEventsChannels_AnUnreadableChannelIsAnIOError(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	ch := filepath.Join(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "loop")
	if err := os.MkdirAll(filepath.Dir(ch), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ch, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runEvents([]string{"channels", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitIO {
		t.Errorf("rc = %d, want 2", rc)
	}
}

type eventsTestWaiter struct {
	wakes []func() (wake.Wake, error)
}

func (w *eventsTestWaiter) Arm(wake.Targets) error { return nil }

func (w *eventsTestWaiter) Wait(ctx context.Context, _ time.Time) (wake.Wake, error) {
	if err := ctx.Err(); err != nil {
		return wake.Wake{}, err
	}
	if len(w.wakes) == 0 {
		return wake.Wake{Deadline: true}, nil
	}
	next := w.wakes[0]
	w.wakes = w.wakes[1:]
	return next()
}

func testWatchEnv(w *eventsTestWaiter, ctx context.Context) eventsWatchEnv {
	return eventsWatchEnv{
		waiter:     func(*os.File) (reader.Waiter, error) { return w, nil },
		newContext: func() (context.Context, context.CancelFunc) { return context.WithCancel(ctx) },
		startOf:    func(int) (string, error) { return "", syscall.ESRCH },
		events:     policy.EventsDefaults(),
	}
}

func runTestWatch(t *testing.T, env eventsWatchEnv, root string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runEventsWatchWith(env, append(args, "--project-root", root), &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestEventsWatch_UntilPrintsTheMatchAndExitsZero(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopStarted), loopEvent(2, signalcenter.KindLoopExit), loopEvent(3, signalcenter.KindLoopStarted))
	rc, out, errOut := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), root, "--channel", "loop", "--since", "all", "--until", "kind=loop.exit", "--json")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if rc != cliExitOK || len(lines) != 2 {
		t.Fatalf("rc = %d, %d lines, want 0 and two lines ending at loop.exit\nstdout %s\nstderr %s", rc, len(lines), out, errOut)
	}
	var last eventWire
	if err := json.Unmarshal([]byte(lines[1]), &last); err != nil {
		t.Fatal(err)
	}
	if last.Schema != eventWireSchema || last.Channel != "loop" || last.Cursor == nil || *last.Cursor == 0 || last.Signal == nil || last.Signal.Kind != signalcenter.KindLoopExit || last.ID != "4242.2."+last.Signal.TS {
		t.Errorf("last = %+v, want the wire record of loop.exit with its cursor and id", last)
	}
}

func TestEventsWatch_AGapNeverEndsAnUntilWatch(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	l, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "loop", channel.Config{LockDeadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(l.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Append([]channel.Record{{Source: "loop", Gap: &channel.Gap{Reason: channel.ReasonQueueFull, Severity: signalcenter.SeverityWarn, PID: 4242, FirstSeq: 1, LastSeq: 2, Dropped: 2}}}); err != nil {
		t.Fatal(err)
	}
	appendEvents(t, root, "loop", loopEvent(3, signalcenter.KindLoopExit))
	rc, out, _ := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), root, "--channel", "loop", "--since", "all", "--until", "kind=loop.exit", "--filter", "kind=loop.exit")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if rc != cliExitOK || len(lines) != 2 || !strings.Contains(lines[0], "gap queue_full WARN pid=4242 seq=1-2 dropped=2") || !strings.Contains(lines[1], "loop.exit") {
		t.Errorf("rc = %d stdout:\n%s\nwant the gap, which passes every filter and never ends the watch, then loop.exit", rc, out)
	}
}

func TestEventsWatch_EachSinceModeStartsWhereTheSpecSays(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopStarted), loopEvent(2, signalcenter.KindLoopStarted), loopEvent(3, signalcenter.KindLoopExit))
	first := countWatchLines(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), root, "all")
	last := countWatchLines(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), root, "last")
	appended := &eventsTestWaiter{}
	appended.wakes = append(appended.wakes, func() (wake.Wake, error) {
		appendEvents(t, root, "loop", loopEvent(4, signalcenter.KindLoopExit))
		return wake.Wake{Changed: true}, nil
	})
	fresh := countWatchLines(t, testWatchEnv(appended, context.Background()), root, "new")
	if first != 3 || last != 1 || fresh != 1 {
		t.Errorf("all = %d, last = %d, new = %d lines; want 3, 1 and only the appended record", first, last, fresh)
	}
}

func countWatchLines(t *testing.T, env eventsWatchEnv, root, since string) int {
	t.Helper()
	rc, out, errOut := runTestWatch(t, env, root, "--channel", "loop", "--since", since, "--until", "kind=loop.exit")
	if rc != cliExitOK {
		t.Fatalf("--since %s: rc = %d stderr %s", since, rc, errOut)
	}
	return len(strings.Split(strings.TrimSpace(out), "\n"))
}

func TestEventsWatch_AnEmptySelectorIsRefused(t *testing.T) {
	t.Parallel()
	for _, sel := range []string{"", " "} {
		if rc, _, errOut := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), eventsRoot(t), "--channel", sel); rc != cliExitUsage || !strings.Contains(errOut, "--channel") {
			t.Errorf("selector %q: rc = %d stderr %q, want 10", sel, rc, errOut)
		}
	}
	if rc, _, _ := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), eventsRoot(t), "--channel", "nosuch"); rc != cliExitInput {
		t.Errorf("a selector with no channel: rc = %d, want 11", rc)
	}
}

func TestEventsWatch_AnUnreadableChannelExitsTwo(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	ch := filepath.Join(paths.EventChannelsRoot(paths.EvolveDirOf(root)), "cycle")
	if err := os.MkdirAll(filepath.Dir(ch), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ch, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, errOut := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), root, "--channel", "cycle"); rc != cliExitIO || errOut == "" {
		t.Errorf("rc = %d stderr %q, want 2", rc, errOut)
	}
}

type brokenPipe struct{}

func (brokenPipe) Write([]byte) (int, error) { return 0, syscall.EPIPE }

func TestEventsWatch_EPIPEExitsThree(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopExit))
	var stderr bytes.Buffer
	rc := runEventsWatchWith(testWatchEnv(&eventsTestWaiter{}, context.Background()), []string{"--channel", "loop", "--since", "all", "--project-root", root}, brokenPipe{}, &stderr)
	if rc != cliExitRefused || !strings.Contains(stderr.String(), "the reader of the output is gone") {
		t.Errorf("rc = %d stderr %q, want 3", rc, stderr.String())
	}
}

func TestEventsWatch_AStdoutHangupExitsThree(t *testing.T) {
	t.Parallel()
	w := &eventsTestWaiter{wakes: []func() (wake.Wake, error){func() (wake.Wake, error) { return wake.Wake{Hangup: true}, nil }}}
	if rc, _, _ := runTestWatch(t, testWatchEnv(w, context.Background()), eventsRoot(t), "--channel", "loop"); rc != cliExitRefused {
		t.Errorf("rc = %d, want 3", rc)
	}
}

func TestEventsWatch_WithoutUntilTheWatchPrintsEachRecordUntilItsReaderIsGone(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopStarted), loopEvent(2, signalcenter.KindLoopExit))
	w := &eventsTestWaiter{wakes: []func() (wake.Wake, error){func() (wake.Wake, error) { return wake.Wake{Hangup: true}, nil }}}
	rc, out, _ := runTestWatch(t, testWatchEnv(w, context.Background()), root, "--channel", "loop", "--since", "all")
	if rc != cliExitRefused || strings.Count(out, "\n") != 2 {
		t.Errorf("rc = %d stdout %q, want both records and then exit 3 at the hangup", rc, out)
	}
}

func TestEventsWatch_SIGINTExitsZero(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rc, out, _ := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, ctx), eventsRoot(t), "--channel", "loop"); rc != cliExitOK || out != "" {
		t.Errorf("rc = %d stdout %q, want 0 and nothing", rc, out)
	}
}

func TestEventsWatch_EachSetupFaultHasItsExitCode(t *testing.T) {
	t.Parallel()
	bad := testWatchEnv(&eventsTestWaiter{}, context.Background())
	bad.events = policy.EventsPolicy{Channels: []policy.EventChannel{{Name: "loop", Route: "kind=no.such"}}}
	noKernel := testWatchEnv(&eventsTestWaiter{}, context.Background())
	noKernel.waiter = func(*os.File) (reader.Waiter, error) { return nil, wake.ErrRefused }
	cases := []struct {
		name string
		env  eventsWatchEnv
		args []string
		want int
	}{
		{"a broken route", bad, nil, cliExitIO},
		{"no kernel wake", noKernel, nil, cliExitRefused},
		{"a deadline the watch never sets", testWatchEnv(&eventsTestWaiter{}, context.Background()), nil, cliExitIO},
		{"an unknown flag", testWatchEnv(&eventsTestWaiter{}, context.Background()), []string{"--bogus"}, cliExitUsage},
	}
	for _, c := range cases {
		if rc, _, errOut := runTestWatch(t, c.env, eventsRoot(t), append([]string{"--channel", "loop"}, c.args...)...); rc != c.want || errOut == "" {
			t.Errorf("%s: rc = %d stderr %q, want %d", c.name, rc, errOut, c.want)
		}
	}
}

func TestEventsWatch_TheTextFormNamesTheChannelAndTheCursor(t *testing.T) {
	t.Parallel()
	it := reader.Item{Channel: "loop", Record: channel.Record{Cursor: 7, Source: "loop", Message: json.RawMessage(`{"m":1}`)}}
	if got, err := (eventsWatch{}).line(it); got != `loop 7 message {"m":1}` || err != nil {
		t.Errorf("line = %q, %v", got, err)
	}
	bad := reader.Item{Channel: "loop", Record: channel.Record{Cursor: 8, Source: "loop", Message: json.RawMessage(`{bad`)}}
	var stderr bytes.Buffer
	if done, rc := (eventsWatch{asJSON: true, stdout: io.Discard}).emit(bad, &stderr); done || rc != cliExitIO || !strings.Contains(stderr.String(), "encode the record at loop:8") {
		t.Errorf("a record that does not encode: done %t rc %d stderr %q, want exit 2", done, rc, stderr.String())
	}
	synthetic := reader.Item{Channel: "loop", Record: channel.Record{Source: channel.SourceWatch, Gap: &channel.Gap{Reason: channel.ReasonReset, From: 3, To: 9}}}
	if w := wireOf(synthetic); w.Cursor != nil || w.ID != "gap.loop.3" {
		t.Errorf("synthetic wire = %+v, want no cursor and the gap id", w)
	}
}

func TestEventsWatch_TheDefaultEnvironmentArmsTheKernelAndTheSignals(t *testing.T) {
	env := defaultEventsWatchEnv()
	w, err := env.waiter(nil)
	if err != nil {
		t.Fatalf("the kernel wake: %v", err)
	}
	if closer, ok := w.(*wake.Waiter); !ok || closer.Close() != nil {
		t.Errorf("waiter = %T, want a closable *wake.Waiter", w)
	}
	ctx, stop := env.newContext()
	stop()
	if ctx.Err() == nil {
		t.Error("the stop of the signal context leaves it open")
	}
	var stderr bytes.Buffer
	if rc := runEventsWatch([]string{}, io.Discard, &stderr); rc != cliExitUsage {
		t.Errorf("runEventsWatch with no selector: rc = %d, want 10", rc)
	}
}

func TestEventsWatch_AFilterPrintsOnlyItsRecordsAndAnUnknownCodeOnlyWarns(t *testing.T) {
	t.Parallel()
	root := eventsRoot(t)
	appendEvents(t, root, "loop", loopEvent(1, signalcenter.KindLoopStarted), loopEvent(2, signalcenter.KindLoopExit))
	rc, out, errOut := runTestWatch(t, testWatchEnv(&eventsTestWaiter{}, context.Background()), root, "--channel", "loop", "--since", "all", "--filter", "kind=loop.exit code!=NO_SUCH_CODE", "--until", "kind=loop.exit")
	if rc != cliExitOK || strings.Count(out, "\n") != 1 || !strings.Contains(out, "loop.exit") {
		t.Errorf("rc = %d stdout %q, want only the loop.exit line", rc, out)
	}
	if !strings.Contains(errOut, "--filter: WARN") || !strings.Contains(errOut, "NO_SUCH_CODE") {
		t.Errorf("stderr %q, want a warning that names the unknown code", errOut)
	}
}

func TestChannelEnd_ABadChannelNameIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := channelEnd(t.TempDir(), "Bad Name"); err == nil {
		t.Error("a bad channel name gave no error")
	}
}

func TestEvents_AnUnreadableWorkingDirectoryIsAnIOError(t *testing.T) {
	breakWorkingDirectory(t)
	var stdout, stderr bytes.Buffer
	if rc := runEvents([]string{"channels"}, nil, &stdout, &stderr); rc != cliExitIO {
		t.Errorf("channels: rc = %d, want 2", rc)
	}
	if rc := runEventsWatchWith(testWatchEnv(&eventsTestWaiter{}, context.Background()), []string{"--channel", "loop"}, &stdout, &stderr); rc != cliExitIO {
		t.Errorf("watch: rc = %d, want 2", rc)
	}
}
