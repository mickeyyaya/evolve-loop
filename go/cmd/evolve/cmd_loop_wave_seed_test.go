package main

import (
	"context"
	"encoding/json"
	"io"
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
	if _, err := seedWavePlanFromInbox(dir, 2); err == nil {
		t.Error("seedWavePlanFromInbox with 1 inbox todo must return an error (can't seed a >= 2-lane wave)")
	}
}

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
	plan := productionWavePlanFn(loopConfig{ProjectRoot: dir, EvolveDir: dir}, &erroringReadStorage{}, 2, io.Discard)
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
