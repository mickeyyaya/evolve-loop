package pipelinerates

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func tryOf(cycle int, verdict, task, sha, ended string) *dossier.Dossier {
	tasks := []string{task}
	return &dossier.Dossier{Cycle: cycle, FinalVerdict: verdict, Tasks: &tasks, CommitSHA: sha, EndedAt: ended}
}

func recoveries(ds ...*dossier.Dossier) RecoveryResult {
	return MeasureRecovery(RecoveryInput{Dossiers: ds, Window: "wave:1", Source: "dossiers", Cycle: 9})
}

func TestRecovery_AFailedItemThatLaterShipsIsOneRecoveryCountedFromTheFirstFailure(t *testing.T) {
	res := recoveries(
		tryOf(3, dossier.VerdictPass, "a", "sha", "2026-11-03T10:00:00Z"),
		tryOf(1, dossier.VerdictFail, "a", "", "2026-11-01T00:00:00Z"),
		tryOf(2, dossier.VerdictFail, "a", "", "2026-11-02T00:00:00Z"),
	)
	if res.Recoveries != 1 || !slices.Equal(res.Hours, []float64{58}) || res.MedianHours != 58 {
		t.Fatalf("result = %+v, want 1 recovery of 58 hours from the first failure", res)
	}
	r := res.OQ4
	if r.Index != "OQ4" || r.N != 1 || r.Interval.Value != 1 || r.Method != qualitypanel.MethodWilson95 || r.Window != "wave:1" || r.Cycle != 9 || r.Extra["median_hours"] != "58.000" {
		t.Errorf("reading = %+v", r)
	}
}

func TestRecovery_AnItemThatFailsAgainAfterShippingOpensANewEpisode(t *testing.T) {
	res := recoveries(
		tryOf(1, dossier.VerdictFail, "a", "", "2026-11-01T00:00:00Z"),
		tryOf(2, dossier.VerdictPass, "a", "s1", "2026-11-01T02:00:00Z"),
		tryOf(3, dossier.VerdictFail, "a", "", "2026-11-02T00:00:00Z"),
	)
	if res.Recoveries != 1 || res.OQ4.N != 2 || res.OQ4.Interval.Value != 0.5 {
		t.Errorf("result = %+v, want 1 recovery in 2 episodes", res)
	}
}

func TestRecovery_ADossierWithNoTasksOrANilEntryOrARepeatedCycleIsIgnored(t *testing.T) {
	noTasks := &dossier.Dossier{Cycle: 5, FinalVerdict: dossier.VerdictFail}
	res := recoveries(nil, noTasks, tryOf(1, dossier.VerdictFail, "a", "", ""), tryOf(1, dossier.VerdictPass, "a", "s", ""))
	if res.Recoveries != 0 || res.OQ4.N != 1 {
		t.Errorf("result = %+v, want the repeated cycle 1 read once as a failure", res)
	}
}

func TestRecovery_ADossierWithoutATimeIsARecoveryWithNoHours(t *testing.T) {
	for _, ended := range [][2]string{{"", ""}, {"2026-11-01T00:00:00Z", ""}, {"", "2026-11-01T00:00:00Z"}} {
		res := recoveries(tryOf(1, dossier.VerdictFail, "a", "", ended[0]), tryOf(2, dossier.VerdictPass, "a", "s", ended[1]))
		if res.Recoveries != 1 || len(res.Hours) != 0 || res.MedianHours != 0 {
			t.Errorf("ended %v: result = %+v, want 1 recovery and no hours", ended, res)
		}
	}
}

func TestRecovery_AnItemThatNeverFailedIsNoData(t *testing.T) {
	res := recoveries(tryOf(1, dossier.VerdictPass, "a", "s", "2026-11-01T00:00:00Z"))
	ix, _ := qualitypanel.Lookup("OQ4")
	if res.Recoveries != 0 || res.OQ4.N != 0 || qualitypanel.Judge(ix, res.OQ4) != qualitypanel.StatusNoData {
		t.Errorf("result = %+v, want no recovery and no data", res)
	}
}

func TestRecovery_EventsCarryTheReadingAndNeverABreachAtShadow(t *testing.T) {
	res := recoveries(tryOf(1, dossier.VerdictFail, "a", "", ""))
	events := res.Events(qualitypanel.StageShadow)
	if len(events) == 0 || events[0].Fields["index"] != "OQ4" || events[0].Fields["stage"] != "shadow" || events[0].Fields["median_hours"] != "0.000" {
		t.Errorf("events = %+v, want the OQ4 reading at stage shadow", events)
	}
}
