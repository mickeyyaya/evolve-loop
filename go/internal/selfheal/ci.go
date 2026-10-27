package selfheal

import (
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	HangAfter  = 90 * time.Minute
	mainBranch = "main"
	completed  = "completed"
)

type CIRun struct {
	ID         int64
	Branch     string
	Status     string
	Conclusion string
	Attempt    int
	Created    time.Time
	Updated    time.Time
}

func (r CIRun) evidence(at time.Time) Evidence {
	return Evidence{ID: "ci:" + strconv.FormatInt(r.ID, 10), At: at}
}

func (r CIRun) hung(now time.Time) bool {
	if r.Branch != mainBranch {
		return false
	}
	if r.Status != completed {
		return now.Sub(r.Created) >= HangAfter
	}
	return (r.Conclusion == "cancelled" || r.Conclusion == "timed_out") && r.Updated.Sub(r.Created) >= HangAfter
}

func latestMain(runs []CIRun) (CIRun, bool) {
	var latest CIRun
	found := false
	for _, r := range runs {
		if r.Branch == mainBranch && r.Status == completed && (!found || r.Created.After(latest.Created)) {
			latest, found = r, true
		}
	}
	return latest, found
}

func MainGreen(runs []CIRun) bool {
	latest, found := latestMain(runs)
	return found && latest.Conclusion == "success"
}

func FromCIRuns(runs []CIRun, now time.Time) []Observation {
	set := newObservations()
	for _, r := range runs {
		if r.hung(now) {
			set.add(Observation{Class: ClassCIHang, Subject: mainBranch, Severity: signalcenter.SeverityWarn, Summary: "a CI run on main ran past the hang limit"}, r.evidence(r.Created))
		}
		if r.Attempt > 1 && r.Status == completed && r.Conclusion == "success" {
			set.add(Observation{Class: ClassFlakyTest, Subject: "rerun-green", Severity: signalcenter.SeverityWarn, Summary: "a CI rerun of the same code turned green"}, r.evidence(r.Updated))
		}
	}
	if latest, found := latestMain(runs); found && latest.Conclusion == "failure" {
		set.add(Observation{Class: ClassMainRed, Subject: mainBranch, Severity: signalcenter.SeverityIncident, Summary: "the latest completed CI run on main failed"}, latest.evidence(latest.Updated))
	}
	return set.list()
}
