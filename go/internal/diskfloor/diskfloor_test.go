package diskfloor_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
)

func TestResolve_DefaultsAndNewKeys(t *testing.T) {
	t.Parallel()
	var warn bytes.Buffer
	res, err := diskfloor.Resolve(nil, nil, nil, 0, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if res.FloorBytes != diskfloor.DefaultGCFloorBytes || res.WarnBytes != diskfloor.DefaultGCWarnBytes || res.FloorSet || res.WarnSet {
		t.Fatalf("default resolution = %+v", res)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warnings on default: %q", warn.String())
	}

	res, err = diskfloor.Resolve(&diskfloor.Policy{FloorGB: 0.001, WarnGB: 2.5}, nil, nil, diskfloor.DefaultPreflightFloorBytes, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if res.FloorBytes != 1_000_000 || res.WarnBytes != 2_500_000_000 || !res.FloorSet || !res.WarnSet {
		t.Fatalf("new-key resolution = %+v", res)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warnings when only new keys set: %q", warn.String())
	}
}

func TestResolve_LegacyAliasesWarnAndConflictsError(t *testing.T) {
	t.Parallel()
	type legacyGC struct {
		DiskFloorGB int
		DiskWarnGB  int
	}
	type legacyPreflight struct {
		MinFreeGiB float64
	}

	var warn bytes.Buffer
	res, err := diskfloor.Resolve(nil, &legacyGC{DiskFloorGB: 12, DiskWarnGB: 30}, nil, 0, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if res.FloorBytes != 12*diskfloor.BytesPerGB || res.WarnBytes != 30*diskfloor.BytesPerGB {
		t.Fatalf("legacy gc resolution = %+v", res)
	}
	for _, want := range []string{diskfloor.OldKeyGCFloorGB, diskfloor.KeyFloorGB, diskfloor.OldKeyGCWarnGB, diskfloor.KeyWarnGB} {
		if !strings.Contains(warn.String(), want) {
			t.Errorf("warn %q missing %q", warn.String(), want)
		}
	}

	warn.Reset()
	res, err = diskfloor.Resolve(nil, (*legacyGC)(nil), &legacyPreflight{MinFreeGiB: 2}, 0, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if res.FloorBytes != 2*diskfloor.BytesPerGiB || !strings.Contains(warn.String(), diskfloor.OldKeyPreflightMinFreeGiB) {
		t.Fatalf("legacy preflight = %+v, warn=%q", res, warn.String())
	}

	if _, err := diskfloor.Resolve(&diskfloor.Policy{FloorGB: 8}, &legacyGC{DiskFloorGB: 5}, nil, 0, nil); !errors.Is(err, diskfloor.ErrConflict) {
		t.Fatalf("floor conflict err = %v, want ErrConflict", err)
	}
	if _, err := diskfloor.Resolve(&diskfloor.Policy{WarnGB: 80}, &legacyGC{DiskWarnGB: 70}, nil, 0, nil); !errors.Is(err, diskfloor.ErrConflict) {
		t.Fatalf("warn conflict err = %v, want ErrConflict", err)
	}
	if _, err := diskfloor.Resolve(&diskfloor.Policy{FloorGB: 30}, legacyGC{DiskFloorGB: 30}, nil, 0, nil); err != nil {
		t.Fatalf("equal values should not conflict: %v", err)
	}
}

func TestApplyToPreflight_AndEdgeInputs(t *testing.T) {
	t.Parallel()
	type preflightCfg struct {
		MinFreeGiB float64
	}
	var dst preflightCfg
	bytes, err := diskfloor.ApplyToPreflight(&diskfloor.Policy{FloorGB: 5}, nil, nil, &dst, nil)
	if err != nil || bytes != 5*diskfloor.BytesPerGB || dst.MinFreeGiB <= 0 {
		t.Fatalf("ApplyToPreflight = (%d, %v), dst=%+v", bytes, err, dst)
	}
	if _, err := diskfloor.ApplyToPreflight(&diskfloor.Policy{FloorGB: 5}, nil, preflightCfg{MinFreeGiB: 2}, &dst, nil); !errors.Is(err, diskfloor.ErrConflict) {
		t.Fatalf("ApplyToPreflight conflict = %v, want ErrConflict", err)
	}
	bytes, err = diskfloor.ApplyToPreflight(nil, nil, preflightCfg{MinFreeGiB: 2}, &dst, nil)
	if err != nil || bytes != 2*diskfloor.BytesPerGiB || dst.MinFreeGiB != 2 {
		t.Fatalf("ApplyToPreflight legacy GiB = (%d, %v), dst=%+v", bytes, err, dst)
	}
	for _, nonStructDst := range []any{nil, preflightCfg{}, (*preflightCfg)(nil), new(int)} {
		bytes, err = diskfloor.ApplyToPreflight(&diskfloor.Policy{FloorGB: 5}, nil, nil, nonStructDst, nil)
		if err != nil || bytes != 5*diskfloor.BytesPerGB {
			t.Fatalf("ApplyToPreflight(%T) = (%d, %v), want (%d, nil)", nonStructDst, bytes, err, 5*diskfloor.BytesPerGB)
		}
	}
	if got := diskfloor.PreflightBytes(preflightCfg{MinFreeGiB: 0.25}); got != 256<<20 {
		t.Fatalf("PreflightBytes(0.25) = %d, want %d", got, 256<<20)
	}
	if got := diskfloor.PreflightBytes(struct{ MinFreeGiB string }{MinFreeGiB: "2"}); got != diskfloor.DefaultPreflightFloorBytes {
		t.Fatalf("PreflightBytes(non-numeric) = %d, want default", got)
	}
	if got := diskfloor.PreflightBytes(nil); got != diskfloor.DefaultPreflightFloorBytes {
		t.Fatalf("PreflightBytes(nil) = %d, want default", got)
	}
	res, err := diskfloor.Resolve(&diskfloor.Policy{FloorGB: math.Inf(1)}, "not-a-struct", struct{}{}, 0, nil)
	if err != nil || res.FloorBytes != diskfloor.DefaultGCFloorBytes {
		t.Fatalf("edge resolution = %+v, err=%v", res, err)
	}
	res, err = diskfloor.Resolve(&diskfloor.Policy{FloorGB: 1e12}, nil, nil, 0, nil)
	if err != nil || res.FloorBytes != math.MaxUint64 {
		t.Fatalf("overflow saturation = %+v, err=%v", res, err)
	}
}
