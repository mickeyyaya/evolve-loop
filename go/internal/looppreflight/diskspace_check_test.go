package looppreflight

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckDiskSpace_ComparesFreeWithTheFloor(t *testing.T) {
	cases := []struct {
		name  string
		free  uint64
		err   error
		floor uint64
		want  CheckLevel
	}{
		{"below the floor halts", 143 << 20, nil, 1 << 30, LevelHalt},
		{"at the floor passes", 1 << 30, nil, 1 << 30, LevelPass},
		{"above the floor passes", 8 << 30, nil, 1 << 30, LevelPass},
		{"an unmeasurable disk warns", 0, errors.New("statfs: unsupported"), 1 << 30, LevelWarn},
	}
	for _, c := range cases {
		var probed string
		got := CheckDiskSpace("/evolve", c.floor, func(p string) (uint64, error) { probed = p; return c.free, c.err })
		if got.Name != "disk-space" || got.Level != c.want {
			t.Errorf("%s: %s/%s, want disk-space/%s", c.name, got.Name, got.Level, c.want)
		}
		if probed != "/evolve" {
			t.Errorf("%s: probed %q, want the given dir", c.name, probed)
		}
		if c.want == LevelHalt && !strings.Contains(got.Detail, "evolve gc") {
			t.Errorf("%s: halt detail must name `evolve gc`: %q", c.name, got.Detail)
		}
	}
}

func TestCheckDiskSpace_DiskFloorErrHaltsPreflight(t *testing.T) {
	opts := diskOpts(t, 50<<30, 1<<30)
	opts.DiskFloorErr = errors.New("conflicting disk floor")
	res, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Checks {
		if c.Name == "disk-space" {
			if c.Level != LevelHalt || !strings.Contains(c.Message, "conflicting disk floor") {
				t.Fatalf("disk-space check = %+v, want halt with error message", c)
			}
			return
		}
	}
	t.Fatal("missing disk-space check")
}
