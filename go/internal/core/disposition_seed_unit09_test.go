package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 48 (ADR-0103 unit 09) — the adoption seeder decodes the ancestor ledger
// through the leaf: a leaf-written ledger with OPEN and FIXED rows seeds only
// the OPEN ids, byte-identical to the G6 skeleton captured on 8e8f080f; a
// directory at the ancestor ledger path seeds nothing; and the leaf's Read is
// Center-free — a carryover lifecycle reading the same broken file reports
// CARRYOVER_WORKSPACE_READ_FAILED and never an AUDIT_* code.
func TestSeedDispositionSkeleton_DecodesThroughTheLeaf_AndReadsAreSignalFree(t *testing.T) {
	root := t.TempDir()
	ws := RunWorkspacePath(root, 1431)
	ancestorWS := RunWorkspacePath(root, 1425)
	for _, d := range []string{ws, ancestorWS} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := defectledger.Write(ancestorWS, defectledger.Doc{OriginCycle: 1425, Entries: []defectledger.Entry{
		{ID: "d0f3a7c1e59b246d8a0c4e6f13579bde2", Text: "salvage parser drops fenced JSON", Status: defectledger.StatusOpen},
		{ID: "d9c8b7a6958473625140f3e2d1c0b9a87", Text: "closed upstream", Status: defectledger.StatusFixed, Evidence: "docs/x.md"},
		{ID: "d1111111111111111111111111111111f", Text: "quote parity", Status: defectledger.StatusOpen},
	}}); err != nil {
		t.Fatal(err)
	}
	SeedDispositionSkeleton(ws, root, 1425)
	got, err := os.ReadFile(filepath.Join(ws, defectledger.DispositionsFile))
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(filepath.Join("defectledger", "testdata", "seeded_skeleton.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(golden) {
		t.Fatalf("the seeded skeleton is byte-identical to the golden:\n%s", got)
	}

	broken := RunWorkspacePath(root, 1400)
	if err := os.MkdirAll(filepath.Join(broken, defectledger.LedgerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	ws2 := RunWorkspacePath(root, 1432)
	if err := os.MkdirAll(ws2, 0o755); err != nil {
		t.Fatal(err)
	}
	SeedDispositionSkeleton(ws2, root, 1400)
	if _, err := os.Stat(filepath.Join(ws2, defectledger.DispositionsFile)); err == nil {
		t.Fatal("an unreadable ancestor ledger seeds nothing")
	}
	c := signalcenter.New()
	var events []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	var state State
	carryover.New(carryover.WithSignals(func() *signalcenter.Center { return c })).MergePrescriptions(&state, broken, 1400, time.Now())
	if len(events) != 1 || events[0].Code != carryover.CodeWorkspaceReadFailed || events[0].Module != signalcenter.ModuleCarryover {
		t.Fatalf("one read fault ⇒ one code under the CALLER's module: %+v", events)
	}
}
