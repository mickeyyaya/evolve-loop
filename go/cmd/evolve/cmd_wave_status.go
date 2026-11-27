package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

type waveSummary struct {
	Wave    int    `json:"wave"`
	Outcome string `json:"outcome"`
}

type waveStatusReport struct {
	ProjectRoot   string                  `json:"project_root"`
	Wave          int                     `json:"wave"`
	RunID         string                  `json:"run_id"`
	PID           int                     `json:"pid"`
	PIDAlive      bool                    `json:"pid_alive"`
	LoopLive      bool                    `json:"loop_live"`
	PoolLive      *int                    `json:"pool_live,omitempty"`
	Outcome       string                  `json:"outcome"`
	Cycles        []wave.Cycle            `json:"cycles"`
	CyclesOmitted int                     `json:"cycles_omitted,omitempty"`
	LastWave      *waveSummary            `json:"last_wave,omitempty"`
	NotesQueued   int                     `json:"notes_queued"`
	Measures      wavemeasure.Measures    `json:"measures"`
	Runs          []waveRunMeasures       `json:"runs"`
	Warnings      []string                `json:"warnings,omitempty"`
	Held          []heldproposal.Record   `json:"held,omitempty"`
	Benched       []clihealth.Entry       `json:"benched,omitempty"`
	Seats         []waveSeat              `json:"seats"`
	Health        *pipelinerates.WaveView `json:"health,omitempty"`
	Host          *waveHostReading        `json:"host,omitempty"`
	Blockers      []waveBlocker           `json:"blockers"`
	BlockerCounts map[string]int          `json:"blocker_counts"`
}

func parseWaveReadArgs(args []string) (projectRoot string, asJSON bool, err error) {
	operands, err := cmdutil.Flags().Bools(map[string]*bool{"--json": &asJSON}).Values(map[string]*string{"--project-root": &projectRoot}).Parse(args)
	if err == nil && len(operands) > 0 {
		err = fmt.Errorf("unexpected argument %q", operands[0])
	}
	return projectRoot, asJSON, err
}

type waveRead struct {
	op   string
	args []string
}

