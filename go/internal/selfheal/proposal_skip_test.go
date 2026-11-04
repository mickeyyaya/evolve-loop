package selfheal

import (
	"testing"
	"time"
)

func TestSkipAfterGuardRefusal_RecordsEachOtherIncidentAsSkippedWithTheReason(t *testing.T) {
	t.Parallel()
	incidents := liveRecords(t)[:3]

	got := SkipAfterGuardRefusal(incidents, "shadow", t0)

	if len(got) != 3 {
		t.Fatalf("one skipped record for each incident, got %d", len(got))
	}
	for i, p := range got {
		if p.IncidentID != incidents[i].ID || p.State != ProposalSkipped || p.Cause != "guard_refused" || p.RunLabel != "" || p.Error != guardRefusedSkip {
			t.Errorf("record %d: %+v", i, p)
		}
	}
	if len(SkipAfterGuardRefusal(nil, "shadow", t0)) != 0 {
		t.Error("no incident after the refusal records nothing")
	}
}

func TestPending_ASkippedIncidentIsTriedAgainAfterItsBackoff(t *testing.T) {
	t.Parallel()
	inc := liveRecords(t)[0]
	done := map[string]Proposal{inc.ID: SkipAfterGuardRefusal([]Incident{inc}, "shadow", t0)[0]}

	if got := Pending([]Incident{inc}, done, PassLimits{Max: 8, RetryAt: t0}); len(got) != 1 {
		t.Errorf("a skipped incident is tried again after the backoff, got %v", ids(got))
	}
	if got := Pending([]Incident{inc}, done, PassLimits{Max: 8, RetryAt: t0.Add(-time.Hour)}); len(got) != 0 {
		t.Errorf("a skipped incident waits for the backoff, got %v", ids(got))
	}
}
