package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type failingWriter struct {
	okWrites int
	writes   int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.okWrites {
		return 0, errors.New("broken pipe")
	}
	return len(p), nil
}

func hasWarning(e waveEnvelope, want string) bool {
	return slices.ContainsFunc(e.Warnings, func(w string) bool { return strings.Contains(w, want) })
}

func TestWave_ARootThatCannotBeFoundExitsIO(t *testing.T) {
	for _, args := range [][]string{{"next", "--json"}, {"note", "list"}, {"status"}, {"watch"}} {
		h := newWaveHarness()
		h.rootErr = errors.New("getwd: no such file")

		rc := h.run(args...)

		if rc != exitIO || !strings.Contains(h.stderr.String(), "getwd: no such file") {
			t.Errorf("wave %q: rc = %d stderr %q, want %d naming the root error", args, rc, h.stderr.String(), exitIO)
		}
		if args[0] == "next" {
			if e := decodeWaveEnvelope(t, h.stdout.Bytes()); !strings.Contains(e.Error, "getwd") || e.Launched {
				t.Errorf("envelope = %+v, want the root error and no launch", e)
			}
		}
	}
}

func TestWaveNote_StoreErrorsExitIO(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()
	first := h.run("note", "add", "one", "--project-root", p.root)

	second := h.run("note", "add", "two", "--project-root", p.root)
	secondErr := h.stderr.String()
	if first != 0 || second != exitIO || !strings.Contains(secondErr, "add note") {
		t.Errorf("same-clock adds: rc %d then %d, stderr %q; want 0 then %d naming the note", first, second, secondErr, exitIO)
	}
	notes := filepath.Join(p.evolveDir(), "waves", "notes")
	if err := os.RemoveAll(notes); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/waves/notes", "not a dir")
	for _, args := range [][]string{{"note", "list"}, {"note", "list", "--json"}, {"note", "clear"}} {
		if rc := h.run(append(args, "--project-root", p.root)...); rc != exitIO || !strings.Contains(h.stderr.String(), "list notes") {
			t.Errorf("wave %q: rc = %d stderr %q, want %d naming the notes dir", args, rc, h.stderr.String(), exitIO)
		}
	}
}

func TestDispatchWaveVerb_BuildArgumentAndRunnerErrors(t *testing.T) {
	run := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, errors.New("make: not found")
	}
	dispatch := dispatchWaveVerb(run)
	var out, errb bytes.Buffer

	usageRC := dispatch(waveBuildVerb, nil, &out, &errb)
	usageErr := errb.String()
	errb.Reset()
	runRC := dispatch(waveBuildVerb, []string{"--project-root", "/plane"}, &out, &errb)

	if usageRC != exitUsage || !strings.Contains(usageErr, "wave-build needs --project-root") {
		t.Errorf("no root: rc = %d stderr %q, want %d and the reason", usageRC, usageErr, exitUsage)
	}
	if runRC != exitIO || !strings.Contains(errb.String(), "build: make: not found") {
		t.Errorf("runner error: rc = %d stderr %q, want %d and the runner error", runRC, errb.String(), exitIO)
	}
}

func TestWaveNext_WarningsFromItsInputs(t *testing.T) {
	cases := []struct {
		name string
		prep func(p wavePlane, h *waveHarness)
		want string
	}{
		{"merged PRs cannot be read", func(p wavePlane, h *waveHarness) {
			p.seedWave81()
			h.prsErr = errors.New("list the merges since aaa: bad revision")
		}, "list the merges since aaa: bad revision"},
		{"the running binary cannot be found", func(_ wavePlane, h *waveHarness) { h.exeErr = errors.New("no executable") }, "cannot find the running binary: no executable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			h := newWaveHarness()
			c.prep(p, h)

			rc := h.run("next", "--dry-run", "--json", "--number", "90", "--project-root", p.root)

			if e := decodeWaveEnvelope(t, h.stdout.Bytes()); rc != 0 || !hasWarning(e, c.want) {
				t.Errorf("rc = %d warnings %q, want 0 and %q", rc, e.Warnings, c.want)
			}
		})
	}
}

func TestWaveNext_ASymlinkToThePlaneBinaryIsThePlaneBinary(t *testing.T) {
	p := newWavePlane(t)
	p.write("go/bin/evolve", "binary")
	link := filepath.Join(t.TempDir(), "evolve")
	if err := os.Symlink(filepath.Join(p.root, "go", "bin", "evolve"), link); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()
	h.exe = link

	h.run("next", "--dry-run", "--json", "--project-root", p.root)

	if e := decodeWaveEnvelope(t, h.stdout.Bytes()); hasWarning(e, "is not the plane binary") {
		t.Errorf("warnings = %q, want no plane-binary warning for a symlink to it", e.Warnings)
	}
}

