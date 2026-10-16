package posteditvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

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

func TestDefaultValidateJSON_MissingFile(t *testing.T) {
	ok, msg := defaultValidateJSON("/no/such/file.json")
	if ok {
		t.Errorf("missing file should fail")
	}
	if msg == "" {
		t.Errorf("expected error message")
	}
}

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

func TestDefaultValidateBash_SyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.sh")
	if err := os.WriteFile(path, []byte("if true\nthen echo no fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _ = defaultValidateBash(path)
}

func TestDefaultValidatePy_ValidScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.py")
	if err := os.WriteFile(path, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, _ := defaultValidatePy(path)
	if !ok {
		t.Errorf("valid python should pass (or skip if python absent)")
	}
	cleanPyCache(path)
}

func TestDefaultValidatePy_SyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.py")
	if err := os.WriteFile(path, []byte("def f(:\n  pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = defaultValidatePy(path)
	cleanPyCache(path)
}

func TestCleanPyCache_MissingDir_NoCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.py")
	cleanPyCache(path)
}

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

func TestAppendGuardsLog_EmptyPath_NoOp(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	appendGuardsLog("", time.Now(), "should be dropped")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty path must write nothing; found %d entries", len(entries))
	}
}

func TestAppendGuardsLog_OpenError_NoOp(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	target := filepath.Join(blocker, "guards.log")

	appendGuardsLog(target, time.Now(), "dropped")

	body, err := os.ReadFile(blocker)
	if err != nil {
		t.Fatalf("read blocker: %v", err)
	}
	if string(body) != "x" {
		t.Errorf("blocker file mutated; got %q want %q", body, "x")
	}
}

func withEmptyPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

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

func TestDefaultValidatePy_FallsBackToPython2Name(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available for stub interpreter")
	}
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "python")
	script := "#!/bin/sh\necho 'STUB-PYTHON-INVOKED' 1>&2\nexit 1\n"
	fakeclitest.Install(t, stub, script)
	t.Setenv("PATH", binDir)

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
