//go:build integration

package ship

// RED-phase contract for cycle-249 task `macos-ebadf-test-hardening`
// (inbox: macos-ci-ebadf-flake-hardening).
//
// TestShipFromWorktree_GitAddFails_Errors flakes on macos-latest CI with
// `read |0: bad file descriptor` — a darwin pipe-teardown race in the
// test git-runner's CombinedOutput path. The mitigation is TEST-INFRA
// ONLY: a capture helper that retries exactly once when the error chain
// contains syscall.EBADF or io.ErrClosedPipe, used by runGit/runGitOut.
//
// Contract (to be implemented in a _test.go helper file — production
// ship/ files must NOT change):
//
//	func captureWithEBADFRetry(run func() ([]byte, error)) ([]byte, error)
//
// Fails at baseline: captureWithEBADFRetry is undefined (compile RED).

import (
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
)

// ebadfPathError mirrors the exact error shape the flake produces:
// (*os.PathError){Op: "read", Path: "|0", Err: syscall.EBADF}.
func ebadfPathError() error {
	return &os.PathError{Op: "read", Path: "|0", Err: syscall.EBADF}
}

func TestCaptureWithEBADFRetry_RetriesOnceOnEBADF(t *testing.T) {
	calls := 0
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, ebadfPathError()
		}
		return []byte("ok-output"), nil
	})
	if err != nil {
		t.Fatalf("expected the retry to absorb the transient EBADF, got err: %v", err)
	}
	if string(out) != "ok-output" {
		t.Errorf("out = %q, want %q", out, "ok-output")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (one EBADF + one retry)", calls)
	}
}

func TestCaptureWithEBADFRetry_RetriesOnceOnClosedPipe(t *testing.T) {
	calls := 0
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, io.ErrClosedPipe
		}
		return []byte("recovered"), nil
	})
	if err != nil {
		t.Fatalf("expected the retry to absorb io.ErrClosedPipe, got err: %v", err)
	}
	if string(out) != "recovered" || calls != 2 {
		t.Errorf("out=%q calls=%d, want %q / 2", out, calls, "recovered")
	}
}

// Negative: retry ONCE, not forever — a persistent EBADF must surface
// after exactly two attempts, preserving the original error chain.
func TestCaptureWithEBADFRetry_PersistentEBADF_FailsAfterOneRetry(t *testing.T) {
	calls := 0
	_, err := captureWithEBADFRetry(func() ([]byte, error) {
		calls++
		return nil, ebadfPathError()
	})
	if err == nil {
		t.Fatal("persistent EBADF must surface an error, not loop or swallow")
	}
	if calls != 2 {
		t.Errorf("calls = %d, want exactly 2 (no infinite retry)", calls)
	}
	if !errors.Is(err, syscall.EBADF) {
		t.Errorf("returned error must preserve the EBADF chain; got: %v", err)
	}
}

// Negative: a non-EBADF failure (e.g. a real git error) must NOT be
// retried — masking genuine failures would hide real test signal.
func TestCaptureWithEBADFRetry_NonEBADFError_NoRetry(t *testing.T) {
	calls := 0
	sentinel := errors.New("exit status 128: not a git repository")
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		calls++
		return []byte("partial"), sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("non-EBADF error must pass through unchanged; got: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on non-EBADF errors)", calls)
	}
	if string(out) != "partial" {
		t.Errorf("captured output must pass through on failure; got %q", out)
	}
}

func TestCaptureWithEBADFRetry_SuccessFirstTry_NoRetry(t *testing.T) {
	calls := 0
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		calls++
		return []byte("clean"), nil
	})
	if err != nil || string(out) != "clean" || calls != 1 {
		t.Errorf("clean first attempt must be returned as-is: out=%q err=%v calls=%d", out, err, calls)
	}
}
