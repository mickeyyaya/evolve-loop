package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func holdPlan(evolveDir string, paused []core.PausedScope, warn *bytes.Buffer) pausedPlan {
	return pausedPlan{paused: paused, now: time.Now(), adoptable: inboxAdoptable(evolveDir), itemFiles: inboxItemFiles(evolveDir), warn: warn}
}

func seedHoldInbox(t *testing.T, evolveDir string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-"+id+".json"), `{"id":"`+id+`","files":["go/`+id+`.go"]}`)
	}
}

func claimFor(t *testing.T, evolveDir string, cycle int, ids ...string) {
	t.Helper()
	if err := hostInboxClaimer(nil, nil, nil)(filepath.Join(evolveDir, "inbox"), cycle, ids); err != nil {
		t.Fatal(err)
	}
}

func TestPausedPlan_AScopeThatAnotherCycleHoldsIsNotPutFirstAndWarns(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	seedHoldInbox(t, evolveDir, "x")
	claimFor(t, evolveDir, 8, "x")
	var warn bytes.Buffer

	got := holdPlan(evolveDir, []core.PausedScope{{Cycle: 9, Scope: []string{"x"}, AutoMaxAttempts: 3}}, &warn).order(nil)

	if len(got) != 0 || !strings.Contains(warn.String(), "paused cycle 9") || !strings.Contains(warn.String(), "cycle 8 holds") {
		t.Fatalf("backlog = %+v warn %q, want none and a warning that names cycles 9 and 8", got, warn.String())
	}
}

