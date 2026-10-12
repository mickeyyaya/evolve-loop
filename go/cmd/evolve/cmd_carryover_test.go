package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

func TestCarryoverSubcommandRegistered(t *testing.T) {
	t.Parallel()
	found := false
	for _, c := range commands {
		if c.Name == "carryover" {
			found = true
			if c.Run == nil {
				t.Fatalf("carryover command registered with a nil Run handler")
			}
		}
	}
	if !found {
		t.Fatalf("carryover subcommand is not registered in the commands table")
	}
}

func writeFixtureState(t *testing.T, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	todos := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		todos = append(todos, map[string]any{"id": id})
	}
	state := map[string]any{"carryoverTodos": todos, "someOtherKey": "preserved"}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture state: %v", err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture state: %v", err)
	}
	return path
}

func readCarryoverIDs(t *testing.T, statePath string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json corrupted / not valid JSON: %v", err)
	}
	ids := make(map[string]bool)
	for _, e := range state.CarryoverTodos {
		ids[e.ID] = true
	}
	return ids
}

func TestCarryoverApplyDecisions_DropsEntriesToCeiling(t *testing.T) {
	t.Parallel()
	ids := make([]string, 0, 30)
	doc := carryoverDecisionsDoc{SourceCount: 30}
	for i := 0; i < 30; i++ {
		id := "todo-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		ids = append(ids, id)
		var decision, group string
		switch {
		case i < 3:
			decision = "keep"
		case i < 28:
			decision = "drop"
		default:
			decision, group = "cluster", "sweep-x"
		}
		doc.Decisions = append(doc.Decisions, carryoverDecisionRow{
			ID: id, Decision: decision, Reason: "fixture reason", ClusterGroup: group,
		})
	}
	statePath := writeFixtureState(t, ids...)

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions: %v", err)
	}
	if res.Before != 30 {
		t.Errorf("Before = %d, want 30", res.Before)
	}
	if res.Dropped != 25 {
		t.Errorf("Dropped = %d, want 25", res.Dropped)
	}
	if res.Clustered != 2 {
		t.Errorf("Clustered = %d, want 2", res.Clustered)
	}
	if res.After != 3 {
		t.Errorf("After = %d, want 3", res.After)
	}
	if res.After > carryoverApplyCeiling {
		t.Errorf("After = %d exceeds ceiling %d", res.After, carryoverApplyCeiling)
	}
	surviving := readCarryoverIDs(t, statePath)
	if len(surviving) != 3 {
		t.Errorf("surviving id count = %d, want 3", len(surviving))
	}
	for _, d := range doc.Decisions {
		switch d.Decision {
		case "drop", "cluster":
			if surviving[d.ID] {
				t.Errorf("id %q was %s but survived", d.ID, d.Decision)
			}
		case "keep":
			if !surviving[d.ID] {
				t.Errorf("id %q was keep but was removed", d.ID)
			}
		}
	}
}

func TestCarryoverApplyDecisions_RejectsMissingReason(t *testing.T) {
	t.Parallel()
	statePath := writeFixtureState(t, "todo-keep-me", "todo-drop-me")
	before := readCarryoverIDs(t, statePath)
	beforeRaw, _ := os.ReadFile(statePath)

	doc := carryoverDecisionsDoc{
		SourceCount: 2,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-keep-me", Decision: "keep", Reason: "still live"},
			{ID: "todo-drop-me", Decision: "drop", Reason: "  "},
		},
	}
	if err := validateCarryoverDecisions(doc); err == nil {
		t.Fatalf("validateCarryoverDecisions accepted a row with an empty reason")
	}

	code := runCarryoverApplyDecisions([]string{"--apply", "--state", statePath, "--decisions", writeDecisionsFile(t, doc)}, os.Stderr, os.Stderr)
	if code == 0 {
		t.Fatalf("runCarryoverApplyDecisions returned 0 for an empty-reason decisions file")
	}
	afterRaw, _ := os.ReadFile(statePath)
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated despite a rejected decisions file")
	}
	after := readCarryoverIDs(t, statePath)
	if len(after) != len(before) {
		t.Fatalf("carryover count changed on a rejected apply: %d → %d", len(before), len(after))
	}
}

