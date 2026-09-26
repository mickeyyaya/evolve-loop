package core

import (
	"context"
	"errors"
	"testing"
)

func TestOrchestrator_CatalogRefresh_LedgerStampsOkOutcome(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners,
		WithCatalogRefresher(func(context.Context) error { return nil }),
		WithCatalogRefreshStage(func() string { return "shadow" }),
	)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}

	found := 0
	for _, e := range led.entries {
		if e.Kind == "catalog_refresh" {
			found++
			if e.Action != "ok" {
				t.Errorf("catalog_refresh Action = %q, want %q", e.Action, "ok")
			}
			if e.Message != "shadow" {
				t.Errorf("catalog_refresh Message (resolved refresh_stage) = %q, want %q", e.Message, "shadow")
			}
		}
	}
	if found != 1 {
		t.Fatalf("want exactly 1 catalog_refresh ledger entry, got %d", found)
	}
}

func TestOrchestrator_CatalogRefresh_LedgerStampsFailedOutcome(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners,
		WithCatalogRefresher(func(context.Context) error { return errors.New("refresh boom") }),
	)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle must not fail on refresher error (best-effort contract): %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}

	found := 0
	for _, e := range led.entries {
		if e.Kind == "catalog_refresh" {
			found++
			if e.Action != "failed" {
				t.Errorf("catalog_refresh Action = %q, want %q", e.Action, "failed")
			}
		}
	}
	if found != 1 {
		t.Fatalf("want exactly 1 catalog_refresh ledger entry on failure, got %d", found)
	}
}

func TestOrchestrator_CatalogRefresh_NilRefresherNoLedgerEntry(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners) // no WithCatalogRefresher

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	for _, e := range led.entries {
		if e.Kind == "catalog_refresh" {
			t.Errorf("nil catalogRefresh must not stamp the ledger; got %+v", e)
		}
	}
}

func TestOrchestrator_CatalogRefresh_NoStageAccessorLeavesStageEmpty(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners,
		WithCatalogRefresher(func(context.Context) error { return nil }),
	)

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	found := 0
	for _, e := range led.entries {
		if e.Kind == "catalog_refresh" {
			found++
			if e.Message != "" {
				t.Errorf("catalog_refresh Message with no stage accessor = %q, want empty", e.Message)
			}
		}
	}
	if found != 1 {
		t.Fatalf("want exactly 1 catalog_refresh ledger entry, got %d", found)
	}
}
