package component

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestStorageDurability_StatePersistsAcrossAdapterInstances(t *testing.T) {
	t.Parallel()
	ws := fixtures.NewWorkspace(t).Build()
	ctx := context.Background()

	writer := storage.New(ws.EvolveDir)
	fixtures.RequireNoErr(t,
		writer.WriteState(ctx, core.State{LastCycleNumber: 42, Version: 3}),
		"WriteState")

	reader := storage.New(ws.EvolveDir)
	got, err := reader.ReadState(ctx)
	fixtures.RequireNoErr(t, err, "ReadState")
	if got.LastCycleNumber != 42 || got.Version != 3 {
		t.Fatalf("reloaded state = %+v, want LastCycleNumber=42 Version=3", got)
	}
}