func writeDecisionsFile(t *testing.T, doc carryoverDecisionsDoc) string {
	t.Helper()
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal decisions: %v", err)
	}
	path := filepath.Join(t.TempDir(), "decisions.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write decisions: %v", err)
	}
	return path
}

func TestCarryoverApplyDecisions_UsesLockedRMW(t *testing.T) {
	t.Parallel()
	statePath := writeFixtureState(t, "todo-a", "todo-b", "todo-keep")
	doc := carryoverDecisionsDoc{
		SourceCount: 3,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-a", Decision: "drop", Reason: "stale"},
			{ID: "todo-b", Decision: "drop", Reason: "stale"},
			{ID: "todo-keep", Decision: "keep", Reason: "live"},
		},
	}

	if _, err := applyCarryoverDecisions(statePath, doc); err != nil {
		t.Fatalf("applyCarryoverDecisions: %v", err)
	}
	if _, err := os.Stat(statePath + ".lock"); err != nil {
		t.Fatalf("sidecar lock %s.lock was not created — apply did not go through flock.WithPathLock: %v", statePath, err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = applyCarryoverDecisions(statePath, doc)
		}()
	}
	wg.Wait()

	surviving := readCarryoverIDs(t, statePath)
	if !surviving["todo-keep"] || surviving["todo-a"] || surviving["todo-b"] {
		t.Fatalf("post-concurrency state wrong: %v", surviving)
	}
}

func TestCarryoverApplyDecisions_PreservesStateSymlink(t *testing.T) {
	t.Parallel()
	canonicalDir := t.TempDir()
	worktreeDir := t.TempDir()
	canonical := filepath.Join(canonicalDir, "state.json")
	seed := map[string]any{
		"stateRevision": float64(3),
		"carryoverTodos": []any{
			map[string]any{"id": "keep-me"},
			map[string]any{"id": "drop-me"},
		},
	}
	raw, _ := json.Marshal(seed)
	if err := os.WriteFile(canonical, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(worktreeDir, "state.json")
	if err := os.Symlink(canonical, link); err != nil {
		t.Fatal(err)
	}

	doc := carryoverDecisionsDoc{Decisions: []carryoverDecisionRow{
		{ID: "drop-me", Decision: "drop", Reason: "superseded by regression pin"},
	}}
	res, err := applyCarryoverDecisions(link, doc)
	if err != nil {
		t.Fatalf("apply through symlink: %v", err)
	}
	if res.Before != 2 || res.After != 1 || res.Dropped != 1 {
		t.Fatalf("res=%+v, want Before=2 After=1 Dropped=1", res)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("state.json symlink SEVERED by apply (the cycle-999 defect)")
	}
	got, err := statemap.ReadStateMap(canonical)
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := got["carryoverTodos"].([]any)
	if len(arr) != 1 {
		t.Fatalf("canonical not updated through link: %v", got["carryoverTodos"])
	}
}

func TestCarryoverApplyDecisions_DropsConsumedFleetAlias(t *testing.T) {
	t.Parallel()
	const (
		consumedAlias = "pipeline-defect-pipeline-blocker"
		liveID        = "todo-still-live"
	)
	statePath := writeFixtureState(t, consumedAlias, liveID)
	doc := carryoverDecisionsDoc{
		SourceCount: 2,
		Decisions: []carryoverDecisionRow{
			{ID: consumedAlias, Decision: "drop", Reason: "consumed fleet alias"},
			{ID: liveID, Decision: "keep", Reason: "still actionable"},
		},
	}

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions: %v", err)
	}
	if res.Before != 2 || res.After != 1 || res.Dropped != 1 || res.Clustered != 0 {
		t.Fatalf("result = %+v, want Before=2 After=1 Dropped=1 Clustered=0", res)
	}
	surviving := readCarryoverIDs(t, statePath)
	if surviving[consumedAlias] || !surviving[liveID] || len(surviving) != 1 {
		t.Fatalf("surviving ids = %v, want only %q", surviving, liveID)
	}
}

