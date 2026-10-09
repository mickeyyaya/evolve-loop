//go:build darwin || linux

package wake

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	wakeBound = 5 * time.Second
	quietSpan = 100 * time.Millisecond
	absentPid = 999999999
)

func armedChannel(t *testing.T) (*Waiter, string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ch", "loop")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	seg := filepath.Join(dir, "seg-00000000000000000000.ndjson")
	if err := os.WriteFile(seg, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	w := newHostWaiter(t, nil)
	if err := w.Arm(Targets{Dirs: []string{dir}, Files: []string{seg}}); err != nil {
		t.Fatalf("Arm = %v, want nil", err)
	}
	return w, dir, seg
}

func newHostWaiter(t *testing.T, output *os.File) *Waiter {
	t.Helper()
	w, err := New(output)
	if err != nil {
		t.Fatalf("New = %v, want a waiter", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("Close = %v, want nil", err)
		}
	})
	return w
}

func appendLine(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"source\":\"loop\"}\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, w *Waiter, span time.Duration) Wake {
	t.Helper()
	got, err := w.Wait(context.Background(), time.Now().Add(span))
	if err != nil {
		t.Fatalf("Wait = %v, want no error", err)
	}
	return got
}

func TestWaiter_AnAppendWakesTheWaiter(t *testing.T) {
	t.Parallel()
	w, _, seg := armedChannel(t)

	appendLine(t, seg)
	got := waitFor(t, w, wakeBound)

	if !got.Changed || got.Deadline {
		t.Errorf("Wait after an append = %+v, want Changed before the deadline", got)
	}
}

func TestWaiter_ASecondWaitAfterOneWriteTimesOut(t *testing.T) {
	t.Parallel()
	w, _, seg := armedChannel(t)
	appendLine(t, seg)
	if first := waitFor(t, w, wakeBound); !first.Changed {
		t.Fatalf("first Wait = %+v, want Changed", first)
	}

	start := time.Now()
	got := waitFor(t, w, quietSpan)

	if !got.Deadline || got.Changed || time.Since(start) < quietSpan {
		t.Errorf("second Wait = %+v after %v, want Deadline after %v: one write gives one wake (EV_CLEAR, a drained inotify queue)", got, time.Since(start), quietSpan)
	}
}

func TestWaiter_AWatchOnAnAbsentDirectoryWakesOnTheFirstSegment(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "events", "ch", "cycle")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	w := newHostWaiter(t, nil)
	if err := w.Arm(Targets{Dirs: []string{dir}}); err != nil {
		t.Fatalf("Arm = %v, want nil", err)
	}

	appendLine(t, filepath.Join(dir, "seg-00000000000000000000.ndjson"))
	got := waitFor(t, w, wakeBound)

	if !got.Changed {
		t.Errorf("Wait after the first segment = %+v, want Changed", got)
	}
}

func TestWaiter_ADirectoryCreatedAfterArmWakesTheParentWatch(t *testing.T) {
	t.Parallel()
	ch := t.TempDir()
	w := newHostWaiter(t, nil)
	if err := w.Arm(Targets{Dirs: []string{ch}}); err != nil {
		t.Fatalf("Arm = %v, want nil", err)
	}

	if err := os.Mkdir(filepath.Join(ch, "ci.release"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, w, wakeBound)

	if !got.Changed {
		t.Errorf("Wait after a new channel directory = %+v, want Changed", got)
	}
}

func TestWaiter_RotationArmsTheNewTailSegment(t *testing.T) {
	t.Parallel()
	w, dir, _ := armedChannel(t)
	next := filepath.Join(dir, "seg-00000000000016777216.ndjson")
	appendLine(t, next)
	if got := waitFor(t, w, wakeBound); !got.Changed {
		t.Fatalf("Wait after a new segment = %+v, want Changed", got)
	}
	if err := w.Arm(Targets{Dirs: []string{dir}, Files: []string{next}}); err != nil {
		t.Fatalf("Arm of the new tail = %v, want nil", err)
	}

	appendLine(t, next)
	got := waitFor(t, w, wakeBound)

	if !got.Changed {
		t.Errorf("Wait after an append to the new tail = %+v, want Changed", got)
	}
}

func TestWaiter_CancelReturnsAtOnce(t *testing.T) {
	t.Parallel()
	w, _, _ := armedChannel(t)
	ctx, cancel := context.WithCancel(context.Background())
	stop := time.AfterFunc(quietSpan, cancel)
	defer stop.Stop()

	start := time.Now()
	got, err := w.Wait(ctx, time.Time{})

	if !errors.Is(err, context.Canceled) || got.Changed || got.Deadline || time.Since(start) > wakeBound {
		t.Errorf("Wait with no deadline = %+v, %v after %v, want context.Canceled at the cancel", got, err, time.Since(start))
	}
}

func TestWaiter_AStdoutHangupWakesTheWaiter(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	w := newHostWaiter(t, out)
	if err := w.Arm(Targets{}); err != nil {
		t.Fatalf("Arm = %v, want nil", err)
	}

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, w, wakeBound)

	if !got.Hangup {
		t.Errorf("Wait after the reader of the output closed = %+v, want Hangup", got)
	}
}