func readWaveRecords(env waveEnv, read waveRead, stderr io.Writer) (wavePlaneDirs, []wave.Record, bool, int) {
	projectRoot, asJSON, err := parseWaveReadArgs(read.args)
	if err != nil {
		return wavePlaneDirs{}, nil, false, waveUsageError(stderr, err)
	}
	plane, err := resolveWavePlane(env, projectRoot, stderr)
	if err == nil {
		if rc := checkRootHasEvolve(read.op, plane.root, stderr); rc != 0 {
			return plane, nil, asJSON, rc
		}
	}
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
	plane, records, asJSON, rc := readWaveRecords(env, waveRead{"wave status", args}, stderr)
	if rc != 0 {
		return rc
	}
	if len(records) == 0 && !asJSON {
		fmt.Fprintf(stdout, "wave: no wave is recorded under %s\n", plane.root)
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
	report := waveStatusReport{ProjectRoot: plane.root, Cycles: []wave.Cycle{}, Blockers: []waveBlocker{}, BlockerCounts: map[string]int{}, Runs: []waveRunMeasures{}, LoopLive: len(waveLiveRuns(plane.evolveDir, env.now())) > 0}
	if notes, err := plane.store.Notes(); err == nil {
		report.NotesQueued = len(notes)
	}
	report.Held, report.Warnings = heldForOperator(plane.evolveDir, report.Warnings)
	var seatWarnings []string
	if env.seatDeps != nil {
		report.Seats, seatWarnings = collectWaveSeatsWithDeps(plane.evolveDir, env.now(), *env.seatDeps)
	} else {
		report.Seats, seatWarnings = collectWaveSeats(plane.evolveDir, env.now())
	}
	report.Warnings = append(report.Warnings, seatWarnings...)
	report.Benched = activeBenches(clihealth.NewStore(plane.root, env.now))
	if len(records) == 0 {
		return report
	}
	cur := records[len(records)-1]
	read, warnings := readWaveOf(plane, cur)
	cycles, loop := read.Cycles, read.Loop
	report.Wave, report.RunID, report.PID = cur.Number, cur.RunID, cur.PID
	report.Warnings = append(report.Warnings, warnings...)
	report.Warnings = append(report.Warnings, poolPreflightWarnings(loop, cur.StartedAt)...)
	var startWarning string
	report.PIDAlive, startWarning = waveOwnsPID(env, cur)
	if live, ok := wave.PoolLive(cur, nil, loop); ok && (report.PIDAlive || report.LoopLive) {
		report.PoolLive = &live
	}
	if startWarning != "" {
		report.Warnings = append(report.Warnings, startWarning)
	}
	report.Outcome = wave.Facts{Cycles: cycles, Ships: read.Landed}.Outcome()
	runs, measureWarnings := measureWaveRuns(plane, records, cyclesAboveFloor(cycles, cur.CycleFloor), loop, report.PIDAlive || report.LoopLive, env.now())
	report.Runs, report.Measures = runs, runs[len(runs)-1].Measures
	report.Warnings = append(report.Warnings, measureWarnings...)
	report.Health, report.Warnings = waveHealth(plane.root, env.now(), report.Warnings)
	var hostWarnings []string
	report.Host, hostWarnings = readWaveHost(plane.evolveDir, cur)
	report.Warnings = append(report.Warnings, hostWarnings...)
	report.Blockers, report.BlockerCounts, report.Warnings = waveBlockers(plane.evolveDir, cyclesAboveFloor(cycles, cur.CycleFloor), report.Warnings)
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

func waveHealth(root string, now time.Time, warnings []string) (*pipelinerates.WaveView, []string) {
	view, err := pipelinerates.ReadWaveView(root, now)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, warnings
	case err != nil:
		return nil, append(warnings, "wave health: "+err.Error())
	}
	return &view, warnings
}

func poolPreflightWarnings(loop []signalcenter.Event, since time.Time) []string {
	var out []string
	for _, sig := range loop {
		if sig.Code != loopwave.CodeWaveDispatchFailed || sig.Fields["path"] != "pool" {
			continue
		}
		if at, err := time.Parse(time.RFC3339Nano, sig.TS); err == nil && at.Before(since) {
			continue
		}
		out = append(out, sig.Reason)
	}
	return out
}

func writeWaveStatusText(w io.Writer, r waveStatusReport) {
	fmt.Fprintf(w, "wave:    %d run %s\n", r.Wave, r.RunID)
	fmt.Fprintf(w, "loop:    live=%t pid=%d alive=%t\n", r.LoopLive, r.PID, r.PIDAlive)
	if r.PoolLive != nil {
		fmt.Fprintf(w, "pool:    live %d\n", *r.PoolLive)
	}
	fmt.Fprintf(w, "outcome: %s\n", r.Outcome)
	writeWaveMeasuresText(w, r.Measures)
	if r.Health != nil {
		fmt.Fprintln(w, r.Health.Line())
	}
	writeWaveHostText(w, r.Host)
	if r.CyclesOmitted > 0 {
		fmt.Fprintf(w, "  (%d older cycle(s) not shown)\n", r.CyclesOmitted)
	}
	for _, c := range r.Cycles {
		fmt.Fprintf(w, "  %s%s\n", waveCycleText(c), blockerText(r.Blockers, c.ID))
	}
	if r.LastWave != nil {
		fmt.Fprintf(w, "last:    wave %d: %s\n", r.LastWave.Wave, r.LastWave.Outcome)
	}
	fmt.Fprintf(w, "notes:   %d queued\n", r.NotesQueued)
	for _, b := range r.Benched {
		fmt.Fprintf(w, "benched: %s %s until %s\n", b.Key(), b.Reason, b.BenchedUntil.Format(time.RFC3339))
	}
	writeWaveSeatsText(w, r.Seats)
	for _, h := range r.Held {
		fmt.Fprintf(w, "held:    cycle %d held for operator (tree %s): evolve landing approve|reject --cycle %d --tree %s\n", h.Cycle, h.Tree, h.Cycle, h.Tree)
	}
	for _, warn := range r.Warnings {
		fmt.Fprintf(w, "WARN:    %s\n", warn)
	}
}

func heldForOperator(evolveDir string, warnings []string) ([]heldproposal.Record, []string) {
	records, err := heldproposal.ReadRecords(evolveDir)
	if err != nil {
		return nil, append(warnings, "held proposals: "+err.Error())
	}
	var held []heldproposal.Record
	for _, r := range records {
		if r.Status == heldproposal.StatusHeld {
			held = append(held, r)
		}
	}
	return held, warnings
}

func waveCycleText(c wave.Cycle) string {
	text := fmt.Sprintf("cycle %d: phase %s", c.ID, c.Phase)
	if c.Termination == cyclestate.CycleTerminationLaneDeadline {
		text = fmt.Sprintf("cycle %d: %s (phase %s)", c.ID, c.Termination, c.Phase)
	}
	if c.Verdict != "" {
		shipped := "not shipped"
		if c.Shipped {
			shipped = "shipped"
		}
		text = fmt.Sprintf("cycle %d: verdict %s, %s", c.ID, c.Verdict, shipped)
	}
	text += waveVerdictText(c)
	if c.Resumed {
		text += " (resumed)"
	}
	if c.HandoffStop != "" {
		text += ", handed-off (" + c.HandoffStop + ")"
	}
	if c.QuotaPauses > 0 {
		text += fmt.Sprintf(", quota pauses %d", c.QuotaPauses)
	}
	if c.LoginWalls > 0 {
		text += fmt.Sprintf(", login walls %d", c.LoginWalls)
	}
	return text
}

func waveVerdictText(c wave.Cycle) string {
	v := c.LLMVerdict
	if v.LLMMinutesToVerdict <= 0 && v.HostMinutesToVerdict <= 0 {
		return ""
	}
	var text string
	if v.LLMMinutesToVerdict > 0 {
		if c.Shipped {
			text = fmt.Sprintf(", %.1f LLM min to ship", v.LLMMinutesToVerdict)
		} else {
			text = fmt.Sprintf(", %.1f LLM min to verdict", v.LLMMinutesToVerdict)
		}
	}
	if v.HostMinutesToVerdict > 0 {
		text += fmt.Sprintf(", %.1f host min", v.HostMinutesToVerdict)
	}
	if !c.Shipped && v.FailPointMinutes != nil && v.WasteMinutes != nil && v.FailFastRatio != nil {
		text += fmt.Sprintf(", fail point %.1f min, waste %.1f min, fail-fast %.2f", *v.FailPointMinutes, *v.WasteMinutes, *v.FailFastRatio)
	}
	return text
}

func runWaveWatch(env waveEnv, args []string, stdout, stderr io.Writer) int {
	plane, records, asJSON, rc := readWaveRecords(env, waveRead{"wave watch", args}, stderr)
	if rc != 0 {
		return rc
	}
	if len(records) == 0 {
		fmt.Fprintf(stderr, "%sno wave is recorded; start one with: evolve wave next\n", wavePrefix)
		return exitIO
	}
	w := &waveWatcher{env: env, plane: plane, cur: records[len(records)-1], prev: wave.Snapshot{LoopLive: true}, seen: map[string]bool{}, asJSON: asJSON, stdout: stdout, stderr: stderr}
	events := w.openEvents()
	for {
		done, err := w.poll()
		if err != nil {
			fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
			return exitIO
		}
		if done {
			return 0
		}
		events = w.wait(events)
	}
}

type waveWatcher struct {
	env            waveEnv
	plane          wavePlaneDirs
	cur            wave.Record
	prev           wave.Snapshot
	seen           map[string]bool
	stallsSeen     map[string]bool
	asJSON         bool
	stdout, stderr io.Writer
	signalOffset   int64
	eventsFailed   bool
}

func (w *waveWatcher) poll() (bool, error) {
	if err := w.followNewerWave(); err != nil {
		return false, err
	}
	if err := w.pollSignals(); err != nil {
		return false, err
	}
	read, warnings := readWaveOf(w.plane, w.cur)
	cycles := read.Cycles
	loopLive, startWarning := waveLoopLive(w.env, w.plane.evolveDir, w.cur)
	for _, warn := range append(warnings, startWarning) {
		w.warnOnce(warn)
	}
	snap := wave.Snapshot{Cycles: cycles, LoopLive: loopLive}
	for _, e := range wave.Diff(w.prev, snap) {
		if err := writeWaveEvent(w.stdout, w.cur.Number, e, w.asJSON); err != nil {
			return false, fmt.Errorf("write: %w", err)
		}
	}
	if err := w.pollStalls(read, loopLive); err != nil {
		return false, err
	}
	done := w.prev.LoopLive && !snap.LoopLive
	w.prev = snap
	return done, nil
}

func (w *waveWatcher) pollStalls(read wave.WaveRead, loopLive bool) error {
	if !loopLive {
		return nil
	}
	now := w.env.now()
	var policyWarn bytes.Buffer
	pol := policy.LoadOrDefaults(filepath.Join(w.plane.evolveDir, "policy.json"), &policyWarn)
	if msg := strings.TrimSpace(policyWarn.String()); msg != "" {
		w.warnOnce(msg)
	}
	snap := wavehealth.Snapshot{
		Now:      now,
		Settings: pol.WaveCheckinSettings(),
		Loop:     wavehealth.Have(wavehealth.LoopState{Running: true, BrakeHeld: loopchain.BrakeEngaged(w.plane.evolveDir)}),
	}
	populated, err := populateWaveHealthSnapshot(w.plane, w.cur, read, snap)
	if err != nil {
		w.warnOnce(err.Error())
	}
	report := wavehealth.Evaluate(populated)
	for _, t := range wavehealth.StallFindings(populated, report) {
		key := stallTriggerKey(t)
		if w.stallsSeen[key] {
			continue
		}
		e := w.stallWaveEvent(t, read, now)
		if err := writeWaveEvent(w.stdout, w.cur.Number, e, w.asJSON); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		if w.stallsSeen == nil {
			w.stallsSeen = map[string]bool{}
		}
		w.stallsSeen[key] = true
	}
	return nil
}

func stallTriggerKey(t wavehealth.StallTrigger) string {
	if t.Kind == "" {
		return t.Trigger
	}
	return t.Trigger + ":" + t.Kind
}

func (w *waveWatcher) stallWaveEvent(t wavehealth.StallTrigger, read wave.WaveRead, now time.Time) wave.Event {
	e := wave.Event{
		Kind:    wave.EventStall,
		Detail:  t.Detail,
		Trigger: t.Trigger,
	}
	switch t.Trigger {
	case wavehealth.TriggerNoShip:
		e.Lanes = wave.OpenLanes(w.cur, signalsUpTo(read.Loop, now))
		if len(e.Lanes) == 0 {
			e.Lanes = cycleNamesOf(read.Cycles)
		}
		e.Evidence = existingRelPaths(w.plane.root, filepath.Join(w.plane.evolveDir, signalcenter.StreamFileName))
	case wavehealth.TriggerAbnormal:
		e.Lanes = cycleNamesFromIDs(t.Cycles)
		candidates := make([]string, 0, len(t.Cycles))
		for _, id := range t.Cycles {
			candidates = append(candidates, filepath.Join(paths.RunWorkspace(w.plane.root, id), dispatchevents.EventsFile))
		}
		e.Evidence = existingRelPaths(w.plane.root, candidates...)
	case wavehealth.TriggerLaneStalls:
		e.Lanes = t.Lanes
		e.Evidence = existingRelPaths(w.plane.root, filepath.Join(w.plane.evolveDir, signalcenter.StreamFileName))
	}
	return e
}

func cycleNamesOf(cycles []wave.Cycle) []string {
	ids := make([]int, len(cycles))
	for i, c := range cycles {
		ids[i] = c.ID
	}
	sort.Ints(ids)
	return cycleNamesFromIDs(ids)
}

func cycleNamesFromIDs(ids []int) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = "cycle-" + strconv.Itoa(id)
	}
	return out
}

