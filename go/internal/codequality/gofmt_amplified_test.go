package codequality

import (
	"testing"
)

func TestFirstLine_OnlyNewline(t *testing.T) {
	if got := firstLine("\n"); got != "" {
		t.Errorf("firstLine(%q) = %q, want empty string", "\n", got)
	}
}

func TestFirstLine_MultipleLines(t *testing.T) {
	s := "first\nsecond\nthird"
	if got := firstLine(s); got != "first" {
		t.Errorf("firstLine(%q) = %q, want %q", s, got, "first")
	}
}

func TestUnformattedGoFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error for empty dir: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 offenders in empty dir, got %d: %v", len(got), got)
	}
}

func TestUnformattedGoFiles_MultipleDirty(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "alpha.go", "package p\nfunc A( ){\nx:=1\n_=x\n}\n")
	write(t, dir, "beta.go", "package p\nfunc B( ){\ny:=2\n_=y\n}\n")

	got, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("want >= 2 offenders (both dirty files flagged), got %d: %v", len(got), got)
	}
}
