package audit

// disposition_seed_singlesource_test.go — pins core's disposition-skeleton
// preseed against THIS package's gate (ADR-0084 I2: writer and reader of a
// machine-graded artifact bind against each other). core cannot import audit,
// so its seeder re-reads the ledger wire shape; this test feeds one real
// document through both sides and proves:
//  1. same OPEN id set: every OPEN ancestor entry gets exactly one seeded row;
//  2. honest gate semantics on an UNTOUCHED skeleton: the preflight sees the
//     file as present and covering (never MISSING/INCOMPLETE), while the
//     per-id reconcile still blocks every seeded id by name — a seed the
//     auditor ignores can never launder a defect.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestDispositionSeed_BindsGateSemantics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1431")
	ancestorWS := filepath.Join(root, ".evolve", "runs", "cycle-1425")
	for _, d := range []string{ws, ancestorWS} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	openA := "d0f3a7c1e59b246d8a0c4e6f13579bde2"
	openB := "d1111111111111111111111111111111f"
	ledger := `{"origin_cycle":1425,"entries":[
		{"id":"` + openA + `","text":"salvage parser drops fenced JSON","status":"OPEN"},
		{"id":"d9c8b7a6958473625140f3e2d1c0b9a87","text":"closed upstream","status":"FIXED","evidence":"docs/x.md"},
		{"id":"` + openB + `","text":"quote parity","status":"OPEN"}]}`
	if err := os.WriteFile(filepath.Join(ancestorWS, defectLedgerFile), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 1425, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}

	// Adoption-side seed (the production entry point core wires at adoption).
	core.SeedDispositionSkeleton(ws, root, 1425)

	req := core.PhaseRequest{ProjectRoot: root, Workspace: ws}
	diags, blocking, _ := reconcileContinuationDefects(req)
	if !blocking {
		t.Fatal("an untouched skeleton PASSED the gate — the seed laundered the inherited defects")
	}
	joined := ""
	for _, d := range diags {
		joined += d.Message + "\n"
	}
	if strings.Contains(joined, dispositionPreflightMissingMarker) || strings.Contains(joined, dispositionPreflightIncompleteMarker) {
		t.Errorf("seeded skeleton still graded MISSING/INCOMPLETE — the seed did not cover the OPEN set:\n%s", joined)
	}
	for _, id := range []string{openA, openB} {
		if !strings.Contains(joined, id) {
			t.Errorf("seeded id %s not blocked by name — writer/reader OPEN-set drift:\n%s", id, joined)
		}
	}
	if strings.Contains(joined, "d9c8b7a6958473625140f3e2d1c0b9a87") {
		t.Error("non-OPEN ancestor entry surfaced in diagnostics — the seed over-covered")
	}
}