func existingRelPaths(root string, candidates ...string) []string {
	var out []string
	for _, abs := range candidates {
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			out = append(out, filepath.ToSlash(abs))
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

func (w *waveWatcher) pollSignals() error {
	signalPath := filepath.Join(w.plane.evolveDir, signalcenter.StreamFileName)
	chunk, err := signalcenter.ReadStream(signalPath, w.signalOffset)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		w.warnOnce(err.Error())
		return nil
	}
	w.signalOffset = chunk.Next
	for _, sig := range chunk.Events {
		at, _ := time.Parse(time.RFC3339Nano, sig.TS)
		if !w.cur.StartedAt.IsZero() && at.Before(w.cur.StartedAt) {
			continue
		}
		if sig.Kind != signalcenter.KindLoopWave {
			continue
		}
		if !sig.Severity.AtLeast(signalcenter.SeverityWarn) && !isPoolTransition(sig.Code) {
			continue
		}
		e := wave.Event{Kind: wave.EventPool, Detail: poolEventDetail(sig)}
		if err := writeWaveEvent(w.stdout, w.cur.Number, e, w.asJSON); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	return nil
}

func isPoolTransition(c signalcenter.Code) bool {
	switch c {
	case orchestrator.CodeLaneLaunched,
		orchestrator.CodeLaneExited,
		orchestrator.CodeLaneBackfilled,
		orchestrator.CodePoolIdle,
		orchestrator.CodePoolFloorHold,
		orchestrator.CodeLaneStalled,
		orchestrator.CodeLaneLateExit:
		return true
	}
	return false
}

func poolEventDetail(sig signalcenter.Event) string {
	if sig.Reason != "" {
		return fmt.Sprintf("%s: %s", sig.Code, sig.Reason)
	}
	return string(sig.Code)
}

func (w *waveWatcher) warnOnce(warn string) {
	if warn == "" || w.seen[warn] {
		return
	}
	w.seen[warn] = true
	fmt.Fprintf(w.stderr, "%sWARN: %s\n", wavePrefix, warn)
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
	w.cur, w.prev, w.stallsSeen = newer, wave.Snapshot{LoopLive: true}, nil
	return nil
}

func writeWaveEvent(w io.Writer, number int, e wave.Event, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(e)
	}
	_, err := fmt.Fprintf(w, "wave %d: %s\n", number, e)
	return err
}

func readWaveOf(plane wavePlaneDirs, rec wave.Record) (wave.WaveRead, []string) {
	stage, stageWarning := shipStageOf(plane.evolveDir)
	read := wave.ReadWave(plane.root, wave.WaveWindow{Floor: rec.CycleFloor, From: wave.RunFrom(rec), Stage: stage})
	warnings := foldWaveRead(read)
	if stageWarning != "" {
		warnings = append(warnings, stageWarning)
	}
	return read, warnings
}

func foldWaveRead(read wave.WaveRead) []string {
	warnings := append([]string(nil), read.Warnings...)
	for _, f := range append(append([]wave.ReadFailure(nil), read.Failures...), read.ShipFailures...) {
		warnings = append(warnings, f.Error())
	}
	return warnings
}

func cyclesAboveFloor(cycles []wave.Cycle, floor int) []wave.Cycle {
	var out []wave.Cycle
	for _, c := range cycles {
		if c.ID > floor {
			out = append(out, c)
		}
	}
	return out
}
