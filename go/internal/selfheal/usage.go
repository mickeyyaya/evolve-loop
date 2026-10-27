package selfheal

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const UsageStaleAfter = 24 * time.Hour

type UsageReading struct {
	Family     string
	Declared   bool
	ObservedAt time.Time
	Windows    int
	Error      string
}

func (r UsageReading) blind(now time.Time) bool {
	return r.Declared && (r.Windows == 0 || r.Error != "" || now.Sub(r.ObservedAt) > UsageStaleAfter)
}

func FromUsage(rs []UsageReading, now time.Time) []Observation {
	set := newObservations()
	for _, r := range rs {
		if !r.blind(now) {
			continue
		}
		set.add(Observation{Class: ClassUsageBlind, Subject: r.Family, Severity: signalcenter.SeverityWarn, Summary: "the CLI " + r.Family + " declares usage windows, and the last usage reading has none, an error or a stale time"},
			Evidence{ID: "usage:" + r.Family + "@" + r.ObservedAt.UTC().Format(time.RFC3339), At: r.ObservedAt})
	}
	return set.list()
}