func TestPausedPlan_AScopeThePausedCycleHoldsItselfIsPutFirst(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	seedHoldInbox(t, evolveDir, "x")
	claimFor(t, evolveDir, 9, "x")

	got := holdPlan(evolveDir, []core.PausedScope{{Cycle: 9, Scope: []string{"x"}, AutoMaxAttempts: 3}}, &bytes.Buffer{}).order(nil)

	if want := []fleet.Todo{{ID: "x", Files: []string{"go/x.go"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}

func TestPausedPlan_EachItemOfAnUntriagedScopeAddsItsFiles(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	seedHoldInbox(t, evolveDir, "a", "b")

	got := holdPlan(evolveDir, []core.PausedScope{{Cycle: 9, Scope: []string{"a", "b"}, AutoMaxAttempts: 3}}, &bytes.Buffer{}).order(nil)

	if want := []fleet.Todo{{ID: "a,b", Files: []string{"go/a.go", "go/b.go"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}

func TestPausedPlan_ATriagedScopeTheLaneRefusesKeepsItsTriagePlaceAndWarns(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	seedHoldInbox(t, evolveDir, "a")
	triage := []fleet.Todo{{ID: "a", Files: []string{"go/a.go"}}, {ID: "z"}}
	var warn bytes.Buffer

	got := holdPlan(evolveDir, []core.PausedScope{{Cycle: 9, Scope: []string{"a", "b"}, AutoMaxAttempts: 3}}, &warn).order(triage)

	if !reflect.DeepEqual(got, triage) || !strings.Contains(warn.String(), "paused cycle 9") || !strings.Contains(warn.String(), "a,b") {
		t.Fatalf("backlog = %+v warn %q, want the triage order and a warning that names cycle 9 and a,b", got, warn.String())
	}
}

func TestPausedPlan_ALostFileClaimWarnsAndNamesTheItem(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-typed.json"), `{"id":"typed","files":5}`)
	var warn bytes.Buffer

	got := holdPlan(evolveDir, []core.PausedScope{{Cycle: 9, Scope: []string{"typed"}, AutoMaxAttempts: 3}}, &warn).order(nil)

	if want := []fleet.Todo{{ID: "typed"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
	if !strings.Contains(warn.String(), "paused cycle 9") || !strings.Contains(warn.String(), "typed") {
		t.Fatalf("warn = %q, want a line that names cycle 9 and the item typed", warn.String())
	}
}

func TestInboxAdoptable_RefusesAnEmptyScope(t *testing.T) {
	t.Parallel()
	if err := inboxAdoptable(t.TempDir())(core.PausedScope{Cycle: 3}); err == nil {
		t.Fatal("an empty scope must be refused")
	}
}

func TestScopeHold_AdoptableByTakesAPendingItemAndHeldByDoesNot(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	seedHoldInbox(t, evolveDir, "p", "m")
	claimFor(t, evolveDir, 4, "m")
	inbox := filepath.Join(evolveDir, "inbox")

	if err := (scopeHold{inbox: inbox, scope: []string{"p", "m"}}).adoptableBy(4); err != nil {
		t.Errorf("adoptableBy(4) = %v, want nil", err)
	}
	if err := (scopeHold{inbox: inbox, scope: []string{"p", "m"}}).heldBy(4); err == nil || !strings.Contains(err.Error(), "p is not claimed by cycle 4") {
		t.Errorf("heldBy(4) = %v, want a refusal that names p", err)
	}
	if err := (scopeHold{inbox: inbox, scope: []string{"m"}}).adoptableBy(5); err == nil || !strings.Contains(err.Error(), "cycle 4 holds it") {
		t.Errorf("adoptableBy(5) = %v, want a refusal that names cycle 4", err)
	}
}

func poolPlanFor(t *testing.T, root, evolveDir string, stderr *bytes.Buffer) []fleet.Todo {
	t.Helper()
	plan := productionPoolPlanFn(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, &erroringReadStorage{}, 2, stderr)
	todos, err := plan(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return todos
}

func TestProductionPoolPlanFn_AScopeThatAnotherCycleHoldsIsNotPutFirst(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-gamma.json": `{"id":"gamma","weight":0.1,"files":["go/internal/gamma/c.go"]}`})
	claimFor(t, evolveDir, 8, "gamma")
	seedPausedRun(t, root, 8, 0, 3, "other")
	seedPausedRun(t, root, 41, 0, 3, "gamma")
	var stderr bytes.Buffer

	todos := poolPlanFor(t, root, evolveDir, &stderr)

	for _, td := range todos {
		if td.ID == "gamma" {
			t.Fatalf("todos = %+v stderr %q, want gamma not put first: cycle 8 holds it", todos, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), "paused cycle 41") || !strings.Contains(stderr.String(), "cycle 8 holds") {
		t.Fatalf("stderr %q, want a warning for cycle 41 that names cycle 8", stderr.String())
	}
}

func TestProductionPoolPlanFn_AScopeThePausedCycleHoldsStillLeads(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-gamma.json": `{"id":"gamma","weight":0.1,"files":["go/internal/gamma/c.go"]}`})
	claimFor(t, evolveDir, 41, "gamma")
	seedPausedRun(t, root, 41, 0, 3, "gamma")

	todos := poolPlanFor(t, root, evolveDir, &bytes.Buffer{})

	if len(todos) == 0 || todos[0].ID != "gamma" || !reflect.DeepEqual(todos[0].Files, []string{"go/internal/gamma/c.go"}) {
		t.Fatalf("todos = %+v, want gamma first with its files: cycle 41 holds it", todos)
	}
}

func TestProductionPoolPlanFn_ATriagedScopeThatIsPartlyGoneKeepsItsTriagePlace(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, nil)
	seedPausedRun(t, root, 41, 0, 3, "alpha", "gone")
	var stderr bytes.Buffer

	todos := poolPlanFor(t, root, evolveDir, &stderr)

	for _, td := range todos {
		if td.ID == "alpha,gone" {
			t.Fatalf("todos = %+v, want the scope not put first: the lane would refuse it", todos)
		}
	}
	if !strings.Contains(stderr.String(), "paused cycle 41") {
		t.Fatalf("stderr %q, want a warning for cycle 41", stderr.String())
	}
}

func TestProductionPoolPlanFn_ALostFileClaimWarnsOnTheStderrOfThePool(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-typed.json": `{"id":"typed","weight":0.1,"files":5}`})
	seedPausedRun(t, root, 41, 0, 3, "typed")
	var stderr bytes.Buffer

	todos := poolPlanFor(t, root, evolveDir, &stderr)

	if len(todos) == 0 || todos[0].ID != "typed" || !strings.Contains(stderr.String(), "paused cycle 41") || !strings.Contains(stderr.String(), "typed") {
		t.Fatalf("todos = %+v stderr %q, want typed first and a warning that names cycle 41 and typed", todos, stderr.String())
	}
}
