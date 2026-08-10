package core

// disposition_seed_test.go — RED contract for the disposition-skeleton preseed
// (inbox disposition-skeleton-preseed 0.9; 2026-08-10 investigation). The
// orchestrator KNOWS the inherited OPEN ids at adoption; making the auditor
// hand-enumerate them from the ancestor ledger was an avoidable failure
// surface (15/30 FAILs cycles 1390-1429 on disposition-preflight). The seam
// writes a skeleton — one status-OPEN entry per inherited OPEN id — that the
// gate treats as present-but-undispositioned (per-id block, never MISSING,
// never laundered: OPEN is not FIXED/DEFERRED, so nothing passes untouched).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func writeAncestorLedger(t *testing.T, root string, cycle int, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "defect-ledger.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSeedDispositionSkeleton_OpenIdsOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1431")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAncestorLedger(t, root, 1425, `{"origin_cycle":1425,"entries":[
		{"id":"d0f3a7c1e59b246d8a0c4e6f13579bde2","text":"salvage parser drops fenced JSON","status":"OPEN"},
		{"id":"d9c8b7a6958473625140f3e2d1c0b9a87","text":"closed upstream","status":"FIXED","evidence":"docs/x.md"},
		{"id":"d1111111111111111111111111111111f","text":"quote parity","status":"OPEN"}]}`)

	SeedDispositionSkeleton(ws, root, 1425)

	raw, err := os.ReadFile(filepath.Join(ws, "defect-dispositions.json"))
	if err != nil {
		t.Fatalf("skeleton not written: %v", err)
	}
	var doc struct {
		Dispositions []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"dispositions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("skeleton unparseable: %v", err)
	}
	if len(doc.Dispositions) != 2 {
		t.Fatalf("skeleton has %d entries, want the 2 OPEN ids only", len(doc.Dispositions))
	}
	for _, d := range doc.Dispositions {
		if d.Status != "OPEN" {
			t.Errorf("seeded entry %s status %q, want OPEN — a seeded DEFERRED/FIXED would launder the defect", d.ID, d.Status)
		}
	}
}

func TestSeedDispositionSkeleton_NeverClobbersAndNoOpCases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1431")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// Existing file (an agent or prior attempt wrote it): untouched.
	existing := filepath.Join(ws, "defect-dispositions.json")
	if err := os.WriteFile(existing, []byte(`{"dispositions":[{"id":"dx","status":"FIXED","evidence":"docs/x.md"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAncestorLedger(t, root, 1425, `{"origin_cycle":1425,"entries":[{"id":"d0f3a7c1e59b246d8a0c4e6f13579bde2","text":"x","status":"OPEN"}]}`)
	SeedDispositionSkeleton(ws, root, 1425)
	raw, _ := os.ReadFile(existing)
	if string(raw) != `{"dispositions":[{"id":"dx","status":"FIXED","evidence":"docs/x.md"}]}` {
		t.Error("seed clobbered an existing dispositions file")
	}

	// No OPEN entries in the ancestor: no file minted.
	root2 := t.TempDir()
	ws2b := filepath.Join(root2, ".evolve", "runs", "cycle-1432")
	if err := os.MkdirAll(ws2b, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAncestorLedger(t, root2, 1425, `{"origin_cycle":1425,"entries":[{"id":"da","text":"x","status":"FIXED","evidence":"docs/x.md"}]}`)
	SeedDispositionSkeleton(ws2b, root2, 1425)
	if _, err := os.Stat(filepath.Join(ws2b, "defect-dispositions.json")); !os.IsNotExist(err) {
		t.Error("seed minted a skeleton for an ancestor with no OPEN entries")
	}

	// Absent/unreadable ancestor ledger: silent no-op (the GATE blocks loudly).
	root3 := t.TempDir()
	ws3 := filepath.Join(root3, ".evolve", "runs", "cycle-1433")
	if err := os.MkdirAll(ws3, 0o755); err != nil {
		t.Fatal(err)
	}
	SeedDispositionSkeleton(ws3, root3, 1425)
	if _, err := os.Stat(filepath.Join(ws3, "defect-dispositions.json")); !os.IsNotExist(err) {
		t.Error("seed minted a skeleton with no ancestor ledger")
	}
}
