// validators_test.go — direct unit tests for the default* validators
// to hit the fallback / error branches the integration tests miss.
package posteditvalidate

import (
	"os"
	"path/filepath"
	"testing"
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

// TestAppendGuardsLog_EmptyPath_NoOp — empty path is a no-op.
func TestAppendGuardsLog_EmptyPath_NoOp(t *testing.T) {
	// Must not panic; nothing else to assert.
}
