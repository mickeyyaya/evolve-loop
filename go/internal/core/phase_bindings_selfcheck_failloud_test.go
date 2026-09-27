package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr redirects os.Stderr for the duration of fn and returns
// everything written to it. Not parallel-safe with other stderr-capturing
// tests in this package (os.Stderr is process-global) — callers must not
// mark themselves t.Parallel().
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

// A persistence failure (forced here by making ".evolve" collide with an
// existing regular file, so MkdirAll fails deterministically and portably)
// MUST WARN to stderr naming the artifact and the failure — never disappear
// silently.
func TestWriteBuildSelfCheck_WriteFailureSurfaces(t *testing.T) {
	wt := t.TempDir()
	if err := os.WriteFile(filepath.Join(wt, ".evolve"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	fails := []selfCheckFailure{{Pkg: "./internal/foo", Output: "--- FAIL: TestFoo"}}

	out := captureStderr(t, func() {
		writeBuildSelfCheckArtifact(wt, fails)
	})

	if !strings.Contains(out, "WARN") {
		t.Fatalf("expected a WARN on build-selfcheck artifact write failure; got stderr:\n%q", out)
	}
	if !strings.Contains(out, "build-selfcheck") {
		t.Errorf("WARN should name the build-selfcheck artifact so an operator can find it; got:\n%q", out)
	}
}

// The positive/regression twin of the test above: the common healthy-write
// path must stay byte-identical (no stderr noise) — the WARN must only fire
// ON failure, never unconditionally log every write (which would itself
// become noise the WARN convention exists to avoid).
func TestWriteBuildSelfCheck_HealthyWriteIsSilent(t *testing.T) {
	wt := t.TempDir()
	fails := []selfCheckFailure{{Pkg: "./internal/foo", Output: "--- FAIL: TestFoo"}}

	out := captureStderr(t, func() {
		writeBuildSelfCheckArtifact(wt, fails)
	})

	if out != "" {
		t.Errorf("a successful write must not emit stderr noise; got:\n%q", out)
	}
	data, err := os.ReadFile(filepath.Join(wt, ".evolve", "build-selfcheck.json"))
	if err != nil {
		t.Fatalf("artifact not written on the healthy path: %v", err)
	}
	if !strings.Contains(string(data), "./internal/foo") {
		t.Errorf("artifact must still contain the failing package: %s", data)
	}
}
