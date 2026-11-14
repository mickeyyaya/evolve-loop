package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type waveLatencyCheck struct {
	Waves    []lanetime.Verdict `json:"waves"`
	Exceeded bool               `json:"exceeded"`
	Item     string             `json:"item,omitempty"`
	Warnings []string           `json:"warnings,omitempty"`
}

type latencyItem struct {
	ID            string   `json:"id"`
	Weight        float64  `json:"weight"`
	Kind          string   `json:"kind"`
	Priority      string   `json:"priority"`
	PriorityClass string   `json:"priority_class"`
	Origin        string   `json:"origin"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Fix           string   `json:"fix"`
	Acceptance    []string `json:"acceptance"`
	Source        string   `json:"source"`
}

func tolerateMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func readLatencyInputs(plane wavePlaneDirs) (cycles []lanetime.Cycle, stream lanetime.Stream, warnings []string, err error) {
	cycles, warnings, err = lanetime.ReadCycles(plane.evolveDir, 0, 0)
	if err = tolerateMissing(err); err != nil {
		return
	}
	var bad int
	stream, bad, err = lanetime.ReadStream(filepath.Join(plane.evolveDir, signalcenter.StreamFileName))
	if err = tolerateMissing(err); err != nil {
		return
	}
	if bad > 0 {
		warnings = append(warnings, fmt.Sprintf("%d malformed orchestration line(s) were skipped", bad))
	}
	return
}

func checkWaves(records []wave.Record, cycles []lanetime.Cycle, stream lanetime.Stream, bounds lanetime.Bounds) []lanetime.Verdict {
	verdicts := make([]lanetime.Verdict, 0, len(records))
	for i, rec := range records {
		span := wave.BoundsOf(records, i)
		in := lanetime.WaveInput{Number: rec.Number, PID: rec.PID, Start: rec.StartedAt, Samples: stream.Samples, Marks: stream.Marks}
		if rec.EndedAt != nil {
			in.End = *rec.EndedAt
		}
		for _, c := range cycles {
			if span.Holds(c.ID) {
				in.Cycles = append(in.Cycles, c)
			}
		}
		verdicts = append(verdicts, lanetime.Check(in, bounds))
	}
	return verdicts
}

func breachWaves(run []lanetime.Verdict) []string {
	waves := make([]string, len(run))
	for i, v := range run {
		waves[i] = strconv.Itoa(v.Wave)
	}
	return waves
}

func latencyBreachItem(run []lanetime.Verdict) latencyItem {
	waves := breachWaves(run)
	list := strings.Join(waves, ", ")
	return latencyItem{
		ID:            "latency-wave-breach-from-wave-" + waves[0],
		Weight:        0.7,
		Kind:          "fix",
		Priority:      "high",
		PriorityClass: inboxbatch.ClassPerformance,
		Origin:        inboxbatch.OriginCycle,
		Title:         fmt.Sprintf("Wave latency budget exceeded in waves %s", list),
		Summary:       fmt.Sprintf("evolve wave latency --check found waves %s in breach in a row: %s. A wave with a quota pause, a loop stop_reason or an operator brake does not count. The bounds are wave_latency.min_live_share and wave_latency.max_gate_share in policy.json.", list, strings.Join(run[len(run)-1].Reasons, "; ")),
		Fix:           fmt.Sprintf("Run evolve wave latency --wave %s, find where the live lanes or the host gate time went, and fix the cause at its source.", waves[len(waves)-1]),
		Acceptance: []string{
			"evolve wave latency --check reports the newest wave ok or left out",
			"The cause has a regression test that fails before the fix and passes after it",
		},
		Source: "wave-latency-check",
	}
}

func fileLatencyBreach(env waveEnv, plane wavePlaneDirs, run []lanetime.Verdict, stderr io.Writer) (string, error) {
	item := latencyBreachItem(run)
	raw, _ := json.Marshal(item)
	res, err := inboxfile.Mint(filepath.Join(plane.evolveDir, "inbox"), raw, env.now)
	if err != nil || res.Skipped {
		return item.ID, err
	}
	signals, closeSignals := newRoleSignalCenter("latency", plane.root, plane.evolveDir, stderr)
	defer closeSignals()
	signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleLatency, Origin: "fileLatencyBreach", Kind: signalcenter.KindLoopWarning,
		Severity: signalcenter.SeverityWarn, Code: signalcenter.CodeLatencyBudgetExceeded, Reason: item.Title,
		Fields: map[string]string{"waves": strings.Join(breachWaves(run), ","), "item": item.ID, "reasons": strings.Join(run[len(run)-1].Reasons, "; ")},
	})
	signals.Flush()
	return item.ID, nil
}

func runWaveLatencyCheck(env waveEnv, plane wavePlaneDirs, asJSON bool, stdout, stderr io.Writer) int {
	records, err := plane.store.Records()
	var cycles []lanetime.Cycle
	var stream lanetime.Stream
	var warnings []string
	if err == nil {
		cycles, stream, warnings, err = readLatencyInputs(plane)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	pol := policy.LoadOrDefaults(filepath.Join(plane.evolveDir, "policy.json"), stderr)
	bounds := lanetime.Bounds{MinLiveShare: pol.WaveLatencyMinLiveShare(), MaxGateShare: pol.WaveLatencyMaxGateShare()}
	report := waveLatencyCheck{Waves: checkWaves(records, cycles, stream, bounds), Warnings: warnings}
	if run := lanetime.BreachRun(report.Waves); len(run) >= lanetime.BreachRunToAlarm {
		report.Exceeded = true
		if report.Item, err = fileLatencyBreach(env, plane, run, stderr); err != nil {
			fmt.Fprintf(stderr, "%sfile the latency item: %v\n", wavePrefix, err)
			return exitIO
		}
	}
	if asJSON {
		return writeWaveJSON(report, stdout, stderr)
	}
	writeWaveLatencyCheckText(stdout, report)
	return 0
}

func writeWaveLatencyCheckText(w io.Writer, r waveLatencyCheck) {
	for _, v := range r.Waves {
		fmt.Fprintf(w, "wave %d: %s, live %.1f%%, gate share %.1f%%", v.Wave, v.Status, v.LiveShare*100, v.GateShare*100)
		if len(v.Reasons) > 0 {
			fmt.Fprintf(w, " (%s)", strings.Join(v.Reasons, "; "))
		}
		fmt.Fprintln(w)
	}
	if r.Exceeded {
		fmt.Fprintf(w, "%s: two waves in a row breached; item %s\n", signalcenter.CodeLatencyBudgetExceeded, r.Item)
	}
	for _, warning := range r.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}
