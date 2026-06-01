package posteditvalidate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func payload(filePath string) []byte {
	return []byte(fmt.Sprintf(`{"tool_input":{"file_path":"%s"}}`, filePath))
}

// passingValidators are stub validators that always return OK.
func passingValidators() (json, sh, py func(string) (bool, string)) {
	ok := func(string) (bool, string) { return true, "" }
	return ok, ok, ok
}

// === No payload → skip =====================================================
func TestRun_NoPayload(t *testing.T) {
	res := Run(Options{ProjectRoot: t.TempDir()})
	if res.Kind != "skip" {
		t.Errorf("Kind = %q, want 'skip'", res.Kind)
	}
}

// === Bypass env honored ===================================================
func TestRun_Bypass(t *testing.T) {
	res := Run(Options{
		Payload:     payload("/tmp/whatever.json"),
		ProjectRoot: t.TempDir(),
		Bypass:      true,
	})
	if res.Kind != "bypass" {
		t.Errorf("Kind = %q, want 'bypass'", res.Kind)
	}
}

// === No file_path in payload → skip ========================================
func TestRun_NoFilePathInPayload(t *testing.T) {
	res := Run(Options{
		Payload:     []byte(`{"tool_input":{"other":"x"}}`),
		ProjectRoot: t.TempDir(),
	})
	if res.Kind != "skip" {
		t.Errorf("Kind = %q, want 'skip'", res.Kind)
	}
}

// === File doesn't exist (deleted) → skip ===================================
func TestRun_FileDoesNotExist(t *testing.T) {
	res := Run(Options{
		Payload:     payload("/tmp/this-file-definitely-does-not-exist-postedit.json"),
		ProjectRoot: t.TempDir(),
	})
	if res.Kind != "skip" {
		t.Errorf("Kind = %q, want 'skip'", res.Kind)
	}
}

// === Other extensions (.md, .txt) → noop ===================================
func TestRun_OtherExtension(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "readme.md")
	if err := os.WriteFile(p, []byte("# hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res := Run(Options{
		Payload:     payload(p),
		ProjectRoot: d,
	})
	if res.Kind != "noop" || !res.OK {
		t.Errorf("res = %+v, want noop + OK", res)
	}
}

