package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const (
	waveTestStanding = "Work the highest-weight lane-eligible inbox items end to end."
	waveTestHead     = "bbbbbbbbbbbb"
	waveTestPID      = 4242
)

var waveTestClock = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

type wavePlane struct {
	t    *testing.T
	root string
}

func newWavePlane(t *testing.T) wavePlane {
	t.Helper()
	p := wavePlane{t: t, root: t.TempDir()}
	p.write(".evolve/wave-goal.md", waveTestStanding+"\n")
	p.write(".evolve/state.json", `{"lastCycleNumber":1836}`)
	return p
}

func (p wavePlane) evolveDir() string { return filepath.Join(p.root, ".evolve") }

func (p wavePlane) store() wave.Store { return wave.NewStore(p.evolveDir()) }

func (p wavePlane) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p wavePlane) loopLog(runID string, pid int) string {
	p.t.Helper()
	rel := filepath.Join(".evolve", "logs", runID, "loop.log")
	p.write(rel, "running\n")
	p.write(rel+gcpolicy.LogWriterPIDSuffix, fmt.Sprintf("%d\n", pid))
	return filepath.Join(p.root, rel)
}

func (p wavePlane) seedWave81() {
	p.t.Helper()
	p.write(".evolve/runs/cycle-1835/run.json", `{"cycle_id":1835,"phase":"ship"}`)
	p.write(".evolve/runs/cycle-1835/signals.ndjson",
		`{"schema_version":"1.0","seq":1,"pid":1,"ts":"2026-10-08T11:00:00Z","module":"orchestrator","origin":"x","kind":"cycle.sealed","severity":"INFO","reason":"r","fields":{"final_verdict":"WARN"}}`+"\n")
	p.write(".evolve/landing/cycle-1835.json", `{"cycle":1835,"status":"complete"}`)
	p.write(".evolve/runs/cycle-1836/run.json", `{"cycle_id":1836,"phase":"scout"}`)
	logPath := filepath.Join(p.root, "old-loop.log")
	p.write("old-loop.log", "done\n")
	ended := waveTestClock.Add(-time.Hour)
	if err := os.Chtimes(logPath, ended, ended); err != nil {
		p.t.Fatal(err)
	}
	rec := wave.Record{Number: 81, RunID: "20261008T080000Z", CycleFloor: 1834, MainSHA: "aaaaaaaaaaaa", LogPath: logPath, StartedAt: waveTestClock.Add(-4 * time.Hour)}
	if err := p.store().Save(rec); err != nil {
		p.t.Fatal(err)
	}
	if _, err := p.store().AddNote("queued note", waveTestClock.Add(-time.Minute)); err != nil {
		p.t.Fatal(err)
	}
}

type fakeWaveVerbs struct {
	fakeBoundaryVerbs
	writePID bool
	pidText  string
	onLoop   func()
}