func TestWaveNext_TextDryRunPrintsTheGoalAndTheSteps(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--project-root", p.root)

	goal := waveFirstGoal()
	out := h.stdout.String()
	want := fmt.Sprintf("wave: dry run for wave 1; the goal is %d bytes:\n%swave: dry run; the steps, in order:\n  1. evolve checkpoint save --all", len(goal), goal)
	if rc != 0 || !strings.HasPrefix(out, want) || !strings.Contains(out, "\n  10. evolve loop --detach") || !strings.Contains(h.stderr.String(), "WARN: no earlier wave is recorded") {
		t.Errorf("rc = %d stdout:\n%s\nstderr: %s", rc, out, h.stderr.String())
	}
}

func TestWaveNext_AnEnvelopeThatCannotBeWrittenExitsIO(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()
	var errb bytes.Buffer

	rc := runWaveWith(h.env(p.root), []string{"next", "--dry-run", "--json", "--project-root", p.root}, &failingWriter{}, &errb)

	if rc != exitIO || !strings.Contains(errb.String(), "encode: broken pipe") {
		t.Errorf("rc = %d stderr %q, want %d and the encode error", rc, errb.String(), exitIO)
	}
}

func TestWaveNext_AGoalFileThatCannotBeWrittenRunsNoStep(t *testing.T) {
	p := newWavePlane(t)
	p.write(".evolve/waves/wave-1-goal.md/blocked", "x")
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if rc != exitIO || !strings.Contains(e.Error, "write wave 1 goal") || e.Launched || len(e.Steps) != 0 || len(h.verbs.calls) != 0 {
		t.Errorf("rc = %d envelope = %+v with %d steps run; want %d, the goal error, no launch, no step", rc, e, len(h.verbs.calls), exitIO)
	}
}

func TestWaveNext_RecordingProblemsAreWarningsAfterALaunch(t *testing.T) {
	cases := []struct {
		name string
		prep func(p wavePlane, h *waveHarness)
		want string
		pid  int
	}{
		{"no writer pid file", func(_ wavePlane, h *waveHarness) { h.verbs.writePID = false }, "read the loop pid", 0},
		{"a bad writer pid file", func(_ wavePlane, h *waveHarness) { h.verbs.pidText = "abc" }, "parse the loop pid", 0},
		{"no main SHA", func(_ wavePlane, h *waveHarness) { h.headErr = errors.New("no HEAD") }, "read the main SHA: no HEAD", waveTestPID},
		{"the last record cannot be closed", func(p wavePlane, h *waveHarness) {
			h.verbs.onLoop = func() {
				path := filepath.Join(p.evolveDir(), "waves", "wave-81.json")
				_ = os.Remove(path)
				_ = os.MkdirAll(filepath.Join(path, "blocked"), 0o755)
			}
		}, "write wave record 81", waveTestPID},
		{"a note cannot be removed", func(p wavePlane, h *waveHarness) {
			h.verbs.onLoop = func() {
				notes, _ := p.store().Notes()
				path := filepath.Join(p.evolveDir(), "waves", "notes", notes[0].Name)
				_ = os.Remove(path)
				_ = os.MkdirAll(filepath.Join(path, "blocked"), 0o755)
			}
		}, "remove note", waveTestPID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			p.seedWave81()
			h := newWaveHarness()
			c.prep(p, h)

			rc := h.run("next", "--json", "--project-root", p.root)

			e := decodeWaveEnvelope(t, h.stdout.Bytes())
			if rc != 0 || !e.Launched || !e.Recorded || e.PID != c.pid || !hasWarning(e, c.want) {
				t.Errorf("rc = %d envelope = %+v; want a recorded launch with pid %d and a warning %q", rc, e, c.pid, c.want)
			}
		})
	}
}

func TestWaveStatus_ErrorsAndEmptyJSON(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	emptyRC := h.run("status", "--json", "--project-root", p.root)
	empty := strings.Join(strings.Fields(h.stdout.String()), " ")
	p.write(".evolve/waves", "not a dir")
	brokenRC := h.run("status", "--project-root", p.root)

	if emptyRC != 0 || !strings.Contains(empty, `"wave": 0`) || !strings.Contains(empty, `"cycles": []`) {
		t.Errorf("no record JSON: rc = %d stdout %s", emptyRC, empty)
	}
	if brokenRC != exitIO || !strings.Contains(h.stderr.String(), "list wave records") {
		t.Errorf("waves is a file: rc = %d stderr %q, want %d", brokenRC, h.stderr.String(), exitIO)
	}
}

