package practicewatch

import (
	"strings"
	"testing"
)

func TestAddedLines_ReturnsOnlyLinesAbsentFromTheBefore(t *testing.T) {
	got := addedLines("a\nb\nc", "a\nnew\nb\nc\nlast", 1024)

	if got != "new\nlast" {
		t.Errorf("addedLines = %q, want %q", got, "new\nlast")
	}
}

func TestAddedLines_TreatsARepeatedLineAsAddedWhenItsCountGrew(t *testing.T) {
	if got := addedLines("x\nx", "x\nx\nx", 1024); got != "x" {
		t.Errorf("addedLines = %q, want %q", got, "x")
	}
}

func TestAddedLines_CapsTheBodyAndSaysSo(t *testing.T) {
	after := strings.Repeat("0123456789\n", 1000)

	got := addedLines("", after, 100)

	if len(got) > 100+len(truncationNotice) || !strings.HasSuffix(got, truncationNotice) {
		t.Errorf("addedLines len = %d, want a body capped at 100 bytes plus the notice; got %q", len(got), got[len(got)-40:])
	}
}

func TestAddedLines_NamesAChangeOutsideTheExcerpt(t *testing.T) {
	if got := addedLines("same", "same", 100); got != noAddedLines {
		t.Errorf("addedLines = %q, want %q", got, noAddedLines)
	}
}

func TestCapText_KeepsValidUTF8AtTheCut(t *testing.T) {
	got := capText("aé", 2)

	if got != "a" {
		t.Errorf("capText = %q, want a cut before the split rune", got)
	}
}
