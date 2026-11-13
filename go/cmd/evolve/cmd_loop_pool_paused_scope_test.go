package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestPausedPlan_APartlyGoneScopeIsNotPutFirstAndWarns(t *testing.T) {
	t.Parallel()
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"a", "b"}, AutoMaxAttempts: 3}}
	for _, present := range []string{"a", "b"} {
		evolveDir := t.TempDir()
		writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-"+present+".json"), `{"id":"`+present+`","files":["go/x.go"]}`)
		var warn bytes.Buffer

		got := pausedPlan{paused: paused, now: time.Now(), adoptable: inboxAdoptable(evolveDir, nil), itemFiles: inboxItemFiles(evolveDir), warn: &warn}.order(nil)

		if len(got) != 0 {
			t.Errorf("only %s located: backlog = %+v, want none", present, got)
		}
		if !strings.Contains(warn.String(), "paused cycle 9") || !strings.Contains(warn.String(), "a,b") {
			t.Errorf("only %s located: warn = %q, want a line that names cycle 9 and the scope", present, warn.String())
		}
	}
}

func TestPausedPlan_ATypedFilesItemTheLaneCanAdoptIsPutFirst(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-typed.json"), `{"id":"typed","files":5}`)
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"typed"}, AutoMaxAttempts: 3}}

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: inboxAdoptable(evolveDir, nil), itemFiles: inboxItemFiles(evolveDir)}.order(nil)

	if want := []fleet.Todo{{ID: "typed"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}

func TestPausedPlan_ANilWarnWriterDoesNotPanicOnAnUnreadableResetTime(t *testing.T) {
	t.Parallel()
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"w"}, AutoMaxAttempts: 3, QuotaResetAt: "soon"}}
	located := func(core.PausedScope) ([]string, error) { return nil, nil }

	got := pausedPlan{paused: paused, now: time.Now(), adoptable: adoptAll, itemFiles: located}.order(nil)

	if len(got) != 1 {
		t.Fatalf("backlog = %+v, want the scope once", got)
	}
}

func setResetAt(t *testing.T, statePath, value string) {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	state["checkpoint"].(map[string]any)["quotaResetAt"] = value
	out, _ := json.Marshal(state)
	if err := os.WriteFile(statePath, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedPoolInbox(t *testing.T, evolveDir string, extra map[string]string) {
	t.Helper()
	items := map[string]string{
		"2026-11-03T00-00-00Z-alpha.json": `{"id":"alpha","weight":0.9,"files":["a.go"]}`,
		"2026-11-03T00-00-01Z-beta.json":  `{"id":"beta","weight":0.8,"files":["b.go"]}`,
	}
	for name, body := range extra {
		items[name] = body
	}
	for name, body := range items {
		writeE2File(t, filepath.Join(evolveDir, "inbox", name), body)
	}
}

func TestProductionPoolPlanFn_AnUnreadableResetTimeWarnsOnTheStderrOfThePool(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-gamma.json": `{"id":"gamma","weight":0.1,"files":["go/internal/gamma/c.go"]}`})
	setResetAt(t, seedPausedRun(t, root, 41, 0, 3, "gamma"), "soon")
	var stderr bytes.Buffer
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, storage: &erroringReadStorage{}, stderr: &stderr}.plan(2)

	todos, err := plan(context.Background(), 0)

	if err != nil || len(todos) == 0 || todos[0].ID != "gamma" || !strings.Contains(stderr.String(), "paused cycle 41") {
		t.Fatalf("todos = %+v err %v stderr %q, want gamma first and a warning for cycle 41", todos, err, stderr.String())
	}
}

func TestProductionPoolPlanFn_APartlyGonePausedScopeIsSkippedWithAWarning(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-gamma.json": `{"id":"gamma","weight":0.1,"files":["go/internal/gamma/c.go"]}`})
	seedPausedRun(t, root, 41, 0, 3, "gamma", "gone")
	var stderr bytes.Buffer
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, storage: &erroringReadStorage{}, stderr: &stderr}.plan(2)

	todos, err := plan(context.Background(), 0)

	for _, td := range todos {
		if td.ID == "gamma,gone" {
			t.Fatalf("todos = %+v, want the partly gone scope not put first: the lane would refuse it", todos)
		}
	}
	if err != nil || !strings.Contains(stderr.String(), "paused cycle 41") {
		t.Fatalf("err %v stderr %q, want a warning that names cycle 41", err, stderr.String())
	}
}

func TestProductionPoolPlanFn_APausedScopeWithATypedFilesItemStillLeads(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	seedPoolInbox(t, evolveDir, map[string]string{"2026-11-03T00-00-02Z-typed.json": `{"id":"typed","weight":0.1,"files":5}`})
	seedPausedRun(t, root, 41, 0, 3, "typed")
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, storage: &erroringReadStorage{}, stderr: &bytes.Buffer{}}.plan(2)

	todos, err := plan(context.Background(), 0)

	if err != nil || len(todos) == 0 || todos[0].ID != "typed" {
		t.Fatalf("todos = %+v err %v, want typed first: a lane can adopt it", todos, err)
	}
}

func TestDeclaredFiles_AnItemThatIsGoneOrDoesNotDecodeIsAnErrorThatNamesIt(t *testing.T) {
	t.Parallel()
	inbox := filepath.Join(t.TempDir(), "inbox")
	writeE2File(t, filepath.Join(inbox, "2026-11-03T00-00-00Z-typed.json"), `{"id":"typed","files":5}`)
	writeE2File(t, filepath.Join(inbox, "2026-11-03T00-00-00Z-held.json"), `{"id":"held","files":["go/x.go"]}`)

	for _, id := range []string{"missing", "typed"} {
		if got, err := declaredFiles(inbox, id); err == nil || len(got) != 0 || !strings.Contains(err.Error(), id) {
			t.Errorf("declaredFiles(%s) = %v err %v, want an error that names the item", id, got, err)
		}
	}
	if got, err := declaredFiles(inbox, "held"); err != nil || !reflect.DeepEqual(got, []string{"go/x.go"}) {
		t.Errorf("declaredFiles(held) = %v err %v", got, err)
	}
}

func adoptAll(core.PausedScope) error { return nil }