func TestWaveStatus_TextShowsOmittedCyclesLastWaveWarningsAndPauses(t *testing.T) {
	p := newWavePlane(t)
	for id := 1; id <= waveStatusMaxCycles+2; id++ {
		p.write(fmt.Sprintf(".evolve/runs/cycle-%d/run.json", id), fmt.Sprintf(`{"cycle_id":%d,"phase":"scout"}`, id))
	}
	p.write(".evolve/runs/cycle-22/signals.ndjson",
		`{"schema_version":"1.0","seq":1,"pid":1,"ts":"2026-10-08T11:30:00Z","module":"orchestrator","origin":"x","kind":"quota.paused","phase":"scout","severity":"WARN","reason":"r","fields":{"phase":"scout"}}`+"\n")
	p.write(".evolve/runs/cycle-23/cycle-state.json", "{")
	_ = p.store().Save(wave.Record{Number: 1, RunID: "r1", Outcome: "o1"})
	_ = p.store().Save(wave.Record{Number: 2, RunID: "r2"})
	h := newWaveHarness()

	rc := h.run("status", "--project-root", p.root)

	out := h.stdout.String()
	for _, want := range []string{"  (3 older cycle(s) not shown)\n", "cycle 22: phase scout, quota pauses 1\n", "last:    wave 1: o1\n", "WARN:    cycle-23: cycle-state.json"} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc = %d; stdout lacks %q:\n%s", rc, want, out)
		}
	}
}

func TestWaveWatch_ErrorsAndWarnings(t *testing.T) {
	t.Run("an unreadable newer record exits IO", func(t *testing.T) {
		p := newWavePlane(t)
		_ = p.store().Save(wave.Record{Number: 82, RunID: "r82", PID: 777, LogPath: p.loopLog("r82", 777)})
		p.write(".evolve/runs/cycle-1/cycle-state.json", "{")
		h := newWaveHarness()
		h.alive[777] = true
		h.onSleep = func(n int) {
			if n == 2 {
				p.write(".evolve/waves/wave-83.json", "{")
			}
		}

		rc := h.run("watch", "--project-root", p.root)

		if rc != exitIO || !strings.Contains(h.stderr.String(), "parse wave record") || strings.Count(h.stderr.String(), "WARN: cycle-1") != 1 {
			t.Errorf("rc = %d stderr %q; want %d, the record error, and the cycle-1 warning once", rc, h.stderr.String(), exitIO)
		}
	})
	t.Run("an event that cannot be written exits IO", func(t *testing.T) {
		p := newWavePlane(t)
		_ = p.store().Save(wave.Record{Number: 82, RunID: "r82"})
		h := newWaveHarness()
		var errb bytes.Buffer

		rc := runWaveWith(h.env(p.root), []string{"watch", "--project-root", p.root}, &failingWriter{}, &errb)

		if rc != exitIO || !strings.Contains(errb.String(), "write: broken pipe") {
			t.Errorf("rc = %d stderr %q, want %d and the write error", rc, errb.String(), exitIO)
		}
	})
	t.Run("a wave-started line that cannot be written exits IO", func(t *testing.T) {
		p := newWavePlane(t)
		_ = p.store().Save(wave.Record{Number: 82, RunID: "r82", PID: 777, LogPath: p.loopLog("r82", 777)})
		p.write(".evolve/runs/cycle-1/run.json", `{"cycle_id":1,"phase":"scout"}`)
		h := newWaveHarness()
		h.alive[777] = true
		h.onSleep = func(int) { _ = p.store().Save(wave.Record{Number: 83, RunID: "r83"}) }
		var errb bytes.Buffer

		rc := runWaveWith(h.env(p.root), []string{"watch", "--project-root", p.root}, &failingWriter{okWrites: 1}, &errb)

		if rc != exitIO || !strings.Contains(errb.String(), "write: broken pipe") || h.sleeps != 1 {
			t.Errorf("rc = %d after %d polls, stderr %q; want %d at the wave-started line", rc, h.sleeps, errb.String(), exitIO)
		}
	})
}

func TestWaveNext_UnreadableNotesExitIOBeforeAnyStep(t *testing.T) {
	p := newWavePlane(t)
	p.write(".evolve/waves/notes", "not a dir")
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if rc != exitIO || !strings.Contains(e.Error, "list notes") || e.Launched || len(h.verbs.calls) != 0 {
		t.Errorf("rc = %d envelope = %+v with %d steps; want %d, the notes error, no step", rc, e, len(h.verbs.calls), exitIO)
	}
}
