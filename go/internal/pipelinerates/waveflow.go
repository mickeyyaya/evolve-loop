package pipelinerates

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	waverec "github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	utilizationIndex = "PQ7"
	gateIndex        = "PQ9"
	signalsStream    = "signals.ndjson"
)

var (
	utilizationBase = censusBase{index: utilizationIndex, source: signalsStream, origin: "pipelinerates.SlotUtilization"}
	gateBase        = censusBase{index: gateIndex, source: runsSource, origin: "pipelinerates.GateShare"}
)

type WaveFlow struct {
	Wave      int
	Slots     lanetime.Slots
	Phases    lanetime.PhaseReport
	IdleHours map[string]float64
}

type FlowSet struct {
	Flows      []WaveFlow
	SamplesErr error
}

func (s FlowSet) WithSamples() ([]WaveFlow, error) { return s.Flows, s.SamplesErr }

func waveWindow(n int) string { return "wave:" + strconv.Itoa(n) }

func LoadWaveFlows(root string) (FlowSet, error) {
	evolve := paths.EvolveDirOf(root)
	records, err := waverec.NewStore(evolve).Records()
	if err == nil && len(records) == 0 {
		err = &fs.PathError{Op: "read", Path: filepath.Join(evolve, "waves"), Err: fs.ErrNotExist}
	}
	if err != nil {
		return FlowSet{}, err
	}
	first := max(len(records)-rollingWaves, 0)
	cycles, _, err := lanetime.ReadCycles(evolve, records[first].CycleFloor, 0)
	if err != nil {
		return FlowSet{}, err
	}
	samples, _, err := lanetime.ReadSamples(filepath.Join(evolve, signalsStream))
	set := FlowSet{}
	if errors.Is(err, fs.ErrNotExist) {
		set.SamplesErr = err
	} else if err != nil {
		return FlowSet{}, err
	}
	for i := first; i < len(records); i++ {
		set.Flows = append(set.Flows, flowOf(records, i, cycles, samples))
	}
	return set, nil
}

func flowOf(records []waverec.Record, i int, cycles []lanetime.Cycle, samples []lanetime.Sample) WaveFlow {
	rec, bounds := records[i], waverec.BoundsOf(records, i)
	var in []lanetime.Cycle
	for _, c := range cycles {
		if bounds.Holds(c.ID) {
			in = append(in, c)
		}
	}
	to := time.Time{}
	if rec.EndedAt != nil {
		to = *rec.EndedAt
	}
	own, from, to := lanetime.WaveSamples(samples, rec.PID, rec.StartedAt, to)
	return WaveFlow{Wave: rec.Number, Slots: lanetime.SlotUse(own, from, to), Phases: lanetime.Phases(in), IdleHours: lanetime.IdleUse(own, from, to)}
}

func SlotUtilization(flows []WaveFlow) []qualitypanel.Reading {
	var out []qualitypanel.Reading
	var baseline []float64
	for _, f := range flows {
		r := utilizationBase.reading(waveWindow(f.Wave), f.Slots.Utilization, boolInt(f.Slots.Window > 0))
		if chart, ok := qualitypanel.NewXmRChart(baseline); ok && r.N > 0 {
			r.Extra = map[string]string{qualitypanel.XmRRule1Field: chart.Rule1(f.Slots.Utilization)}
		}
		if r.N > 0 {
			baseline = append(baseline, f.Slots.Utilization)
		}
		out = append(out, r)
	}
	return out
}

func GateShare(flows []WaveFlow) []qualitypanel.Reading {
	var out []qualitypanel.Reading
	for _, f := range flows {
		var gate time.Duration
		for _, p := range f.Phases.Phases {
			gate += p.GateTotal
		}
		share := 0.0
		if f.Phases.Lane > 0 {
			share = float64(gate) / float64(f.Phases.Lane)
		}
		out = append(out, gateBase.reading(waveWindow(f.Wave), share, boolInt(f.Phases.Lane > 0)))
	}
	return out
}
