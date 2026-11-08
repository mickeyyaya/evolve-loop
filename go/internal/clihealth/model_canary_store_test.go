package clihealth

import (
	"testing"
	"time"
)

func TestStore_ExpiredModelsListsOnlyModelBenchesPastTheirReset(t *testing.T) {
	t.Parallel()
	clock := modelBenchNow
	store := NewStore(t.TempDir(), func() time.Time { return clock })
	if _, err := store.BenchModelUntil("claude", "Opus", opusWeekWall(modelBenchNow.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BenchModelUntil("claude", "Fable", opusWeekWall(modelBenchNow.Add(10*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BenchWallUntil("codex", opusWeekWall(modelBenchNow.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}

	clock = modelBenchNow.Add(3 * time.Hour)

	expired := store.ExpiredModels()
	opus, ok := expired[Entry{Family: "claude", Model: "Opus"}.Key()]
	if len(expired) != 1 || !ok || opus.Model != "Opus" || opus.Family != "claude" {
		t.Fatalf("ExpiredModels() = %v, want only the claude/Opus bench", expired)
	}
	if _, family := store.Expired()["codex"]; !family {
		t.Error("Expired() lost the family bench, which the family canary still owns")
	}
}

func TestStore_ClearModelClearsOneModelBenchAndNothingElse(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), func() time.Time { return modelBenchNow })
	if _, err := store.BenchWall("claude", "rate_limit", "rate limited"); err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"Opus", "Fable"} {
		if _, err := store.BenchModelUntil("claude", model, opusWeekWall(modelBenchNow.Add(time.Hour))); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.ClearModel("claude", "Opus"); err != nil {
		t.Fatal(err)
	}

	models := store.ActiveModels()
	if _, gone := models[Entry{Family: "claude", Model: "Opus"}.Key()]; gone || len(models) != 1 {
		t.Errorf("ActiveModels() = %v, want only claude/Fable left", models)
	}
	if _, kept := store.Active()["claude"]; !kept {
		t.Error("ClearModel cleared the family bench")
	}
}

func TestStore_ClearModelOfAnAbsentBenchIsANoOp(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), func() time.Time { return modelBenchNow })
	if err := store.ClearModel("claude", "Opus"); err != nil {
		t.Errorf("ClearModel on an absent bench: %v", err)
	}
}
