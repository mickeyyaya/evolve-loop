package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func e3RunsDir(root string) string { return filepath.Join(paths.EvolveDirOf(root), "runs") }

func e3Mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestNewestTailStream_PicksTheHighestCanonicalCycleThatHasAStream(t *testing.T) {
	root := t.TempDir()
	runs := e3RunsDir(root)
	appendTailEvents(t, root, 3, tailTestEvent(3, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	appendTailEvents(t, root, 5, tailTestEvent(5, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	e3Mkdir(t, filepath.Join(runs, "cycle-9"))
	e3Mkdir(t, filepath.Join(runs, "cycle-007", signalcenter.StreamFileName))
	e3Mkdir(t, filepath.Join(runs, "cycle-x"))
	if err := os.WriteFile(filepath.Join(runs, "cycle-12"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(runs)
	if err != nil {
		t.Fatal(err)
	}

	got := newestTailStream(runs, entries)

	if got != 5 {
		t.Errorf("newestTailStream = %d, want 5: cycle-9 has no stream, cycle-007 and cycle-x are not canonical, cycle-12 is a file", got)
	}
	if n := newestTailStream(runs, nil); n != 0 {
		t.Errorf("no entries = %d, want 0", n)
	}
}

func TestNewestTailStream_OrderOfEntriesDoesNotMatter(t *testing.T) {
	root := t.TempDir()
	for _, c := range []int{8, 2, 11} {
		appendTailEvents(t, root, c, tailTestEvent(c, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	}
	entries, err := os.ReadDir(e3RunsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	reversed := make([]os.DirEntry, len(entries))
	for i, e := range entries {
		reversed[len(entries)-1-i] = e
	}
	if a, b := newestTailStream(e3RunsDir(root), entries), newestTailStream(e3RunsDir(root), reversed); a != 11 || b != 11 {
		t.Errorf("newest = %d / %d (reversed), want 11 both ways", a, b)
	}
}

func TestSignalsTail_WithNoLiveRunTailsTheNewestStreamOnce(t *testing.T) {
	root := t.TempDir()
	old := tailTestEvent(3, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome)
	newest := tailTestEvent(5, "2026-10-05T10:00:01Z", signalcenter.KindPhaseOutcome)
	appendTailEvents(t, root, 3, old)
	appendTailEvents(t, root, 5, newest)
	for _, args := range [][]string{nil, {"--follow"}} {
		t.Run(strings.Join(append([]string{"args"}, args...), " "), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var stdout, stderr bytes.Buffer

			rc := runSignalsTail(ctx, args, tailEnv{ProjectRoot: root, Poll: time.Millisecond}, &stdout, &stderr)

			if rc != 0 || ctx.Err() != nil || stdout.String() != tailTestLine(newest) || stderr.Len() != 0 {
				t.Fatalf("rc=%d ctxErr=%v stdout=%q stderr=%q", rc, ctx.Err(), stdout.String(), stderr.String())
			}
		})
	}
}

func TestSignalsTail_WaveStopsWhenThereIsNothingToTail(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, root string)
		wantRC int
		want   string
	}{
		{"no runs dir", func(*testing.T, string) {}, 0, "signals tail: no signal streams under "},
		{"runs without streams", func(t *testing.T, root string) { e3Mkdir(t, filepath.Join(e3RunsDir(root), "cycle-4")) }, 0, "signals tail: no signal streams under "},
		{"runs is a file", func(t *testing.T, root string) {
			e3Mkdir(t, paths.EvolveDirOf(root))
			if err := os.WriteFile(e3RunsDir(root), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, 2, "signals tail: list "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			var stdout, stderr bytes.Buffer

			rc := runSignalsTail(context.Background(), nil, tailEnv{ProjectRoot: root}, &stdout, &stderr)

			if rc != tc.wantRC || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), tc.want+e3RunsDir(root)) {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want rc %d and %q", rc, stdout.String(), stderr.String(), tc.wantRC, tc.want)
			}
		})
	}
}

func TestSignalsTail_AStreamThatCannotBeReadExitsTwo(t *testing.T) {
	root := t.TempDir()
	e3Mkdir(t, filepath.Join(paths.RunWorkspace(root, 6), signalcenter.StreamFileName))
	var stdout, stderr bytes.Buffer

	rc := runSignalsTail(context.Background(), []string{"--cycle", "6"}, tailEnv{ProjectRoot: root}, &stdout, &stderr)

	if rc != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "signals tail: signalcenter: read stream ") {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

func TestSignalsTail_MalformedLinesAreCountedAndSkipped(t *testing.T) {
	root := t.TempDir()
	good := tailTestEvent(2, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome)
	appendTailEvents(t, root, 2, good)
	stream := filepath.Join(paths.RunWorkspace(root, 2), signalcenter.StreamFileName)
	f, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{torn\nnot json either\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runSignalsTail(context.Background(), []string{"--cycle", "2"}, tailEnv{ProjectRoot: root}, &stdout, &stderr)

	if rc != 0 || stdout.String() != tailTestLine(good) {
		t.Fatalf("rc=%d stdout=%q", rc, stdout.String())
	}
	if stderr.String() != "signals tail: skipped 2 malformed line(s) in "+stream+"\n" {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestSignalsTail_ASingleMalformedLineIsReported(t *testing.T) {
	root := t.TempDir()
	appendTailEvents(t, root, 2, tailTestEvent(2, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	stream := filepath.Join(paths.RunWorkspace(root, 2), signalcenter.StreamFileName)
	f, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{torn\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runSignalsTail(context.Background(), []string{"--cycle", "2"}, tailEnv{ProjectRoot: root}, &stdout, &stderr)

	if rc != 0 || stderr.String() != "signals tail: skipped 1 malformed line(s) in "+stream+"\n" {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
}

func TestSignalsTail_AFailedWriteExitsTwoInBothFormats(t *testing.T) {
	root := t.TempDir()
	appendTailEvents(t, root, 2, tailTestEvent(2, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	for _, args := range [][]string{{"--cycle", "2"}, {"--cycle", "2", "--json"}} {
		var stderr bytes.Buffer

		rc := runSignalsTail(context.Background(), args, tailEnv{ProjectRoot: root}, e3FailingWriter{}, &stderr)

		if rc != 2 || stderr.String() != "signals tail: write: stdout closed\n" {
			t.Errorf("%v: rc=%d stderr=%q", args, rc, stderr.String())
		}
	}
}

func TestSignalsTail_AFilteredOutEventIsNeverWritten(t *testing.T) {
	root := t.TempDir()
	appendTailEvents(t, root, 2, tailTestEvent(2, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	var stderr bytes.Buffer

	rc := runSignalsTail(context.Background(), []string{"--cycle", "2", "--kind", "cycle.sealed"}, tailEnv{ProjectRoot: root}, e3FailingWriter{}, &stderr)

	if rc != 0 || stderr.Len() != 0 {
		t.Fatalf("rc=%d stderr=%q: a filtered event reached the writer", rc, stderr.String())
	}
}

func TestSignalsTail_JoinLiveLanesSortsAndMarksDepartedLanesGone(t *testing.T) {
	tail := &signalsTail{project: "/p"}
	tail.lanes = []*tailLane{tail.lane(7), tail.lane(5)}

	tail.joinLiveLanes(map[int]bool{9: true, 3: true, 5: true})

	var cycles []int
	gone := map[int]bool{}
	for _, l := range tail.lanes {
		cycles = append(cycles, l.cycle)
		gone[l.cycle] = l.gone
	}
	if !reflect.DeepEqual(cycles, []int{3, 5, 7, 9}) {
		t.Errorf("lanes = %v, want sorted 3 5 7 9", cycles)
	}
	if !gone[7] || gone[5] || gone[3] || gone[9] {
		t.Errorf("gone = %v, want only the departed cycle 7 gone", gone)
	}
	if tail.lanes[0].path != filepath.Join(paths.RunWorkspace("/p", 3), signalcenter.StreamFileName) {
		t.Errorf("a joined lane tails %q", tail.lanes[0].path)
	}
}

func TestSignalsTail_AFallbackTailNeverJoinsALaterLiveRun(t *testing.T) {
	root := t.TempDir()
	appendTailEvents(t, root, 4, tailTestEvent(4, "2026-10-05T10:00:00Z", signalcenter.KindPhaseOutcome))
	tail := &signalsTail{project: root, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	if rc, stop := tail.refreshWave(true); rc != 0 || stop || !tail.fallback || len(tail.lanes) != 1 || tail.lanes[0].cycle != 4 || !tail.lanes[0].gone {
		t.Fatalf("first refresh: rc=%d stop=%v fallback=%v lanes=%+v", rc, stop, tail.fallback, tail.lanes)
	}
	e3Mkdir(t, paths.RunWorkspace(root, 8))
	if err := runlease.Write(paths.RunWorkspace(root, 8), runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	rc, stop := tail.refreshWave(false)

	if rc != 0 || stop || len(tail.lanes) != 1 || tail.lanes[0].cycle != 4 {
		t.Errorf("rc=%d stop=%v lanes=%+v: the fallback tail adopted a live run", rc, stop, tail.lanes)
	}
}
