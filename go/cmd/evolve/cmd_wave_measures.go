package main

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

const waveMeasuredRuns = 3

type waveRunMeasures struct {
	Wave        int                  `json:"wave"`
	RunID       string               `json:"run_id"`
	WidthSource string               `json:"width_source"`
	Measures    wavemeasure.Measures `json:"measures"`
}

func recordWaveSized(signals *signalcenter.Center, n, desired, sized int) {
	fields := map[string]string{wavemeasure.FieldWidthSized: strconv.Itoa(sized), "desired": strconv.Itoa(desired)}
	loopwave.EmitWave(signals, n, "loopBatchCoordinator.dispatchFleetIteration", "",
		fmt.Sprintf("wave %d sized to %d of %d lane(s)", n, sized, desired), fields)
}

func measureWaveRuns(plane wavePlaneDirs, records []wave.Record, current []wave.Cycle, loop []signalcenter.Event, live bool, now time.Time) ([]waveRunMeasures, []string) {
	var warnings []string
	supply, err := orchestrator.ReadSupply(waveSupplyPath(plane.evolveDir))
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	policyWidth := loadFleetConfig(plane.evolveDir, io.Discard, nilSignals()).Count
	runs := []waveRunMeasures{}
	for i := max(0, len(records)-waveMeasuredRuns); i < len(records); i++ {
		rec, next, cycles := records[i], (*wave.Record)(nil), current
		if i+1 < len(records) {
			next = &records[i+1]
			var warn []string
			cycles, warn = wave.ReadCycles(plane.root, rec.CycleFloor, next.CycleFloor)
			warnings = append(warnings, warn...)
		} else if ended, ok := waveLogEnded(rec); ok && !live && rec.EndedAt == nil {
			rec.EndedAt = &ended
		}
		facts, source := wave.MeasureFacts(rec, next, cycles, loop, policyWidth)
		if census, ok := orchestrator.SupplyIn(supply, facts.Start, facts.End); ok {
			counts := census.Measure()
			facts.Supply = &counts
		}
		runs = append(runs, waveRunMeasures{Wave: rec.Number, RunID: rec.RunID, WidthSource: source, Measures: wavemeasure.Compute(facts, now)})
	}
	return runs, warnings
}

func writeWaveMeasuresText(w io.Writer, m wavemeasure.Measures) {
	fmt.Fprintf(w, "measures: width %d/%d · utilization %.2f · conflicts %d/%d", m.WidthRealized, m.WidthSized, m.Utilization, m.Conflicts, m.Landed)
	if s := m.Supply; s != nil {
		fmt.Fprintf(w, " · supply %d admissible, %d waiting, %d console", s.Admissible, s.WaitingCell+s.WaitingDeps, s.ConsoleRoute+s.ConsoleKind+s.ConsoleProtected)
	}
	if m.OptimisticAdmissions > 0 {
		fmt.Fprintf(w, " · optimistic %d/%d failed (%.2f)", m.OptimisticFailures, m.OptimisticAdmissions, m.OptimisticFailureRate)
	}
	fmt.Fprintln(w)
}
