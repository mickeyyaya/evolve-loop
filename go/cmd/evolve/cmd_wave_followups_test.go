package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestWaveNext_TheJSONDecisionComesFromTheParser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		args     []string
		wantRC   int
		wantJSON bool
		wantErr  string
	}{
		{"a note whose text is --json", []string{"--dry-run", "--note", "--json"}, 0, false, ""},
		{"the inline --json=true form", []string{"--dry-run", "--json=true"}, exitUsage, false, "unknown flag"},
		{"the bare --json flag", []string{"--dry-run", "--json"}, 0, true, ""},
		{"--json beside a usage error", []string{"--json", "--bogus"}, exitUsage, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			p.seedWave81()
			h := newWaveHarness()

			rc := h.run(append([]string{"next", "--project-root", p.root}, c.args...)...)

			isJSON := json.Valid(h.stdout.Bytes())
			if !c.wantJSON && strings.Contains(h.stdout.String(), `"schema"`) {
				t.Errorf("text mode stdout carries a JSON envelope:\n%s", h.stdout.String())
			}
			if rc != c.wantRC || isJSON != c.wantJSON || !strings.Contains(h.stderr.String(), c.wantErr) {
				t.Errorf("rc = %d, json stdout = %t, stderr %q; want rc %d, json stdout %t, stderr containing %q\nstdout:\n%s",
					rc, isJSON, h.stderr.String(), c.wantRC, c.wantJSON, c.wantErr, h.stdout.String())
			}
			if !c.wantJSON && rc == 0 && !strings.HasPrefix(h.stdout.String(), "wave: dry run") {
				t.Errorf("text mode stdout = %q, want the text dry run", h.stdout.String())
			}
		})
	}
}

type waveSaveCalls struct {
	numbers []int
	failNew int
	newSeen int
}

func (c *waveSaveCalls) save(s wave.Store, r wave.Record) error {
	c.numbers = append(c.numbers, r.Number)
	if r.Number == 82 {
		c.newSeen++
		if c.newSeen <= c.failNew {
			return errors.New("disk busy")
		}
	}
	return s.Save(r)
}

func TestWaveNext_TheRecordSaveFailsOnceThenPasses(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()
	calls := &waveSaveCalls{failNew: 1}
	h.save = calls.save

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if rc != 0 || !e.Recorded || calls.newSeen != 2 {
		t.Errorf("rc = %d recorded = %t after %d save attempts of wave 82 (error %q); want rc 0, recorded, 2 attempts", rc, e.Recorded, calls.newSeen, e.Error)
	}
	records, _ := p.store().Records()
	if len(records) != 2 || records[1].Number != 82 {
		t.Errorf("records = %+v, want waves 81 and 82 on disk after the retry", records)
	}
}

func TestWaveNext_TheRecordSaveStopsAfterTwoFailures(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()
	calls := &waveSaveCalls{failNew: 99}
	h.save = calls.save

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if rc != exitIO || e.Recorded || !e.Launched || !strings.Contains(e.Error, "NOT written") || calls.newSeen != waveRecordAttempts {
		t.Errorf("rc = %d launched = %t recorded = %t error %q after %d attempts; want exitIO, launched, not recorded, NOT written, %d attempts",
			rc, e.Launched, e.Recorded, e.Error, calls.newSeen, waveRecordAttempts)
	}
}

