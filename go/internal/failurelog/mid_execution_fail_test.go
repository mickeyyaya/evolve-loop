package failurelog

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestClassificationMidExecutionFail_IsOutsideTheTaxonomy(t *testing.T) {
	if got := NormalizeLegacy(cyclestate.ClassificationMidExecutionFail); got != UnknownClassification {
		t.Fatalf("the default class normalizes to UnknownClassification today, got %q", got)
	}
	now := time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC)
	if got, want := ComputeExpiresAt(NormalizeLegacy(cyclestate.ClassificationMidExecutionFail), now), now.Add(LegacyEffectiveTTL).Format(time.RFC3339); got != want {
		t.Fatalf("the 1-day TTL trap (F11): ExpiresAt = %s, want %s", got, want)
	}
}
