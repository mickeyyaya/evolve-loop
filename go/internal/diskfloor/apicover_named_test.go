package diskfloor_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
)

func TestAPICover_AllExportsNamedAndExecuted(t *testing.T) {
	t.Parallel()
	if diskfloor.BytesPerGB != 1_000_000_000 || diskfloor.BytesPerGiB != 1<<30 {
		t.Fatalf("unexpected byte constants: %d, %d", diskfloor.BytesPerGB, diskfloor.BytesPerGiB)
	}
	if diskfloor.DefaultGCFloorBytes != 10*diskfloor.BytesPerGB || diskfloor.DefaultGCWarnBytes != 20*diskfloor.BytesPerGB || diskfloor.DefaultPreflightFloorBytes != diskfloor.BytesPerGiB {
		t.Fatalf("unexpected default constants")
	}
	keys := []string{
		diskfloor.KeyFloorGB,
		diskfloor.KeyWarnGB,
		diskfloor.OldKeyGCFloorGB,
		diskfloor.OldKeyGCWarnGB,
		diskfloor.OldKeyPreflightMinFreeGiB,
	}
	for _, k := range keys {
		if k == "" {
			t.Fatal("empty key constant")
		}
	}

	var p diskfloor.Policy = diskfloor.Policy{FloorGB: 2, WarnGB: 4}
	var warn bytes.Buffer
	var res diskfloor.Resolution
	var err error
	res, err = diskfloor.Resolve(&p, nil, nil, diskfloor.DefaultPreflightFloorBytes, &warn)
	if err != nil || res.FloorBytes != 2*diskfloor.BytesPerGB || res.WarnBytes != 4*diskfloor.BytesPerGB {
		t.Fatalf("Resolve = (%+v, %v)", res, err)
	}

	type cfg struct{ MinFreeGiB float64 }
	var c cfg
	if n, aerr := diskfloor.ApplyToPreflight(&p, nil, nil, &c, nil); aerr != nil || n != 2*diskfloor.BytesPerGB {
		t.Fatalf("ApplyToPreflight = (%d, %v)", n, aerr)
	}
	if got := diskfloor.PreflightBytes(c); got == 0 {
		t.Fatal("PreflightBytes returned 0")
	}
	if _, cerr := diskfloor.Resolve(&diskfloor.Policy{FloorGB: 8}, &cfg{}, &cfg{MinFreeGiB: 2}, 0, nil); !errors.Is(cerr, diskfloor.ErrConflict) {
		t.Fatalf("conflict error = %v, want ErrConflict", cerr)
	}
}
