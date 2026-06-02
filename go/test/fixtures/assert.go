package fixtures

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is a thin assertion facade over the most-duplicated must*/want*
// helpers scattered across the suite (55+ at last count). It deliberately
// stays small — it is NOT a testify clone. Every helper marks itself with
// t.Helper() so a failure points at the caller's line, and every message
// names the value under test for debuggability.

// RequireNoErr fails the test immediately if err is non-nil. msg is an
// optional context label shown in the failure.
func RequireNoErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		if msg == "" {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Fatalf("%s: %v", msg, err)
	}
}

// RequireErr fails if err is nil (the test expected a failure).
func RequireErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected an error, got nil", msg)
	}
}

// RequireErrContains fails unless err is non-nil and its message contains sub.
func RequireErrContains(t *testing.T, err error, sub string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", sub)
	}
	if !strings.Contains(err.Error(), sub) {
		t.Fatalf("error %q does not contain %q", err.Error(), sub)
	}
}

// MustWrite writes body to path, creating parent directories as needed, and
// fails on error. Returns path for chaining.
func MustWrite(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MustWrite(%q): mkdir: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("MustWrite(%q): %v", path, err)
	}
	return path
}

// MustRead reads path and fails on error.
func MustRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("MustRead(%q): %v", path, err)
	}
	return string(raw)
}

// WantFileContains fails unless path's contents include sub.
func WantFileContains(t *testing.T, path, sub string) {
	t.Helper()
	if got := MustRead(t, path); !strings.Contains(got, sub) {
		t.Fatalf("file %q does not contain %q", path, sub)
	}
}

// FilePresent is a PURE boolean file-existence check (no t, never logs). Use
// it for genuine skip preconditions; reserve t.Skip for real environment
// absence. This is the fix for the "FileExists-as-skip" anti-pattern where
// acsassert.FileExists logs an Errorf and still gets used as a skip guard.
func FilePresent(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
