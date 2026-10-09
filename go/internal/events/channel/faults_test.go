package channel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

type faultyFile struct {
	segmentFile
	statErr, readErr, writeErr error
}

func (f faultyFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.segmentFile.Stat()
}

func (f faultyFile) ReadAt(p []byte, off int64) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return f.segmentFile.ReadAt(p, off)
}

func (f faultyFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.segmentFile.Write(p)
}

type openCall struct {
	path       string
	flag       int
	lockIsHeld bool
}

func recordOpens(l *Log, wrap func(call int, f segmentFile) (segmentFile, error)) *[]openCall {
	var calls []openCall
	l.open = func(path string, flag int, perm os.FileMode) (segmentFile, error) {
		calls = append(calls, openCall{path: path, flag: flag, lockIsHeld: lockIsHeld(l.lockPath)})
		f, err := openSegment(path, flag, perm)
		if err != nil {
			return nil, err
		}
		return wrap(len(calls), f)
	}
	return &calls
}

func lockIsHeld(path string) bool {
	release, held, err := flock.TryLock(path)
	if err == nil && !held {
		release()
	}
	return held
}

func passThrough(_ int, f segmentFile) (segmentFile, error) { return f, nil }

func TestAppend_OpensTheTailWithAppendUnderTheLock(t *testing.T) {
	l, root := newTestLog(t, 1<<20)
	writeSegment(t, root, 0, recordLine(t, signalRecord(1, "old")))
	writeSegment(t, root, 500, recordLine(t, signalRecord(2, "tail")))
	calls := recordOpens(l, passThrough)

	mustAppend(t, l, signalRecord(3, "new"))

	if len(*calls) == 0 {
		t.Fatal("Append opened no segment")
	}
	first := (*calls)[0]
	if first.path != segmentPath(root, 500) || first.flag != os.O_WRONLY|os.O_APPEND|os.O_CREATE || !first.lockIsHeld {
		t.Fatalf("first open = %+v, want the tail %s with O_WRONLY|O_APPEND|O_CREATE under the lock", first, segmentPath(root, 500))
	}
	for _, c := range *calls {
		if !c.lockIsHeld {
			t.Fatalf("open %+v ran without the channel lock", c)
		}
	}
}

func TestAppend_ReportsEachWriteStepFailure(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name string
		cap  int64
		torn bool
		wrap func(call int, f segmentFile) (segmentFile, error)
		want string
	}{
		{"open the tail", 1 << 20, false, func(int, segmentFile) (segmentFile, error) { return nil, boom }, "open tail"},
		{"stat the tail", 1 << 20, false, func(_ int, f segmentFile) (segmentFile, error) { return faultyFile{segmentFile: f, statErr: boom}, nil }, "stat tail"},
		{"open the tail to read its last byte", 1 << 20, true, func(c int, f segmentFile) (segmentFile, error) {
			if c == 2 {
				return nil, boom
			}
			return f, nil
		}, "open tail to read"},
		{"read the last byte", 1 << 20, true, func(c int, f segmentFile) (segmentFile, error) { return faultyFile{segmentFile: f, readErr: boom}, nil }, "read last byte"},
		{"write the batch", 1 << 20, false, func(_ int, f segmentFile) (segmentFile, error) {
			return faultyFile{segmentFile: f, writeErr: boom}, nil
		}, "write batch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, root := newTestLog(t, tc.cap)
			if tc.torn {
				writeSegment(t, root, 0, `{"torn`)
			}
			recordOpens(l, tc.wrap)

			_, err := l.Append([]Record{signalRecord(1, "r1")})

			if !errors.Is(err, boom) || !strings.Contains(err.Error(), "channel loop: "+tc.want) {
				t.Fatalf("Append with a failed %s = %v, want boom with %q", tc.name, err, "channel loop: "+tc.want)
			}
		})
	}
}

func TestAppend_AnUncreatableChannelDirectoryFails(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ch")
	l, err := New(root, testChannel, testConfig(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, testChannel), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, appendErr := l.Append([]Record{signalRecord(1, "r1")})
	_, readErr := l.Read(0)

	if appendErr == nil || !strings.Contains(appendErr.Error(), "channel loop") {
		t.Fatalf("Append where the channel directory is a file = %v, want an error naming the channel", appendErr)
	}
	if readErr == nil || !strings.Contains(readErr.Error(), "channel loop") {
		t.Fatalf("Read where the channel directory is a file = %v, want an error naming the channel", readErr)
	}
}

func TestAppend_ATailThatCannotOpenFails(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	if err := os.MkdirAll(segmentPath(root, 0), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := l.Append([]Record{signalRecord(1, "r1")})

	if err == nil || !strings.Contains(err.Error(), "channel loop: open tail") {
		t.Fatalf("Append to a tail that is a directory = %v, want an open error", err)
	}
}

func TestAppend_AnUnlistableChannelDirectoryFails(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory without the read permission")
	}
	l, root := newTestLog(t, 1<<20)
	dir := filepath.Join(root, testChannel)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := l.Append([]Record{signalRecord(1, "r1")})

	if err == nil || !strings.Contains(err.Error(), "channel loop: list segments") {
		t.Fatalf("Append where the directory cannot be listed = %v, want a list error", err)
	}
}
