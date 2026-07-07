package deliverable

// reviewer_breaker_failloud_test.go — RED contract for the fable5 deep-scan
// finding selfcheck-breaker-fail-loud (inbox weight 0.91, cycle-618 scout),
// deliverable.reviewer.go circuit-breaker-persistence half.
//
// Context. writeBreaker persists the contract-gate's consecutive-block
// counter so it survives the per-cycle orchestrator reconstruction. Today the
// write/rename errors are discarded outright:
//
//	func writeBreaker(path string, n int) {
//	    ...
//	    data, _ := json.Marshal(breakerState{Consecutive: n})
//	    tmp := path + ".tmp"
//	    if os.WriteFile(tmp, data, 0o644) == nil {
//	        _ = os.Rename(tmp, path) // atomic
//	    }
//	}
//
// A write failure here silently resets the breaker's effective state to
// "never persisted" — readBreaker treats a missing/unreadable file as zero
// (fail-open by omission), so a genuinely tripped breaker can lose its count
// across a restart with zero operator-visible signal. This does not change the
// documented fail-open posture (the breaker still degrades enforce→advisory
// rather than aborting) — it only makes the persistence failure WARN-visible,
// matching the sibling `[contract-gate]` WARN convention already used
// elsewhere in this file (see logf calls in the enforce/shadow branches).
//
// RED today: writeBreaker discards the WriteFile error with no stderr output,
// so the assertion below fails for the right reason (empty captured stderr)
// rather than a compile error — this is a behavioral (fail-loud) fix, not a
// new API.

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

// TestBreakerWriteFailureLogged pins the fail-loud contract: a persistence
// failure (forced here by pointing at a path whose parent directory does not
// exist, so os.WriteFile fails ENOENT deterministically and portably) MUST
// WARN to stderr naming the breaker persistence failure — never disappear
// silently.
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

// TestBreakerWriteSuccess_NoStderrNoise is the positive/regression twin: the
// common healthy-write path must stay byte-identical (no stderr noise) — the
// fail-loud fix must only fire ON failure.
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
