package looppreflight

import (
	"errors"
	"strings"
	"testing"
)

func diskOpts(t *testing.T, free uint64, floor uint64) Options {
	o := goodPipelineOptions(t)
	o.DiskFreeBytes = func(string) (uint64, error) { return free, nil }
	o.MinFreeBytes = floor
	return o
}

func TestRun_DiskSpace_BelowFloorHaltsNamingTheFix(t *testing.T) {
	r, err := Run(diskOpts(t, 143<<20, 1<<30))
	if err != nil {
		t.Fatal(err)
	}
	c := findCheck(t, r, "disk-space")
	if c.Level != LevelHalt || !r.Halted() {
		t.Fatalf("level=%s halted=%v, want halt", c.Level, r.Halted())
	}
	if !strings.Contains(c.Detail+c.Message, "evolve gc") || !strings.Contains(c.Detail+c.Message, "143") {
		t.Errorf("halt must name the operator fix and the measured MiB: %q / %q", c.Message, c.Detail)
	}
}

func TestRun_DiskSpace_AtAndAboveFloorPass(t *testing.T) {
	for _, free := range []uint64{1 << 30, 8 << 30} {
		r, err := Run(diskOpts(t, free, 1<<30))
		if err != nil {
			t.Fatal(err)
		}
		if c := findCheck(t, r, "disk-space"); c.Level != LevelPass {
			t.Errorf("free=%d: level=%s, want pass", free, c.Level)
		}
	}
}

func TestRun_DiskSpace_UnmeasurableWarnsNeverHalts(t *testing.T) {
	o := diskOpts(t, 0, 1<<30)
	o.DiskFreeBytes = func(string) (uint64, error) { return 0, errors.New("statfs: unsupported") }
	r, err := Run(o)
	if err != nil {
		t.Fatal(err)
	}
	if c := findCheck(t, r, "disk-space"); c.Level != LevelWarn {
		t.Errorf("level=%s, want warn: an unmeasurable disk is not evidence of a full one", c.Level)
	}
}

func TestRun_DiskSpace_ZeroFloorUsesThePolicyDefault(t *testing.T) {
	r, err := Run(diskOpts(t, 1<<20, 0))
	if err != nil {
		t.Fatal(err)
	}
	if c := findCheck(t, r, "disk-space"); c.Level != LevelHalt {
		t.Errorf("1 MiB free under the default floor: level=%s, want halt", c.Level)
	}
}
