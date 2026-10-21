package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const pruneGuardItem = `{"id":"prune-guard","title":"Prune guard","acceptance":["prune aborts on a checked-out branch","a continuation-bound branch is never pruned"]}`

func consumedPruneGuard(t *testing.T) (projectRoot, workspace string) {
	t.Helper()
	projectRoot = t.TempDir()
	consumed := filepath.Join(projectRoot, ".evolve", "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	writeItem(t, consumed, "2026-09-30T00-00-00Z-prune-guard", pruneGuardItem)
	workspace = t.TempDir()
	writeWS(t, workspace, "triage-report.md", "## top_n\n- prune-guard: guard prune\n\n## deferred_acceptance\n- prune-guard: acceptance 2 — defer_reason=needs the continuation registry\n")
	return projectRoot, workspace
}

func TestCloseoutCarryover_OnlyAShippedCycleCarriesADeferredCriterion(t *testing.T) {
	t.Parallel()
	root, ws := consumedPruneGuard(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	failed, shipped := &State{}, &State{}

	o.closeoutCarryover(failed, ws, root, 1773, false, now)
	o.closeoutCarryover(shipped, ws, root, 1773, true, now)

	if len(failed.CarryoverTodos) != 0 || len(shipped.CarryoverTodos) != 1 || shipped.CarryoverTodos[0].ID != "prune-guard-acceptance-2" ||
		!strings.Contains(shipped.CarryoverTodos[0].Action, "a continuation-bound branch is never pruned") {
		t.Errorf("failed = %+v, shipped = %+v; a FAIL keeps the parent item, so only a ship carries the deferred criterion, with its words", failed.CarryoverTodos, shipped.CarryoverTodos)
	}
}

func TestFinalizeCycle_AFailedCycleCarriesNoDeferredCriterion(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		verdict string
		want    int
	}{{VerdictFAIL, 0}, {VerdictPASS, 1}} {
		root, ws := consumedPruneGuard(t)
		o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "post-ship", nil }}
		state := &State{}

		if _, err := o.finalizeCycle(context.Background(), CycleState{WorkspacePath: ws}, 1773, "pre-ship", root, &CycleResult{FinalVerdict: c.verdict}, state, nil); err != nil {
			t.Fatalf("finalizeCycle(%s): %v", c.verdict, err)
		}

		if len(state.CarryoverTodos) != c.want {
			t.Errorf("verdict %s: carryover = %+v, want %d todo(s): only a shipped cycle consumes the parent item", c.verdict, state.CarryoverTodos, c.want)
		}
	}
}

func TestResolvedDeferrals_OnlyARealPartOfTheAcceptanceIsDeferred(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeItem(t, dir, "prune-guard", pruneGuardItem)
	pathFor := func(id string) string {
		if id == "prune-guard" {
			return path
		}
		return ""
	}
	for name, c := range map[string]struct {
		cards string
		want  int
		warn  bool
	}{
		"in range, with its words":     {"- prune-guard: acceptance 2 — defer_reason=needs the continuation registry\n", 1, false},
		"past the end":                 {"- prune-guard: acceptance 3 — defer_reason=needs the continuation registry\n", 0, false},
		"the same criterion twice":     {"- prune-guard: acceptance 2 — defer_reason=needs the continuation registry\n- prune-guard: acceptance 2 — defer_reason=a second reason here\n", 1, false},
		"every criterion of the item":  {"- prune-guard: acceptance 1 — defer_reason=not this cycle either\n- prune-guard: acceptance 2 — defer_reason=needs the continuation registry\n", 0, true},
		"an item the host cannot read": {"- other-item: acceptance 1 — defer_reason=needs the continuation registry\n", 0, false},
	} {
		ws := t.TempDir()
		writeWS(t, ws, "triage-report.md", "## top_n\n- prune-guard: guard prune\n- other-item: other\n\n## deferred_acceptance\n"+c.cards)
		var errOut bytes.Buffer
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&errOut))

		got := o.resolvedDeferrals(ws, pathFor)

		if len(got) != c.want || (c.want == 1 && got[0].Criterion != "a continuation-bound branch is never pruned") || strings.Contains(errOut.String(), "deferred every acceptance criterion") != c.warn {
			t.Errorf("%s: deferrals = %+v, stderr = %q; want %d deferral(s), warn %t", name, got, errOut.String(), c.want, c.warn)
		}
	}
}

func TestItemPathFor_AConsumedItemIsFoundByItsID(t *testing.T) {
	t.Parallel()
	root, _ := consumedPruneGuard(t)
	consumed := filepath.Join(root, ".evolve", "inbox", "consumed")
	newest := writeItem(t, consumed, "2026-10-20T00-00-00Z-prune-guard", pruneGuardItem)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(_, id string) string {
		if id == "held" {
			return "/inbox/processing/held.json"
		}
		return ""
	}))

	got, none, held := o.itemPathFor(root, "prune-guard"), o.itemPathFor(root, "guard"), o.itemPathFor(root, "held")
	if got != newest || none != "" || held != "/inbox/processing/held.json" {
		t.Errorf("itemPathFor = %q, %q, %q; want the newest consumed record, nothing for an id that only ends a file name, and the resolver's path first", got, none, held)
	}
}