func (f *fakeWaveVerbs) dispatch(verb string, args []string, stdout, stderr io.Writer) int {
	rc := f.fakeBoundaryVerbs.dispatch(verb, args, stdout, stderr)
	if verb == "loop" && rc == 0 && f.writePID {
		logPath := boundaryFlagValue(args, "--log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
		text := fmt.Sprintf("%d\n", waveTestPID)
		if f.pidText != "" {
			text = f.pidText
		}
		_ = os.WriteFile(logPath+gcpolicy.LogWriterPIDSuffix, []byte(text), 0o644)
	}
	if verb == "loop" && f.onLoop != nil {
		f.onLoop()
	}
	return rc
}

type waveHarness struct {
	rootErr error
	exeErr  error
	prsErr  error
	exe     string
	started map[int]time.Time
	verbs   *fakeWaveVerbs
	since   []string
	headErr error
	alive   map[int]bool
	sleeps  int
	onSleep func(n int)
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func newWaveHarness() *waveHarness {
	return &waveHarness{verbs: &fakeWaveVerbs{writePID: true}, alive: map[int]bool{}, started: map[int]time.Time{}}
}

func (h *waveHarness) env(root string) waveEnv {
	exe := h.exe
	if exe == "" {
		exe = filepath.Join(root, "go", "bin", "evolve")
	}
	return waveEnv{
		resolveRoot: func(projectRoot string, stderr io.Writer) (string, error) {
			if h.rootErr != nil {
				return "", h.rootErr
			}
			return loopStopRoot(projectRoot, stderr)
		},
		executable: func() (string, error) { return exe, h.exeErr },
		pidStarted: func(pid int) (time.Time, bool) {
			at, ok := h.started[pid]
			return at, ok
		},
		dispatch: h.verbs.dispatch,
		now:      func() time.Time { return waveTestClock },
		sleep: func(time.Duration) {
			h.sleeps++
			if h.onSleep != nil {
				h.onSleep(h.sleeps)
			}
		},
		pidAlive: func(pid int) bool { return h.alive[pid] },
		mainHead: func(string) (string, error) { return waveTestHead, h.headErr },
		mergedPRs: func(_, since string) ([]string, error) {
			h.since = append(h.since, since)
			return []string{"812"}, h.prsErr
		},
	}
}

func (h *waveHarness) run(args ...string) int {
	h.stdout.Reset()
	h.stderr.Reset()
	return runWaveWith(h.env(boundaryFlagValue(args, "--project-root")), args, &h.stdout, &h.stderr)
}

type waveEnvelope struct {
	Schema     string `json:"schema"`
	Launched   bool   `json:"launched"`
	Recorded   bool   `json:"recorded"`
	Refused    string `json:"refused"`
	FailedStep string `json:"failed_step"`
	Error      string `json:"error"`
	Wave       int    `json:"wave"`
	RunID      string `json:"run_id"`
	PID        int    `json:"pid"`
	GoalPath   string `json:"goal_path"`
	GoalBytes  int    `json:"goal_bytes"`
	Goal       string `json:"goal"`
	DryRun     bool   `json:"dry_run"`
	Steps      []struct {
		Name   string `json:"name"`
		Verb   string `json:"verb"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	} `json:"steps"`
	Warnings []string `json:"warnings"`
}

func decodeWaveEnvelope(t *testing.T, raw []byte) waveEnvelope {
	t.Helper()
	var e waveEnvelope
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("stdout is not one JSON envelope: %v\n%s", err, raw)
	}
	return e
}

func waveStepVerbs(e waveEnvelope) []string {
	out := make([]string, 0, len(e.Steps))
	for _, s := range e.Steps {
		out = append(out, s.Verb)
	}
	return out
}

var waveFullSteps = []string{"checkpoint", "loop-stop", "pr", "sync-main", "gc", waveBuildVerb, "reset-sha", "doctor", "loop-stop", boundaryLogVerb, "loop"}

func TestWaveNext_DryRunComposesTheGoalAndWritesNothing(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--json", "--merge", "816,814", "--note", "extra note", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if e.Wave != 82 || !e.DryRun || e.GoalBytes != len(e.Goal) || e.PID != 0 {
		t.Errorf("envelope = wave %d dry_run %t goal_bytes %d (goal %d) pid %d; want wave 82, a dry run, matching bytes, no pid", e.Wave, e.DryRun, e.GoalBytes, len(e.Goal), e.PID)
	}
	if got := waveStepVerbs(e); !slices.Equal(got, waveFullSteps) {
		t.Errorf("planned steps = %q, want %q", got, waveFullSteps)
	}
	if launch := e.Steps[len(e.Steps)-1].Name; strings.Contains(launch, waveTestStanding) || !strings.Contains(launch, fmt.Sprintf("--goal-text <goal: %d bytes>", len(strings.TrimSpace(e.Goal)))) {
		t.Errorf("launch step name = %q, want the goal shown by its size only", launch)
	}
	if build := e.Steps[5]; build.Name != "make -C "+filepath.Join(p.root, "go")+" build" || build.OK || build.Detail != "planned" {
		t.Errorf("planned build step = %+v, want the make command, not run", build)
	}
	for _, want := range []string{"Wave 82.", waveTestStanding, "Wave 81 facts (run 20261008T080000Z): 1 of 2 cycles shipped.",
		"- Cycle 1835: final verdict WARN. It shipped.", "#812, #814, #816", "- queued note", "- extra note"} {
		if !strings.Contains(e.Goal, want) {
			t.Errorf("goal does not contain %q:\n%s", want, e.Goal)
		}
	}
	if len(h.verbs.calls) != 0 {
		t.Errorf("a dry run dispatched %v", h.verbs.verbs())
	}
	if !slices.Equal(h.since, []string{"aaaaaaaaaaaa"}) {
		t.Errorf("merged PRs were read since %q, want the last wave's main SHA", h.since)
	}
	records, _ := p.store().Records()
	notes, _ := p.store().Notes()
	if len(records) != 1 || records[0].Outcome != "" || len(notes) != 1 {
		t.Errorf("a dry run changed state: %d records (wave 81 outcome %q), %d notes", len(records), records[0].Outcome, len(notes))
	}
	if _, err := os.Stat(e.GoalPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a dry run wrote the goal file %s (stat err %v)", e.GoalPath, err)
	}
}

func TestWaveNext_LaunchRecordsTheWaveAndConsumesTheNotes(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	goal, err := os.ReadFile(e.GoalPath)
	if err != nil || len(goal) != e.GoalBytes {
		t.Fatalf("goal file %s: %d bytes, err %v; want %d bytes", e.GoalPath, len(goal), err, e.GoalBytes)
	}
	wantSteps := slices.DeleteFunc(slices.Clone(waveFullSteps), func(v string) bool { return v == "pr" })
	if got := h.verbs.verbs(); !slices.Equal(got, wantSteps) {
		t.Errorf("dispatched %q, want %q", got, wantSteps)
	}
	for _, s := range e.Steps {
		if !s.OK || s.Detail != "rc=0" {
			t.Errorf("step %q = ok %t detail %q, want ok rc=0", s.Name, s.OK, s.Detail)
		}
	}
	launch := h.verbs.calls[len(h.verbs.calls)-1].args
	if boundaryFlagValue(launch, "--goal-text") != strings.TrimSpace(string(goal)) || boundaryFlagValue(launch, "--max-cycles") != "1" {
		t.Errorf("launch args %q: want the goal file text and the policy default --max-cycles 1", launch)
	}
	if i := slices.IndexFunc(h.verbs.calls, func(c fakeBoundaryCall) bool { return c.verb == "doctor" }); i < 0 || !slices.Equal(h.verbs.calls[i].args, []string{"live", "claude-tmux"}) {
		t.Errorf("health check at %d in %+v, want doctor live claude-tmux", i, h.verbs.calls)
	}
	records, _ := p.store().Records()
	if len(records) != 2 {
		t.Fatalf("records = %+v, want waves 81 and 82", records)
	}
	closed, opened := records[0], records[1]
	if closed.Outcome != "1 of 2 cycles shipped" || closed.EndedAt == nil || !closed.EndedAt.Equal(waveTestClock.Add(-time.Hour)) {
		t.Errorf("wave 81 = outcome %q ended %v; want 1 of 2 shipped, ended at its log's last write", closed.Outcome, closed.EndedAt)
	}
	want := wave.Record{
		Number: 82, RunID: e.RunID, GoalHash: goalhash.Compute(string(goal)), GoalPath: e.GoalPath, GoalBytes: e.GoalBytes,
		MainSHA: waveTestHead, CycleFloor: 1836, PID: waveTestPID, LogPath: boundaryLoopLogPath(p.root, e.RunID), StartedAt: waveTestClock,
	}
	if !waveRecordsEqual(opened, want) || e.PID != waveTestPID || e.RunID != gcpolicy.LogRunID(waveTestClock) {
		t.Errorf("wave 82 record = %+v\nwant %+v (envelope pid %d run %q)", opened, want, e.PID, e.RunID)
	}
	if notes, _ := p.store().Notes(); len(notes) != 0 {
		t.Errorf("notes after the launch = %+v, want none", notes)
	}
}

func waveRecordsEqual(a, b wave.Record) bool {
	return a.Number == b.Number && a.RunID == b.RunID && a.GoalHash == b.GoalHash && a.GoalPath == b.GoalPath &&
		a.GoalBytes == b.GoalBytes && a.MainSHA == b.MainSHA && a.CycleFloor == b.CycleFloor && a.PID == b.PID &&
		a.LogPath == b.LogPath && a.StartedAt.Equal(b.StartedAt) && a.EndedAt == nil && a.Outcome == ""
}

func TestWaveNext_FailedStepRecordsNothingAndKeepsTheNotes(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()
	h.verbs.failAt, h.verbs.code = 3, 7

	rc := h.run("next", "--json", "--project-root", p.root)

	if rc != 7 {
		t.Fatalf("rc = %d, want the failed step's 7", rc)
	}
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if len(e.Steps) != 3 || e.Steps[2].OK || e.Steps[2].Detail != "rc=7" || !strings.Contains(e.Steps[2].Name, "sync-main") {
		t.Errorf("steps = %+v, want 3 with sync-main failed rc=7", e.Steps)
	}
	records, _ := p.store().Records()
	notes, _ := p.store().Notes()
	if len(records) != 1 || records[0].Outcome != "" || len(notes) != 1 {
		t.Errorf("a failed boundary changed state: %d records (wave 81 outcome %q), %d notes", len(records), records[0].Outcome, len(notes))
	}
}

func TestWaveNext_RefusesWhileALoopIsLive(t *testing.T) {
	p := newWavePlane(t)
	runDir := filepath.Join(p.evolveDir(), "runs", "cycle-1837")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r-live", OwnerPID: os.Getpid()}, waveTestClock); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	rc := h.run("next", "--number", "82", "--project-root", p.root)
	dryRC := h.run("next", "--number", "82", "--dry-run", "--json", "--project-root", p.root)

	if rc != exitRefused || len(h.verbs.calls) != 0 {
		t.Errorf("rc = %d with %d steps, want %d and no step", rc, len(h.verbs.calls), exitRefused)
	}
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if dryRC != 0 || !slices.ContainsFunc(e.Warnings, func(w string) bool { return strings.Contains(w, "r-live") }) {
		t.Errorf("dry run rc = %d warnings %q, want 0 and a warning that names r-live", dryRC, e.Warnings)
	}
}

func TestWaveNext_NumberSeedAndRefusal(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	seeded := h.run("next", "--number", "82", "--json", "--project-root", p.root)
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	again := h.run("next", "--number", "82", "--project-root", p.root)

	if seeded != 0 || e.Wave != 82 || !slices.ContainsFunc(e.Warnings, func(w string) bool { return strings.Contains(w, "no earlier wave") }) {
		t.Errorf("seeded rc = %d wave %d warnings %q, want 0, wave 82 and a warning about no earlier wave", seeded, e.Wave, e.Warnings)
	}
	if again != exitRefused || !strings.Contains(h.stderr.String(), "not more than the last recorded wave") {
		t.Errorf("repeat rc = %d stderr %q, want %d naming the number rule", again, h.stderr.String(), exitRefused)
	}
}

func TestWaveNext_PolicyAndFlagsSetCyclesAndHealthDrivers(t *testing.T) {
	cases := []struct {
		name, policy string
		args         []string
		wantCycles   string
		wantDoctors  int
	}{
		{"policy max_cycles and no driver", `{"wave":{"max_cycles":3,"health_drivers":[]}}`, nil, "3", 0},
		{"the flag replaces the policy", `{"wave":{"max_cycles":3,"health_drivers":["agy-tmux","claude-tmux"]}}`, []string{"--max-cycles", "2"}, "2", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			p.write(".evolve/policy.json", c.policy)
			h := newWaveHarness()

			rc := h.run(append([]string{"next", "--project-root", p.root}, c.args...)...)

			if rc != 0 {
				t.Fatalf("rc = %d; stderr:\n%s", rc, h.stderr.String())
			}
			doctors := 0
			for _, call := range h.verbs.calls {
				if call.verb == "doctor" {
					doctors++
				}
			}
			launch := h.verbs.calls[len(h.verbs.calls)-1].args
			if got := boundaryFlagValue(launch, "--max-cycles"); got != c.wantCycles || doctors != c.wantDoctors {
				t.Errorf("--max-cycles %q with %d health checks, want %q with %d", got, doctors, c.wantCycles, c.wantDoctors)
			}
		})
	}
}

func TestWaveNext_InputErrors(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		prep   func(p wavePlane)
		wantRC int
	}{
		{"unknown flag", []string{"--bogus"}, nil, exitUsage},
		{"stray operand", []string{"now"}, nil, exitUsage},
		{"zero max cycles", []string{"--max-cycles", "0"}, nil, exitUsage},
		{"bad merge list", []string{"--merge", "12,x"}, nil, exitUsage},
		{"bad number", []string{"--number", "-3"}, nil, exitUsage},
		{"blank note", []string{"--note", " "}, nil, exitUsage},
		{"no standing goal", nil, func(p wavePlane) { _ = os.Remove(filepath.Join(p.evolveDir(), "wave-goal.md")) }, exitIO},
		{"blank standing goal", nil, func(p wavePlane) { p.write(".evolve/wave-goal.md", "\n") }, exitIO},
		{"malformed policy", nil, func(p wavePlane) { p.write(".evolve/policy.json", "{") }, exitIO},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			if c.prep != nil {
				c.prep(p)
			}
			h := newWaveHarness()

			rc := h.run(append([]string{"next", "--project-root", p.root}, c.args...)...)

			if rc != c.wantRC || len(h.verbs.calls) != 0 || h.stderr.Len() == 0 {
				t.Errorf("rc = %d with %d steps, stderr %q; want %d, no step, a reason", rc, len(h.verbs.calls), h.stderr.String(), c.wantRC)
			}
		})
	}
}

func TestWaveNext_TextOutputEndsWithTheLaunchedWave(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	rc := h.run("next", "--project-root", p.root)

	out := h.stdout.String()
	if rc != 0 || !strings.Contains(out, "boundary: step 1/10: evolve checkpoint save --all") ||
		!strings.HasSuffix(out, fmt.Sprintf("wave: launched wave 1: run %s, pid %d, goal %d bytes\n", gcpolicy.LogRunID(waveTestClock), waveTestPID, len(waveFirstGoal()))) {
		t.Errorf("rc = %d stdout:\n%s", rc, out)
	}
}

func waveFirstGoal() string {
	return wave.Compose(wave.GoalInput{Next: 1, Standing: waveTestStanding})
}

func TestWaveStatus_ReportsTheCurrentWaveFromState(t *testing.T) {
	p := newWavePlane(t)
	p.seedWave81()
	if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1836, PID: 777, LogPath: p.loopLog("r82", 777), StartedAt: waveTestClock}); err != nil {
		t.Fatal(err)
	}
	if err := p.store().Save(wave.Record{Number: 81, RunID: "20261008T080000Z", CycleFloor: 1834, Outcome: "1 of 2 cycles shipped"}); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1837/run.json", `{"cycle_id":1837,"phase":"build"}`)
	h := newWaveHarness()
	h.alive[777] = true

	rc := h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil || rc != 0 {
		t.Fatalf("rc = %d, decode %v:\n%s", rc, err, h.stdout.String())
	}
	want := waveStatusReport{
		Wave: 82, RunID: "r82", PID: 777, PIDAlive: true, Outcome: "0 of 1 cycles shipped",
		Cycles:   []wave.Cycle{{ID: 1837, Phase: "build"}},
		LastWave: &waveSummary{Wave: 81, Outcome: "1 of 2 cycles shipped"}, NotesQueued: 1,
	}
	if got.Wave != want.Wave || got.RunID != want.RunID || got.PID != want.PID || !got.PIDAlive || got.LoopLive ||
		got.Outcome != want.Outcome || !slices.Equal(got.Cycles, want.Cycles) || got.LastWave == nil || *got.LastWave != *want.LastWave || got.NotesQueued != 1 {
		t.Errorf("status = %+v\nwant %+v", got, want)
	}
}

func TestWaveStatus_TextAndNoRecord(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	empty := h.run("status", "--project-root", p.root)
	emptyOut := h.stdout.String()
	p.seedWave81()
	full := h.run("status", "--project-root", p.root)

	if empty != 0 || emptyOut != "wave: no wave is recorded\n" {
		t.Errorf("no record: rc = %d stdout %q", empty, emptyOut)
	}
	for _, want := range []string{"wave:    81 run 20261008T080000Z", "loop:    live=false pid=0 alive=false", "cycle 1835: verdict WARN, shipped", "cycle 1836: phase scout"} {
		if full != 0 || !strings.Contains(h.stdout.String(), want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", full, want, h.stdout.String())
		}
	}
}

func TestWaveStatus_BoundsTheCycleList(t *testing.T) {
	p := newWavePlane(t)
	for id := 1; id <= waveStatusMaxCycles+5; id++ {
		p.write(fmt.Sprintf(".evolve/runs/cycle-%d/run.json", id), fmt.Sprintf(`{"cycle_id":%d,"phase":"scout"}`, id))
	}
	if err := p.store().Save(wave.Record{Number: 1, RunID: "r1"}); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Cycles) != waveStatusMaxCycles || got.Cycles[0].ID != 6 || got.CyclesOmitted != 5 || got.Outcome != "0 of 25 cycles shipped" {
		t.Errorf("cycles = %d from %d, omitted %d, outcome %q; want the newest %d, 5 omitted, the outcome over all 25",
			len(got.Cycles), got.Cycles[0].ID, got.CyclesOmitted, got.Outcome, waveStatusMaxCycles)
	}
}

func TestWaveWatch_StreamsStateEventsUntilTheLoopExits(t *testing.T) {
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1836, PID: 777, LogPath: p.loopLog("r82", 777), StartedAt: waveTestClock}); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-1837/run.json", `{"cycle_id":1837,"phase":"scout"}`)
	h := newWaveHarness()
	h.alive[777] = true
	h.onSleep = func(n int) {
		switch n {
		case 1:
			p.write(".evolve/runs/cycle-1837/run.json", `{"cycle_id":1837,"phase":"build"}`)
		case 2:
			p.write(".evolve/runs/cycle-1837/signals.ndjson",
				`{"schema_version":"1.0","seq":1,"pid":1,"ts":"2026-10-08T12:30:00Z","module":"orchestrator","origin":"x","kind":"cycle.sealed","severity":"INFO","reason":"r","fields":{"final_verdict":"PASS"}}`+"\n")
			p.write(".evolve/landing/cycle-1837.json", `{"cycle":1837,"status":"complete"}`)
		case 3:
			h.alive[777] = false
		}
	}

	rc := h.run("watch", "--project-root", p.root)

	want := "wave 82: cycle 1837: phase scout\nwave 82: cycle 1837: phase build\nwave 82: cycle 1837: sealed PASS\nwave 82: cycle 1837: shipped\nwave 82: loop: exit\n"
	if rc != 0 || h.stdout.String() != want || h.sleeps != 3 {
		t.Errorf("rc = %d after %d polls; stdout:\n%s\nwant:\n%s", rc, h.sleeps, h.stdout.String(), want)
	}
}

func TestWaveWatch_JSONLinesAndNoRecord(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	none := h.run("watch", "--project-root", p.root)
	noneErr := h.stderr.String()
	if err := p.store().Save(wave.Record{Number: 3, RunID: "r3", PID: 9}); err != nil {
		t.Fatal(err)
	}
	ended := h.run("watch", "--json", "--project-root", p.root)

	if none != exitIO || !strings.Contains(noneErr, "no wave is recorded") {
		t.Errorf("no record: rc = %d stderr %q, want %d naming the missing record", none, noneErr, exitIO)
	}
	var e wave.Event
	if err := json.Unmarshal(h.stdout.Bytes(), &e); err != nil || ended != 0 || e.Kind != wave.EventLoopExit {
		t.Errorf("rc = %d, stdout %q (decode %v), want one loop-exit JSON line", ended, h.stdout.String(), err)
	}
}

func TestWaveNote_AddListClear(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	addRC := h.run("note", "add", "watch", "PR", "814", "--project-root", p.root)
	listRC := h.run("note", "list", "--project-root", p.root)
	listed := h.stdout.String()
	clearRC := h.run("note", "clear", "--project-root", p.root)
	notes, _ := p.store().Notes()

	if addRC != 0 || listRC != 0 || clearRC != 0 || !strings.HasSuffix(listed, ": watch PR 814\n") || len(notes) != 0 {
		t.Errorf("add %d list %d (%q) clear %d, %d notes left; want 0, 0, the note, 0, none", addRC, listRC, listed, clearRC, len(notes))
	}
}

func TestWave_UsageErrors(t *testing.T) {
	p := newWavePlane(t)
	for _, args := range [][]string{nil, {"launch"}, {"note"}, {"note", "add"}, {"note", "drop"}, {"status", "extra"}, {"watch", "--bogus"}} {
		h := newWaveHarness()
		if rc := h.run(append(args, "--project-root", p.root)...); rc != exitUsage || !strings.Contains(h.stderr.String(), "usage: evolve wave") {
			t.Errorf("wave %q: rc = %d stderr %q, want %d with the usage", args, rc, h.stderr.String(), exitUsage)
		}
	}
}

func TestDispatchWaveVerb_RunsTheBuildAndFallsBackToTheBoundary(t *testing.T) {
	var gotName, gotDir string
	var gotArgs []string
	run := func(_ context.Context, name, dir string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		gotName, gotDir, gotArgs = name, dir, args
		return 3, nil
	}
	dispatch := dispatchWaveVerb(run)
	var out, errb bytes.Buffer

	buildRC := dispatch(waveBuildVerb, []string{"--project-root", "/plane"}, &out, &errb)
	unknownRC := dispatch("no-such-verb", nil, &out, &errb)

	if buildRC != 3 || gotName != "make" || gotDir != "" || !slices.Equal(gotArgs, []string{"-C", filepath.Join("/plane", "go"), "build"}) {
		t.Errorf("build rc = %d ran %q %q in %q; want rc 3 from make -C /plane/go build", buildRC, gotName, gotArgs, gotDir)
	}
	if unknownRC != exitIO || !strings.Contains(errb.String(), `no handler for step "no-such-verb"`) {
		t.Errorf("unknown verb rc = %d stderr %q, want the boundary dispatcher's refusal", unknownRC, errb.String())
	}
}

func TestRegistry_WaveIsAPublicVerb(t *testing.T) {
	cmd := lookupCommand("wave")
	if cmd == nil {
		t.Fatal(`lookupCommand("wave") = nil, want the public wave verb`)
	}
	var out, errb bytes.Buffer

	rc := cmd.Run(nil, strings.NewReader(""), &out, &errb)

	if rc != exitUsage || !strings.Contains(errb.String(), "usage: evolve wave next") || !strings.Contains(usage, "( wave next") {
		t.Errorf("wave with no sub-verb: rc = %d stderr %q, main usage lists wave: %t", rc, errb.String(), strings.Contains(usage, "( wave next"))
	}
}

func TestParseMergedPRs(t *testing.T) {
	log := "Merge pull request #816 from x/y\nfix: a commit\n  Merge pull request #12 from z\nMerge branch 'main'\n"

	got := parseMergedPRs(log)

	if !slices.Equal(got, []string{"816", "12"}) {
		t.Errorf("parseMergedPRs = %q, want [816 12]", got)
	}
}

func TestWaveNext_HistoryHoldsTheLastKWavesBeforeTheLastOne(t *testing.T) {
	p := newWavePlane(t)
	p.write(".evolve/policy.json", `{"wave":{"history_k":2}}`)
	for n := 76; n <= 81; n++ {
		if err := p.store().Save(wave.Record{Number: n, RunID: fmt.Sprintf("r%d", n), CycleFloor: 1836, Outcome: fmt.Sprintf("outcome %d", n)}); err != nil {
			t.Fatal(err)
		}
	}
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	_, history, _ := strings.Cut(e.Goal, "\n\nEarlier waves:\n")
	want := "- Wave 79 (run r79): outcome 79.\n- Wave 80 (run r80): outcome 80.\n"
	if rc != 0 || history != want || !strings.Contains(e.Goal, "Wave 81 facts (run r81)") {
		t.Errorf("rc = %d; history part = %q, want %q after the wave 81 facts", rc, history, want)
	}
}
