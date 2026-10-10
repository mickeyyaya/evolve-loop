package quotareset_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

func TestCompute_AnUnknownResetIsNowAndSaysUnknownNeverAFarFutureDefault(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts quotareset.Options
	}{
		{"no evidence and no configured hours", quotareset.Options{Now: fixedClock()}},
		{"a negative configured hours is no configuration", quotareset.Options{DefaultHours: -5, Now: fixedClock()}},
		{"a bench that already expired is no evidence", quotareset.Options{BenchedUntil: refNow().Add(-time.Minute), Now: fixedClock()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := quotareset.Compute("", tc.opts)

			if err != nil {
				t.Fatalf("Compute err=%v", err)
			}
			if r.Source != "unknown" || !r.WakeAt.Equal(refNow()) {
				t.Errorf("Source=%q WakeAt=%v, want unknown at now %v: an unknown reset is never a fabricated far-future time", r.Source, r.WakeAt, refNow())
			}
		})
	}
}

func TestCompute_ABenchedFamilysResetIsTheEvidenceBeforeAnyConfiguredDefault(t *testing.T) {
	reset := refNow().Add(89*time.Hour + 29*time.Minute)

	r, _ := quotareset.Compute(hintWorkspace(t, "no clock here"), quotareset.Options{BenchedUntil: reset, DefaultHours: 3, Now: fixedClock()})

	if r.Source != "bench" || !r.WakeAt.Equal(reset) {
		t.Errorf("Source=%q WakeAt=%v, want bench at %v: the benched wall's own reset is evidence", r.Source, r.WakeAt, reset)
	}
}

func TestCompute_AWorkspaceWithNoHintFileIsNoEvidence(t *testing.T) {
	r, _ := quotareset.Compute(t.TempDir(), quotareset.Options{Now: fixedClock()})

	if r.Source != "unknown" || !r.WakeAt.Equal(refNow()) {
		t.Errorf("Source=%q WakeAt=%v, want unknown at now: a missing hint file is not a reset", r.Source, r.WakeAt)
	}
}

func TestCompute_TheUsageQueryResetComesAfterTheBenchAndBeforeAConfiguredDefault(t *testing.T) {
	reset := refNow().Add(4 * time.Hour)
	asked := 0
	query := func() (time.Time, bool) { asked++; return reset, true }

	r, _ := quotareset.Compute("", quotareset.Options{UsageReset: query, DefaultHours: 3, Now: fixedClock()})
	benched, _ := quotareset.Compute("", quotareset.Options{UsageReset: query, BenchedUntil: refNow().Add(time.Hour), Now: fixedClock()})

	if r.Source != "usage" || !r.WakeAt.Equal(reset) {
		t.Errorf("Source=%q WakeAt=%v, want usage at %v: the queried screen is evidence, a configured default is a guess", r.Source, r.WakeAt, reset)
	}
	if benched.Source != "bench" || asked != 1 {
		t.Errorf("Source=%q after %d queries, want bench with no second query: the query runs only when nothing better exists", benched.Source, asked)
	}
}

func TestCompute_AUsageQueryWithNoResetIsNoEvidence(t *testing.T) {
	r, _ := quotareset.Compute("", quotareset.Options{UsageReset: func() (time.Time, bool) { return time.Time{}, false }, Now: fixedClock()})

	if r.Source != "unknown" {
		t.Errorf("Source=%q, want unknown: a usage screen without a reset time proves nothing", r.Source)
	}
}

func TestCompute_AUsageResetAlreadyPastIsNoEvidence(t *testing.T) {
	past := func() (time.Time, bool) { return refNow().Add(-time.Minute), true }

	r, _ := quotareset.Compute("", quotareset.Options{UsageReset: past, Now: fixedClock()})

	if r.Source != "unknown" {
		t.Errorf("Source=%q WakeAt=%v, want unknown: a reset already past says nothing about when the wall lifts", r.Source, r.WakeAt)
	}
}
