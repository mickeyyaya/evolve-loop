package core

// phase_bindings_selfcheck_failloud_test.go — RED contract for the fable5
// deep-scan finding selfcheck-breaker-fail-loud (inbox weight 0.91, cycle-618
// scout), core.phase_bindings_selfcheck.go half.
//
// Context. writeBuildSelfCheckArtifact persists the build self-check's failing
// packages so the audit/toolchain gate can read exact ground truth. Today every
// I/O step silently swallows its error:
//
//	if err := os.MkdirAll(dir, 0o755); err != nil {
//	    return
//	}
//	data, err := json.MarshalIndent(fails, "", "  ")
//	if err != nil {
//	    return
//	}
//	_ = os.WriteFile(dst, append(data, '\n'), 0o644)
//
// A write failure (disk full, permission denied, path collision) leaves the
// gate reading a STALE or MISSING artifact with zero operator-visible signal —
// the exact silent-skip shape this repo's fail-loud rule (AGENTS.md Rule 12)
// forbids. The self-check itself must stay fail-OPEN (never abort build on a
// persistence error — audit is the backstop, per the file's own header
// comment), but the failure must be WARNed to stderr the same way the sibling
// WARN two lines above it already is.
//
// RED today: writeBuildSelfCheckArtifact discards the MkdirAll error with no
// stderr output, so the assertion below fails for the right reason (empty
// captured stderr) rather than a compile error — this task's fix is a
// behavioral (fail-loud) change, not a new API.

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

// TestWriteBuildSelfCheck_WriteFailureSurfaces pins the fail-loud contract: a
// persistence failure (forced here by making ".evolve" collide with an
// existing regular file, so MkdirAll fails deterministically and portably)
// MUST WARN to stderr naming the artifact and the failure — never disappear
// silently.
func TestWriteBuildSelfCheck_WriteFailureSurfaces(t *testing.T) {
	wt := t.TempDir()
	// Collide the artifact directory with a regular file so MkdirAll fails
	// with ENOTDIR — deterministic, portable, no permission tricks needed.
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

// TestWriteBuildSelfCheck_HealthyWriteIsSilent is the positive/regression
// twin: the common healthy-write path must stay byte-identical (no stderr
// noise) — the fail-loud fix must only fire ON failure, never unconditionally
// log every write (which would itself become noise the WARN convention exists
// to avoid).
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
