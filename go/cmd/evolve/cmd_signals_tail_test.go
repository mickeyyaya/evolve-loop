package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func tailTestEvent(cycle int, ts string, kind signalcenter.Kind) signalcenter.Event {
	return signalcenter.Event{SchemaVersion: "signal/1.0", Seq: 1, TS: ts, Cycle: cycle, Module: signalcenter.ModuleOrchestrator, Origin: "Fixture.emit", Kind: kind, Severity: signalcenter.SeverityInfo, Reason: string(kind)}
}

func writeTailEvents(root string, cycle int, events ...signalcenter.Event) error {
	dir := paths.RunWorkspace(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, signalcenter.StreamFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, e := range events {
		if err := json.NewEncoder(f).Encode(e); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}

func appendTailEvents(t *testing.T, root string, cycle int, events ...signalcenter.Event) {
	t.Helper()
	if err := writeTailEvents(root, cycle, events...); err != nil {
		t.Fatal(err)
	}
}

func tailTestLine(e signalcenter.Event) string { return e.TS + " " + signalcenter.FormatLine(e) + "\n" }

func TestSignalsTailDispatchFromRunSignalsPrintsTheCycleInInstantOrder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	late, early := tailTestEvent(3, "2026-10-05T10:00:05Z", signalcenter.KindPhaseOutcome), tailTestEvent(3, "2026-10-05T10:00:04.5Z", signalcenter.KindPhaseDispatched)
	appendTailEvents(t, root, 3, late, early)
	var stdout, stderr bytes.Buffer
	if rc := runSignals([]string{"tail", "--cycle", "3"}, nil, &stdout, &stderr); rc != 0 || stdout.String() != tailTestLine(early)+tailTestLine(late) {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if rc := runSignals([]string{"tail", "--cycle", "3", "--json", "--kind", "phase.outcome"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), `"kind":"phase.outcome"`) || strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("--json --kind: rc=%d stdout=%q", rc, stdout.String())
	}
	stderr.Reset()
	if rc := runSignals(nil, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), signalsTailUsage) {
		t.Fatalf("bare signals must advertise tail: rc=%d stderr=%q", rc, stderr.String())
	}
}

func TestSignalsTailUsageErrorsExitTen(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--cycle", "0"}, {"stray"}, {"--kind", "nope"}, {"--code", "a,b"}, {"--bogus"}} {
		var stdout, stderr bytes.Buffer
		if rc := runSignalsTail(context.Background(), args, tailEnv{ProjectRoot: t.TempDir()}, &stdout, &stderr); rc != 10 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("%v: rc=%d stdout=%q stderr=%q", args, rc, stdout.String(), stderr.String())
		}
	}
}

func TestSignalsTailWaveFollowJoinsLiveLanesAndEndsWhenEverySeals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	backlog := tailTestEvent(7, "2026-10-05T10:00:01Z", signalcenter.KindPhaseOutcome)
	appendTailEvents(t, root, 7, backlog)
	if err := runlease.Write(paths.RunWorkspace(root, 7), runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	appendTailEvents(t, root, 6, tailTestEvent(6, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	seal := tailTestEvent(7, "2026-10-05T10:00:02Z", signalcenter.KindCycleSealed)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		time.Sleep(10 * time.Millisecond)
		if err := writeTailEvents(root, 7, seal); err != nil {
			t.Error(err)
		}
	}()
	var stdout, stderr bytes.Buffer
	rc := runSignalsTail(ctx, []string{"--follow"}, tailEnv{ProjectRoot: root, Poll: 10 * time.Millisecond}, &stdout, &stderr)
	if rc != 0 || ctx.Err() != nil || stdout.String() != tailTestLine(backlog)+tailTestLine(seal) {
		t.Fatalf("rc=%d ctxErr=%v stdout=%q stderr=%q", rc, ctx.Err(), stdout.String(), stderr.String())
	}
}

func TestSignalsTailFollowEndsCleanlyOnCancel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	appendTailEvents(t, root, 4, tailTestEvent(4, "2026-10-05T10:00:01Z", signalcenter.KindPhaseOutcome))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var stdout, stderr bytes.Buffer
	if rc := runSignalsTail(ctx, []string{"--cycle", "4", "--follow"}, tailEnv{ProjectRoot: root, Poll: 5 * time.Millisecond}, &stdout, &stderr); rc != 0 || strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

func TestSignalsTailDefaultsAPollAndReportsAMissingCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	if rc := runSignalsTail(context.Background(), []string{"--cycle", "9"}, tailEnv{ProjectRoot: root}, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), paths.RunWorkspace(root, 9)) {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
}

