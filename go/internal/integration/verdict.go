package integration

import (
	"fmt"
	"maps"
	"slices"
)

const (
	PoolClean      = "clean"
	PoolNotClean   = "not clean"
	PoolNoEvidence = "no evidence"
)

type PoolEvidence struct {
	Found        bool
	Record       WaveRecord
	Shipped      []int
	ReadFailures []string
}

func composes(rec WaveRecord) int {
	n := 0
	for _, m := range rec.Members {
		switch m.Status {
		case MemberComposed, MemberConflicted, MemberConflictedBaseAhead:
			n++
		}
	}
	return n
}

func hasWaveEndDecision(rec WaveRecord) bool {
	return slices.ContainsFunc(rec.Decisions, func(d FlushRecord) bool { return d.Trigger == TriggerWaveEnd })
}

func pendingAtEnd(rec WaveRecord) []string {
	var out []string
	for _, c := range slices.Sorted(maps.Keys(rec.Members)) {
		if rec.Members[c].Status == MemberPending {
			out = append(out, fmt.Sprintf("cycle %d is still pending at wave end", c))
		}
	}
	return out
}

func PoolVerdict(ev PoolEvidence) (string, []string) {
	var failed []string
	for _, f := range ev.ReadFailures {
		failed = append(failed, "unreadable: "+f)
	}
	if !ev.Found {
		return PoolNotClean, append(failed, "no wave record")
	}
	rec := ev.Record
	if !rec.Ended {
		failed = append(failed, "the wave record has no Ended")
	}
	if !hasWaveEndDecision(rec) {
		failed = append(failed, "the wave record has no wave-end decision")
	}
	if rec.LastCycle < 1 {
		failed = append(failed, "the wave record has no last cycle, so the shipped cycles cannot be counted")
	}
	for _, c := range ev.Shipped {
		if _, ok := rec.Members[c]; !ok {
			failed = append(failed, fmt.Sprintf("missing record: cycle %d shipped and has no member", c))
		}
	}
	if rec.Ended {
		failed = append(failed, pendingAtEnd(rec)...)
	}
	for _, f := range rec.Failures {
		failed = append(failed, fmt.Sprintf("driver failure at %s: %s", f.Step, f.Error))
	}
	switch {
	case len(failed) > 0:
		return PoolNotClean, failed
	case composes(rec) == 0:
		return PoolNoEvidence, nil
	}
	return PoolClean, nil
}
