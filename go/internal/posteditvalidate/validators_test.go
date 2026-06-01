// validators_test.go — direct unit tests for the default* validators
// to hit the fallback / error branches the integration tests miss.
package posteditvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDefaultValidateJSON_ValidJSON_FallbackToEncoding — even when jq
// is absent, the encoding/json fallback parses valid JSON. We can't
// portably remove jq from PATH; this test simply confirms valid JSON
// passes (which exercises one branch of the fallback path on systems
// without jq, and the jq-success path on systems with jq).
func TestDefaultValidateJSON_ValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.json")
	if err := os.WriteFile(path, []byte(`{"a": 1, "b": [2, 3]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidateJSON(path)
	if !ok {
		t.Errorf("valid JSON should pass; got msg=%q", msg)
	}
}

// TestDefaultValidateJSON_InvalidJSON — malformed JSON fails through
// either the jq path or the encoding/json fallback.
func TestDefaultValidateJSON_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"a":`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidateJSON(path)
	if ok {
		t.Errorf("malformed JSON should fail")
	}
	if msg == "" {
		t.Errorf("expected error message")
	}
}

// TestDefaultValidateJSON_MissingFile — non-existent file fails.
func TestDefaultValidateJSON_MissingFile(t *testing.T) {
	ok, msg := defaultValidateJSON("/no/such/file.json")
	if ok {
		t.Errorf("missing file should fail")
	}
	if msg == "" {
		t.Errorf("expected error message")
	}
}

// TestDefaultValidateBash_ValidScript — bash -n on a valid script
// returns true. Skipped if bash isn't present (handled by the impl as
// "true with empty message" — i.e., don't false-positive).
func TestDefaultValidateBash_ValidScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.sh")
	if err := os.WriteFile(path, []byte("#!/bin/bash\necho hello\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, _ := defaultValidateBash(path)
	if !ok {
		t.Errorf("valid bash should pass")
	}
}

// TestDefaultValidateBash_SyntaxError — syntax error caught by bash -n
// (when present). When bash is absent the impl returns ok=true to
// avoid false positives — both outcomes are acceptable.
func TestDefaultValidateBash_SyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.sh")
	if err := os.WriteFile(path, []byte("if true\nthen echo no fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// We don't assert ok=false because bash may be absent on some CI
	// runners; we just verify the call doesn't panic.
	_, _ = defaultValidateBash(path)
}

// TestDefaultValidatePy_ValidScript — basic happy path when python is
// available. When absent the impl returns ok=true to avoid false
// positives.
func TestDefaultValidatePy_ValidScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.py")
	if err := os.WriteFile(path, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, _ := defaultValidatePy(path)
	if !ok {
		t.Errorf("valid python should pass (or skip if python absent)")
	}
	// Best-effort cleanup of __pycache__
	cleanPyCache(path)
}

// TestDefaultValidatePy_SyntaxError — syntax error caught when python
// is present.
func TestDefaultValidatePy_SyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.py")
	if err := os.WriteFile(path, []byte("def f(:\n  pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Don't assert — python may be absent. Just exercise the path.
	_, _ = defaultValidatePy(path)
	cleanPyCache(path)
}

// TestCleanPyCache_MissingDir_NoCrash — missing __pycache__ is a no-op.
func TestCleanPyCache_MissingDir_NoCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.py")
	cleanPyCache(path) // should not panic
}

// TestCleanPyCache_RemovesMatchingFiles — drops .pyc files matching
// the source basename.
func TestCleanPyCache_RemovesMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "mod.py")
	cacheDir := filepath.Join(dir, "__pycache__")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cacheDir, "mod.cpython-311.pyc")
	other := filepath.Join(cacheDir, "another.cpython-311.pyc")
	_ = os.WriteFile(target, []byte("x"), 0o644)
	_ = os.WriteFile(other, []byte("y"), 0o644)

	cleanPyCache(srcPath)

	if _, err := os.Stat(target); err == nil {
		t.Errorf("expected %s to be removed", target)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("expected unrelated .pyc to survive; err=%v", err)
	}
}

// TestAppendGuardsLog_EmptyPath_NoOp — an empty path is an early-return no-op:
// no file is created in the cwd and nothing panics.
func TestAppendGuardsLog_EmptyPath_NoOp(t *testing.T) {
	// Arrange: run from an isolated cwd so a stray write would be detectable.
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Act
	appendGuardsLog("", time.Now(), "should be dropped")

	// Assert: empty path → no file written anywhere under the isolated cwd.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty path must write nothing; found %d entries", len(entries))
	}
}

