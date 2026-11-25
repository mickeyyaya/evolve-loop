package policy

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
)

func TestPolicy_DiskFloorAndGCDiskFloorAndConflictRefusal(t *testing.T) {
	t.Parallel()
	pol, err := Parse([]byte(`{"disk":{"floor_gb":12,"warn_gb":24}}`))
	if err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	res, err := pol.DiskFloor(&warn)
	if err != nil || res.FloorBytes != 12*diskfloor.BytesPerGB || res.WarnBytes != 24*diskfloor.BytesPerGB {
		t.Fatalf("DiskFloor = (%+v, %v)", res, err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected deprecation warning: %q", warn.String())
	}
	gcRes, err := pol.GCDiskFloor(nil)
	if err != nil || gcRes.FloorBytes != 12*diskfloor.BytesPerGB {
		t.Fatalf("GCDiskFloor = (%+v, %v)", gcRes, err)
	}
	if got := pol.WaveCheckinSettings().DiskFloorBytes; got != 12*diskfloor.BytesPerGB {
		t.Fatalf("WaveCheckinSettings().DiskFloorBytes = %d, want 12 GB", got)
	}

	if _, err := Parse([]byte(`{"disk":{"floor_gb":8},"gc":{"disk_floor_gb":5}}`)); !errors.Is(err, diskfloor.ErrConflict) {
		t.Fatalf("Parse conflict = %v, want ErrConflict", err)
	}
	if _, err := Parse([]byte(`{"disk":{"floor_gb":8},"preflight":{"min_free_gib":2}}`)); !errors.Is(err, diskfloor.ErrConflict) {
		t.Fatalf("Parse preflight conflict = %v, want ErrConflict", err)
	}

	oldPol, err := Parse([]byte(`{"gc":{"disk_floor_gb":15}}`))
	if err != nil {
		t.Fatal(err)
	}
	warn.Reset()
	res, err = oldPol.DiskFloor(&warn)
	if err != nil || res.FloorBytes != 15*diskfloor.BytesPerGB || !strings.Contains(warn.String(), "disk.floor_gb") {
		t.Fatalf("legacy DiskFloor = (%+v, %v), warn=%q", res, err, warn.String())
	}
}
