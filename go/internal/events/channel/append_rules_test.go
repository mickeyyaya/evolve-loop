package channel

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

type countingFile struct {
	segmentFile
	writes *[][]byte
}

func (f countingFile) Write(p []byte) (int, error) {
	*f.writes = append(*f.writes, append([]byte(nil), p...))
	return f.segmentFile.Write(p)
}

func TestAppend_AnEncodeErrorWritesNothing(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	torn := `{"torn`
	writeSegment(t, root, 0, torn)
	boom := errors.New("boom")
	l.marshal = func(any) ([]byte, error) { return nil, boom }

	_, err := l.Append([]Record{signalRecord(1, "r1")})

	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "channel loop: encode") {
		t.Fatalf("Append with an encode error = %v, want boom with %q", err, "channel loop: encode")
	}
	if raw, rerr := os.ReadFile(segmentPath(root, 0)); rerr != nil || string(raw) != torn {
		t.Fatalf("the tail after a refused batch = %q, %v, want %q: no repair and no batch", raw, rerr, torn)
	}
}

func TestAppend_RefusesARecordThatReadWouldReject(t *testing.T) {
	t.Parallel()
	both := signalRecord(1, "both")
	both.Gap = &Gap{Reason: ReasonQueueFull}
	watch := signalRecord(2, "watch")
	watch.Source = SourceWatch
	cases := map[string]Record{
		"a record with a signal and a gap": both,
		"a record with neither":            {Source: "loop"},
		"a record from a reader":           watch,
	}
	for name, rec := range cases {
		t.Run(name, func(t *testing.T) {
			l, root := newTestLog(t, 1<<20)

			_, err := l.Append([]Record{signalRecord(9, "ok"), rec})

			if !errors.Is(err, ErrInvalidRecord) || !strings.Contains(err.Error(), "channel loop") {
				t.Fatalf("Append(%s) = %v, want ErrInvalidRecord naming the channel", name, err)
			}
			if _, err := os.Stat(filepath.Join(root, testChannel)); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("a refused batch touched the channel: %v", err)
			}
		})
	}
}

func TestAppend_RepairsAndWritesTheBatchInOneWrite(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	torn := `{"torn`
	writeSegment(t, root, 0, torn)
	var writes [][]byte
	recordOpens(l, func(_ int, f segmentFile) (segmentFile, error) {
		return countingFile{segmentFile: f, writes: &writes}, nil
	})

	cursor := mustAppend(t, l, signalRecord(1, "a"), signalRecord(2, "b"))

	want := "\n" + recordLine(t, signalRecord(1, "a")) + recordLine(t, signalRecord(2, "b"))
	if len(writes) != 1 || string(writes[0]) != want {
		t.Fatalf("writes = %q, want one write %q", writes, want)
	}
	if cursor != int64(len(torn)+1) {
		t.Fatalf("Append cursor = %d, want %d", cursor, len(torn)+1)
	}
}

func TestAppend_ARotationErrorKeepsTheBatchAndTheNextAppendRotates(t *testing.T) {
	t.Parallel()
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	l, root := newTestLog(t, 1)
	writeSegment(t, root, 0, recordLine(t, signalRecord(1, "r0")))
	boom := errors.New("boom")
	l.open = func(path string, flag int, perm os.FileMode) (segmentFile, error) {
		if path != segmentPath(root, 0) {
			return nil, boom
		}
		return openSegment(path, flag, perm)
	}

	cursor, err := l.Append([]Record{signalRecord(1, "r1")})

	if !errors.Is(err, ErrRotate) || !errors.Is(err, boom) || cursor != line {
		t.Fatalf("Append with a failed rotation = %d, %v, want cursor %d and ErrRotate wrapping boom", cursor, err, line)
	}
	if got := describe(mustRead(t, l, line).Records); len(got) != 1 || got[0] != "at "+itoa(line)+" r1" {
		t.Fatalf("Read after a failed rotation = %q, want the batch on disk", got)
	}
	l.open = openSegment
	next := mustAppend(t, l, signalRecord(2, "r2"))
	if bases := segmentBases(t, root); next != 2*line || len(bases) != 2 || bases[1] != 3*line {
		t.Fatalf("the next append = cursor %d, bases %v, want cursor %d and a rotation to %d", next, bases, 2*line, 3*line)
	}
}

func TestAppend_OneLockCallWaitsForEachChannelAndProcess(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "ch")
	l, err := New(root, testChannel, Config{SegmentBytes: 1 << 20, LockDeadline: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	holder, err := flock.Lock(filepath.Join(root, testChannel+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	settled := make(chan struct{}, 8)
	l.lock = func(path string, wait time.Duration, onSettled func()) (func(), error) {
		calls.Add(1)
		return flock.LockWithin(path, wait, func() { onSettled(); settled <- struct{}{} })
	}

	_, first := l.Append([]Record{signalRecord(1, "r1")})
	var rest []error
	for i := uint64(2); i <= 5; i++ {
		_, err := l.Append([]Record{signalRecord(i, "r")})
		rest = append(rest, err)
	}

	if !errors.Is(first, flock.ErrLockDeadline) || errors.Is(first, ErrLockPending) {
		t.Fatalf("the first append = %v, want a lock deadline that is not pending", first)
	}
	for i, err := range rest {
		if !errors.Is(err, ErrLockPending) || !errors.Is(err, flock.ErrLockDeadline) || !strings.Contains(err.Error(), "channel loop") {
			t.Fatalf("append %d while a call waits = %v, want ErrLockPending wrapping flock.ErrLockDeadline", i+2, err)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("lock calls = %d, want 1: one waiting call for each channel and process", n)
	}
	holder()
	select {
	case <-settled:
	case <-time.After(10 * time.Second):
		t.Fatal("the waiting call never settled after the holder released")
	}
	l.cfg.LockDeadline = time.Minute
	if _, err := l.Append([]Record{signalRecord(6, "r6")}); err != nil || calls.Load() != 2 {
		t.Fatalf("append after the waiting call settled = %v with %d lock calls, want success and 2", err, calls.Load())
	}
}

func TestAppend_AnOpenErrorOfTheLockClearsThePendingCall(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ch")
	l, err := New(root, testChannel, testConfig(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, testChannel+".lock"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, first := l.Append([]Record{signalRecord(1, "r1")})
	_, second := l.Append([]Record{signalRecord(2, "r2")})

	if first == nil || errors.Is(first, ErrLockPending) || second == nil || errors.Is(second, ErrLockPending) {
		t.Fatalf("appends with an unopenable lock = %v, %v, want two open errors and no pending call", first, second)
	}
}

func TestGap_FromAndToAreAlwaysWritten(t *testing.T) {
	t.Parallel()
	line := recordLine(t, Record{Source: "loop", Gap: &Gap{Reason: ReasonQueueFull}})

	if !strings.Contains(line, `"from":0`) || !strings.Contains(line, `"to":0`) {
		t.Fatalf("a gap line = %s, want from and to also at 0", line)
	}
	rec, ok := parseRecord(bytes.TrimSuffix([]byte(line), []byte("\n")))
	if !ok || rec.Gap.From != 0 || rec.Gap.To != 0 {
		t.Fatalf("parse of %s = %+v, %v", line, rec, ok)
	}
}
