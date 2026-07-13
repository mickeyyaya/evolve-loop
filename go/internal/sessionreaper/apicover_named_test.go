package sessionreaper

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func TestExportedAPIContract(t *testing.T) {
	var kill swarm.TmuxKiller = func(context.Context, string) error { return nil }
	opts := Options{
		Now:      func() time.Time { return time.Unix(0, 0) },
		LeaseTTL: time.Minute,
		Kill:     kill,
	}
	orphan := OrphanReap{RunDir: "run", Report: swarm.ReapRunReport{Killed: 1}}
	report := Report{LiveRunsSkipped: 1, Orphaned: []OrphanReap{orphan}}
	if opts.Now == nil || opts.LeaseTTL != time.Minute || opts.Kill == nil {
		t.Fatal("Options fields not bound")
	}
	if report.LiveRunsSkipped != 1 || report.Orphaned[0].RunDir != "run" || report.Orphaned[0].Report.Killed != 1 {
		t.Fatal("Report or OrphanReap fields not bound")
	}
	if DefaultReapTimeout != 15*time.Second {
		t.Fatal("DefaultReapTimeout must stay boot-scale (orphanGCTimeout discipline, (0, 30s])")
	}
	if _, err := ReapOrphans(context.Background(), t.TempDir(), opts); err != nil {
		t.Fatal(err)
	}
}
