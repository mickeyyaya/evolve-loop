package codequality

import (
	"os"
	"path/filepath"
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
