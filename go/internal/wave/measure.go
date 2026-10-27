package wave

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

const (
	WidthFromSignal = "signal"
	WidthFromPolicy = "policy"
)

func ReadLoopSignals(root string) ([]signalcenter.Event, []string) {
	path := filepath.Join(paths.EvolveDirOf(root), signalcenter.StreamFileName)
	chunk, err := signalcenter.ReadStream(path, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{err.Error()}
	}
	if chunk.Skipped > 0 {
		return chunk.Events, []string{fmt.Sprintf("skipped %d malformed signal line(s) in %s", chunk.Skipped, path)}
	}
	return chunk.Events, nil
}

func MeasureFacts(rec Record, next *Record, cycles []Cycle, loop []signalcenter.Event, policyWidth int) (wavemeasure.Facts, string) {
	f := wavemeasure.Facts{Start: rec.StartedAt, End: runEnd(rec, next), WidthSized: policyWidth}
	run := loopRunOf(rec, next)
	pool := poolLanes(run, loop)
	for _, c := range cycles {
		lane := wavemeasure.Lane{Landed: c.Shipped, Conflict: c.Conflict}
		if len(pool) == 0 {
			lane.Start, lane.End = c.StartedAt, c.EndedAt
		}
		f.Lanes = append(f.Lanes, lane)
		f.Cycles = append(f.Cycles, wavemeasure.CycleSpend{Shipped: c.Shipped, Spend: c.Spend, HandoffWould: c.HandoffWould, HandedOff: c.HandoffStop != ""})
	}
	f.Lanes = append(f.Lanes, pool...)
	f.OptimisticAdmissions, f.OptimisticFailures = countOptimistic(run, loop)
	return applyLoopSignals(f, loop)
}

func inRun(f wavemeasure.Facts, at time.Time) bool {
	return !at.Before(f.Start) && (f.End.IsZero() || !at.After(f.End))
}

func applyLoopSignals(f wavemeasure.Facts, events []signalcenter.Event) (wavemeasure.Facts, string) {
	source := WidthFromPolicy
	for _, e := range events {
		if !inRun(f, eventTime(e)) {
			continue
		}
		switch {
		case e.Code == wavemeasure.CodeAdmitWait:
			f.AdmissionWaits++
		case e.Kind == signalcenter.KindLoopWave:
			if w, err := strconv.Atoi(e.Fields[wavemeasure.FieldWidthSized]); err == nil {
				f.WidthSized, source = w, WidthFromSignal
			}
		}
	}
	return f, source
}

func countOptimistic(run loopRun, events []signalcenter.Event) (admissions, failures int) {
	admitted, failed := map[string]bool{}, map[string]bool{}
	for _, e := range events {
		switch todo := e.Fields[todoField]; {
		case e.Code == orchestrator.CodeAdmissionOptimistic && run.holds(e):
			admitted[todo] = true
		case e.Code == orchestrator.CodeOptimisticAdmissionFailed && run.spans(eventTime(e)):
			failed[todo] = true
		}
	}
	delete(admitted, "")
	for todo := range failed {
		if admitted[todo] {
			failures++
		}
	}
	return len(admitted), failures
}

func runEnd(rec Record, next *Record) time.Time {
	switch {
	case rec.EndedAt != nil:
		return *rec.EndedAt
	case next != nil:
		return next.StartedAt
	default:
		return time.Time{}
	}
}

func eventTime(e signalcenter.Event) time.Time {
	at, _ := time.Parse(time.RFC3339Nano, e.TS)
	return at
}

func phaseSpan(start, end time.Time, phases []dossier.PhaseRecord) (time.Time, time.Time) {
	for _, p := range phases {
		if at, err := time.Parse(time.RFC3339Nano, p.StartedAt); err == nil {
			start = earliest(start, at)
		}
		if at, err := time.Parse(time.RFC3339Nano, p.EndedAt); err == nil && at.After(end) {
			end = at
		}
	}
	return start, end
}

func earliest(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}
