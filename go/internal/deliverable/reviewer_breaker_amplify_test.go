package deliverable

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func amplCaptureStderrBreaker(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	w.Close()
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	r.Close()
	return string(buf[:n])
}

func TestWriteBreaker_TmpWriteFailure_WARNsAndLeavesPriorStateUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "breaker.json")

	writeBreaker(path, 5) // establish a healthy prior state

	if err := os.Chmod(dir, 0o555); err != nil { // read+execute only: blocks new file creation
		t.Fatalf("setup chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) // restore so TempDir cleanup can remove it

	stderr := amplCaptureStderrBreaker(t, func() {
		writeBreaker(path, 99)
	})

	if !strings.Contains(stderr, "[contract-gate] WARN") || !strings.Contains(stderr, "could not persist breaker state") {
		t.Fatalf("expected a WARN on tmp-file write failure; got stderr:\n%q", stderr)
	}
	if got := readBreaker(path); got != 5 {
		t.Fatalf("expected prior state (5) to survive a failed write untouched, got %d", got)
	}
}

func TestWriteBreaker_RenameOntoExistingDir_WARNsDistinctFromWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "breaker-as-dir")
	// path is an existing NON-EMPTY directory: the atomic tmp-write succeeds
	// (its sibling ".tmp" file has no conflict), but os.Rename(tmp, path)
	// must fail because path is not a plain file.
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "occupant.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup occupant file: %v", err)
	}

	stderr := amplCaptureStderrBreaker(t, func() {
		writeBreaker(path, 1)
	})

	if !strings.Contains(stderr, "[contract-gate] WARN") || !strings.Contains(stderr, "could not commit breaker state") {
		t.Fatalf("expected a WARN on rename failure distinct from the write-failure message; got stderr:\n%q", stderr)
	}
	if strings.Contains(stderr, "could not persist breaker state") {
		t.Fatalf("rename failure must not be reported as a write failure; got stderr:\n%q", stderr)
	}
}

func TestWriteBreaker_RoundTrip_EdgeValues(t *testing.T) {
	cases := []int{0, -1, math.MaxInt32, math.MinInt32}
	for _, n := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "breaker.json")
		writeBreaker(path, n)
		if got := readBreaker(path); got != n {
			t.Errorf("round-trip mismatch for n=%d: got %d", n, got)
		}
	}
}

func TestReadBreaker_CorruptJSON_ReturnsZeroNoPanic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "breaker.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if got := readBreaker(path); got != 0 {
		t.Fatalf("expected corrupt JSON to fail safe to 0, got %d", got)
	}
}

func TestReadBreaker_EmptyPath_ReturnsZero(t *testing.T) {
	if got := readBreaker(""); got != 0 {
		t.Fatalf("expected empty path to return 0, got %d", got)
	}
}

func TestWriteBreaker_EmptyPath_NoPanic(t *testing.T) {
	writeBreaker("", 42) // must be a safe no-op, not a panic
}
