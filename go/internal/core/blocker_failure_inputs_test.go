package core_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func endedBuildWorkspace(t *testing.T, outcome core.BlockerOutcome) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	for _, dir := range []string{inbox, ws} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	item, _ := json.Marshal(map[string]any{"id": "seam", "title": "fixture seam", "kind": "bug"})
	if err := os.WriteFile(filepath.Join(inbox, "2026-07-29T00-00-00Z-seam.json"), item, 0o644); err != nil {
		t.Fatal(err)
	}
	decision, _ := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": "seam"}}})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), decision, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "seam", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	code := map[core.BlockerOutcome]string{
		core.Unroutable: cyclestate.DiagCodeBlockerUnroutable,
		core.Incomplete: cyclestate.DiagCodeBlockerIncomplete,
	}[outcome]
	entries := []phasetiming.Entry{
		{Phase: "scout", Verdict: "PASS"},
		{Phase: "build", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Code: code, Subject: "seam", Message: "the end"}}},
		{Phase: string(cyclestate.PhaseRetro), Verdict: "PASS"},
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phasetiming.Path(ws), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, inbox, ws
}

func closedItem(t *testing.T, inbox string) map[string]any {
	t.Helper()
	path, err := inboxmover.FindFileByTaskID(inbox, "seam")
	if err != nil {
		t.Fatalf("seam not in %s: %v", inbox, err)
	}
	body, _ := os.ReadFile(path)
	var rec map[string]any
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestBlockerEnd_UnroutableIsTaskLevelAndChargesTheItem(t *testing.T) {
	t.Parallel()
	root, inbox, ws := endedBuildWorkspace(t, core.Unroutable)
	in := cycleoutcome.FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	if in.SystemLevel || in.Refusal != cyclestate.DiagCodeBlockerUnroutable {
		t.Fatalf("inputs = SystemLevel %v, Refusal %q; an unroutable proof blames the task and keeps its code", in.SystemLevel, in.Refusal)
	}
	if _, err := cycleoutcome.ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if rec := closedItem(t, inbox); rec["failure_count"] != float64(1) {
		t.Fatalf("item = %v; an unroutable end charges the item once, so a repeat reaches the ceiling", rec)
	}
}

func TestBlockerEnd_IncompleteIsSystemLevelAndLeavesTheCountUnchanged(t *testing.T) {
	t.Parallel()
	root, inbox, ws := endedBuildWorkspace(t, core.Incomplete)
	in := cycleoutcome.FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	if !in.SystemLevel || in.Refusal != cyclestate.DiagCodeBlockerIncomplete {
		t.Fatalf("inputs = SystemLevel %v, Refusal %q; a missing base is the pipeline's fault and keeps its code", in.SystemLevel, in.Refusal)
	}
	if _, err := cycleoutcome.ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if rec := closedItem(t, inbox); rec["failure_count"] != nil || rec["route"] != nil {
		t.Fatalf("item = %v; an incomplete end must not charge or route the item", rec)
	}
}

func TestBlockerEnd_AQuarantinedItemReleasesThenRoutesToTheConsole(t *testing.T) {
	t.Parallel()
	root, inbox, ws := endedBuildWorkspace(t, core.Unroutable)
	in := cycleoutcome.FailureInputsFor(root, filepath.Join(root, ".evolve"), ws, 7, io.Discard)
	in.Ceiling = 1
	if _, err := cycleoutcome.ApplyFailure(in); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	if _, err := inboxmover.FindFileByTaskID(inbox, "seam"); err == nil {
		t.Fatal("the item stayed on the queue at the ceiling; an unroutable end must reach quarantine")
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}

	if _, err := inboxmover.ReleaseFromQuarantine(opts, "seam"); err != nil {
		t.Fatalf("quarantine release: %v", err)
	}
	if _, err := inboxmover.RouteConsole(opts, "seam", "the lane needs a protected path", 7); err != nil {
		t.Fatalf("route-console: %v", err)
	}

	rec := closedItem(t, inbox)
	if rec["route"] != "console-manual" || rec["failure_count"] != float64(0) {
		t.Fatalf("item = %v; the release resets the count and the route puts the item with the console", rec)
	}
}
