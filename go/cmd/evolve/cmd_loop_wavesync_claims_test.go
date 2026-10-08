package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncMainAtWaveBoundary_OriginsEditOfAClaimedItemGoesIntoTheClaim(t *testing.T) {
	origin, runtime := syncFixture(t)
	item := filepath.Join(".evolve", "inbox", "held.json")
	commitOnOrigin(t, origin, item, `{"id":"held","weight":0.5}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	claim := filepath.Join(runtime, ".evolve", "inbox", "processing", "cycle-1836", "held.json")
	if err := os.MkdirAll(filepath.Dir(claim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(runtime, item), claim); err != nil {
		t.Fatal(err)
	}
	commitOnOrigin(t, origin, item, `{"id":"held","weight":0.9}`)
	var warn bytes.Buffer

	if synced, halt := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); !synced || halt != nil {
		t.Fatalf("a claim must not block the fast-forward: synced=%v halt=%v\n%s", synced, halt, warn.String())
	}

	if _, err := os.Stat(filepath.Join(runtime, item)); !os.IsNotExist(err) {
		t.Errorf("the fast-forward wrote the claimed item back to the root (%v): a duplicate the planner would offer", err)
	}
	if got, _ := os.ReadFile(claim); string(got) != `{"id":"held","weight":0.5}` {
		t.Errorf("the claim = %s, want the claim copy untouched", got)
	}
	parked := filepath.Join(runtime, ".evolve", "inbox", "origin-conflicts", "cycle-1836", "held.json")
	if got, _ := os.ReadFile(parked); string(got) != `{"id":"held","weight":0.9}` {
		t.Errorf("origin's copy = %s, want it parked", got)
	}
	if !strings.Contains(warn.String(), "[loop] wave-boundary sync: held differs from the claim of cycle-1836: the claim stays, and origin's copy is parked at "+parked) {
		t.Errorf("warn = %q, want the absorb line", warn.String())
	}
}

func TestSyncMainAtWaveBoundary_AClaimDirThatRefusesTheMoveWarns(t *testing.T) {
	origin, runtime := syncFixture(t)
	item := filepath.Join(".evolve", "inbox", "held.json")
	commitOnOrigin(t, origin, item, `{"id":"held","weight":0.5}`)
	gitrun(t, runtime, "pull", "-q", "--ff-only", "origin", "main")
	claim := filepath.Join(runtime, ".evolve", "inbox", "processing", "cycle-1836", "held.json")
	if err := os.MkdirAll(filepath.Dir(claim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(runtime, item), claim); err != nil {
		t.Fatal(err)
	}
	commitOnOrigin(t, origin, item, `{"id":"held","weight":0.9}`)
	if err := os.WriteFile(filepath.Join(runtime, ".evolve", "inbox", "origin-conflicts"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer

	if synced, _ := syncMainFromOriginAtWaveBoundary(context.Background(), runtime, &warn); !synced {
		t.Fatalf("the fast-forward itself succeeds: %s", warn.String())
	}

	if !strings.Contains(warn.String(), "[loop] WARN: wave-boundary sync: absorb held: park origin's copy") {
		t.Errorf("warn = %q, want one WARN naming the failed move", warn.String())
	}
}