// === Valid JSON → OK, no WARN ===============================================
func TestRun_ValidJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "good.json")
	if err := os.WriteFile(p, []byte(`{"k":"v"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	vJSON := func(string) (bool, string) { return true, "" }
	res := Run(Options{
		Payload:      payload(p),
		ProjectRoot:  d,
		LLMStderr:    &buf,
		ValidateJSON: vJSON,
	})
	if res.Kind != "json" || !res.OK || res.WarnEmitted {
		t.Errorf("res = %+v, want json+OK no warn", res)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no LLM output: %q", buf.String())
	}
}

// === Invalid JSON → WARN to LLM stderr =====================================
func TestRun_InvalidJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte(`{`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	vJSON := func(string) (bool, string) { return false, "unexpected EOF" }
	res := Run(Options{
		Payload:      payload(p),
		ProjectRoot:  d,
		LLMStderr:    &buf,
		ValidateJSON: vJSON,
	})
	if res.Kind != "json" || res.OK || !res.WarnEmitted {
		t.Errorf("res = %+v, want json+!OK+warn", res)
	}
	if !strings.Contains(buf.String(), "does NOT parse as valid JSON") {
		t.Errorf("LLM stderr missing JSON warn: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "Bypass: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1") {
		t.Errorf("LLM stderr missing bypass hint")
	}
}

// === Bash syntax error → WARN ==============================================
func TestRun_BashError(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.sh")
	if err := os.WriteFile(p, []byte("if then fi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	vBash := func(string) (bool, string) { return false, "unexpected token 'then'" }
	res := Run(Options{
		Payload:      payload(p),
		ProjectRoot:  d,
		LLMStderr:    &buf,
		ValidateBash: vBash,
	})
	if res.Kind != "sh" || res.OK {
		t.Errorf("res = %+v, want sh+!OK", res)
	}
	if !strings.Contains(buf.String(), "bash syntax error") {
		t.Errorf("LLM stderr missing bash warn: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "declare -A") {
		t.Errorf("LLM stderr missing bash-3.2 hint: %q", buf.String())
	}
}

// === Python compile error → WARN ===========================================
func TestRun_PyError(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.py")
	if err := os.WriteFile(p, []byte("def x("), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	vPy := func(string) (bool, string) { return false, "SyntaxError" }
	res := Run(Options{
		Payload:     payload(p),
		ProjectRoot: d,
		LLMStderr:   &buf,
		ValidatePy:  vPy,
	})
	if res.Kind != "py" || res.OK {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(buf.String(), "Python compile error") {
		t.Errorf("LLM stderr missing py warn: %q", buf.String())
	}
}

// === Valid bash → OK, no WARN, OK logged ===================================
func TestRun_ValidBash(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "good.sh")
	if err := os.WriteFile(p, []byte("echo hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	fixedNow := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	vBash := func(string) (bool, string) { return true, "" }
	res := Run(Options{
		Payload:      payload(p),
		ProjectRoot:  d,
		LLMStderr:    &buf,
		ValidateBash: vBash,
		Now:          func() time.Time { return fixedNow },
	})
	if res.Kind != "sh" || !res.OK || res.WarnEmitted {
		t.Errorf("res = %+v, want sh+OK no warn", res)
	}
	if buf.Len() != 0 {
		t.Errorf("valid bash must emit no LLM output: %q", buf.String())
	}
	logBody, err := os.ReadFile(filepath.Join(d, ".evolve", "guards.log"))
	if err != nil {
		t.Fatalf("read guards.log: %v", err)
	}
	if !strings.Contains(string(logBody), "(bash syntax)") {
		t.Errorf("guards.log missing bash OK entry: %q", logBody)
	}
}

// === Valid python → OK, no WARN, OK logged =================================
func TestRun_ValidPy(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "good.py")
	if err := os.WriteFile(p, []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buf bytes.Buffer
	vPy := func(string) (bool, string) { return true, "" }
	res := Run(Options{
		Payload:     payload(p),
		ProjectRoot: d,
		LLMStderr:   &buf,
		ValidatePy:  vPy,
	})
	if res.Kind != "py" || !res.OK || res.WarnEmitted {
		t.Errorf("res = %+v, want py+OK no warn", res)
	}
	if buf.Len() != 0 {
		t.Errorf("valid py must emit no LLM output: %q", buf.String())
	}
	logBody, err := os.ReadFile(filepath.Join(d, ".evolve", "guards.log"))
	if err != nil {
		t.Fatalf("read guards.log: %v", err)
	}
	if !strings.Contains(string(logBody), "(py_compile)") {
		t.Errorf("guards.log missing py OK entry: %q", logBody)
	}
}

// === guards.log gets appended ==============================================
func TestRun_AppendsGuardsLog(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "ok.json")
	if err := os.WriteFile(p, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fixedNow := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	vJSON := func(string) (bool, string) { return true, "" }
	_ = Run(Options{
		Payload:      payload(p),
		ProjectRoot:  d,
		ValidateJSON: vJSON,
		Now:          func() time.Time { return fixedNow },
	})
	body, err := os.ReadFile(filepath.Join(d, ".evolve", "guards.log"))
	if err != nil {
		t.Fatalf("read guards.log: %v", err)
	}
	if !strings.Contains(string(body), "2026-05-24T12:00:00Z") {
		t.Errorf("guards.log missing timestamp: %q", body)
	}
	if !strings.Contains(string(body), "OK:") || !strings.Contains(string(body), "(json)") {
		t.Errorf("guards.log missing OK entry: %q", body)
	}
}

// === extractFilePath table =================================================
func TestExtractFilePath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"happy", `{"tool_input":{"file_path":"/x/y.json"}}`, "/x/y.json"},
		{"no tool_input", `{"other":"x"}`, ""},
		{"empty file_path", `{"tool_input":{"file_path":""}}`, ""},
		{"malformed", `not json`, ""},
		{"nested", `{"a":{"tool_input":{"file_path":"/x"}}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFilePath([]byte(tc.in))
			if got != tc.want {
				t.Errorf("extractFilePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// === defaultValidators on real fixtures ====================================
func TestDefaultValidateJSON_Fallback(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.json")
	if err := os.WriteFile(p, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ok, msg := defaultValidateJSON(p)
	if !ok {
		t.Errorf("defaultValidateJSON valid JSON returned !ok: %q", msg)
	}

	bad := filepath.Join(d, "bad.json")
	if err := os.WriteFile(bad, []byte(`{`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if ok, _ := defaultValidateJSON(bad); ok {
		t.Errorf("defaultValidateJSON malformed returned ok")
	}
}

func TestDefaultValidateBash(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "ok.sh")
	if err := os.WriteFile(p, []byte("echo hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if ok, msg := defaultValidateBash(p); !ok {
		t.Errorf("valid bash returned !ok: %q", msg)
	}
	bad := filepath.Join(d, "bad.sh")
	if err := os.WriteFile(bad, []byte("if; then\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if ok, _ := defaultValidateBash(bad); ok {
		t.Errorf("invalid bash returned ok")
	}
}

// === cleanPyCache removes stamped pyc =====================================
func TestCleanPyCache(t *testing.T) {
	d := t.TempDir()
	pyFile := filepath.Join(d, "mod.py")
	cacheDir := filepath.Join(d, "__pycache__")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Drop both a stamped pyc for mod.py and an unrelated one.
	for _, name := range []string{
		"mod.cpython-312.pyc",
		"mod.cpython-311.pyc",
		"other.cpython-312.pyc",
	} {
		if err := os.WriteFile(filepath.Join(cacheDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	cleanPyCache(pyFile)
	// mod.* should be gone, other.* should remain.
	for _, gone := range []string{"mod.cpython-312.pyc", "mod.cpython-311.pyc"} {
		if _, err := os.Stat(filepath.Join(cacheDir, gone)); err == nil {
			t.Errorf("%s should have been removed", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "other.cpython-312.pyc")); err != nil {
		t.Errorf("other.cpython-312.pyc should have survived")
	}
}

// === appendGuardsLog tolerates missing parent dir =========================
func TestAppendGuardsLog_CreatesDir(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "nested", "deeper", "guards.log")
	appendGuardsLog(path, time.Now(), "test msg")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "test msg") {
		t.Errorf("log missing message: %q", body)
	}
}