// TestAppendGuardsLog_OpenError_NoOp — when the target path can't be opened
// (its parent is a regular file, so MkdirAll + OpenFile both fail), the
// function swallows the error and writes nothing rather than panicking. This
// pins the "best-effort, read-only-sandbox-tolerant" contract.
func TestAppendGuardsLog_OpenError_NoOp(t *testing.T) {
	// Arrange: make a regular file, then try to log "underneath" it.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	target := filepath.Join(blocker, "guards.log") // parent is a file → open fails

	// Act + Assert: must not panic, and the blocker file is untouched.
	appendGuardsLog(target, time.Now(), "dropped")

	body, err := os.ReadFile(blocker)
	if err != nil {
		t.Fatalf("read blocker: %v", err)
	}
	if string(body) != "x" {
		t.Errorf("blocker file mutated; got %q want %q", body, "x")
	}
}

// withEmptyPATH points PATH at an empty dir so exec.LookPath fails for every
// external tool. Cannot be combined with t.Parallel (t.Setenv forbids it).
func withEmptyPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

// TestDefaultValidateJSON_NoJq_EncodingFallback_Valid — with jq off PATH, a
// valid JSON file passes through the encoding/json fallback branch.
func TestDefaultValidateJSON_NoJq_EncodingFallback_Valid(t *testing.T) {
	withEmptyPATH(t)
	path := filepath.Join(t.TempDir(), "ok.json")
	if err := os.WriteFile(path, []byte(`{"a":1,"b":[2,3]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidateJSON(path)
	if !ok || msg != "" {
		t.Errorf("valid JSON via encoding fallback: got (%v,%q), want (true,\"\")", ok, msg)
	}
}

// TestDefaultValidateJSON_NoJq_EncodingFallback_Invalid — with jq off PATH,
// malformed JSON fails through the encoding/json fallback and returns the
// decoder's error string.
func TestDefaultValidateJSON_NoJq_EncodingFallback_Invalid(t *testing.T) {
	withEmptyPATH(t)
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"a":`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidateJSON(path)
	if ok {
		t.Errorf("malformed JSON via fallback must fail")
	}
	if msg == "" {
		t.Errorf("fallback failure must carry a decoder error message")
	}
}

// TestDefaultValidateJSON_NoJq_EncodingFallback_MissingFile — with jq off
// PATH, a missing file fails on the os.ReadFile branch of the fallback.
func TestDefaultValidateJSON_NoJq_EncodingFallback_MissingFile(t *testing.T) {
	withEmptyPATH(t)
	missing := filepath.Join(t.TempDir(), "nope.json")
	ok, msg := defaultValidateJSON(missing)
	if ok {
		t.Errorf("missing file via fallback must fail")
	}
	if msg == "" {
		t.Errorf("missing-file failure must carry a read error message")
	}
}

// TestDefaultValidateBash_NoBash_NoFalsePositive — with bash off PATH the
// validator returns (true,"") rather than false-positiving on an unparseable
// script. Pins the "tool absent → don't block" contract.
func TestDefaultValidateBash_NoBash_NoFalsePositive(t *testing.T) {
	withEmptyPATH(t)
	path := filepath.Join(t.TempDir(), "syntactically-broken.sh")
	if err := os.WriteFile(path, []byte("if then fi ("), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidateBash(path)
	if !ok || msg != "" {
		t.Errorf("bash absent: got (%v,%q), want (true,\"\")", ok, msg)
	}
}

// TestDefaultValidatePy_NoPython_NoFalsePositive — with python3 and python
// both off PATH the validator returns (true,"") rather than false-positiving.
func TestDefaultValidatePy_NoPython_NoFalsePositive(t *testing.T) {
	withEmptyPATH(t)
	path := filepath.Join(t.TempDir(), "syntactically-broken.py")
	if err := os.WriteFile(path, []byte("def f(:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidatePy(path)
	if !ok || msg != "" {
		t.Errorf("python absent: got (%v,%q), want (true,\"\")", ok, msg)
	}
}

// TestDefaultValidatePy_FallsBackToPython2Name — when python3 is absent but a
// bare `python` exists, the validator selects `python` (the second LookPath
// branch). We synthesize a PATH holding only a stub `python` shell script that
// reports a compile failure, and assert the validator routes through it.
func TestDefaultValidatePy_FallsBackToPython2Name(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available for stub interpreter")
	}
	binDir := t.TempDir()
	// Stub `python`: emits a marker to stderr and exits non-zero so we can
	// prove (a) the `python` branch was chosen and (b) the failure propagates.
	stub := filepath.Join(binDir, "python")
	script := "#!/bin/sh\necho 'STUB-PYTHON-INVOKED' 1>&2\nexit 1\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir) // only `python` is reachable; no `python3`

	path := filepath.Join(t.TempDir(), "mod.py")
	if err := os.WriteFile(path, []byte("print('x')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, msg := defaultValidatePy(path)
	if ok {
		t.Errorf("stub python exited non-zero; want ok=false, got ok=true")
	}
	if !strings.Contains(msg, "STUB-PYTHON-INVOKED") {
		t.Errorf("expected the `python` fallback to be invoked; got msg=%q", msg)
	}
}
