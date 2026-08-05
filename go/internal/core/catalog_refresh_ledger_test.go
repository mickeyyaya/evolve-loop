package core

// catalog_refresh_ledger_test.go — chain-summary-refresh-event-field: the
// cycle-start live-model-catalog refresh (cyclerun.go:586-593) is currently
// silent on success and stderr-only WARN on failure — no ledger entry, no
// summary field, unlike its sibling six lines below (operator_directives,
// cyclerun.go:599-613). The catalog.refresh_stage=shadow soak (memory:
// model_latest_selection) needs a queryable audit trail, not stderr
// scrollback. These tests pin a "catalog_refresh" ledger entry mirroring the
// operator_directives append pattern.

import (
	"context"
	"errors"
	"testing"
)

// TestOrchestrator_CatalogRefresh_LedgerStampsOkOutcome pins AC1: a
// successful refresh (nil error) appends exactly one catalog_refresh ledger
// entry stamped Action="ok", with the resolved catalog.refresh_stage carried
// in Message (via the WithCatalogRefreshStage accessor).
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

// TestOrchestrator_CatalogRefresh_LedgerStampsFailedOutcome pins AC2: a
// failed refresh still appends exactly one catalog_refresh entry, stamped
// Action="failed" — and, matching the existing best-effort contract, the
// cycle itself must NOT fail (RunCycle error nil, verdict PASS).
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

// TestOrchestrator_CatalogRefresh_NilRefresherNoLedgerEntry pins AC3: when
// no catalog refresher is wired (nil, the composition-root-optional default)
// planCycle must append ZERO catalog_refresh entries — a nil refresher never
// ran, so there is no outcome to stamp (distinct from a "skipped" outcome,
// which would require the refresher itself to signal a skip; the injected
// closure's contract is func(ctx) error and carries no skip signal).
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

// TestOrchestrator_CatalogRefresh_NoStageAccessorLeavesStageEmpty pins AC4:
// WithCatalogRefreshStage is optional — when unset, the entry is still
// appended (outcome is independent of stage observability) with an empty
// Message rather than a fabricated stage value.
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