func TestWaiter_ARegularFileOutputArmsNoHangup(t *testing.T) {
	t.Parallel()
	out, err := os.Create(filepath.Join(t.TempDir(), "out.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	w := newHostWaiter(t, out)

	got := waitFor(t, w, quietSpan)

	if !got.Deadline || got.Hangup {
		t.Errorf("Wait with a regular file as output = %+v, want Deadline and no Hangup", got)
	}
}

func TestWaiter_AnAbsentPidWakesAtOnceAsAnExit(t *testing.T) {
	t.Parallel()
	w := newHostWaiter(t, nil)

	err := w.Arm(Targets{Pids: []int{absentPid}})
	got := waitFor(t, w, wakeBound)

	if err != nil || len(got.Exited) != 1 || got.Exited[0] != absentPid {
		t.Errorf("Arm = %v, Wait = %+v, want no error and an exit of %d: ESRCH at the arm is an exit", err, got, absentPid)
	}
}

func TestWaiter_AnAbsentPathIsAnIOErrorAndNotARefusal(t *testing.T) {
	t.Parallel()
	w := newHostWaiter(t, nil)

	err := w.Arm(Targets{Dirs: []string{filepath.Join(t.TempDir(), "gone")}})

	if !errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrRefused) {
		t.Errorf("Arm of an absent directory = %v, want a not-exist error that is not a refusal", err)
	}
}

func TestIsTerminal_APipeIsNotATerminal(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer out.Close()

	if isTerminal(int(out.Fd())) {
		t.Errorf("isTerminal(pipe) = true, want false")
	}
}

func TestNew_AnUnreadableOutputIsAnError(t *testing.T) {
	t.Parallel()
	out, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	w, err := New(out)

	if w != nil || !errors.Is(err, os.ErrClosed) {
		t.Errorf("New(closed output) = %v, %v, want no waiter and the closed-file error", w, err)
	}
}

func TestWaiter_AFileWatchWakesOnEveryChangeOfTheTail(t *testing.T) {
	t.Parallel()
	changes := []struct {
		name   string
		change func(t *testing.T, seg string)
	}{
		{"append", func(t *testing.T, seg string) { appendLine(t, seg) }},
		{"atomic replace by rename", func(t *testing.T, seg string) {
			mustRename(t, seg+".tmp", seg)
		}},
		{"delete", func(t *testing.T, seg string) {
			if err := os.Remove(seg); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename away to another directory", func(t *testing.T, seg string) {
			mustRename(t, seg, filepath.Join(t.TempDir(), "old.ndjson"))
		}},
		{"truncate", func(t *testing.T, seg string) {
			if err := os.Truncate(seg, 0); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			seg := filepath.Join(dir, "seg-00000000000000000000.ndjson")
			appendLine(t, seg)
			appendLine(t, seg+".tmp")
			w := newHostWaiter(t, nil)
			if err := w.Arm(Targets{Files: []string{seg}}); err != nil {
				t.Fatalf("Arm = %v, want nil", err)
			}

			c.change(t, seg)
			got := waitFor(t, w, wakeBound)

			if !got.Changed {
				t.Errorf("Wait after %s = %+v, want Changed", c.name, got)
			}
		})
	}
}

func mustRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}