func appendChannel(t *testing.T, root, name string, records ...channel.Record) {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(paths.EvolveDirOf(root)), name, channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(records); err != nil {
		t.Fatal(err)
	}
}

func TestSignalsTail_ChannelPrintsItsHistoryFilteredAndNamesEachGap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	reading := tailTestEvent(0, "2026-10-21T10:00:01Z", signalcenter.KindQualityReading)
	wave := tailTestEvent(0, "2026-10-21T10:00:00Z", signalcenter.KindLoopWave)
	appendChannel(t, root, "signals", channel.Record{Source: "loop", Signal: &wave}, channel.Record{Source: "quality", Signal: &reading},
		channel.Record{Source: "quality", Gap: &channel.Gap{Reason: channel.ReasonQueueFull, FirstSeq: 4, LastSeq: 6, From: 1, To: 2}})
	var stdout, stderr bytes.Buffer

	rc := runSignalsTail(context.Background(), []string{"--channel", "signals", "--kind", "quality.reading"}, tailEnv{ProjectRoot: root}, &stdout, &stderr)

	if rc != 0 || strings.Count(stdout.String(), "\n") != 1 || !strings.Contains(stdout.String(), "quality.reading") {
		t.Errorf("rc %d stdout %q, want only the quality reading", rc, stdout.String())
	}
	if !strings.Contains(stderr.String(), "gap") || !strings.Contains(stderr.String(), "queue_full") {
		t.Errorf("stderr %q, want the gap record named", stderr.String())
	}
}

func TestSignalsTail_ChannelArgumentsAndAbsentChannel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	if rc := runSignalsTail(context.Background(), []string{"--channel", "quality.reading"}, tailEnv{ProjectRoot: root}, &stdout, &stderr); rc != 0 || stdout.Len() != 0 {
		t.Errorf("an absent channel: rc %d stdout %q, want 0 and nothing", rc, stdout.String())
	}
	for _, args := range [][]string{{"--channel", "quality.reading", "--cycle", "3"}, {"--channel", "Bad Name"}} {
		if rc := runSignalsTail(context.Background(), args, tailEnv{ProjectRoot: root}, &stdout, &stderr); rc != 10 {
			t.Errorf("args %q: rc %d, want 10 (usage)", args, rc)
		}
	}
}

func TestSignalsTail_ChannelFollowPrintsLaterRecordsUntilCancelled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	out := &lockedBuffer{}
	done := make(chan int)
	go func() {
		done <- runSignalsTail(ctx, []string{"--channel", "quality.reading", "--follow"}, tailEnv{ProjectRoot: root, Poll: 2 * time.Millisecond}, out, io.Discard)
	}()
	reading := tailTestEvent(0, "2026-10-21T10:00:01Z", signalcenter.KindQualityReading)

	appendChannel(t, root, "quality.reading", channel.Record{Source: "quality", Signal: &reading})

	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(out.String(), "quality.reading") && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	if rc := <-done; rc != 0 || !strings.Contains(out.String(), "quality.reading") {
		t.Errorf("rc %d output %q, want the appended reading and exit 0 after cancel", rc, out.String())
	}
}

type failingTailWriter struct{}

func (failingTailWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }

func TestSignalsTail_ChannelFailuresExitTwo(t *testing.T) {
	t.Parallel()
	unreadable := t.TempDir()
	chRoot := paths.EventChannelsRoot(paths.EvolveDirOf(unreadable))
	if err := os.MkdirAll(chRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chRoot, "quality.reading"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runSignalsTail(context.Background(), []string{"--channel", "quality.reading"}, tailEnv{ProjectRoot: unreadable}, &stdout, &stderr); rc != 2 || !strings.Contains(stderr.String(), "channel quality.reading") {
		t.Errorf("an unreadable channel: rc %d stderr %q, want 2 naming the channel", rc, stderr.String())
	}
	full := t.TempDir()
	reading := tailTestEvent(0, "2026-10-21T10:00:01Z", signalcenter.KindQualityReading)
	appendChannel(t, full, "quality.reading", channel.Record{Source: "quality", Signal: &reading})
	stderr.Reset()
	if rc := runSignalsTail(context.Background(), []string{"--channel", "quality.reading"}, tailEnv{ProjectRoot: full}, failingTailWriter{}, &stderr); rc != 2 || !strings.Contains(stderr.String(), "write") {
		t.Errorf("a failing stdout: rc %d stderr %q, want 2", rc, stderr.String())
	}
}
