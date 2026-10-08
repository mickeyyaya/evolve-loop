package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type waveSummary struct {
	Wave    int    `json:"wave"`
	Outcome string `json:"outcome"`
}

type waveStatusReport struct {
	Wave          int          `json:"wave"`
	RunID         string       `json:"run_id"`
	PID           int          `json:"pid"`
	PIDAlive      bool         `json:"pid_alive"`
	LoopLive      bool         `json:"loop_live"`
	Outcome       string       `json:"outcome"`
	Cycles        []wave.Cycle `json:"cycles"`
	CyclesOmitted int          `json:"cycles_omitted,omitempty"`
	LastWave      *waveSummary `json:"last_wave,omitempty"`
	NotesQueued   int          `json:"notes_queued"`
	Warnings      []string     `json:"warnings,omitempty"`
}

func parseWaveReadArgs(args []string) (projectRoot string, asJSON bool, err error) {
	operands, err := cliFlags{
		bools:  map[string]*bool{"--json": &asJSON},
		values: map[string]*string{"--project-root": &projectRoot},
	}.parse(args)
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected argument %q", operands[0])
	}
	return projectRoot, asJSON, err
}

func readWaveRecords(env waveEnv, args []string, stderr io.Writer) (wavePlaneDirs, []wave.Record, bool, int) {
	projectRoot, asJSON, err := parseWaveReadArgs(args)
	if err != nil {
		return wavePlaneDirs{}, nil, false, waveUsageError(stderr, err)
	}
	plane, err := resolveWavePlane(env, projectRoot, stderr)
	var records []wave.Record
	if err == nil {
		records, err = plane.store.Records()
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return plane, nil, asJSON, exitIO
	}
	return plane, records, asJSON, 0
}

func runWaveStatus(env waveEnv, args []string, stdout, stderr io.Writer) int {
	plane, records, asJSON, rc := readWaveRecords(env, args, stderr)
	if rc != 0 {
		return rc
	}
	if len(records) == 0 && !asJSON {
		fmt.Fprintln(stdout, "wave: no wave is recorded")
		return 0
	}
	report := buildWaveStatus(env, plane, records)
	if asJSON {
		return writeWaveJSON(report, stdout, stderr)
	}
	writeWaveStatusText(stdout, report)
	return 0
}

func buildWaveStatus(env waveEnv, plane wavePlaneDirs, records []wave.Record) waveStatusReport {
	report := waveStatusReport{Cycles: []wave.Cycle{}, LoopLive: len(waveLiveRuns(plane.evolveDir, env.now())) > 0}
	if notes, err := plane.store.Notes(); err == nil {
		report.NotesQueued = len(notes)
	}
	if len(records) == 0 {
		return report
	}
	cur := records[len(records)-1]
	cycles, warnings := wave.ReadCycles(plane.root, cur.CycleFloor, 0)
	report.Wave, report.RunID, report.PID, report.Warnings = cur.Number, cur.RunID, cur.PID, warnings
	report.PIDAlive = waveOwnsPID(env, cur)
	report.Outcome = wave.Facts{Cycles: cycles}.Outcome()
	if over := len(cycles) - waveStatusMaxCycles; over > 0 {
		cycles, report.CyclesOmitted = cycles[over:], over
	}
	report.Cycles = append(report.Cycles, cycles...)
	if len(records) > 1 {
		prev := records[len(records)-2]
		report.LastWave = &waveSummary{Wave: prev.Number, Outcome: prev.Outcome}
	}
	return report
}

