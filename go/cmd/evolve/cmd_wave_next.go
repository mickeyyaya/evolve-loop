package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	waveDryRunDetail   = "planned"
	waveRefusedLive    = "loop_live"
	waveRefusedNumber  = "number_taken"
	waveRecordAttempts = 2
)

type waveNextArgs struct {
	boundary     boundaryArgs
	notes        []string
	number       int
	dryRun, json bool
}

type waveStepResult struct {
	Name   string `json:"name"`
	Verb   string `json:"verb"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type waveNextEnvelope struct {
	Schema     string           `json:"schema"`
	Wave       int              `json:"wave"`
	RunID      string           `json:"run_id"`
	PID        int              `json:"pid"`
	GoalPath   string           `json:"goal_path"`
	GoalBytes  int              `json:"goal_bytes"`
	Goal       string           `json:"goal,omitempty"`
	DryRun     bool             `json:"dry_run"`
	Launched   bool             `json:"launched"`
	Recorded   bool             `json:"recorded"`
	Refused    string           `json:"refused,omitempty"`
	FailedStep string           `json:"failed_step,omitempty"`
	Error      string           `json:"error,omitempty"`
	Steps      []waveStepResult `json:"steps"`
	Warnings   []string         `json:"warnings"`
}

type waveRefusal struct {
	code string
	err  error
}

func (r *waveRefusal) Error() string { return r.err.Error() }

type wavePlan struct {
	plane    wavePlaneDirs
	args     waveNextArgs
	number   int
	floor    int
	goal     string
	notes    []wave.Note
	last     *wave.Record
	facts    wave.Facts
	steps    []boundaryStep
	warnings []string
}

type waveStepRecorder struct {
	next    boundaryDispatch
	steps   []boundaryStep
	results []waveStepResult
}

func (r *waveStepRecorder) dispatch(verb string, args []string, stdout, stderr io.Writer) int {
	rc := r.next(verb, args, stdout, stderr)
	name := boundaryStep{verb: verb, args: args}.String()
	if i := len(r.results); i < len(r.steps) {
		name = r.steps[i].String()
	}
	r.results = append(r.results, waveStepResult{Name: name, Verb: verb, OK: rc == 0, Detail: fmt.Sprintf("rc=%d", rc)})
	return rc
}

func runWaveNext(env waveEnv, args []string, stdout, stderr io.Writer) int {
	e := waveNextEnvelope{Schema: waveNextSchema, Steps: []waveStepResult{}, Warnings: []string{}}
	rc := waveNext(env, args, &e, stdout, stderr)
	if slices.Contains(args, "--json") {
		if jrc := writeWaveJSON(e, stdout, stderr); rc == 0 {
			rc = jrc
		}
		return rc
	}
	writeWaveWarnings(e.Warnings, stderr)
	if rc == 0 && !e.DryRun {
		fmt.Fprintf(stdout, "wave: launched wave %d: run %s, pid %d, goal %d bytes\n", e.Wave, e.RunID, e.PID, e.GoalBytes)
	}
	return rc
}

func waveNext(env waveEnv, args []string, e *waveNextEnvelope, stdout, stderr io.Writer) int {
	a, err := parseWaveNextArgs(args)
	if err != nil {
		e.Error = "usage: " + err.Error()
		return waveUsageError(stderr, err)
	}
	plan, err := prepareWave(env, a, stderr)
	e.Wave, e.RunID, e.DryRun, e.Warnings = plan.number, plan.args.boundary.runID, a.dryRun, plan.warnings
	if plan.goal != "" {
		e.GoalPath, e.GoalBytes = plan.goalPath(), len(plan.goal)
	}
	if err != nil {
		return failWave(e, err, stderr)
	}
	if a.dryRun {
		return planWave(plan, e, stdout)
	}
	return executeWave(env, plan, e, stdout, stderr)
}

func failWave(e *waveNextEnvelope, err error, stderr io.Writer) int {
	e.Error = err.Error()
	var refusal *waveRefusal
	if errors.As(err, &refusal) {
		e.Refused = refusal.code
		fmt.Fprintf(stderr, "%srefused (%s): %v\n", wavePrefix, refusal.code, err)
		return exitRefused
	}
	fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
	return exitIO
}

func prepareWave(env waveEnv, a waveNextArgs, stderr io.Writer) (wavePlan, error) {
	plane, err := resolveWavePlane(env, a.boundary.projectRoot, stderr)
	if err != nil {
		return wavePlan{args: a, warnings: []string{}}, err
	}
	plan := wavePlan{plane: plane, args: a, warnings: []string{}}
	if err := checkWaveIdle(env, &plan); err != nil {
		return plan, err
	}
	plan.warnings = append(plan.warnings, planeBinaryWarnings(env, plane.root)...)
	cfg, standing, err := loadWaveInputs(plane)
	var records []wave.Record
	if err == nil {
		records, err = plane.store.Records()
	}
	if err == nil {
		plan.number, err = wave.NextNumber(records, a.number)
		if errors.Is(err, wave.ErrNumberTaken) {
			return plan, &waveRefusal{code: waveRefusedNumber, err: err}
		}
	}
	if err == nil {
		plan.floor, err = wave.LastCycleNumber(plane.evolveDir)
	}
	if err == nil {
		err = composeWaveGoal(env, &plan, cfg, standing, records)
	}
	if err != nil {
		return plan, err
	}
	plan.args.boundary = waveBoundaryArgs(env, plan, cfg)
	plan.steps = waveSteps(plan.args.boundary, plane.root, cfg.HealthDrivers)
	return plan, nil
}

func checkWaveIdle(env waveEnv, plan *wavePlan) error {
	for _, r := range waveLiveRuns(plan.plane.evolveDir, env.now()) {
		msg := fmt.Sprintf("run %s (%s) is live, owner pid %d; stop it with: evolve loop-stop --wait", r.Lease.RunID, filepath.Base(r.Dir), r.Lease.OwnerPID)
		if !plan.args.dryRun {
			return &waveRefusal{code: waveRefusedLive, err: errors.New(msg)}
		}
		plan.warnings = append(plan.warnings, msg)
	}
	return nil
}

func planeBinaryWarnings(env waveEnv, root string) []string {
	plane := filepath.Join(root, "go", "bin", "evolve")
	running, err := env.executable()
	if err != nil {
		return []string{fmt.Sprintf("cannot find the running binary: %v", err)}
	}
	if sameBinaryPath(running, plane) {
		return nil
	}
	return []string{fmt.Sprintf("the running binary %s is not the plane binary %s; run %s wave next so that the build, the pin and the loop use one binary", running, plane, plane)}
}

func sameBinaryPath(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func (p wavePlan) goalPath() string {
	return filepath.Join(p.plane.evolveDir, "waves", fmt.Sprintf("wave-%d-goal.md", p.number))
}

func planWave(plan wavePlan, e *waveNextEnvelope, stdout io.Writer) int {
	for _, s := range plan.steps {
		e.Steps = append(e.Steps, waveStepResult{Name: s.String(), Verb: s.verb, Detail: waveDryRunDetail})
	}
	if plan.args.json {
		e.Goal = plan.goal
		return 0
	}
	fmt.Fprintf(stdout, "wave: dry run for wave %d; the goal is %d bytes:\n%s", plan.number, len(plan.goal), plan.goal)
	fmt.Fprintln(stdout, "wave: dry run; the steps, in order:")
	for i, s := range plan.steps {
		fmt.Fprintf(stdout, "  %d. %s\n", i+1, s)
	}
	return 0
}

func writeWaveJSON(v any, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "%sencode: %v\n", wavePrefix, err)
		return exitIO
	}
	return 0
}

func writeWaveWarnings(warnings []string, stderr io.Writer) {
	for _, w := range warnings {
		fmt.Fprintf(stderr, "%sWARN: %s\n", wavePrefix, w)
	}
}

func executeWave(env waveEnv, plan wavePlan, e *waveNextEnvelope, stdout, stderr io.Writer) int {
	if _, err := plan.plane.store.WriteGoal(plan.number, plan.goal); err != nil {
		return failWave(e, err, stderr)
	}
	stepOut := stdout
	if plan.args.json {
		stepOut = stderr
	}
	rec := &waveStepRecorder{next: env.dispatch, steps: plan.steps}
	rc := runBoundarySteps(rec.dispatch, plan.steps, stepOut, stderr)
	e.Steps = append(e.Steps, rec.results...)
	if rc != 0 {
		failed := rec.results[len(rec.results)-1].Name
		e.FailedStep, e.Error = failed, fmt.Sprintf("step failed with exit code %d: %s", rc, failed)
		return rc
	}
	e.Launched = true
	pid, err := recordWave(env, &plan)
	e.PID, e.Warnings = pid, plan.warnings
	if err != nil {
		e.Error = "loop launched, wave record NOT written: " + err.Error()
		fmt.Fprintf(stderr, "%s%s\n", wavePrefix, e.Error)
		return exitIO
	}
	e.Recorded = true
	return 0
}

func recordWave(env waveEnv, plan *wavePlan) (int, error) {
	runID := plan.args.boundary.runID
	logPath := boundaryLoopLogPath(plan.plane.root, runID)
	pid, err := readWriterPID(logPath)
	if err != nil {
		plan.warnings = append(plan.warnings, err.Error())
	}
	head, err := env.mainHead(plan.plane.root)
	if err != nil {
		plan.warnings = append(plan.warnings, fmt.Sprintf("read the main SHA: %v", err))
	}
	if err := closeLastWave(env, plan); err != nil {
		plan.warnings = append(plan.warnings, err.Error())
	}
	if err := plan.plane.store.RemoveNotes(plan.notes); err != nil {
		plan.warnings = append(plan.warnings, err.Error())
	}
	rec := wave.Record{
		Number: plan.number, RunID: runID, GoalHash: goalhash.Compute(plan.goal), GoalPath: plan.goalPath(), GoalBytes: len(plan.goal),
		MainSHA: head, CycleFloor: plan.floor, PID: pid, LogPath: logPath, StartedAt: env.now(),
	}
	for attempt := 0; attempt < waveRecordAttempts; attempt++ {
		if err = plan.plane.store.Save(rec); err == nil {
			return pid, nil
		}
	}
	return pid, err
}

func readWriterPID(logPath string) (int, error) {
	b, err := os.ReadFile(logPath + gcpolicy.LogWriterPIDSuffix)
	if err != nil {
		return 0, fmt.Errorf("read the loop pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("parse the loop pid in %s: %w", logPath+gcpolicy.LogWriterPIDSuffix, err)
	}
	return pid, nil
}

func closeLastWave(env waveEnv, plan *wavePlan) error {
	if plan.last == nil {
		return nil
	}
	closed := *plan.last
	ended := env.now()
	if info, err := os.Stat(closed.LogPath); err == nil && closed.LogPath != "" {
		ended = info.ModTime().UTC()
	}
	closed.EndedAt, closed.Outcome = &ended, plan.facts.Outcome()
	return plan.plane.store.Save(closed)
}
