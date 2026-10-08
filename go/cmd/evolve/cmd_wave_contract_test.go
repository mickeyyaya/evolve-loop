package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestWaveNext_JSONEnvelopeOnEveryExit(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		prep        func(p wavePlane, h *waveHarness)
		wantRC      int
		wantRefused string
		wantFailed  string
		wantError   string
		launched    bool
	}{
		{name: "success", wantRC: 0, launched: true},
		{name: "live loop", wantRC: exitRefused, wantRefused: "loop_live", wantError: "r-live", prep: func(p wavePlane, _ *waveHarness) {
			runDir := filepath.Join(p.evolveDir(), "runs", "cycle-1837")
			_ = os.MkdirAll(runDir, 0o755)
			_ = runlease.Write(runDir, runlease.Lease{RunID: "r-live", OwnerPID: os.Getpid()}, waveTestClock)
		}},
		{name: "number taken", args: []string{"--number", "1"}, wantRC: exitRefused, wantRefused: "number_taken", wantError: "not more than", prep: func(p wavePlane, _ *waveHarness) {
			_ = p.store().Save(wave.Record{Number: 4})
		}},
		{name: "usage", args: []string{"--max-cycles", "0"}, wantRC: exitUsage, wantError: "usage: --max-cycles"},
		{name: "missing standing goal", wantRC: exitIO, wantError: "standing goal", prep: func(p wavePlane, _ *waveHarness) {
			_ = os.Remove(filepath.Join(p.evolveDir(), "wave-goal.md"))
		}},
		{name: "failed step", wantRC: 7, wantFailed: "evolve sync-main", wantError: "sync-main", prep: func(_ wavePlane, h *waveHarness) {
			h.verbs.failAt, h.verbs.code = 3, 7
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			h := newWaveHarness()
			if c.prep != nil {
				c.prep(p, h)
			}

			rc := h.run(append([]string{"next", "--json", "--project-root", p.root}, c.args...)...)

			e := decodeWaveEnvelope(t, h.stdout.Bytes())
			if rc != c.wantRC || e.Schema != waveNextSchema || e.Refused != c.wantRefused || e.Launched != c.launched || e.Recorded != c.launched ||
				!strings.HasPrefix(e.FailedStep, c.wantFailed) || (c.wantFailed == "") != (e.FailedStep == "") ||
				!strings.Contains(e.Error, c.wantError) || (c.wantError == "") != (e.Error == "") {
				t.Errorf("rc = %d envelope = %+v; want rc %d refused %q failed_step %q error containing %q launched %t",
					rc, e, c.wantRC, c.wantRefused, c.wantFailed, c.wantError, c.launched)
			}
		})
	}
}

func TestWaveNext_RecordSaveFailureAfterTheLaunchIsLoudAndConsumesTheNotes(t *testing.T) {
	p := newWavePlane(t)
	if _, err := p.store().AddNote("a note", waveTestClock); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.evolveDir(), "waves", "wave-1.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	notes, _ := p.store().Notes()
	if rc != exitIO || !e.Launched || e.Recorded || e.PID != waveTestPID || !strings.Contains(e.Error, "wave record NOT written") {
		t.Errorf("rc = %d envelope = %+v; want %d, launched, not recorded, the pid and the error", rc, e, exitIO)
	}
	if !strings.Contains(h.stderr.String(), "loop launched, wave record NOT written") || len(notes) != 0 {
		t.Errorf("stderr %q, %d notes left; want the loud line and the notes consumed with the launched goal", h.stderr.String(), len(notes))
	}
}

func TestWaveNext_WarnsWhenTheRunningBinaryIsNotThePlaneBinary(t *testing.T) {
	cases := []struct {
		name, exe string
		wantWarn  bool
	}{
		{"the plane binary", "", false},
		{"another binary", "/usr/local/bin/evolve", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			h := newWaveHarness()
			h.exe = c.exe

			h.run("next", "--number", "5", "--dry-run", "--json", "--project-root", p.root)

			e := decodeWaveEnvelope(t, h.stdout.Bytes())
			plane := filepath.Join(p.root, "go", "bin", "evolve")
			warned := slices.ContainsFunc(e.Warnings, func(w string) bool { return strings.Contains(w, "is not the plane binary "+plane) })
			if warned != c.wantWarn {
				t.Errorf("warnings = %q, want a plane-binary warning: %t", e.Warnings, c.wantWarn)
			}
		})
	}
}

