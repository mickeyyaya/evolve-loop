package sessionreaper

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func TestExportedAPIContract(t *testing.T) {
	if DefaultReapTimeout != 15*time.Second {
		t.Fatal("DefaultReapTimeout must stay boot-scale (orphanGCTimeout discipline, (0, 30s])")
	}
	evolveDir, runDir := makeRun(t, "contract", "evolve-bridge-contract")
	var killed []string
	var kill swarm.TmuxKiller = func(_ context.Context, session string) error {
		killed = append(killed, session)
		return nil
	}
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{
		Now:      func() time.Time { return time.Unix(0, 0) },
		LeaseTTL: time.Minute,
		Kill:     kill,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(killed, []string{"evolve-bridge-contract"}) {
		t.Fatalf("killed = %v, want the registered session", killed)
	}
	if rep.LiveRunsSkipped != 0 || len(rep.Orphaned) != 1 {
		t.Fatalf("report = %+v, want one orphan", rep)
	}
	var orphan OrphanReap = rep.Orphaned[0]
	if orphan.RunDir != runDir || orphan.Report.Killed != 1 {
		t.Fatalf("orphan = %+v, want run %s with Killed=1", orphan, runDir)
	}
}
