// cmd_loop_rulesweep_test.go — R8.2: the batch-end I4 measured auto-enforce
// sweep. Bar: ≥5 would-fire signals across the batch ∧ zero anomalous
// outcomes for that rule ∧ the flip's own healthy-corpus re-validation.
// Until this sweep existed, "measured auto-enforce never fires" was true by
// construction.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// plantFires writes an interaction ledger with n would-fire lines (+ one
// anomalous line when bad) for ruleID into cycle c's workspace.
func plantFires(t *testing.T, root string, c, n int, ruleID string, bad bool) {
	t.Helper()
	ws := cycleWorkspace(root, c)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `{"kind":"rule_shadow_fire","rule_id":%q,"result":"would_fire"}`+"\n", ruleID)
	}
	if bad {
		fmt.Fprintf(&b, `{"kind":"rule_shadow_fire","rule_id":%q,"result":"contradicted"}`+"\n", ruleID)
	}
	if err := os.WriteFile(filepath.Join(ws, "build-interactions.ndjson"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func promoteSweepRule(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "instincts", "interaction-rules")
	id, err := interaction.PromoteRule(dir, "Do you want to overwrite the existing\\?", "n,Enter", "sweep test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func ruleStage(t *testing.T, root, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".evolve", "instincts", "interaction-rules", id+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ln := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(ln, "stage: "); ok {
			return v
		}
	}
	return ""
}

func TestSweepRulePromotions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		fires     int
		bad       bool
		wantStage string
	}{
		{name: "measured_clean_flips", fires: 5, wantStage: "enforce"},
		{name: "below_bar_stays_shadow", fires: 4, wantStage: "shadow"},
		{name: "anomaly_disqualifies", fires: 9, bad: true, wantStage: "shadow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			id := promoteSweepRule(t, root)
			// Spread the evidence across two cycles (the sweep aggregates).
			plantFires(t, root, 1, tc.fires/2, id, false)
			plantFires(t, root, 2, tc.fires-tc.fires/2, id, tc.bad)

			sweepRulePromotions(io.Discard, root, []core.CycleResult{{Cycle: 1}, {Cycle: 2}})

			if got := ruleStage(t, root, id); got != tc.wantStage {
				t.Fatalf("stage after sweep = %q, want %q (fires=%d bad=%v)", got, tc.wantStage, tc.fires, tc.bad)
			}
		})
	}
	// Sanity: the flip leaves the rule loadable in the enforce set.
	t.Run("flipped_rule_enters_enforce_set", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		id := promoteSweepRule(t, root)
		plantFires(t, root, 1, 5, id, false)
		sweepRulePromotions(io.Discard, root, []core.CycleResult{{Cycle: 1}})
		if err := bridge.EnforceMeasuredRule(root, id); err != nil {
			t.Fatalf("idempotent re-flip after sweep: %v", err)
		}
	})
}
