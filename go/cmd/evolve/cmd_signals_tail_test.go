package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for _, args := range [][]string{{"--cycle", "0"}, {"stray"}, {"--kind", "nope"}, {"--code", "a,b"}, {"--bogus"}} {
		var stdout, stderr bytes.Buffer
		if rc := runSignalsTail(context.Background(), args, tailEnv{ProjectRoot: t.TempDir()}, &stdout, &stderr); rc != 10 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("%v: rc=%d stdout=%q stderr=%q", args, rc, stdout.String(), stderr.String())
		}
	}
}

func TestSignalsTailWaveFollowJoinsLiveLanesAndEndsWhenEverySeals(t *testing.T) {
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
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	if rc := runSignalsTail(context.Background(), []string{"--cycle", "9"}, tailEnv{ProjectRoot: root}, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), paths.RunWorkspace(root, 9)) {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
}
