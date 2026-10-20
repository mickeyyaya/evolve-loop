package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

var cycleRangeFlag = regexp.MustCompile(`^(\d+)-(\d+)$`)

type waveLatencyReport struct {
	Wave       int                   `json:"wave,omitempty"`
	FirstCycle int                   `json:"first_cycle"`
	LastCycle  int                   `json:"last_cycle"`
	Cycles     int                   `json:"cycles"`
	Slots      lanetime.Slots        `json:"slots"`
	Phases     lanetime.PhaseReport  `json:"phases"`
	Families   []lanetime.FamilyLane `json:"families"`
	Warnings   []string              `json:"warnings,omitempty"`
}

type latencyRequest struct {
	wave     int
	from, to int
}

type latencyScope struct {
	wave           int
	pid            int
	floor, ceiling int
	from, to       time.Time
}

func parseLatencyRequest(waveFlag, cyclesFlag string) (latencyRequest, error) {
	if waveFlag != "" && cyclesFlag != "" {
		return latencyRequest{}, errors.New("--wave and --cycles exclude each other")
	}
	if cyclesFlag != "" {
		m := cycleRangeFlag.FindStringSubmatch(cyclesFlag)
		if m == nil {
			return latencyRequest{}, fmt.Errorf("--cycles wants A-B, got %q", cyclesFlag)
		}
		from, _ := strconv.Atoi(m[1])
		to, _ := strconv.Atoi(m[2])
		if from < 1 || to < from {
			return latencyRequest{}, fmt.Errorf("--cycles %q is not a range of cycle numbers from 1 up", cyclesFlag)
		}
		return latencyRequest{from: from, to: to}, nil
	}
	if waveFlag == "" {
		return latencyRequest{}, nil
	}
	n, err := strconv.Atoi(waveFlag)
	if err != nil || n < 1 {
		return latencyRequest{}, fmt.Errorf("--wave wants a wave number, got %q", waveFlag)
	}
	return latencyRequest{wave: n}, nil
}

func (r latencyRequest) scope(records []wave.Record) (latencyScope, error) {
	if r.from > 0 {
		return latencyScope{floor: r.from - 1, ceiling: r.to}, nil
	}
	idx := len(records) - 1
	for i, rec := range records {
		if r.wave == rec.Number {
			idx = i
		}
	}
	if idx < 0 {
		return latencyScope{}, errors.New("no wave is recorded")
	}
	rec := records[idx]
	if r.wave != 0 && rec.Number != r.wave {
		return latencyScope{}, fmt.Errorf("wave %d is not recorded", r.wave)
	}
	s := latencyScope{wave: rec.Number, pid: rec.PID, floor: rec.CycleFloor, from: rec.StartedAt}
	if rec.EndedAt != nil {
		s.to = *rec.EndedAt
	}
	if idx+1 < len(records) && records[idx+1].CycleFloor > rec.CycleFloor {
		s.ceiling = records[idx+1].CycleFloor
	}
	return s, nil
}

func (s latencyScope) holds(id int) bool {
	return id > s.floor && (s.ceiling == 0 || id <= s.ceiling)
}

func runWaveLatency(env waveEnv, args []string, stdout, stderr io.Writer) int {
	var projectRoot, waveFlag, cyclesFlag string
	var asJSON bool
	operands, err := cliFlags{
		bools:  map[string]*bool{"--json": &asJSON},
		values: map[string]*string{"--project-root": &projectRoot, "--wave": &waveFlag, "--cycles": &cyclesFlag},
	}.parse(args)
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected argument %q", operands[0])
	}
	var req latencyRequest
	if err == nil {
		req, err = parseLatencyRequest(waveFlag, cyclesFlag)
	}
	if err != nil {
		return waveUsageError(stderr, err)
	}
	plane, err := resolveWavePlane(env, projectRoot, stderr)
	var records []wave.Record
	if err == nil && req.from == 0 {
		records, err = plane.store.Records()
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	scope, err := req.scope(records)
	if err != nil {
		return waveUsageError(stderr, err)
	}
	report, err := buildWaveLatency(plane, scope)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return exitIO
	}
	if asJSON {
		return writeWaveJSON(report, stdout, stderr)
	}
	writeWaveLatencyText(stdout, report)
	return 0
}

