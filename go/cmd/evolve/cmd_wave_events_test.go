package main

import (
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func channelledEvents(t *testing.T, evolveDir, name string, kind signalcenter.Kind) []signalcenter.Event {
	t.Helper()
	var out []signalcenter.Event
	for _, r := range channelRecords(t, evolveDir, name) {
		if r.Signal.Kind == kind {
			out = append(out, *r.Signal)
		}
	}
	return out
}

func loneInfoEvent(t *testing.T, got []signalcenter.Event, kind signalcenter.Kind, module signalcenter.Module) signalcenter.Event {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("%s events = %+v, want exactly one", kind, got)
	}
	e := got[0]
	if e.Module != module || e.Severity != signalcenter.SeverityInfo || e.Code != "" || e.Reason == "" || !signalcenter.ValidOrigin(e.Origin) {
		t.Errorf("%s = module %s severity %s code %q origin %q reason %q, want an INFO %s event with no code", kind, e.Module, e.Severity, e.Code, e.Origin, e.Reason, module)
	}
	return e
}

func TestWaveNext_EmitsWaveStarted(t *testing.T) {
	useRealRootEvents(t)
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	rc := h.run("next", "--json", "--project-root", p.root)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}
	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	started := loneInfoEvent(t, channelledEvents(t, p.evolveDir(), "loop", signalcenter.KindWaveStarted), signalcenter.KindWaveStarted, signalcenter.ModuleLoop)
	want := map[string]string{"wave": "82", "run_id": e.RunID, "pid": strconv.Itoa(waveTestPID)}
	for k, v := range want {
		if started.Fields[k] != v {
			t.Errorf("wave.started fields[%s] = %q, want %q (fields %v)", k, started.Fields[k], v, started.Fields)
		}
	}
}

func TestWaveNext_EmitsWaveEndedForTheWaveItCloses(t *testing.T) {
	useRealRootEvents(t)
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()

	if rc := h.run("next", "--json", "--project-root", p.root); rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}

	ended := loneInfoEvent(t, channelledEvents(t, p.evolveDir(), "loop", signalcenter.KindWaveEnded), signalcenter.KindWaveEnded, signalcenter.ModuleLoop)
	want := map[string]string{"wave": "81", "outcome": "1 of 2 cycles shipped"}
	for k, v := range want {
		if ended.Fields[k] != v {
			t.Errorf("wave.ended fields[%s] = %q, want %q (fields %v)", k, ended.Fields[k], v, ended.Fields)
		}
	}
}

func TestWaveNext_TheFirstWaveEndsNothing(t *testing.T) {
	useRealRootEvents(t)
	p := newWavePlane(t)
	h := newWaveHarness()

	if rc := h.run("next", "--json", "--project-root", p.root); rc != 0 {
		t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
	}

	if got := channelledEvents(t, p.evolveDir(), "loop", signalcenter.KindWaveStarted); len(got) != 1 {
		t.Errorf("wave.started events = %+v, want the one launch", got)
	}
	if got := channelledEvents(t, p.evolveDir(), "loop", signalcenter.KindWaveEnded); len(got) != 0 {
		t.Errorf("wave.ended events = %+v, want none: no earlier wave was open", got)
	}
}

func TestWaveNext_ADryRunAndAFailedBoundaryEmitNothing(t *testing.T) {
	useRealRootEvents(t)
	cases := map[string]func(h *waveHarness) []string{
		"dry run":         func(*waveHarness) []string { return []string{"next", "--dry-run", "--json"} },
		"failed boundary": func(h *waveHarness) []string { h.verbs.failAt, h.verbs.code = 4, 7; return []string{"next", "--json"} },
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			p := newWavePlane(t)
			p.seedWave81()
			h := newWaveHarness()
			argv := append(args(h), "--project-root", p.root)

			if rc := h.run(argv...); rc == 0 && name != "dry run" {
				t.Fatalf("rc = 0, want a failure for %s", name)
			}

			for _, kind := range []signalcenter.Kind{signalcenter.KindWaveStarted, signalcenter.KindWaveEnded} {
				if got := channelledEvents(t, p.evolveDir(), "loop", kind); len(got) != 0 {
					t.Errorf("%s emitted %+v, want none", name, got)
				}
			}
		})
	}
}
