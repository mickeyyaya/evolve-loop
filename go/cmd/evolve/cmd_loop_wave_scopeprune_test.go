package main

// cmd_loop_wave_scopeprune_test.go — RED tests for cycle-1172, inbox item
// `wave-planner-pass-scope-prune` (scout task wave-planner-plan-time-scope-prune).
//
// THE GAP: productionWavePlanFn's PRIMARY path reads the prior cycle's
// triage-decision.json and widens it (widenNarrowDecision). Nothing on that
// path re-resolves each top_n id against the inbox lifecycle, so an id that was
// consumed — moved to inbox/processed|rejected|retry/ — during an earlier wave
// is still planned into the NEXT wave's lane-scope.json (one confirmed
// instance: cycle-1116). The dispatch-time freshness gate
// (productionFreshnessProbe / freshnessGatedLauncher) then skips it, so the
// lane does not actually re-execute the dead work — but the PLAN is still
// wrong: lane-scope.json advertises work that no longer exists, which is what
// every downstream reader (operator, dossier, retro) sees.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//
//  1. Before the plan is returned, productionWavePlanFn drops every top_n id
//     whose inboxmover.ResolveDispatchState is a CONSUMED state (processed,
//     rejected, retry). Reuse ResolveDispatchState — the same resolver the
//     dispatch-time probe already uses. No second bookkeeping file (the inbox
//     item's own fix note).
//  2. Prune BEFORE widening, so the freed lane slots are refilled from the live
//     backlog instead of being lost.
//  3. FAIL OPEN: `pending` ids and ids with NO lifecycle evidence (`unknown` —
//     not every planned id is inbox-backed) are retained untouched. Over-
//     pruning would starve the wave, which is strictly worse than the stale
//     entry this fixes.
//  4. The dispatch-time freshness gate STAYS as defense in depth — this is a
//     plan-hygiene addition, not a replacement.

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// scopePruneEnv builds a project root whose .evolve/ carries an inbox and a
// prior cycle's triage-decision.json at cycleWorkspace(projectRoot, lastCycle).
// Returns the loopConfig the planner is built from.
func scopePruneEnv(t *testing.T, lastCycle int, topN []map[string]any) loopConfig {
	t.Helper()
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	for _, dir := range []string{
		filepath.Join(evolveDir, "inbox"),
		filepath.Join(evolveDir, "inbox", "processed"),
		cycleWorkspace(projectRoot, lastCycle),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(map[string]any{"top_n": topN})
	if err != nil {
		t.Fatal(err)
	}
	decision := filepath.Join(cycleWorkspace(projectRoot, lastCycle), "triage-decision.json")
	if err := os.WriteFile(decision, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}
}

// scopePruneItem writes an inbox item into `sub` ("" = the pending root,
// "processed" = consumed) with its own distinct file so lanes stay disjoint.
func scopePruneItem(t *testing.T, cfg loopConfig, sub, id string, weight float64, file string) {
	t.Helper()
	dir := filepath.Join(cfg.EvolveDir, "inbox")
	if sub != "" {
		dir = filepath.Join(dir, sub)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{"id": id, "weight": weight, "files": []string{file}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// planTopNIDs drives productionWavePlanFn's primary (prior-decision) path and
// returns the planned top_n ids.
func planTopNIDs(t *testing.T, cfg loopConfig, lastCycle, count int) []string {
	t.Helper()
	storage := &fixtures.FakeStorage{State: core.State{LastCycleNumber: lastCycle}}
	data, _, err := productionWavePlanFn(cfg, storage, count, io.Discard)(context.Background(), 1)
	if err != nil {
		t.Fatalf("wave plan: %v", err)
	}
	var decision struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(data, &decision); err != nil {
		t.Fatalf("planned decision is not valid JSON: %v (raw=%s)", err, data)
	}
	ids := make([]string, 0, len(decision.TopN))
	for _, c := range decision.TopN {
		ids = append(ids, c.ID)
	}
	return ids
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestProductionWavePlanFn_PrunesConsumedScopeFromPriorDecision is the crux
// (the cycle-1116 shape): `gamma` was consumed during the prior wave and now
// lives in inbox/processed/, yet the prior decision still lists it. The next
// wave's plan must not re-pick it. RED today: the primary path never consults
// the lifecycle, so gamma survives into lane-scope.json.
func TestProductionWavePlanFn_PrunesConsumedScopeFromPriorDecision(t *testing.T) {
	cfg := scopePruneEnv(t, 7, []map[string]any{
		{"id": "alpha", "files": []string{"go/internal/alpha/alpha.go"}},
		{"id": "gamma", "files": []string{"go/internal/gamma/gamma.go"}},
	})
	scopePruneItem(t, cfg, "", "alpha", 0.90, "go/internal/alpha/alpha.go")
	scopePruneItem(t, cfg, "", "beta", 0.80, "go/internal/beta/beta.go")
	scopePruneItem(t, cfg, "processed", "gamma", 0.70, "go/internal/gamma/gamma.go")

	ids := planTopNIDs(t, cfg, 7, 2)

	if containsID(ids, "gamma") {
		t.Errorf("wave plan re-picked consumed scope \"gamma\" (top_n=%v) — an id resolved to inbox/processed/ must be pruned at PLAN time, not merely skipped at dispatch", ids)
	}
	if !containsID(ids, "alpha") {
		t.Errorf("wave plan dropped still-pending scope \"alpha\" (top_n=%v) — pruning must be scoped to consumed ids", ids)
	}
	if len(ids) < 2 {
		t.Errorf("wave plan = %v — after pruning a consumed id the freed lane slot must be refilled from the live backlog (prune BEFORE widen), not lost", ids)
	}
}

// TestProductionWavePlanFn_KeepsPendingAndUnknownScopes is the fail-open
// NEGATIVE: a pending id and an id with NO inbox evidence at all must both
// survive. A prune that keeps only ids it can positively confirm as pending
// would starve every non-inbox-backed lane — strictly worse than the stale
// entry being fixed.
func TestProductionWavePlanFn_KeepsPendingAndUnknownScopes(t *testing.T) {
	cfg := scopePruneEnv(t, 4, []map[string]any{
		{"id": "alpha", "files": []string{"go/internal/alpha/alpha.go"}},
		{"id": "not-inbox-backed", "files": []string{"go/internal/manual/manual.go"}},
	})
	scopePruneItem(t, cfg, "", "alpha", 0.90, "go/internal/alpha/alpha.go")
	scopePruneItem(t, cfg, "", "beta", 0.80, "go/internal/beta/beta.go")

	ids := planTopNIDs(t, cfg, 4, 2)

	if !containsID(ids, "alpha") {
		t.Errorf("pending scope \"alpha\" was pruned (top_n=%v) — only CONSUMED ids may be dropped", ids)
	}
	if !containsID(ids, "not-inbox-backed") {
		t.Errorf("scope with no lifecycle evidence was pruned (top_n=%v) — the resolver's `unknown` state must fail OPEN; not every planned id is inbox-backed", ids)
	}
}

// TestProductionWavePlanFn_AllConsumedStillPlansLiveWork is the edge case: when
// EVERY id in the prior decision is consumed, the planner must not return an
// empty/dead plan (which collapses the wave to the sequential fallback — the
// only path that can leak into the main tree). It must widen from the live
// backlog instead.
func TestProductionWavePlanFn_AllConsumedStillPlansLiveWork(t *testing.T) {
	cfg := scopePruneEnv(t, 9, []map[string]any{
		{"id": "gamma", "files": []string{"go/internal/gamma/gamma.go"}},
		{"id": "delta", "files": []string{"go/internal/delta/delta.go"}},
	})
	scopePruneItem(t, cfg, "processed", "gamma", 0.70, "go/internal/gamma/gamma.go")
	scopePruneItem(t, cfg, "processed", "delta", 0.70, "go/internal/delta/delta.go")
	scopePruneItem(t, cfg, "", "alpha", 0.90, "go/internal/alpha/alpha.go")
	scopePruneItem(t, cfg, "", "beta", 0.80, "go/internal/beta/beta.go")

	ids := planTopNIDs(t, cfg, 9, 2)

	if containsID(ids, "gamma") || containsID(ids, "delta") {
		t.Errorf("wave plan = %v — every prior-decision id was consumed and must be pruned", ids)
	}
	if len(ids) < 2 {
		t.Errorf("wave plan = %v, want >= 2 live lanes widened from the pending backlog — a fully-consumed prior decision must not collapse the wave", ids)
	}
}
