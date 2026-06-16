package codequality

import (
	"testing"
)

// TestFirstLine_OnlyNewline covers the boundary case where the separator is the
// very first character — firstLine("\n") should return "" (empty first line),
// not panic or return the whole string. This sits precisely between the two
// branches now covered by TestFirstLine_NoNewline and TestFirstLine_WithNewline.
func TestFirstLine_OnlyNewline(t *testing.T) {
	if got := firstLine("\n"); got != "" {
		t.Errorf("firstLine(%q) = %q, want empty string", "\n", got)
	}
}

// TestFirstLine_MultipleLines ensures only the first line is returned when the
// input contains three or more newline-separated segments.
func TestFirstLine_MultipleLines(t *testing.T) {
	s := "first\nsecond\nthird"
	if got := firstLine(s); got != "first" {
		t.Errorf("firstLine(%q) = %q, want %q", s, got, "first")
	}
}

// TestUnformattedGoFiles_EmptyDir: a completely empty directory must return
// nil offenders and no error — the gate must not fail when there are simply no
// Go files to check.
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

// TestUnformattedGoFiles_MultipleDirty: two independently unformatted Go files
// must both be reported. A single-result cap (e.g. early-return after first hit)
// would silently suppress the second offender.
func TestUnformattedGoFiles_MultipleDirty(t *testing.T) {
	dir := t.TempDir()
	// Two distinct gofmt-dirty files.
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
