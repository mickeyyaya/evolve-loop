package textutil

import (
	"strings"
	"testing"
)

func TestTruncateInline(t *testing.T) {
	if got := TruncateInline("short", 100); got != "short" {
		t.Fatalf("TruncateInline short = %q, want unchanged", got)
	}

	got := TruncateInline("0123456789", 4)
	if got != "0123… (6 bytes elided)" {
		t.Fatalf("TruncateInline truncated = %q", got)
	}
}

func TestTruncateMiddle(t *testing.T) {
	if got := TruncateMiddle("short", 100, 100); got != "short" {
		t.Fatalf("TruncateMiddle short = %q, want unchanged", got)
	}

	in := strings.Repeat("a", 50) + strings.Repeat("b", 50)
	got := TruncateMiddle(in, 8, 6)
	want := "aaaaaaaa… (86 bytes elided) …bbbbbb"
	if got != want {
		t.Fatalf("TruncateMiddle truncated = %q, want %q", got, want)
	}
}
