package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
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

func TestDefaultWaveEnv_MeasuresTheRealDisk(t *testing.T) {
	t.Parallel()
	got := defaultWaveEnv().diskFree

	if got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(looppreflight.DiskFreeBytes).Pointer() {
		t.Fatal("the production wave env must measure through looppreflight.DiskFreeBytes")
	}
}

func TestWaveNext_ALiveLoopOutranksALowDisk(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	runDir := filepath.Join(p.evolveDir(), "runs", "cycle-1837")
	_ = os.MkdirAll(runDir, 0o755)
	_ = runlease.Write(runDir, runlease.Lease{RunID: "r-live", OwnerPID: os.Getpid()}, waveTestClock)
	h := newWaveHarness()
	h.diskFree = func(string) (uint64, error) { return 273 << 20, nil }

	rc := h.run("next", "--json", "--project-root", p.root)

	if e := decodeWaveEnvelope(t, h.stdout.Bytes()); rc != cliExitRefused || e.Refused != waveRefusedLive {
		t.Fatalf("rc = %d envelope = %+v; want %s, because stopping the live loop comes before freeing the disk", rc, e, waveRefusedLive)
	}
}

func TestWaveNext_ABadPolicyBelowTheFloorNamesTheParseError(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.write(".evolve/policy.json", `{`)
	h := newWaveHarness()
	h.diskFree = func(string) (uint64, error) { return 273 << 20, nil }

	rc := h.run("next", "--json", "--project-root", p.root)

	e := decodeWaveEnvelope(t, h.stdout.Bytes())
	if rc != cliExitRefused || e.Refused != waveRefusedDiskLow {
		t.Fatalf("rc = %d envelope = %+v; want a %s refusal on the default floor", rc, e, waveRefusedDiskLow)
	}
	if !strings.Contains(h.stderr.String(), "policy: parse") || !strings.Contains(h.stderr.String(), "compiled defaults") {
		t.Errorf("stderr %q; want a warning that the policy was refused and the defaults apply", h.stderr.String())
	}
}
