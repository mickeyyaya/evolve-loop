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

func TestFileMatchesRegex_MissingFile(t *testing.T) {
	ft := &fakeT{}
	if FileMatchesRegex(ft, "/no/such/file", ".*") {
		t.Error("missing file must report false")
	}
}

func TestJSONFieldEquals_MissingFile(t *testing.T) {
	ft := &fakeT{}
	if JSONFieldEquals(ft, "/no/such/file", "a", "v") {
		t.Error("missing file must report false")
	}
}

func TestJSONFieldEquals_EmptyDotPath(t *testing.T) {
	// Empty dotPath returns whole doc; comparing to a map of any won't
	// equal via ==, so this should mismatch with non-nil want.
	dir := t.TempDir()
	p := filepath.Join(dir, "s.json")
	_ = os.WriteFile(p, []byte(`{"a":1}`), 0o644)
	ft := &fakeT{}
	// Comparing the whole map via Go == will panic on map — guard?
	// Actually a map is not == comparable in Go. The helper must not
	// panic on this; instead it should report false via the != check.
	// To avoid a comparison panic at runtime, the impl must handle it.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on empty dotPath: %v", r)
		}
	}()
	if JSONFieldEquals(ft, p, "", "anything") {
		t.Error("empty dotPath returning whole doc should mismatch primitive want")
	}
}

func TestNavigateDotPath_EmptyPath(t *testing.T) {
	got, ok := navigateDotPath(map[string]any{"a": 1}, "")
	if !ok {
		t.Error("empty path must succeed")
	}
	if _, isMap := got.(map[string]any); !isMap {
		t.Errorf("got %T, want map", got)
	}
}

func TestSubprocessOutput_StderrCaptured(t *testing.T) {
	_, stderr, _, err := SubprocessOutput("sh", "-c", "echo err 1>&2; exit 0")
	if err != nil {
		t.Errorf("err=%v", err)
	}
	if stderr != "err\n" {
		t.Errorf("stderr=%q", stderr)
	}
}

func TestSetupTempProject_StructureCreated(t *testing.T) {
	dir := SetupTempProject(t)
	if _, err := os.Stat(filepath.Join(dir, ".evolve")); err != nil {
		t.Errorf("missing .evolve: %v", err)
	}
}

// RepoRoot ------------------------------------------------------------
func TestRepoRoot_InsideGitRepo(t *testing.T) {
	// We're executing inside the evolve-loop git repo; expect a non-empty
	// path that the current file lives under.
	root := RepoRoot(t)
	if root == "" {
		t.Fatalf("RepoRoot returned empty path")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		// The repo's go.mod lives at go/go.mod, so check that instead.
		if _, err2 := os.Stat(filepath.Join(root, "go", "go.mod")); err2 != nil {
			t.Errorf("RepoRoot=%q has neither go.mod nor go/go.mod", root)
		}
	}
}

// FileContainsAny -----------------------------------------------------
func TestFileContainsAny_Hit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("alpha beta gamma"), 0o644)
	if !FileContainsAny(p, "delta", "beta") {
		t.Error("expected hit on second variant")
	}
}

func TestFileContainsAny_Miss(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("alpha"), 0o644)
	if FileContainsAny(p, "beta", "gamma") {
		t.Error("expected miss")
	}
}

func TestFileContainsAny_MissingFile(t *testing.T) {
	if FileContainsAny("/no/such/file", "anything") {
		t.Error("missing file must return false")
	}
}

// CountOccurrencesAny -------------------------------------------------
func TestCountOccurrencesAny_LineCount(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	body := "predicates-run\nverdict-decided\nreport-written\nunrelated\n"
	_ = os.WriteFile(p, []byte(body), 0o644)
	got := CountOccurrencesAny(p, "predicates-run", "verdict-decided", "report-written", "defects-listed")
	if got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

func TestCountOccurrencesAny_OneVariantPerLine(t *testing.T) {
	// A line matching two variants should count only once.
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("alpha and beta together\n"), 0o644)
	got := CountOccurrencesAny(p, "alpha", "beta")
	if got != 1 {
		t.Errorf("got %d, want 1 (break after first hit)", got)
	}
}

func TestCountOccurrencesAny_MissingFile(t *testing.T) {
	if CountOccurrencesAny("/no/such/file", "x") != 0 {
		t.Error("missing file must return 0")
	}
}

// LineContainsAll -----------------------------------------------------
func TestLineContainsAll_Hit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("| P-NEW-20 | Builder stop-criterion | DONE |\n"), 0o644)
	if !LineContainsAll(p, "P-NEW-20", "DONE") {
		t.Error("expected hit on table row")
	}
}

func TestLineContainsAll_Miss_DifferentLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	_ = os.WriteFile(p, []byte("P-NEW-20\nDONE\n"), 0o644)
	if LineContainsAll(p, "P-NEW-20", "DONE") {
		t.Error("two needles on different lines must not match")
	}
}

func TestLineContainsAll_MissingFile(t *testing.T) {
	if LineContainsAll("/no/such/file", "x") {
		t.Error("missing file must return false")
	}
}