type carryoverListRow struct {
	ID             string `json:"id"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	Age            int    `json:"age"`
	ExpiresAt      string `json:"expiresAt"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
	Priority       string `json:"priority"`
}

func writeListFixtureState(t *testing.T) string {
	t.Helper()
	state := map[string]any{
		"lastCycleNumber": 100,
		"carryoverTodos": []map[string]any{
			{"id": "todo-old", "first_seen_cycle": 60, "cycles_unpicked": 3, "expiresAt": "2026-10-12T00:00:00Z", "priority": "high"},
			{"id": "todo-new", "first_seen_cycle": 95, "cycles_unpicked": 40, "expiresAt": "2026-10-20T00:00:00Z", "priority": "low"},
			{"id": "todo-mid", "first_seen_cycle": 80, "cycles_unpicked": 10, "expiresAt": "2026-10-15T00:00:00Z", "priority": "medium"},
		},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func runCarryoverListJSON(t *testing.T, args ...string) []carryoverListRow {
	t.Helper()
	var out, errb bytes.Buffer
	code := runCarryover(append([]string{"list", "--json"}, args...), nil, &out, &errb)
	if code != 0 {
		t.Fatalf("carryover list exit=%d stderr=%s", code, errb.String())
	}
	var rows []carryoverListRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("list --json output is not a JSON array: %v\n%s", err, out.String())
	}
	return rows
}

func TestCarryoverList(t *testing.T) {
	t.Parallel()
	statePath := writeListFixtureState(t)
	rows := runCarryoverListJSON(t, "--state", statePath)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	byID := map[string]carryoverListRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	want := map[string]carryoverListRow{
		"todo-old": {ID: "todo-old", FirstSeenCycle: 60, Age: 40, ExpiresAt: "2026-10-12T00:00:00Z", CyclesUnpicked: 3, Priority: "high"},
		"todo-new": {ID: "todo-new", FirstSeenCycle: 95, Age: 5, ExpiresAt: "2026-10-20T00:00:00Z", CyclesUnpicked: 40, Priority: "low"},
		"todo-mid": {ID: "todo-mid", FirstSeenCycle: 80, Age: 20, ExpiresAt: "2026-10-15T00:00:00Z", CyclesUnpicked: 10, Priority: "medium"},
	}
	for id, w := range want {
		if byID[id] != w {
			t.Errorf("row %s = %+v, want %+v", id, byID[id], w)
		}
	}
}

func TestCarryoverList_SortAndLimit(t *testing.T) {
	t.Parallel()
	statePath := writeListFixtureState(t)
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"sort age oldest first", []string{"--sort", "age"}, []string{"todo-old", "todo-mid", "todo-new"}},
		{"sort unpicked most first", []string{"--sort", "unpicked"}, []string{"todo-new", "todo-mid", "todo-old"}},
		{"limit after sort", []string{"--sort", "age", "--limit", "2"}, []string{"todo-old", "todo-mid"}},
		{"limit larger than queue", []string{"--limit", "99"}, nil},
	}
	for _, c := range cases {
		rows := runCarryoverListJSON(t, append([]string{"--state", statePath}, c.args...)...)
		if c.want == nil {
			if len(rows) != 3 {
				t.Errorf("%s: rows = %d, want 3", c.name, len(rows))
			}
			continue
		}
		got := make([]string, 0, len(rows))
		for _, r := range rows {
			got = append(got, r.ID)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: order = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCarryoverList_RejectsBadFlags(t *testing.T) {
	t.Parallel()
	statePath := writeListFixtureState(t)
	if rows := runCarryoverListJSON(t, "--state", statePath, "--sort", "unpicked", "--limit", "0"); len(rows) != 3 {
		t.Errorf("valid flags with limit 0 listed %d rows, want all 3", len(rows))
	}
	for _, args := range [][]string{
		{"--sort", "bogus"},
		{"--limit", "-1"},
	} {
		var out, errb bytes.Buffer
		code := runCarryover(append([]string{"list", "--state", statePath}, args...), nil, &out, &errb)
		if code != 10 {
			t.Errorf("args %v: exit=%d, want 10", args, code)
		}
	}
}

func TestCarryoverList_AcceptanceWithApplyDecisions(t *testing.T) {
	t.Parallel()
	statePath := writeListFixtureState(t)
	rows := runCarryoverListJSON(t, "--state", statePath)
	doc := carryoverDecisionsDoc{SourceCount: len(rows)}
	for _, r := range rows {
		decision := "keep"
		if r.ID == "todo-old" {
			decision = "drop"
		}
		doc.Decisions = append(doc.Decisions, carryoverDecisionRow{ID: r.ID, Decision: decision, Reason: "listed id"})
	}
	if err := validateCarryoverDecisions(doc); err != nil {
		t.Fatalf("listed ids rejected by apply-decisions validation: %v", err)
	}
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions: %v", err)
	}
	if res.Before != 3 || res.Dropped != 1 || res.After != 2 {
		t.Errorf("apply result = %+v, want Before=3 Dropped=1 After=2", res)
	}
	if bytes.Equal(before, mustRead(t, statePath)) {
		t.Errorf("apply did not mutate the state the list read")
	}
	after := runCarryoverListJSON(t, "--state", statePath)
	if len(after) != 2 {
		t.Errorf("list after apply = %d rows, want 2", len(after))
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func TestCarryoverList_ListIsReadOnly(t *testing.T) {
	t.Parallel()
	statePath := writeListFixtureState(t)
	before := mustRead(t, statePath)
	runCarryoverListJSON(t, "--state", statePath, "--sort", "age")
	if !bytes.Equal(before, mustRead(t, statePath)) {
		t.Errorf("carryover list modified state.json")
	}
}

func TestCarryoverList_UnreadableStateExits2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	malformed := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(malformed, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	unreadable := filepath.Join(dir, "dir-as-state.json")
	if err := os.Mkdir(unreadable, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, path := range []string{malformed, unreadable} {
		var out, errb bytes.Buffer
		code := runCarryover([]string{"list", "--json", "--state", path}, nil, &out, &errb)
		if code != 2 {
			t.Errorf("state %s: exit=%d, want 2 (stderr=%s)", filepath.Base(path), code, errb.String())
		}
		if out.Len() != 0 {
			t.Errorf("state %s: stdout not empty on failure: %s", filepath.Base(path), out.String())
		}
	}
}

func TestCarryoverList_EmptyCarryover(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"empty-array.json": `{"lastCycleNumber": 5, "carryoverTodos": []}`,
		"absent-key.json":  `{"lastCycleNumber": 5}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		rows := runCarryoverListJSON(t, "--state", path)
		if len(rows) != 0 {
			t.Errorf("%s: rows = %d, want 0", name, len(rows))
		}
		var out, errb bytes.Buffer
		if code := runCarryover([]string{"list", "--json", "--state", path}, nil, &out, &errb); code != 0 || strings.TrimSpace(out.String()) != "[]" {
			t.Errorf("%s: exit=%d out=%q, want 0 and []", name, code, out.String())
		}
	}
}

func TestCarryoverList_TableOutputNamesEveryID(t *testing.T) {
	t.Parallel()
	statePath := writeListFixtureState(t)
	var out, errb bytes.Buffer
	if code := runCarryover([]string{"list", "--state", statePath}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	for _, id := range []string{"todo-old", "todo-new", "todo-mid"} {
		if !strings.Contains(out.String(), id) {
			t.Errorf("table output missing %s:\n%s", id, out.String())
		}
	}
}
