package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func seedPausedRun(t *testing.T, root string, cycle int, attempts, maxAttempts int, scope ...string) string {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pin, _ := json.Marshal(core.LaneScope{TodoIDs: scope, GoalHash: "wave-goal"})
	if err := os.WriteFile(filepath.Join(dir, core.LaneScopeFile), pin, 0o644); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{"cycle_id": cycle, "phase": "build", "checkpoint": map[string]any{
		"enabled": true, "reason": "quota-likely", "resumeFromPhase": "build", "worktreePath": t.TempDir(),
		"savedAt": time.Now().UTC().Format(time.RFC3339), "autoResumeAttempts": attempts, "autoResumeMaxAttempts": maxAttempts,
		"operatorAction": "no quota reset time is known",
	}}
	raw, _ := json.Marshal(state)
	path := filepath.Join(dir, core.CycleStateFile)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCycleResumeAll_GrantsEveryPausedCycle(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	paths := []string{
		seedPausedRun(t, root, 3, 0, 0, "item-a"),
		seedPausedRun(t, root, 4, 0, 0, "item-b"),
		seedPausedRun(t, root, 5, 0, 0, "item-c"),
	}
	for i, p := range paths {
		if _, admitted, err := checkpoint.RecordResume(p, true); err != nil || admitted {
			t.Fatalf("cycle %d before the grant = (admitted %v, err %v), want refused", i+3, admitted, err)
		}
	}
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	for i, p := range paths {
		if _, admitted, err := checkpoint.RecordResume(p, true); err != nil || !admitted {
			t.Errorf("cycle %d after the grant = (admitted %v, err %v), want admitted", i+3, admitted, err)
		}
	}
	out := stdout.String()
	for _, want := range []string{"cycle 3 phase build scope item-a granted", "cycle 4 phase build scope item-b granted", "cycle 5 phase build scope item-c granted", "evolve wave next"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want it to hold %q", out, want)
		}
	}
}

func TestCycleResumeAll_DryRunGrantsNothing(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	path := seedPausedRun(t, root, 3, 0, 0, "item-a")
	before, _ := os.ReadFile(path)
	var stdout, stderr bytes.Buffer

	seedPausedRun(t, root, 4, 1, 3, "item-b")
	rc := runCycle([]string{"resume", "--all", "--dry-run", "--project-root", root}, nil, &stdout, &stderr)

	after, _ := os.ReadFile(path)
	if !strings.Contains(stdout.String(), "cycle 4 phase build scope item-b has budget") {
		t.Errorf("stdout = %q, want the dry run to show that cycle 4 still has budget", stdout.String())
	}
	if rc != cliExitOK || !bytes.Equal(before, after) || !strings.Contains(stdout.String(), "cycle 3 phase build scope item-a would grant") {
		t.Fatalf("rc = %d changed = %v stdout = %q", rc, !bytes.Equal(before, after), stdout.String())
	}
}

func TestCycleResumeAll_JSONListsEachCycle(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 3, 0, 0, "item-a", "item-b")
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var got cycleResumeAllResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || rc != cliExitOK {
		t.Fatalf("rc = %d err = %v stdout = %s", rc, err, stdout.String())
	}
	want := []cycleResumeGrant{{Cycle: 3, Phase: "build", Scope: []string{"item-a", "item-b"}, Granted: true}}
	if got.DryRun || len(got.Cycles) != 1 || got.Cycles[0].Cycle != want[0].Cycle || !got.Cycles[0].Granted || strings.Join(got.Cycles[0].Scope, ",") != "item-a,item-b" {
		t.Fatalf("result = %+v, want %+v", got, want)
	}
}

func TestCycleResumeAll_AllWithCycleIsAUsageError(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--cycle", "3", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitUsage || !strings.Contains(stderr.String(), "--all") {
		t.Fatalf("rc = %d stderr = %q, want a usage error", rc, stderr.String())
	}
}

func TestCycleResumeAll_NothingPausedExitsOK(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitOK || !strings.Contains(stdout.String(), "no paused cycle") {
		t.Fatalf("rc = %d stdout = %q", rc, stdout.String())
	}
}

