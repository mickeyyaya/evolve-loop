package component

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// Component tier example. This wires the REAL filesystem storage adapter
// against a fixtures-built workspace and pins a property no single-function
// unit test covers: state written through one adapter instance survives and
// reloads through a SECOND instance pointed at the same .evolve directory —
// i.e. durability is on disk, not in the adapter's memory.
//
// Pattern to copy when adding a component test:
//  1. ws := fixtures.NewWorkspace(t).Build()   // isolated temp .evolve/
//  2. construct the real adapter(s) under test against ws.EvolveDir
//  3. act, then re-construct/re-read to assert the cross-cutting property
func TestStorageDurability_StatePersistsAcrossAdapterInstances(t *testing.T) {
	t.Parallel()
	ws := fixtures.NewWorkspace(t).Build()
	ctx := context.Background()

	writer := storage.New(ws.EvolveDir)
	fixtures.RequireNoErr(t,
		writer.WriteState(ctx, core.State{LastCycleNumber: 42, Version: 3}),
		"WriteState")

	// A fresh adapter instance reads from disk, not from the writer's memory.
	reader := storage.New(ws.EvolveDir)
	got, err := reader.ReadState(ctx)
	fixtures.RequireNoErr(t, err, "ReadState")
	if got.LastCycleNumber != 42 || got.Version != 3 {
		t.Fatalf("reloaded state = %+v, want LastCycleNumber=42 Version=3", got)
	}
}
