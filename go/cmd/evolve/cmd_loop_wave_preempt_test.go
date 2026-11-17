package main

import (
	"context"
	"regexp"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var preemptLedgerLineRe = regexp.MustCompile(`^preempted (\S+) for (\S+) \(score [0-9]+\.[0-9]+ to [0-9]+\.[0-9]+\)$`)

func preemptLedgerMessages(t *testing.T, evolveDir string) []string {
	t.Helper()
	it, err := ledger.New(evolveDir).Iter(context.Background())
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	defer func() { _ = it.Close() }()
	var lines []string
	for {
		e, ok, err := it.Next()
		if err != nil {
			t.Fatalf("read a ledger entry: %v", err)
		}
		if !ok {
			return lines
		}
		if preemptLedgerLineRe.MatchString(e.Message) {
			lines = append(lines, e.Message)
		}
	}
}

func TestPreempt_ProductionWavePlanFnSwapsTheLowestSlotAndLedgersIt(t *testing.T) {
	t.Parallel()
	cfg := scopePruneEnv(t, 7, []map[string]any{
		{"id": "slot-low", "files": []string{"pkg/slot-low/x.go"}},
		{"id": "keep-high", "files": []string{"pkg/keep-high/x.go"}},
	})
	scopePruneItem(t, cfg, "", "slot-low", 0.3, "pkg/slot-low/x.go")
	scopePruneItem(t, cfg, "", "keep-high", 0.8, "pkg/keep-high/x.go")
	scopePruneItem(t, cfg, "", "fresh-top", 0.95, "pkg/fresh-top/x.go")
	seed := core.LedgerEntry{TS: "2026-11-17T00:00:00Z", Role: "orchestrator", Kind: "seed"}
	if err := ledger.New(cfg.EvolveDir).Append(context.Background(), seed); err != nil {
		t.Fatalf("fixture: seed the ledger: %v", err)
	}
	planned := planTopNIDs(t, cfg, 7, 2)
	slices.Sort(planned)
	if want := []string{"fresh-top", "keep-high"}; !slices.Equal(planned, want) {
		t.Errorf("planned ids = %v, want %v — the production plan path must preempt the lowest slot", planned, want)
	}
	lines := preemptLedgerMessages(t, cfg.EvolveDir)
	if len(lines) != 1 {
		t.Fatalf("ledger preempt lines = %q, want exactly one", lines)
	}
	if m := preemptLedgerLineRe.FindStringSubmatch(lines[0]); m == nil || m[1] != "slot-low" || m[2] != "fresh-top" {
		t.Errorf("ledger line %q, want `preempted slot-low for fresh-top (score a to b)`", lines[0])
	}
}
