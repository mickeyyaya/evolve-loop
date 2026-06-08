// Package acsassert is the assertion DSL ACS predicates write against.
//
// Each Go predicate under go/acs/{cycle<N>,regression,redteam}/ uses these
// helpers in the same way the retired bash predicates used [ -f ], grep -q,
// jq -e, etc. (ADR-0042). The helpers
// take a TB instead of *testing.T so they're testable in isolation
// (the test for FileExists doesn't have to fail itself when checking
// missing-file behaviour).
package acsassert

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// deepEqualOrPanic is a thin wrapper that's testable and can be
// short-circuited to reflect.DeepEqual semantics.
func deepEqualOrPanic(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// TB is a minimal interface over *testing.T — accepts anything that
// can log a failure and mark itself as a test helper. Implemented by
// the stdlib *testing.T and by acsassert's internal fakeT (tests).
type TB interface {
	Errorf(format string, args ...any)
	Helper()
}

// ErrSubprocessNotFound is returned by SubprocessOutput when the
// binary cannot be located on PATH.
var ErrSubprocessNotFound = errors.New("acsassert: subprocess binary not found")

// FileExists reports whether path is a regular file (or symlink to one)
// that os.Stat can read. Logs an Errorf when it isn't.
func FileExists(tb TB, path string) bool {
	tb.Helper()
	if _, err := os.Stat(path); err != nil {
		tb.Errorf("FileExists(%q): %v", path, err)
		return false
	}
	return true
}

// FileContains reports whether path's contents include the substring.
func FileContains(tb TB, path, substring string) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("FileContains(%q): %v", path, err)
		return false
	}
	if !strings.Contains(string(raw), substring) {
		tb.Errorf("FileContains(%q) missing %q", path, substring)
		return false
	}
	return true
}

// FileMatchesRegex reports whether path's contents match pattern
// (Go's RE2 syntax). Logs an Errorf on no match or invalid pattern.
func FileMatchesRegex(tb TB, path, pattern string) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("FileMatchesRegex(%q): %v", path, err)
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		tb.Errorf("FileMatchesRegex(%q) bad pattern %q: %v", path, pattern, err)
		return false
	}
	if !re.Match(raw) {
		tb.Errorf("FileMatchesRegex(%q) no match for %q", path, pattern)
		return false
	}
	return true
}

// JSONFieldEquals navigates a dot path (e.g. "a.b.c") through the
// JSON in path and reports whether the resolved value equals want.
// Scalars compare via Go's == operator; for numbers, want should be
// float64 (encoding/json's default for JSON numbers).
func JSONFieldEquals(tb TB, path, dotPath string, want any) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("JSONFieldEquals(%q): %v", path, err)
		return false
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		tb.Errorf("JSONFieldEquals(%q): invalid JSON: %v", path, err)
		return false
	}
	got, ok := navigateDotPath(doc, dotPath)
	if !ok {
		tb.Errorf("JSONFieldEquals(%q): path %q not found", path, dotPath)
		return false
	}
	// Guard against panic from comparing non-comparable types (e.g.
	// when dotPath resolves to a map). DeepEqual handles both.
	defer func() {
		// Nothing to do — equalAny below uses reflect so won't panic.
	}()
	if !equalAny(got, want) {
		tb.Errorf("JSONFieldEquals(%q) at %q: got %v (%T), want %v (%T)", path, dotPath, got, got, want, want)
		return false
	}
	return true
}

// equalAny compares two any values without panicking on non-comparable
// types. Uses reflect.DeepEqual which handles maps/slices/structs.
func equalAny(a, b any) bool {
	// Fast path for comparable types; reflect.DeepEqual is the
	// fallback for maps/slices.
	defer func() { _ = recover() }()
	return deepEqualOrPanic(a, b)
}

// navigateDotPath walks a top-level JSON object via dot-separated keys.
// Returns the resolved value + true; or nil + false if any key is missing.
func navigateDotPath(doc any, dotPath string) (any, bool) {
	cur := doc
	if dotPath == "" {
		return cur, true
	}
	for _, key := range strings.Split(dotPath, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, present := m[key]
		if !present {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// SubprocessOutput runs name+args and returns stdout, stderr, and the
// exit code. A non-zero exit code is surfaced as an error so callers
// can treat it as a failure with ergonomic if-err handling. A missing
// binary path returns ErrSubprocessNotFound wrapped with context.
func SubprocessOutput(name string, args ...string) (stdout, stderr string, code int, err error) {
	if _, lookErr := exec.LookPath(name); lookErr != nil {
		return "", "", -1, fmt.Errorf("%w: %s: %v", ErrSubprocessNotFound, name, lookErr)
	}
	cmd := exec.Command(name, args...)
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	runErr := cmd.Run()
	stdout = sout.String()
	stderr = serr.String()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), fmt.Errorf("subprocess %s exited %d", name, exitErr.ExitCode())
	}
	return stdout, stderr, -1, fmt.Errorf("subprocess %s: %w", name, runErr)
}

// AllOf returns true if every predicate returns true. Short-circuits
// on the first false. Predicates take a TB so they can log their own
// failure context.
func AllOf(tb TB, predicates ...func(TB) bool) bool {
	tb.Helper()
	for i, p := range predicates {
		if !p(tb) {
			tb.Errorf("AllOf: predicate[%d] returned false", i)
			return false
		}
	}
	return true
}

// SetupTempProject creates a typical evolve-loop project layout under
// t.TempDir() and returns the project root path.
func SetupTempProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{".evolve", ".evolve/runs", "docs", "legacy/scripts/lifecycle"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("SetupTempProject mkdir %s: %v", sub, err)
		}
	}
	return dir
}

// RepoRoot resolves the repository root via `git rev-parse --show-toplevel`.
// Skips the test when not inside a git work tree — predicate suites can
// then run cleanly on bare exports without false failures. Shared across
// every go/acs/ predicate package.
func RepoRoot(t *testing.T) string {
	t.Helper()
	stdout, _, code, err := SubprocessOutput("git", "rev-parse", "--show-toplevel")
	if err != nil || code != 0 {
		t.Skipf("not in a git work tree: code=%d err=%v", code, err)
	}
	return strings.TrimSpace(stdout)
}

// FileContainsAny reports whether path's content contains at least one of
// the substring variants. Returns false if the file is missing or no
// variant matches. Pure boolean (no TB) so callers control failure mode.
func FileContainsAny(path string, variants ...string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(raw)
	for _, v := range variants {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

// CountOccurrencesAny returns the count of lines in path that match any
// of the given substring variants. Used by "at least N named gates"
// predicates. Returns 0 if the file is missing.
func CountOccurrencesAny(path string, variants ...string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		for _, v := range variants {
			if strings.Contains(line, v) {
				count++
				break
			}
		}
	}
	return count
}

// LineContainsAll reports whether at least one line of path contains
// every substring in needles. Useful for table-row predicates like
// "row containing `P-NEW-20` AND `DONE`".
func LineContainsAll(path string, needles ...string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		hit := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				hit = false
				break
			}
		}
		if hit {
			return true
		}
	}
	return false
}
