//go:build acs

package cycle548

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeInboxItem(t *testing.T, path, id string) {
	t.Helper()
	body := []byte(`{"id":` + jsonQuote(id) + `,"weight":0.9,"kind":"feature"}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write inbox item %s: %v", path, err)
	}
}

func jsonQuote(s string) string { return `"` + s + `"` }

func TestC548_001_ReconcileSuperseded_RetiresDifferentlyNamedInboxItem(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	const id = "loop-self-prioritize-unmet-fleet-concurrency"
	const base = "2026-07-05T04-15-00Z-loop-self-prioritize-unmet-fleet-concurrency.json"
	writeInboxItem(t, filepath.Join(inbox, base), id)

	opts := inboxmover.Options{ProjectRoot: root}
	retired, err := inboxmover.ReconcileSuperseded(opts, []string{id}, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err != nil {
		t.Fatalf("ReconcileSuperseded returned error: %v", err)
	}
	if len(retired) != 1 || retired[0] != id {
		t.Fatalf("retired = %v, want exactly [%q] (the superseded id retired by id alone)", retired, id)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, base)); !os.IsNotExist(statErr) {
		t.Fatalf("superseded item still present in the live inbox root (stat err=%v) — it must be retired", statErr)
	}
	dest := filepath.Join(inbox, "processed", "cycle-548", base)
	if _, statErr := os.Stat(dest); statErr != nil {
		t.Fatalf("superseded item not retired to %s: %v", dest, statErr)
	}
}

func TestC548_002_ReconcileSuperseded_LeavesUndeclaredItemsInPlace(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	const retireBase = "2026-07-05T00-00-00Z-retire-me.json"
	const keepBase = "2026-07-06T00-00-00Z-keep-me.json"
	writeInboxItem(t, filepath.Join(inbox, retireBase), "retire-me")
	writeInboxItem(t, filepath.Join(inbox, keepBase), "keep-me")

	opts := inboxmover.Options{ProjectRoot: root}
	retired, err := inboxmover.ReconcileSuperseded(opts, []string{"retire-me"}, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err != nil {
		t.Fatalf("ReconcileSuperseded returned error: %v", err)
	}
	if len(retired) != 1 || retired[0] != "retire-me" {
		t.Fatalf("retired = %v, want exactly [\"retire-me\"] — undeclared ids must not be retired", retired)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, keepBase)); statErr != nil {
		t.Fatalf("UNDECLARED item keep-me was removed from the inbox root (%v) — reconciliation must be selective, never a blanket drain", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, retireBase)); !os.IsNotExist(statErr) {
		t.Fatalf("declared item retire-me still in the inbox root (stat err=%v) — it must be retired", statErr)
	}
}

func TestC548_003_SupersededInboxIDs_ParsesDedupsAndTolerates(t *testing.T) {
	body := []byte(`{"top_n":[{"id":"recover-x"}],"superseded":["loop-self-prioritize-unmet-fleet-concurrency","other-id","loop-self-prioritize-unmet-fleet-concurrency"]}`)
	got := inboxmover.SupersededInboxIDs(body)
	want := []string{"loop-self-prioritize-unmet-fleet-concurrency", "other-id"}
	if len(got) != len(want) {
		t.Fatalf("SupersededInboxIDs = %v, want %v (deduped, order-preserving)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SupersededInboxIDs[%d] = %q, want %q (order must be preserved, dups dropped)", i, got[i], want[i])
		}
	}
	if n := len(inboxmover.SupersededInboxIDs([]byte(`{"top_n":[{"id":"x"}]}`))); n != 0 {
		t.Fatalf("absent superseded field returned %d ids, want 0", n)
	}
	if n := len(inboxmover.SupersededInboxIDs([]byte(`}{ not json`))); n != 0 {
		t.Fatalf("invalid JSON returned %d ids, want 0 (must tolerate, never panic)", n)
	}
}

// acs-predicate: config-check
func TestC548_004_ObserverNotRebuilt_NoFleethealthLeaf(t *testing.T) {
	var _ = fleet.StarvationTracker{}
	root := acsassert.RepoRoot(t)
	leaf := filepath.Join(root, "go", "internal", "fleethealth")
	if _, err := os.Stat(leaf); err == nil {
		t.Fatalf("internal/fleethealth exists (%s) — the fleet-starvation observer already ships in internal/fleet; rebuilding it as a leaf resurrects the cycle-542 apicover-graduation regression", leaf)
	}
	src := filepath.Join(root, "go", "internal", "fleet", "starvation.go")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("internal/fleet/starvation.go absent (%v) — the shipped observer must be REUSED, not rebuilt or relocated (this cycle's fix is the inbox-reconciliation seam, not the observer)", err)
	}
}

func TestC548_005_ReconcileSuperseded_AbsentIDAndEmptyAreCleanNoOp(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	opts := inboxmover.Options{ProjectRoot: root}

	retired, err := inboxmover.ReconcileSuperseded(opts, []string{"never-existed"}, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err != nil {
		t.Fatalf("absent id returned error: %v (must be a clean no-op)", err)
	}
	if len(retired) != 0 {
		t.Fatalf("absent id retired %v, want none", retired)
	}

	retired2, err2 := inboxmover.ReconcileSuperseded(opts, nil, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err2 != nil {
		t.Fatalf("empty id list returned error: %v (must be a clean no-op)", err2)
	}
	if len(retired2) != 0 {
		t.Fatalf("empty id list retired %v, want none", retired2)
	}
}
