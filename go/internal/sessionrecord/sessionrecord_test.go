package sessionrecord

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type failingAppendFile struct {
	writeErr error
	closeErr error
}

func (f failingAppendFile) Write([]byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return 1, nil
}

func (f failingAppendFile) Close() error {
	return f.closeErr
}

func TestAppendReadRoundTrip(t *testing.T) {
	t.Parallel()
	path := PathIn(t.TempDir())
	want := Record{Session: "evolve-bridge-rAAAA0000-c1-build-pid9-7", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Cycle: 1, Agent: "build", PID: 9}
	if err := Append(path, want); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("ReadAll=%+v, want [%+v]", got, want)
	}
}

func TestReadAllMissingFileIsEmpty(t *testing.T) {
	t.Parallel()
	got, err := ReadAll(PathIn(t.TempDir()))
	if err != nil || got != nil {
		t.Errorf("ReadAll(missing) = (%v, %v), want (nil, nil) — no sessions is success", got, err)
	}
}

// TestReadAllSkipsMalformedLines: a crash can half-write a line; the reaper
// must still reap the well-formed remainder rather than abandoning the whole
// registry (which would leak every session of the run).
func TestReadAllSkipsMalformedLines(t *testing.T) {
	t.Parallel()
	path := PathIn(t.TempDir())
	if err := Append(path, Record{Session: "evolve-bridge-rAAAA0000-c1-tdd-pid9-7"}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"session":"evolve-bridge-truncat`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 1 || got[0].Session != "evolve-bridge-rAAAA0000-c1-tdd-pid9-7" {
		t.Errorf("ReadAll=%+v, want exactly the well-formed record", got)
	}
}

func TestRunScopeToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		runID string
		want  string
	}{
		{name: "empty", runID: "", want: "r"},
		{name: "short", runID: "ABC", want: "rABC"},
		{name: "exactly eight", runID: "ABCDEFGH", want: "rABCDEFGH"},
		{name: "truncates long run id", runID: "ABCDEFGH1234", want: "rABCDEFGH"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RunScopeToken(tt.runID); got != tt.want {
				t.Fatalf("RunScopeToken(%q) = %q, want %q", tt.runID, got, tt.want)
			}
		})
	}
}

func TestAppendOpenError(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	notDir := filepath.Join(base, "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Append(filepath.Join(notDir, FileName), Record{Session: "s"}); err == nil {
		t.Fatal("Append must fail when the registry parent is not a directory")
	}
}

func TestReadAllOpenError(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	notDir := filepath.Join(base, "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAll(filepath.Join(notDir, FileName)); err == nil {
		t.Fatal("ReadAll must fail when the registry parent is not a directory")
	}
}

func TestAppendWriteError(t *testing.T) {
	old := openAppendFileFn
	t.Cleanup(func() { openAppendFileFn = old })
	openAppendFileFn = func(path string) (appendFile, error) {
		return failingAppendFile{writeErr: errors.New("write failed")}, nil
	}
	if err := Append(PathIn(t.TempDir()), Record{Session: "s"}); err == nil {
		t.Fatal("Append must surface write errors")
	}
}

func TestAppendCloseError(t *testing.T) {
	old := openAppendFileFn
	t.Cleanup(func() { openAppendFileFn = old })
	openAppendFileFn = func(path string) (appendFile, error) {
		return failingAppendFile{closeErr: errors.New("close failed")}, nil
	}
	if err := Append(PathIn(t.TempDir()), Record{Session: "s"}); err == nil {
		t.Fatal("Append must surface close errors")
	}
}