func TestWaveWatch_AReusedPIDIsNotTheLoop(t *testing.T) {
	cases := []struct {
		name      string
		writerPID int
		started   time.Duration
		wantPolls int
	}{
		{"the log writer pid names the loop", 777, -time.Minute, 1},
		{"the log writer pid names another process", 778, -time.Minute, 0},
		{"the pid started after the wave", 777, time.Minute, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1836, PID: 777, LogPath: p.loopLog("r82", c.writerPID), StartedAt: waveTestClock}); err != nil {
				t.Fatal(err)
			}
			h := newWaveHarness()
			h.alive[777] = true
			h.started[777] = waveTestClock.Add(c.started)
			h.onSleep = func(int) { h.alive[777] = false }

			rc := h.run("watch", "--project-root", p.root)

			if rc != 0 || h.sleeps != c.wantPolls || h.stdout.String() != "wave 82: loop: exit\n" {
				t.Errorf("rc = %d after %d polls, stdout %q; want 0 after %d polls with only the loop exit", rc, h.sleeps, h.stdout.String(), c.wantPolls)
			}
		})
	}
}

func TestWaveWatch_FollowsANewerWave(t *testing.T) {
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1836, PID: 777, LogPath: p.loopLog("r82", 777), StartedAt: waveTestClock}); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()
	h.alive[777] = true
	h.onSleep = func(n int) {
		switch n {
		case 1:
			h.alive[777], h.alive[888] = false, true
			_ = p.store().Save(wave.Record{Number: 83, RunID: "r83", CycleFloor: 1837, PID: 888, LogPath: p.loopLog("r83", 888), StartedAt: waveTestClock})
			p.write(".evolve/runs/cycle-1838/run.json", `{"cycle_id":1838,"phase":"scout"}`)
		case 2:
			h.alive[888] = false
		}
	}

	rc := h.run("watch", "--project-root", p.root)

	want := "wave 82: wave 83 started; watch follows it\nwave 83: cycle 1838: phase scout\nwave 83: loop: exit\n"
	if rc != 0 || h.stdout.String() != want {
		t.Errorf("rc = %d stdout:\n%s\nwant:\n%s", rc, h.stdout.String(), want)
	}
}

func TestWaveNote_ListJSON(t *testing.T) {
	p := newWavePlane(t)
	h := newWaveHarness()

	empty := h.run("note", "list", "--json", "--project-root", p.root)
	emptyOut := strings.TrimSpace(h.stdout.String())
	h.run("note", "add", "watch PR 814", "--project-root", p.root)
	h.run("note", "list", "--json", "--project-root", p.root)

	var notes []wave.Note
	if err := json.Unmarshal(h.stdout.Bytes(), &notes); err != nil || empty != 0 || emptyOut != "[]" || len(notes) != 1 || notes[0].Text != "watch PR 814" {
		t.Errorf("empty %d %q; notes %+v (decode %v); want [] then the one note", empty, emptyOut, notes, err)
	}
}

func TestWaveNext_TheFactsOfABigWaveAreBounded(t *testing.T) {
	p := newWavePlane(t)
	for id := 1835; id < 1835+120; id++ {
		p.write(fmt.Sprintf(".evolve/runs/cycle-%d/run.json", id), fmt.Sprintf(`{"cycle_id":%d,"phase":"build"}`, id))
	}
	if err := p.store().Save(wave.Record{Number: 81, RunID: "r81", CycleFloor: 1834}); err != nil {
		t.Fatal(err)
	}
	h := newWaveHarness()

	rc := h.run("next", "--dry-run", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	cycleLines := strings.Count(e.Goal, "\n- Cycle ")
	if rc != 0 || cycleLines != 20 || !strings.Contains(e.Goal, "- 100 older cycles are not shown.\n- Cycle 1935:") || e.GoalBytes > 4096 {
		t.Errorf("rc = %d: %d cycle lines, %d bytes; want the policy default of 20 newest cycles, the omitted line and under 4 KB", rc, cycleLines, e.GoalBytes)
	}
}
