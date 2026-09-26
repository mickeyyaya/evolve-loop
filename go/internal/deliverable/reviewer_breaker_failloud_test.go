package deliverable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr swaps the process-global os.Stderr, so its callers must not run in parallel.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = orig
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	_ = r.Close()
	return string(buf[:n])
}

// A missing parent directory makes os.WriteFile fail deterministically on every platform.
func TestBreakerWriteFailureLogged(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "nonexistent-subdir", "breaker.json")

	out := captureStderr(t, func() {
		writeBreaker(badPath, 3)
	})

	if !strings.Contains(out, "WARN") {
		t.Fatalf("expected a WARN on breaker persistence write failure; got stderr:\n%q", out)
	}
	if !strings.Contains(out, "breaker") {
		t.Errorf("WARN should name the breaker persistence failure so an operator can find it; got:\n%q", out)
	}
}

func TestBreakerWriteSuccess_NoStderrNoise(t *testing.T) {
	path := filepath.Join(t.TempDir(), "breaker.json")

	out := captureStderr(t, func() {
		writeBreaker(path, 2)
	})

	if out != "" {
		t.Errorf("a successful breaker write must not emit stderr noise; got:\n%q", out)
	}
	if got := readBreaker(path); got != 2 {
		t.Errorf("readBreaker after write = %d, want 2", got)
	}
}
