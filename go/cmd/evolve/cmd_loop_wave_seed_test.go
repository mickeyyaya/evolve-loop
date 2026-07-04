// cmd_loop_wave_seed_test.go — regression for the wave-planner fresh-start/reset
// blocker (inbox wave-planner-requires-prior-cycle-triage-decision, 0.9).
//
// productionWavePlanFn used to hard-error when the prior cycle's
// triage-decision.json was absent (fresh loop, `evolve cycle reset` sealed the
// run dir, or a failed prior cycle), forcing a sequential fallback that never
// runs 2-wide — AND is the only path that leaks into the main tree. It now seeds
// the wave from the durable inbox backlog. These tests pin the seed.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeInboxItem(t *testing.T, inbox, name, id string, weight float64) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"id": id, "weight": weight})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTopInboxTaskIDs_HighestWeightFirst_SkipsMalformedAndEmptyID(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, inbox, "a.json", "low", 0.30)
	writeInboxItem(t, inbox, "b.json", "high", 0.98)
	writeInboxItem(t, inbox, "c.json", "mid", 0.70)
	// Malformed JSON and an empty-id item (even at higher weight) must be skipped.
	if err := os.WriteFile(filepath.Join(inbox, "bad.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "noid.json"), []byte(`{"weight":0.99}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := topInboxTaskIDs(dir, 2)
	if len(got) != 2 || got[0] != "high" || got[1] != "mid" {
		t.Errorf("topInboxTaskIDs = %v, want [high mid] (highest weight first; malformed + empty-id skipped)", got)
	}
}

func TestSeedWavePlanFromInbox_SynthesizesTopNDecision(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, inbox, "a.json", "alpha", 0.90)
	writeInboxItem(t, inbox, "b.json", "beta", 0.80)

	data, err := seedWavePlanFromInbox(dir, 2)
	if err != nil {
		t.Fatalf("seedWavePlanFromInbox: %v", err)
	}
	// The synthesized decision must parse as fleet.PlanFromTriage expects (top_n[].id).
	var decision struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(data, &decision); err != nil {
		t.Fatalf("seed is not valid triage-decision JSON: %v", err)
	}
	if len(decision.TopN) < 2 || decision.TopN[0].ID != "alpha" {
		t.Errorf("seed top_n = %+v, want >= 2 entries with the highest-weight id (alpha) first", decision.TopN)
	}
}

func TestSeedWavePlanFromInbox_FewerThanTwoTodosErrors(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, inbox, "a.json", "only", 0.90)
	// One todo cannot fill a 2-lane wave — must error so the caller falls back to sequential.
	if _, err := seedWavePlanFromInbox(dir, 2); err == nil {
		t.Error("seedWavePlanFromInbox with 1 inbox todo must return an error (can't seed a >= 2-lane wave)")
	}
}

// TestProductionWavePlanFn_SeedsFromInboxWhenNoPriorDecision pins the load-bearing
// seam: when the prior cycle's triage-decision is unavailable (fresh start, reset,
// or a failed prior cycle — modeled here by a storage read error), the planFn
// falls through to the inbox seed instead of erroring the wave into a sequential
// fallback.
func TestProductionWavePlanFn_SeedsFromInboxWhenNoPriorDecision(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItem(t, inbox, "a.json", "alpha", 0.90)
	writeInboxItem(t, inbox, "b.json", "beta", 0.80)

	// erroringReadStorage makes readLastCycleNumber return an error → "no prior
	// cycle" → the planFn must seed from the inbox, not fall back to sequential.
	plan := productionWavePlanFn(loopConfig{ProjectRoot: dir, EvolveDir: dir}, &erroringReadStorage{}, 2)
	data, cards, err := plan(context.Background(), 0)
	if err != nil {
		t.Fatalf("planFn must seed from inbox on a missing prior decision, got error: %v", err)
	}
	if cards != nil {
		t.Errorf("cardPackages = %v, want nil (the seed is a top_n decision, not cards)", cards)
	}
	var d struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if json.Unmarshal(data, &d) != nil || len(d.TopN) < 2 || d.TopN[0].ID != "alpha" {
		t.Errorf("seeded decision = %s, want top_n with >= 2 ids (alpha first)", data)
	}
}