func writeWaveStatusText(w io.Writer, r waveStatusReport) {
	fmt.Fprintf(w, "wave:    %d run %s\n", r.Wave, r.RunID)
	fmt.Fprintf(w, "loop:    live=%t pid=%d alive=%t\n", r.LoopLive, r.PID, r.PIDAlive)
	fmt.Fprintf(w, "outcome: %s\n", r.Outcome)
	if r.CyclesOmitted > 0 {
		fmt.Fprintf(w, "  (%d older cycle(s) not shown)\n", r.CyclesOmitted)
	}
	for _, c := range r.Cycles {
		fmt.Fprintf(w, "  %s\n", waveCycleText(c))
	}
	if r.LastWave != nil {
		fmt.Fprintf(w, "last:    wave %d: %s\n", r.LastWave.Wave, r.LastWave.Outcome)
	}
	fmt.Fprintf(w, "notes:   %d queued\n", r.NotesQueued)
	for _, warn := range r.Warnings {
		fmt.Fprintf(w, "WARN:    %s\n", warn)
	}
}

func waveCycleText(c wave.Cycle) string {
	text := fmt.Sprintf("cycle %d: phase %s", c.ID, c.Phase)
	if c.Verdict != "" {
		shipped := "not shipped"
		if c.Shipped {
			shipped = "shipped"
		}
		text = fmt.Sprintf("cycle %d: verdict %s, %s", c.ID, c.Verdict, shipped)
	}
	if c.QuotaPauses > 0 {
		text += fmt.Sprintf(", quota pauses %d", c.QuotaPauses)
	}
	return text
}

func runWaveWatch(env waveEnv, args []string, stdout, stderr io.Writer) int {
	plane, records, asJSON, rc := readWaveRecords(env, args, stderr)
	if rc != 0 {
		return rc
	}
	if len(records) == 0 {
		fmt.Fprintf(stderr, "%sno wave is recorded; start one with: evolve wave next\n", wavePrefix)
		return exitIO
	}
	w := &waveWatcher{env: env, plane: plane, cur: records[len(records)-1], prev: wave.Snapshot{LoopLive: true}, seen: map[string]bool{}, asJSON: asJSON, stdout: stdout, stderr: stderr}
	for {
		done, err := w.poll()
		if err != nil {
			fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
			return exitIO
		}
		if done {
			return 0
		}
		env.sleep(waveWatchPoll)
	}
}

type waveWatcher struct {
	env            waveEnv
	plane          wavePlaneDirs
	cur            wave.Record
	prev           wave.Snapshot
	seen           map[string]bool
	asJSON         bool
	stdout, stderr io.Writer
}

func (w *waveWatcher) poll() (bool, error) {
	if err := w.followNewerWave(); err != nil {
		return false, err
	}
	cycles, warnings := wave.ReadCycles(w.plane.root, w.cur.CycleFloor, 0)
	for _, warn := range warnings {
		if !w.seen[warn] {
			w.seen[warn] = true
			fmt.Fprintf(w.stderr, "%sWARN: %s\n", wavePrefix, warn)
		}
	}
	snap := wave.Snapshot{Cycles: cycles, LoopLive: waveLoopLive(w.env, w.plane.evolveDir, w.cur)}
	for _, e := range wave.Diff(w.prev, snap) {
		if err := writeWaveEvent(w.stdout, w.cur.Number, e, w.asJSON); err != nil {
			return false, fmt.Errorf("write: %w", err)
		}
	}
	done := w.prev.LoopLive && !snap.LoopLive
	w.prev = snap
	return done, nil
}

func (w *waveWatcher) followNewerWave() error {
	records, err := w.plane.store.Records()
	if err != nil || len(records) == 0 || records[len(records)-1].Number <= w.cur.Number {
		return err
	}
	newer := records[len(records)-1]
	e := wave.Event{Kind: wave.EventWaveStarted, Detail: strconv.Itoa(newer.Number)}
	if err := writeWaveEvent(w.stdout, w.cur.Number, e, w.asJSON); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	w.cur, w.prev = newer, wave.Snapshot{LoopLive: true}
	return nil
}

func writeWaveEvent(w io.Writer, number int, e wave.Event, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(e)
	}
	_, err := fmt.Fprintf(w, "wave %d: %s\n", number, e)
	return err
}