func TestWaveNext_TheOldWaveClosesOnlyAfterTheNewRecordIsSaved(t *testing.T) {
	t.Parallel()
	t.Run("order of the saves", func(t *testing.T) {
		p := newWavePlane(t)
		p.seedWave81()
		h := newWaveHarness()
		calls := &waveSaveCalls{}
		h.save = calls.save

		rc := h.run("next", "--json", "--project-root", p.root)

		if rc != 0 || len(calls.numbers) != 2 || calls.numbers[0] != 82 || calls.numbers[1] != 81 {
			t.Errorf("rc = %d, saves in order %v; want 0 and [82 81]", rc, calls.numbers)
		}
	})
	t.Run("a new record that cannot be saved leaves the old wave open", func(t *testing.T) {
		p := newWavePlane(t)
		p.seedWave81()
		h := newWaveHarness()
		calls := &waveSaveCalls{failNew: 99}
		h.save = calls.save

		h.run("next", "--json", "--project-root", p.root)

		records, _ := p.store().Records()
		if len(records) != 1 || records[0].Outcome != "" || records[0].EndedAt != nil {
			t.Errorf("records = %+v, want only wave 81, still open (no outcome, no end)", records)
		}
	})
}

func TestWaveOwnsPID_AnUnknownStartTimeWarnsOnce(t *testing.T) {
	t.Parallel()
	startWarnings := func(lines []string) []string {
		var out []string
		for _, l := range lines {
			if strings.Contains(l, "WARN") && strings.Contains(strings.ToLower(l), "start time") {
				out = append(out, l)
			}
		}
		return out
	}
	seed := func(t *testing.T) (wavePlane, *waveHarness) {
		p := newWavePlane(t)
		if err := p.store().Save(wave.Record{Number: 82, RunID: "r82", CycleFloor: 1836, PID: 777, LogPath: p.loopLog("r82", 777), StartedAt: waveTestClock}); err != nil {
			t.Fatal(err)
		}
		h := newWaveHarness()
		h.alive[777] = true
		return p, h
	}
	t.Run("watch warns once over several polls", func(t *testing.T) {
		p, h := seed(t)
		h.onSleep = func(n int) {
			if n >= 3 {
				h.alive[777] = false
			}
		}

		rc := h.run("watch", "--project-root", p.root)

		got := startWarnings(strings.Split(h.stderr.String(), "\n"))
		if rc != 0 || h.sleeps < 3 || len(got) != 1 {
			t.Errorf("rc = %d after %d polls, start-time WARN lines %q; want rc 0, 3+ polls, exactly 1 line", rc, h.sleeps, got)
		}
	})
	t.Run("status carries the warning", func(t *testing.T) {
		p, h := seed(t)

		rc := h.run("status", "--json", "--project-root", p.root)

		got := startWarnings(prefixLines(decodeWaveEnvelope(t, h.stdout.Bytes()).Warnings, "WARN "))
		if rc != 0 || len(got) != 1 {
			t.Errorf("rc = %d, start-time warnings %q; want rc 0 and exactly 1", rc, got)
		}
	})
	t.Run("a known start time stays silent", func(t *testing.T) {
		p, h := seed(t)
		h.started[777] = waveTestClock.Add(-time.Minute)
		h.onSleep = func(int) { h.alive[777] = false }

		h.run("watch", "--project-root", p.root)

		if got := startWarnings(strings.Split(h.stderr.String(), "\n")); len(got) != 0 {
			t.Errorf("start-time WARN lines %q for a known start time, want none", got)
		}
	})
}

func prefixLines(lines []string, prefix string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, prefix+l)
	}
	return out
}

func TestWaveLoopLive_ALiveRunLeaseCountsWithoutAPID(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	runDir := filepath.Join(p.evolveDir(), "runs", "cycle-1949")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r-live", OwnerPID: os.Getpid()}, waveTestClock); err != nil {
		t.Fatal(err)
	}
	env := newWaveHarness().env(p.root)

	live, warning := waveLoopLive(env, p.evolveDir(), wave.Record{})
	idle, idleWarning := waveLoopLive(env, filepath.Join(p.root, "no-such-evolve-dir"), wave.Record{})

	if !live || warning != "" {
		t.Errorf("live lease: live=%v warning=%q, want true and no warning", live, warning)
	}
	if idle || idleWarning != "" {
		t.Errorf("no lease and no pid: live=%v warning=%q, want false and no warning", idle, idleWarning)
	}
}
