package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestWaveNext_RefusesBelowTheFreeSpaceFloorBeforeAnyStep(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.write(".evolve/policy.json", `{"preflight":{"min_free_gib":2}}`)
	h := newWaveHarness()
	var measured string
	h.diskFree = func(path string) (uint64, error) { measured = path; return 273 << 20, nil }

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if rc != cliExitRefused || e.Refused != waveRefusedDiskLow || e.Launched {
		t.Fatalf("rc = %d envelope = %+v; want a %s refusal and no launch", rc, e, waveRefusedDiskLow)
	}
	for _, want := range []string{"273 MiB free", "2048 MiB floor", "preflight.min_free_gib", "evolve gc"} {
		if !strings.Contains(e.Error, want) {
			t.Errorf("the refusal %q does not name %q", e.Error, want)
		}
	}
	if measured != p.evolveDir() || len(h.refreshed) != 0 || len(e.Steps) != 0 {
		t.Errorf("measured %q, refreshed %d, steps %+v; want the evolve dir measured and nothing built or run", measured, len(h.refreshed), e.Steps)
	}
}

func TestWaveNext_StartsAtOrAboveTheFloor(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	h := newWaveHarness()
	h.diskFree = func(string) (uint64, error) { return 1 << 30, nil }

	rc := h.run("next", "--json", "--project-root", p.root)

	if e := decodeWaveEnvelope(t, h.stdout.Bytes()); rc != 0 || e.Refused != "" || !e.Launched {
		t.Errorf("rc = %d envelope = %+v; want the default 1 GiB floor met and the wave launched", rc, e)
	}
}

func TestWaveNext_ADryRunAndAnUnmeasurableDiskWarnInsteadOfRefusing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		free func(string) (uint64, error)
		want string
	}{
		{"a dry run below the floor", []string{"--dry-run"}, func(string) (uint64, error) { return 1 << 20, nil }, "below the 1024 MiB floor"},
		{"an unmeasurable disk", nil, func(string) (uint64, error) { return 0, errors.New("statfs denied") }, "statfs denied"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			h := newWaveHarness()
			h.diskFree = c.free

			rc := h.run(append([]string{"next", "--json", "--project-root", p.root}, c.args...)...)

			e := decodeWaveEnvelope(t, h.stdout.Bytes())
			warned := slices.ContainsFunc(e.Warnings, func(w string) bool { return strings.Contains(w, c.want) })
			if rc != 0 || e.Refused != "" || !warned {
				t.Errorf("rc = %d envelope = %+v; want no refusal and a warning naming %q", rc, e, c.want)
			}
		})
	}
}