func buildWaveLatency(plane wavePlaneDirs, scope latencyScope) (waveLatencyReport, error) {
	all, warnings, err := lanetime.ReadCycles(plane.evolveDir, 0, 0)
	if err != nil {
		return waveLatencyReport{}, err
	}
	samples, malformed, err := lanetime.ReadSamples(filepath.Join(plane.evolveDir, "signals.ndjson"))
	if err != nil {
		return waveLatencyReport{}, err
	}
	if malformed > 0 {
		warnings = append(warnings, fmt.Sprintf("%d malformed orchestration line(s) were skipped", malformed))
	}
	var in []lanetime.Cycle
	for _, c := range all {
		if scope.holds(c.ID) {
			in = append(in, c)
		}
	}
	report := waveLatencyReport{Wave: scope.wave, Cycles: len(in), Phases: lanetime.Phases(in), Families: lanetime.LaneTimeToPass(all, lanetime.DefaultBuilds), Warnings: warnings}
	if len(in) == 0 {
		report.Warnings = append(report.Warnings, "no run dir lies in the range; the run dirs may be pruned")
		return report, nil
	}
	report.FirstCycle, report.LastCycle = in[0].ID, in[len(in)-1].ID
	from, to := scope.from, scope.to
	if scope.wave == 0 {
		from, to = lanetime.SpanOf(in)
	}
	if scope.pid > 0 {
		samples, from = lanetime.ForPID(samples, scope.pid), time.Time{}
	}
	report.Slots = lanetime.SlotUse(samples, from, to)
	return report, nil
}

func writeWaveLatencyText(w io.Writer, r waveLatencyReport) {
	title := "cycles"
	if r.Wave > 0 {
		title = fmt.Sprintf("wave %d: cycles", r.Wave)
	}
	fmt.Fprintf(w, "%s %d-%d, %d cycle(s), lane time %s\n", title, r.FirstCycle, r.LastCycle, r.Cycles, roundSecond(r.Phases.Lane))
	writeLatencySlots(w, r.Slots)
	writeLatencyPhases(w, r.Phases)
	writeLatencyFamilies(w, r.Families)
	for _, warning := range r.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}

func writeLatencyPhases(w io.Writer, r lanetime.PhaseReport) {
	width := len("(gap)")
	for _, p := range r.Phases {
		width = max(width, len(p.Phase))
	}
	fmt.Fprintf(w, "%-*s %9s %7s %9s %9s %6s %9s\n", width, "phase", "time", "share", "model", "host", "gates", "gate-med")
	for _, p := range r.Phases {
		fmt.Fprintf(w, "%-*s %9s %6.1f%% %9s %9s %6d %9s\n", width, p.Phase, roundSecond(p.Time), p.Share*100, roundSecond(p.Model), roundSecond(p.Host), p.Gates, roundSecond(p.GateMedian))
	}
	fmt.Fprintf(w, "%-*s %9s %6.1f%%\n", width, "(gap)", roundSecond(r.Gap), r.GapShare*100)
}

func roundSecond(d time.Duration) time.Duration { return d.Round(time.Second) }

func writeLatencySlots(w io.Writer, s lanetime.Slots) {
	if s.Window == 0 {
		fmt.Fprintln(w, "slots: no orchestration sample lies in the window")
		return
	}
	target := s.CapacitySlotHours / s.Window.Hours()
	fmt.Fprintf(w, "slots: avg live %.2f of %.2f, %.2f of %.2f slot-hours used (%.1f%%) over %s\n",
		s.AvgLive, target, s.UsedSlotHours, s.CapacitySlotHours, s.Utilization*100, roundSecond(s.Window))
}

func writeLatencyFamilies(w io.Writer, families []lanetime.FamilyLane) {
	fmt.Fprintf(w, "lane time to a PASS, last %d builds of each family:\n", lanetime.DefaultBuilds)
	width := 0
	for _, f := range families {
		width = max(width, len(f.Family))
	}
	for _, f := range families {
		perPass := "none"
		if f.Passes > 0 {
			perPass = roundSecond(f.PerPass).String()
		}
		fmt.Fprintf(w, "%-*s  builds %d  passes %d  lane %s  per pass %s\n", width, f.Family, f.Builds, f.Passes, roundSecond(f.Lane), perPass)
	}
}
