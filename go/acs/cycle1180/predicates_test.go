//go:build acs

package cycle1180

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func newProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

func writeItem(t *testing.T, dir, id string, failureCount int, weight float64) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := map[string]any{"id": id, "title": "fixture item " + id, "kind": "bug"}
	if failureCount > 0 {
		doc["failure_count"] = failureCount
	}
	if weight > 0 {
		doc["weight"] = weight
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal item %s: %v", id, err)
	}
	path := filepath.Join(dir, "2026-07-29T00-00-00Z-"+id+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
	return path
}

func writeTriageDecision(t *testing.T, workspace string, topN []string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	rows := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		rows = append(rows, map[string]string{"id": id})
	}
	body, err := json.MarshalIndent(map[string]any{"top_n": rows}, "", "  ")
	if err != nil {
		t.Fatalf("marshal triage decision: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "triage-decision.json"), body, 0o644); err != nil {
		t.Fatalf("write triage-decision.json: %v", err)
	}
}

func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

func findItem(t *testing.T, dir, id string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, rErr := os.ReadFile(path)
		if rErr != nil {
			continue
		}
		var doc struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &doc) == nil && doc.ID == id {
			return path
		}
	}
	return ""
}

func failureCountOf(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		FailureCount int `json:"failure_count"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return 0
	}
	return doc.FailureCount
}

func ids(menus [][]triagecap.FleetCandidate) map[string]bool {
	out := map[string]bool{}
	for _, menu := range menus {
		for _, c := range menu {
			out[c.ID] = true
		}
	}
	return out
}

func TestC1180_001_QuarantinedIdIsConsumedAtDispatch(t *testing.T) {
	root, inbox := newProject(t)
	writeItem(t, filepath.Join(inbox, "quarantine"), "poison-todo", 3, 0.9)

	ds := inboxmover.ResolveDispatchState(testOpts(root, io.Discard), "poison-todo")

	if ds.State == inboxmover.StatePending || ds.State == inboxmover.StateUnknown {
		t.Errorf("ResolveDispatchState(quarantined id).State = %q; want a CONSUMED state (\"quarantine\") — pending/unknown both fail the freshness gate OPEN, so the ADR-0072 S5 ceiling never stops re-dispatch", ds.State)
	}
	if ds.State != "quarantine" {
		t.Errorf("ResolveDispatchState(quarantined id).State = %q; want \"quarantine\" so the gate's skip reason names the real cause", ds.State)
	}

	if got := inboxmover.ResolveDispatchState(testOpts(root, io.Discard), "never-filed").State; got != inboxmover.StateUnknown {
		t.Errorf("ResolveDispatchState(id with no lifecycle evidence).State = %q; want %q — a planned id that is not inbox-backed must never be false-skipped", got, inboxmover.StateUnknown)
	}
}

func TestC1180_002_LaneFailureBumpsAndQuarantines(t *testing.T) {
	root, inbox := newProject(t)
	writeItem(t, inbox, "poison-todo", 0, 0.9)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1180")
	writeTriageDecision(t, workspace, []string{"poison-todo"})

	const ceiling = 3
	for attempt := 1; attempt <= ceiling; attempt++ {
		res, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
			ProjectRoot: root,
			Workspace:   workspace,
			Cycle:       1180,
			Ceiling:     ceiling,
			SystemLevel: false,
			Reason:      "cycle-failure-release",
			Stderr:      io.Discard,
		})
		if err != nil {
			t.Fatalf("attempt %d: ApplyFailure returned %v; want nil — a lane FAIL must always reach the lifecycle", attempt, err)
		}

		if attempt < ceiling {
			path := findItem(t, inbox, "poison-todo")
			if path == "" {
				t.Fatalf("attempt %d: 'poison-todo' is not at the inbox root; below the ceiling a failed item must be released for re-pick", attempt)
			}
			if got := failureCountOf(t, path); got != attempt {
				t.Errorf("attempt %d: failure_count = %d; want %d — an unbumped counter is exactly the batch-14 defect (four FAILs, count stuck at 0)", attempt, got, attempt)
			}
			continue
		}

		if path := findItem(t, inbox, "poison-todo"); path != "" {
			t.Errorf("attempt %d (ceiling %d): 'poison-todo' is STILL at the inbox root (%s); at the ceiling it must be parked in quarantine/ so it stops being re-picked", attempt, ceiling, filepath.Base(path))
		}
		if findItem(t, filepath.Join(inbox, "quarantine"), "poison-todo") == "" {
			t.Errorf("attempt %d (ceiling %d): 'poison-todo' is not in .evolve/inbox/quarantine/ — the ADR-0072 S5 ceiling is unreachable for fleet-dispatched work", attempt, ceiling)
		}
		if len(res.Quarantined) == 0 {
			t.Errorf("attempt %d: OutcomeResult.Quarantined is empty; want the quarantined path reported so the caller can log it", attempt)
		}
	}
}

func TestC1180_003_UncommittedMenuAndSystemFailuresStayInert(t *testing.T) {
	root, inbox := newProject(t)
	writeItem(t, inbox, "worked-todo", 0, 0.9)
	writeItem(t, inbox, "menu-only-todo", 0, 0.8)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1180")
	writeTriageDecision(t, workspace, []string{"worked-todo"})

	if _, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
		ProjectRoot: root,
		Workspace:   workspace,
		Cycle:       1180,
		Ceiling:     1,
		SystemLevel: false,
		Reason:      "cycle-failure-release",
		Stderr:      io.Discard,
	}); err != nil {
		t.Fatalf("ApplyFailure (task-level): %v", err)
	}

	menuPath := findItem(t, inbox, "menu-only-todo")
	if menuPath == "" {
		t.Fatalf("'menu-only-todo' left the inbox root; an id no phase worked must never move")
	}
	if got := failureCountOf(t, menuPath); got != 0 {
		t.Errorf("uncommitted 'menu-only-todo' failure_count = %d; want 0 — only triage-COMMITTED ids may accrue task-level failures (PR #366 menu semantics)", got)
	}
	if findItem(t, filepath.Join(inbox, "quarantine"), "menu-only-todo") != "" {
		t.Errorf("uncommitted 'menu-only-todo' was QUARANTINED; healthy backlog must not be poisoned by an unrelated task's FAIL")
	}
	if findItem(t, filepath.Join(inbox, "quarantine"), "worked-todo") == "" {
		t.Errorf("committed 'worked-todo' was NOT quarantined at ceiling 1; the committed id is precisely the one that must move")
	}

	sysRoot, sysInbox := newProject(t)
	writeItem(t, sysInbox, "worked-todo", 0, 0.9)
	sysWorkspace := filepath.Join(sysRoot, ".evolve", "runs", "cycle-1181")
	writeTriageDecision(t, sysWorkspace, []string{"worked-todo"})

	if _, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
		ProjectRoot: sysRoot,
		Workspace:   sysWorkspace,
		Cycle:       1181,
		Ceiling:     1,
		SystemLevel: true,
		Reason:      "system-failure-release",
		Stderr:      io.Discard,
	}); err != nil {
		t.Fatalf("ApplyFailure (system-level): %v", err)
	}
	sysPath := findItem(t, sysInbox, "worked-todo")
	if sysPath == "" {
		t.Fatalf("'worked-todo' left the inbox root on a SYSTEM-level failure; S3 failures release, never quarantine (ADR-0072 AC4)")
	}
	if got := failureCountOf(t, sysPath); got != 0 {
		t.Errorf("system-level failure bumped failure_count to %d; want 0 — a quota/infra storm must not walk healthy ids toward the ceiling", got)
	}
}

func TestC1180_004_WaveSeedPrunesConsumedCommittedIds(t *testing.T) {
	root, inbox := newProject(t)
	evolveDir := filepath.Join(root, ".evolve")

	writeItem(t, inbox, "fresh-a", 0, 0.9)
	writeItem(t, inbox, "fresh-b", 0, 0.8)
	writeItem(t, filepath.Join(inbox, "processed"), "consumed-todo", 0, 0.95)
	writeItem(t, filepath.Join(inbox, "quarantine"), "poison-todo", 3, 0.97)

	committed := []triagecap.FleetCandidate{
		{ID: "consumed-todo", Weight: 0.95, Files: []string{"go/internal/a/a.go"}},
		{ID: "poison-todo", Weight: 0.97, Files: []string{"go/internal/b/b.go"}},
		{ID: "fresh-a", Weight: 0.9, Files: []string{"go/internal/c/c.go"}},
		{ID: "not-inbox-backed", Weight: 0.5, Files: []string{"go/internal/d/d.go"}},
	}

	got := ids(triagecap.SelectWaveSeedMenus(evolveDir, committed, 3, 4, nil))

	if got["consumed-todo"] {
		t.Errorf("wave seed re-pinned 'consumed-todo' (already in inbox/processed/) — this is the cycle-1116 re-pin: a consumed id must never re-enter a wave plan")
	}
	if got["poison-todo"] {
		t.Errorf("wave seed re-pinned 'poison-todo' (in inbox/quarantine/) — quarantine must remove an id from the planner's supply, not just from the launcher's")
	}
	if !got["fresh-a"] {
		t.Errorf("wave seed dropped still-pending 'fresh-a'; the prune must only remove CONSUMED ids, never live committed work")
	}
	if !got["not-inbox-backed"] {
		t.Errorf("wave seed dropped 'not-inbox-backed' (no lifecycle evidence anywhere); the prune must fail OPEN or every non-inbox-backed card starves its wave")
	}
}
