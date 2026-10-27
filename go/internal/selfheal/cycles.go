package selfheal

import (
	"slices"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	RepairLoopAfter = 2
	ChurnAfter      = 3
	systemLevel     = "system"
)

type Cycle struct {
	ID                int
	Item              string
	Started           time.Time
	RepairAttempts    int
	Shipped           bool
	FailureLevel      string
	ProtectedFindings []string
}

func (c Cycle) evidence() Evidence { return Evidence{ID: "cycle:" + strconv.Itoa(c.ID), At: c.Started} }

func FromCycles(cs []Cycle) []Observation {
	sorted := slices.Clone(cs)
	slices.SortFunc(sorted, func(a, b Cycle) int { return a.ID - b.ID })
	set := newObservations()
	unshipped := map[string][]Cycle{}
	var items []string
	for _, c := range sorted {
		if c.FailureLevel == systemLevel {
			set.add(Observation{Class: ClassSystem, Subject: "failure-decision", Severity: signalcenter.SeverityIncident, Summary: "a cycle failure decision has the level system"}, c.evidence())
		}
		if c.Item == "" {
			continue
		}
		if c.RepairAttempts >= RepairLoopAfter && !c.Shipped {
			set.add(Observation{Class: ClassRepairLoop, Subject: c.Item, Severity: signalcenter.SeverityWarn, Protected: c.ProtectedFindings, Summary: "repair rounds did not converge to a ship for item " + c.Item}, c.evidence())
		}
		if _, ok := unshipped[c.Item]; !ok {
			items = append(items, c.Item)
		}
		if c.Shipped {
			unshipped[c.Item] = []Cycle{}
			continue
		}
		unshipped[c.Item] = append(unshipped[c.Item], c)
	}
	for _, item := range items {
		claims := unshipped[item]
		if len(claims) < ChurnAfter {
			continue
		}
		for _, c := range claims {
			set.add(Observation{Class: ClassClaimChurn, Subject: item, Severity: signalcenter.SeverityWarn, Summary: "cycles claimed item " + item + " and none shipped it"}, c.evidence())
		}
	}
	return set.list()
}
