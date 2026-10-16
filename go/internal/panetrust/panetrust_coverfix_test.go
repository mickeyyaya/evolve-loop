package panetrust_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
)

func TestDigest_MultibyteLineWithinRuneCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	got := panetrust.Digest("ééé", 5, 4)
	if got != "ééé" {
		t.Fatalf("Digest = %q, want the three-rune line intact under a four-rune cap", got)
	}
}

func TestDigest_MultibyteLineOverRuneCapIsTruncatedOnARuneBoundary(t *testing.T) {
	t.Parallel()
	got := panetrust.Digest("ééééé", 5, 3)
	if got != "ééé" {
		t.Fatalf("Digest = %q, want exactly three whole runes", got)
	}
}
