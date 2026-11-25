package incidentkb_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

const cutoffLayout = "2006-01-02"

func dayShiftedFromCutoff(t *testing.T, days int) string {
	t.Helper()
	cutoff, err := time.Parse(cutoffLayout, incidentkb.LegacyCutoff)
	if err != nil {
		t.Fatalf("LegacyCutoff %q is not a yyyy-mm-dd date: %v", incidentkb.LegacyCutoff, err)
	}
	return cutoff.AddDate(0, 0, days).Format(cutoffLayout)
}

func TestLegacyCutoff_IsADate(t *testing.T) {
	t.Parallel()
	if _, err := time.Parse(cutoffLayout, incidentkb.LegacyCutoff); err != nil {
		t.Fatalf("LegacyCutoff %q: %v", incidentkb.LegacyCutoff, err)
	}
}

func TestIsLegacy_TheDayBeforeTheCutoffIsLegacyAndTheCutoffDayIsNot(t *testing.T) {
	t.Parallel()
	bare := []byte("# no frontmatter\n")
	for _, c := range []struct {
		shift int
		want  bool
	}{{-1, true}, {0, false}, {1, false}} {
		name := dayShiftedFromCutoff(t, c.shift) + "-sample.md"
		if got := incidentkb.IsLegacy(name, bare); got != c.want {
			t.Errorf("IsLegacy(%q) = %v, want %v", name, got, c.want)
		}
	}
}
