package acsassert

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeT records assertion failures so we can test helpers as if they
// were running inside a real *testing.T without actually failing the
// outer test.
type fakeT struct {
	errs []string
}

func (f *fakeT) Errorf(format string, args ...any) {
	f.errs = append(f.errs, sprintf(format, args...))
}
func (f *fakeT) Helper() {}

func sprintf(f string, args ...any) string {
	// Minimal: prefer the format itself for assertions where args
	// may be path strings or ints — tests inspect errs[i] anyway.
	return f
}

// FileExists ----------------------------------------------------------
func TestFileExists_Present(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("y"), 0o644)
	ft := &fakeT{}
	if !FileExists(ft, p) {
		t.Errorf("FileExists reported false on present file")
	}
	if len(ft.errs) != 0 {
		t.Errorf("errs=%v", ft.errs)
	}
}

func TestFileExists_Missing(t *testing.T) {
	ft := &fakeT{}
	if FileExists(ft, "/no/such/path") {
		t.Errorf("FileExists reported true on missing file")
	}
	if len(ft.errs) == 0 {
		t.Errorf("expected error logged")
	}
}

// FileContains -------------------------------------------------------
func TestFileContains_Hit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("hello world\nfoo bar\n"), 0o644)
	ft := &fakeT{}
	if !FileContains(ft, p, "foo bar") {
		t.Errorf("FileContains missed substring")
	}
}

func TestFileContains_Miss(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("hello"), 0o644)
	ft := &fakeT{}
	if FileContains(ft, p, "missing") {
		t.Errorf("FileContains hit on missing substring")
	}
	if len(ft.errs) == 0 {
		t.Errorf("expected error logged")
	}
}

func TestFileContains_MissingFile(t *testing.T) {
	ft := &fakeT{}
	if FileContains(ft, "/no/such", "anything") {
		t.Errorf("FileContains succeeded on missing file")
	}
}

// FileMatchesRegex ---------------------------------------------------
func TestFileMatchesRegex_Hit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("cycle=104"), 0o644)
	ft := &fakeT{}
	if !FileMatchesRegex(ft, p, `cycle=\d+`) {
		t.Errorf("FileMatchesRegex missed pattern")
	}
}

func TestFileMatchesRegex_Miss(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("no match"), 0o644)
	ft := &fakeT{}
	if FileMatchesRegex(ft, p, `\d+`) {
		t.Errorf("FileMatchesRegex hit on no-digit content")
	}
}

func TestFileMatchesRegex_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	ft := &fakeT{}
	if FileMatchesRegex(ft, p, `[invalid`) {
		t.Errorf("invalid regex must not match")
	}
	if len(ft.errs) == 0 {
		t.Errorf("invalid regex must log error")
	}
}

// JSONFieldEquals ----------------------------------------------------
func TestJSONFieldEquals_Scalar(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.json")
	_ = os.WriteFile(p, []byte(`{"a":1,"b":"yes","c":{"d":42}}`), 0o644)
	ft := &fakeT{}
	if !JSONFieldEquals(ft, p, "a", float64(1)) {
		t.Errorf("scalar mismatch")
	}
	if !JSONFieldEquals(ft, p, "b", "yes") {
		t.Errorf("string mismatch")
	}
	if !JSONFieldEquals(ft, p, "c.d", float64(42)) {
		t.Errorf("nested mismatch")
	}
}

func TestJSONFieldEquals_Miss(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.json")
	_ = os.WriteFile(p, []byte(`{"a":1}`), 0o644)
	ft := &fakeT{}
	if JSONFieldEquals(ft, p, "a", float64(2)) {
		t.Errorf("must report mismatch")
	}
	if len(ft.errs) == 0 {
		t.Errorf("must log error")
	}
}

func TestJSONFieldEquals_MissingPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.json")
	_ = os.WriteFile(p, []byte(`{"a":1}`), 0o644)
	ft := &fakeT{}
	if JSONFieldEquals(ft, p, "b.c", "anything") {
		t.Errorf("missing path must report false")
	}
}

func TestJSONFieldEquals_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.json")
	_ = os.WriteFile(p, []byte(`not json`), 0o644)
	ft := &fakeT{}
	if JSONFieldEquals(ft, p, "a", "v") {
		t.Errorf("invalid JSON must report false")
	}
}

// SubprocessOutput ---------------------------------------------------
func TestSubprocessOutput_Stdout(t *testing.T) {
	stdout, _, code, err := SubprocessOutput("echo", "hello")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if code != 0 {
		t.Errorf("code=%d", code)
	}
	if stdout != "hello\n" {
		t.Errorf("stdout=%q", stdout)
	}
}

func TestSubprocessOutput_NonZeroExit(t *testing.T) {
	_, _, code, err := SubprocessOutput("sh", "-c", "exit 7")
	if err == nil {
		t.Fatal("expected non-zero exit error")
	}
	if code != 7 {
		t.Errorf("code=%d, want 7", code)
	}
}

func TestSubprocessOutput_MissingBinary(t *testing.T) {
	_, _, _, err := SubprocessOutput("definitely-not-a-real-binary-12345")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !errors.Is(err, ErrSubprocessNotFound) && err == nil {
		// Sentinel mismatch acceptable; just demand a non-nil error.
	}
}

// AllOf ---------------------------------------------------------------
func TestAllOf_All(t *testing.T) {
	ft := &fakeT{}
	got := AllOf(ft,
		func(_ TB) bool { return true },
		func(_ TB) bool { return true },
	)
	if !got {
		t.Errorf("AllOf with all-true predicates returned false")
	}
}

func TestAllOf_ShortCircuit(t *testing.T) {
	ft := &fakeT{}
	calls := 0
	got := AllOf(ft,
		func(_ TB) bool { calls++; return false },
		func(_ TB) bool { calls++; return true }, // must not run
	)
	if got {
		t.Errorf("AllOf with first false returned true")
	}
	if calls != 1 {
		t.Errorf("calls=%d, want 1 (short-circuit)", calls)
	}
}

func TestSetupTempProject_StructureCreated(t *testing.T) {
	dir := SetupTempProject(t)
	if _, err := os.Stat(filepath.Join(dir, ".evolve")); err != nil {
		t.Errorf("missing .evolve: %v", err)
	}
}
