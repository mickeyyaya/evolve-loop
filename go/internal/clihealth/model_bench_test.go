package clihealth

import (
	"testing"
	"time"
)

var modelBenchNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func opusWeekWall(reset time.Time) Wall {
	return Wall{Pattern: UsageProbePattern, Evidence: "Opus week window 100% used, resets in 5h", Reset: reset}
}

func TestStore_AModelBenchIsNotAFamilyBench(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), func() time.Time { return modelBenchNow })
	reset := modelBenchNow.Add(5 * time.Hour)

	entry, err := store.BenchModelUntil("claude", "Opus", opusWeekWall(reset))
	if err != nil {
		t.Fatal(err)
	}

	if len(store.Active()) != 0 {
		t.Fatalf("Active() = %v; a model bench must never reach the family view the router reads", store.Active())
	}
	models := store.ActiveModels()
	got, ok := models[entry.Key()]
	if len(models) != 1 || !ok {
		t.Fatalf("ActiveModels() = %v, want exactly the claude/Opus bench keyed %q", models, entry.Key())
	}
	if got.Family != "claude" || got.Model != "Opus" || got.Reason != UsageProbePattern || got.Strikes != 1 {
		t.Errorf("model bench = %+v", got)
	}
	if want := reset.Add(resetMargin); !got.BenchedUntil.Equal(want) {
		t.Errorf("BenchedUntil = %v, want the reset plus the reset margin, %v", got.BenchedUntil, want)
	}
}

func TestEntryKey_AModelEntryNeverCollidesWithAFamilyOrAScope(t *testing.T) {
	t.Parallel()
	family := Entry{Family: "claude"}.Key()
	scoped := Entry{Family: "claude", Scope: "Opus"}.Key()
	model := Entry{Family: "claude", Model: "Opus"}.Key()
	if model == family || model == scoped || family == scoped {
		t.Fatalf("keys collide: family=%q scoped=%q model=%q", family, scoped, model)
	}
	if want := "claude\x00Opus"; model != want {
		t.Errorf("model Key = %q, want %q", model, want)
	}
}

func TestStore_AFamilyBenchAndItsModelBenchesCoexistWithTheirOwnStrikes(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), func() time.Time { return modelBenchNow })
	if _, err := store.BenchWall("claude", "rate_limit", "rate limited"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := store.BenchModelUntil("claude", "Opus", opusWeekWall(modelBenchNow.Add(5*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.BenchModelUntil("claude", "Fable", opusWeekWall(modelBenchNow.Add(3*time.Hour))); err != nil {
		t.Fatal(err)
	}

	family := store.Active()["claude"]
	if family.Strikes != 1 || family.Model != "" {
		t.Errorf("family bench = %+v, want the untouched 1-strike family bench", family)
	}
	models := store.ActiveModels()
	if len(models) != 2 {
		t.Fatalf("ActiveModels() = %v, want one bench per benched model", models)
	}
	if opus := models[Entry{Family: "claude", Model: "Opus"}.Key()]; opus.Strikes != 2 {
		t.Errorf("Opus strikes = %d, want 2 counted per model", opus.Strikes)
	}
	if fable := models[Entry{Family: "claude", Model: "Fable"}.Key()]; fable.Strikes != 1 {
		t.Errorf("Fable strikes = %d, want 1", fable.Strikes)
	}
}

func TestStore_AModelBenchSurvivesAReloadAndKeepsItsModel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := NewStore(root, func() time.Time { return modelBenchNow }).BenchModelUntil("claude", "Opus", opusWeekWall(modelBenchNow.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}

	reloaded := NewStore(root, func() time.Time { return modelBenchNow })

	for _, e := range reloaded.ActiveModels() {
		if e.Family != "claude" || e.Model != "Opus" {
			t.Errorf("reloaded entry = %+v, want claude/Opus", e)
		}
		return
	}
	t.Fatal("the model bench was lost on reload")
}

func TestStore_AModelBenchEndsAtItsResetAndNeverBecomesACanaryFamily(t *testing.T) {
	t.Parallel()
	clock := modelBenchNow
	store := NewStore(t.TempDir(), func() time.Time { return clock })
	if _, err := store.BenchModelUntil("claude", "Opus", opusWeekWall(modelBenchNow.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}

	clock = modelBenchNow.Add(2 * time.Hour)

	if len(store.ActiveModels()) != 0 {
		t.Errorf("ActiveModels() = %v after the reset, want none", store.ActiveModels())
	}
	if expired := store.Expired(); len(expired) != 0 {
		t.Errorf("Expired() = %v; the canary probes every expired key as a family, so a model bench must stay out until the model probe lands", expired)
	}
}

func TestStore_AModelBenchResetIsCappedLikeAFamilyBench(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), func() time.Time { return modelBenchNow })

	entry, err := store.BenchModelUntil("claude", "Opus", opusWeekWall(modelBenchNow.Add(100*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}

	if limit := modelBenchNow.Add(resetHintCap); entry.BenchedUntil.After(limit) {
		t.Errorf("BenchedUntil = %v, want at most %v (the %v reset-hint cap)", entry.BenchedUntil, limit, resetHintCap)
	}
}

func TestStore_ClearingAFamilyLeavesItsModelBenches(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), func() time.Time { return modelBenchNow })
	if _, err := store.BenchWall("claude", "rate_limit", "rate limited"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BenchModelUntil("claude", "Opus", opusWeekWall(modelBenchNow.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}

	if err := store.Clear("claude"); err != nil {
		t.Fatal(err)
	}

	if len(store.Active()) != 0 || len(store.ActiveModels()) != 1 {
		t.Errorf("Active=%v ActiveModels=%v; Clear(family) clears the family bench only", store.Active(), store.ActiveModels())
	}
}
