package codequality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write is a tiny helper that drops a file under dir, creating parents.
func write(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnformattedGoFiles_FlagsBadFormatting(t *testing.T) {
	dir := t.TempDir()
	// Mis-indented / bad-spacing source: gofmt would rewrite it.
	write(t, dir, "bad.go", "package p\nfunc F( ){\nx:=1\n_=x\n}\n")

	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly 1 flagged file, got %d: %v", len(got), got)
	}
}

func TestUnformattedGoFiles_FlagsNonSimplified(t *testing.T) {
	dir := t.TempDir()
	// gofmt-clean but NOT gofmt -s clean: `s[1:len(s)]` simplifies to `s[1:]`.
	// This pins that the gate uses -s (CI parity), not plain format.
	write(t, dir, "simp.go", "package p\n\nvar s = []int{1, 2}\n\nvar _ = s[1:len(s)]\n")

	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want the non-simplified file flagged (proves -s), got %d: %v", len(got), got)
	}
}

func TestUnformattedGoFiles_PassesClean(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "good.go", "package p\n\nfunc F() {\n\tx := 1\n\t_ = x\n}\n")

	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no flagged files for clean source, got %v", got)
	}
}

// An unparseable .go file must be surfaced as an OFFENDER (so the audit FAILs
// it — unparseable Go must never ship; CI vet/build fail too), NOT swallowed as
// an infra error that fails open. gofmt exits non-zero on a parse error but
// still lists any valid-but-dirty siblings on stdout.
func TestUnformattedGoFiles_ParseErrorIsOffenderNotInfraError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "broken.go", "package p\nfunc F( {\n") // missing close paren — unparseable

	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("a gofmt parse error must be reported as an offender, not an infra error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("want the parse error surfaced as an offender so audit FAILs; got none")
	}
}

func TestUnformattedGoFiles_AnUnreadableFileIsAnErrorNotAnOffender(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "good.go", "package p\n")
	locked := write(t, dir, "locked.go", "package p\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	if f, err := os.Open(locked); err == nil {
		_ = f.Close()
		t.Skip("mode 0 does not deny this user a read (root): no I/O error to provoke")
	}

	got, err := UnformattedGoFiles(dir)

	if err == nil || got != nil {
		t.Fatalf("got (%q, %v), want (nil, error): gofmt could not read a file, which says nothing about its formatting, so the gate must fail open instead of naming an offender", got, err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error %q does not carry gofmt's own I/O diagnosis", err)
	}
}

func TestUnformattedGoFiles_AnUnreadableFileDoesNotHideAnUnformattedSibling(t *testing.T) {
	dir := t.TempDir()
	dirty := write(t, dir, "dirty.go", "package p\nfunc  f() {}\n")
	locked := write(t, dir, "locked.go", "package p\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	if f, err := os.Open(locked); err == nil {
		_ = f.Close()
		t.Skip("mode 0 does not deny this user a read (root): no I/O error to provoke")
	}

	got, err := UnformattedGoFiles(dir)

	if err != nil || len(got) != 1 || got[0] != dirty {
		t.Fatalf("got (%q, %v), want ([%q], nil): an I/O error on one file must not discard the offender gofmt did report", got, err, dirty)
	}
}

func TestUnformattedGoFiles_AColumnlessParseDiagnosticIsAnOffender(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "gen.go", "package p\n//line gen.y:10\nfunc {\n")

	got, err := UnformattedGoFiles(dir)

	if err != nil || len(got) != 1 || !strings.HasPrefix(got[0], "gofmt parse error: ") {
		t.Fatalf("got (%q, %v), want one parse-error offender: a //line directive drops the column, and the file still does not parse", got, err)
	}
}

func TestUnformattedGoFiles_AGofmtThatDiesSilentlyIsAnErrorNotAnOffender(t *testing.T) {
	bin := t.TempDir()
	write(t, bin, "gofmt", "#!/bin/sh\nexit 2\n")
	if err := os.Chmod(filepath.Join(bin, "gofmt"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	dir := t.TempDir()
	write(t, dir, "good.go", "package p\n")

	got, err := UnformattedGoFiles(dir)

	if err == nil || got != nil {
		t.Fatalf("got (%q, %v), want (nil, error): a gofmt that exits non-zero with nothing on stderr was killed or broken, which is the host's failure, not a parse error in the tree", got, err)
	}
}

func TestUnformattedGoFiles_SkipsNonGo(t *testing.T) {
	dir := t.TempDir()
	// A deliberately "unformatted-looking" non-Go file must be ignored.
	write(t, dir, "notes.txt", "x:=1\nfunc( ){")
	write(t, dir, "good.go", "package p\n")

	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want non-Go files ignored, got %v", got)
	}
}

func TestFirstLine_NoNewline(t *testing.T) {
	s := "single line without newline"
	if got := firstLine(s); got != s {
		t.Errorf("firstLine(%q) = %q, want %q", s, got, s)
	}
}

func TestFirstLine_Empty(t *testing.T) {
	if got := firstLine(""); got != "" {
		t.Errorf("firstLine(%q) = %q, want empty", "", got)
	}
}

func TestFirstLine_WithNewline(t *testing.T) {
	if got := firstLine("first line\nsecond line"); got != "first line" {
		t.Errorf("firstLine with newline: got %q, want %q", got, "first line")
	}
}

func TestUnformattedGoFiles_GofmtMissing(t *testing.T) {
	t.Setenv("PATH", "")
	dir := t.TempDir()
	write(t, dir, "any.go", "package p\n")

	got, err := UnformattedGoFiles(dir)
	if err == nil {
		t.Fatal("want error when gofmt binary is missing; got nil")
	}
	if got != nil {
		t.Errorf("want nil file list when gofmt missing; got %v", got)
	}
}