func TestCycleResumeAll_AGrantErrorIsReportedAndTheOthersAreStillGranted(t *testing.T) {
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 3, 0, 0, "item-a")
	good := seedPausedRun(t, root, 4, 0, 0, "item-b")
	prev := cycleGrantResume
	t.Cleanup(func() { cycleGrantResume = prev })
	cycleGrantResume = func(path string) (checkpoint.Checkpoint, bool, error) {
		if strings.Contains(path, "cycle-3") {
			return checkpoint.Checkpoint{}, false, errors.New("disk full")
		}
		return prev(path)
	}
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--project-root", root}, nil, &stdout, &stderr)

	if _, admitted, err := checkpoint.RecordResume(good, true); err != nil || !admitted {
		t.Errorf("cycle 4 after the grant = (admitted %v, err %v), want admitted", admitted, err)
	}
	if rc == cliExitOK || !strings.Contains(stderr.String(), "cycle 3") || !strings.Contains(stderr.String(), "disk full") || !strings.Contains(stdout.String(), "cycle 3 phase build scope item-a not granted") {
		t.Fatalf("rc = %d stdout = %q stderr = %q, want the cycle 3 error, the line 'not granted' and a non-zero exit", rc, stdout.String(), stderr.String())
	}
}

func TestPausedPlan_PutsReadyPausesFirstAndKeepsTheirTriageTodo(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	triage := []fleet.Todo{
		{ID: "fresh", Files: []string{"f.go"}},
		{ID: "spent", Files: []string{"s.go"}},
		{ID: "granted", Files: []string{"a.go"}, Priority: 5, OutputContract: "pr", DependsOn: []string{"x"}},
		{ID: "walled", Files: []string{"w.go"}},
	}
	paused := []core.PausedScope{
		{Cycle: 1, Scope: []string{"spent"}, AutoAttempts: 3, AutoMaxAttempts: 3},
		{Cycle: 2, Scope: []string{"granted"}, AutoMaxAttempts: 3},
		{Cycle: 3, Scope: []string{"walled"}, AutoMaxAttempts: 3, QuotaResetAt: now.Add(time.Hour).Format(time.RFC3339)},
		{Cycle: 4, Scope: []string{"untriaged"}, AutoMaxAttempts: 3, QuotaResetAt: now.Add(-time.Hour).Format(time.RFC3339)},
	}
	files := map[string][]string{"untriaged": {"u.go", "u.go", "t.go"}}

	got := pausedPlan{paused: paused, now: now, itemFiles: func(id string) ([]string, bool) { f, ok := files[id]; return f, ok }}.order(triage)

	want := []fleet.Todo{
		{ID: "granted", Files: []string{"a.go"}, Priority: 5, OutputContract: "pr", DependsOn: []string{"x"}},
		{ID: "untriaged", Files: []string{"t.go", "u.go"}},
		{ID: "fresh", Files: []string{"f.go"}},
		{ID: "spent", Files: []string{"s.go"}},
		{ID: "walled", Files: []string{"w.go"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v\nwant      %+v\n(a ready pause leads with its full triage todo and its files; a spent budget and a future reset keep their triage place)", got, want)
	}
}

func TestPausedPlan_AMultiTodoScopeLeadsWithTheFilesOfEachOfItsTriageTodos(t *testing.T) {
	t.Parallel()
	triage := []fleet.Todo{{ID: "a", Files: []string{"a.go"}}, {ID: "other", Files: []string{"o.go"}}, {ID: "b", Files: []string{"b.go", "a.go"}}}
	paused := []core.PausedScope{{Cycle: 9, Scope: []string{"a", "b"}, AutoMaxAttempts: 3}}

	got := pausedPlan{paused: paused, now: time.Now(), itemFiles: func(string) ([]string, bool) { return nil, false }}.order(triage)

	want := []fleet.Todo{{ID: "a,b", Files: []string{"a.go", "b.go"}}, {ID: "other", Files: []string{"o.go"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backlog = %+v, want %+v", got, want)
	}
}

func TestInboxItemFiles_ReadsTheDeclaredFilesOfAPendingItem(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-held.json"), `{"id":"held","files":["go/x.go"]}`)
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-broken.json"), `{"id":"broken","files":`)
	read := inboxItemFiles(evolveDir)

	if got, ok := read("held"); !ok || !reflect.DeepEqual(got, []string{"go/x.go"}) {
		t.Errorf("files of held = %v located %v", got, ok)
	}
	if got, ok := read("missing"); ok || got != nil {
		t.Errorf("files of a missing item = %v located %v, want none", got, ok)
	}
	if got, ok := read("broken"); ok || got != nil {
		t.Errorf("files of an unreadable item = %v located %v, want none", got, ok)
	}
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-typed.json"), `{"id":"typed","files":5}`)
	if got, ok := read("typed"); ok || got != nil {
		t.Errorf("files of an item that does not decode = %v, want none", got)
	}
}

func TestProductionPoolPlanFn_AGrantedPausedLaneLeadsTheRealBacklogWithItsFiles(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	for name, body := range map[string]string{
		"2026-11-03T00-00-00Z-alpha.json": `{"id":"alpha","weight":0.9,"files":["a.go"]}`,
		"2026-11-03T00-00-01Z-beta.json":  `{"id":"beta","weight":0.8,"files":["b.go"]}`,
		"2026-11-03T00-00-02Z-gamma.json": `{"id":"gamma","weight":0.1,"files":["go/internal/gamma/c.go"]}`,
	} {
		writeE2File(t, filepath.Join(evolveDir, "inbox", name), body)
	}
	seedPausedRun(t, root, 41, 0, 3, "gamma")
	plan := productionPoolPlanFn(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, &erroringReadStorage{}, 2, &bytes.Buffer{})

	todos, err := plan(context.Background(), 0)

	if err != nil || len(todos) == 0 || todos[0].ID != "gamma" || !reflect.DeepEqual(todos[0].Files, []string{"go/internal/gamma/c.go"}) {
		t.Fatalf("todos = %+v, %v, want the paused lane gamma first with its file go/internal/gamma/c.go", todos, err)
	}
	for _, td := range todos[1:] {
		if td.ID == "gamma" {
			t.Fatalf("todos = %+v, want gamma once", todos)
		}
	}
}

func TestProductionPoolPlanFn_AFailedPausedScanWarnsAndKeepsTheTriageOrder(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-00Z-alpha.json"), `{"id":"alpha","weight":0.9,"files":["a.go"]}`)
	writeE2File(t, filepath.Join(evolveDir, "inbox", "2026-11-03T00-00-01Z-beta.json"), `{"id":"beta","weight":0.8,"files":["b.go"]}`)
	writeE2File(t, filepath.Join(evolveDir, "runs"), "not a directory")
	var stderr bytes.Buffer
	plan := productionPoolPlanFn(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, &erroringReadStorage{}, 2, &stderr)

	todos, err := plan(context.Background(), 0)

	if err != nil || len(todos) != 2 || todos[0].ID != "alpha" || !strings.Contains(stderr.String(), "scan for paused lane cycles failed") {
		t.Fatalf("todos = %+v err %v stderr %q, want the triage order and a WARN", todos, err, stderr.String())
	}
}

func TestCycleResumeAll_AScanFailureIsAnIOError(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	writeE2File(t, filepath.Join(root, ".evolve", "runs"), "not a directory")
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--project-root", root}, nil, &stdout, &stderr)

	if rc != cliExitIO || !strings.Contains(stderr.String(), "cannot list the paused cycles") {
		t.Fatalf("rc = %d stderr = %q, want cliExitIO and the scan error", rc, stderr.String())
	}
}

func TestCycleResumeAll_ACycleThatAlreadyHasBudgetIsNotReset(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	path := seedPausedRun(t, root, 3, 1, 3, "item-a")
	before, _ := os.ReadFile(path)
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--project-root", root}, nil, &stdout, &stderr)

	after, _ := os.ReadFile(path)
	if rc != cliExitOK || !bytes.Equal(before, after) || !strings.Contains(stdout.String(), "cycle 3 phase build scope item-a has budget") {
		t.Fatalf("rc = %d changed = %v stdout = %q, want the cycle left as it is", rc, !bytes.Equal(before, after), stdout.String())
	}
}

func TestCycleResumeAll_AGrantErrorUnderJSONStillExitsNonZero(t *testing.T) {
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 3, 0, 0, "item-a")
	prev := cycleGrantResume
	t.Cleanup(func() { cycleGrantResume = prev })
	cycleGrantResume = func(string) (checkpoint.Checkpoint, bool, error) {
		return checkpoint.Checkpoint{}, false, errors.New("disk full")
	}
	var stdout, stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--json", "--project-root", root}, nil, &stdout, &stderr)

	var got cycleResumeAllResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || rc == cliExitOK || len(got.Cycles) != 1 || got.Cycles[0].Granted || got.Cycles[0].HasBudget {
		t.Fatalf("rc = %d result = %+v err = %v, want the ungranted cycle listed and a non-zero exit", rc, got, err)
	}
}

func TestCycleResumeAll_AFailedJSONWriteIsItsExitCode(t *testing.T) {
	t.Parallel()
	root := newLoopStatusRoot(t)
	seedPausedRun(t, root, 3, 0, 0, "item-a")
	var stderr bytes.Buffer

	rc := runCycle([]string{"resume", "--all", "--dry-run", "--json", "--project-root", root}, nil, &failingWriter{}, &stderr)

	if rc == cliExitOK {
		t.Fatalf("rc = %d, want the JSON write failure as the exit code", rc)
	}
}

func TestCycleResumeAll_AFailedGrantIsPrintedAsNotGranted(t *testing.T) {
	t.Parallel()
	if got := grantWord(cycleResumeGrant{}, false); got != "not granted" {
		t.Fatalf("grantWord = %q", got)
	}
}
