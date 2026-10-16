package cycleoutcome

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func seedLane(t *testing.T, items []string, scope []string, decision string) (root, ws string) {
	t.Helper()
	root = t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	ws = filepath.Join(root, ".evolve", "runs", "cycle-7")
	for _, dir := range []string{inbox, ws} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range items {
		body, _ := json.Marshal(map[string]any{"id": id, "title": "fixture " + id, "kind": "bug"})
		if err := os.WriteFile(filepath.Join(inbox, "2026-09-26T00-00-00Z-"+id+".json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if scope != nil {
		pin, _ := json.Marshal(map[string]any{"todo_ids": scope, "goal_hash": "g"})
		if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), pin, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decision), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, ws
}

func TestApplyNoWork_HandsTheLanesAnsweredItemToTheConsole(t *testing.T) {
	root, ws := seedLane(t,
		[]string{"answered", "unanswered", "elsewhere"},
		[]string{"answered", "unanswered"},
		`{"top_n":[],"dropped":[{"id":"answered","reason":"stale: closed by #535"},{"id":"elsewhere","reason":"duplicate"}]}`)
	c, events := recordingCenter()

	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}.WithSignals(c))
	if err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	if len(res.Routed) != 1 || res.Routed[0] != "answered" {
		t.Fatalf("routed = %v, want exactly the lane's answered item", res.Routed)
	}
	inbox := filepath.Join(root, ".evolve", "inbox")
	rec := itemRecord(t, inbox, "answered")
	if rec["route"] != "console-manual" || !strings.Contains(rec["routed_reason"].(string), "lane triage (cycle 7) dropped: stale: closed by #535") {
		t.Errorf("the answered item carries the console route and the lane's reason: %v", rec)
	}
	for _, id := range []string{"unanswered", "elsewhere"} {
		if r := itemRecord(t, inbox, id); r["route"] != nil {
			t.Errorf("%s must stay untouched: %v", id, r)
		}
	}
	seen := false
	for _, e := range *events {
		if string(e.Code) == "INBOX_ITEM_ROUTED_CONSOLE" {
			seen = true
		}
	}
	if !seen {
		t.Error("the route is on the stream (INBOX_ITEM_ROUTED_CONSOLE)")
	}
}

func TestApplyNoWork_RoutesNothingWithoutAPinOrAPendingItem(t *testing.T) {
	decision := `{"top_n":[],"skip_shipped":[{"task_id":"shipped","git_sha":"abc123"}]}`
	root, ws := seedLane(t, []string{"shipped"}, nil, decision)
	if res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err != nil || len(res.Routed) != 0 {
		t.Fatalf("no lane pin ⇒ nothing to hand over: routed=%v err=%v", res.Routed, err)
	}

	root, ws = seedLane(t, nil, []string{"shipped"}, decision)
	if res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err != nil || len(res.Routed) != 0 {
		t.Fatalf("an answered item no longer in the inbox is not an error: routed=%v err=%v", res.Routed, err)
	}
}

func TestApplyNoWork_AppendsTheRouteThroughTheInjectedLedger(t *testing.T) {
	root, ws := seedLane(t, []string{"answered"}, []string{"answered"},
		`{"top_n":[],"escalate_block":[{"task_id":"answered","reason":"console-routed"}]}`)
	rec := &recordingLifecycleLedger{}
	in := NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}
	if res, err := ApplyNoWork(in.WithLedger(rec)); err != nil || len(res.Routed) != 1 {
		t.Fatalf("ApplyNoWork: routed=%v err=%v", res.Routed, err)
	}
	if in.Ledger != nil {
		t.Error("WithLedger must not mutate its receiver")
	}
	if len(rec.records) == 0 || rec.records[0].Action != "route-console" {
		t.Fatalf("the route's lifecycle line goes through the injected ledger: %+v", rec.records)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "ledger.jsonl")); !os.IsNotExist(err) {
		t.Errorf("a self-constructed ledger wrote beside the injected one (stat err=%v)", err)
	}
}

func TestApplyNoWork_ReleasesWhatTheLaneClaimed(t *testing.T) {
	root, ws := seedLane(t, []string{"answered"}, []string{"answered"},
		`{"top_n":[],"dropped":[{"id":"answered","reason":"stale: closed by #535"}]}`)
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "answered", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard})
	if err != nil || len(res.Routed) != 1 {
		t.Fatalf("ApplyNoWork: routed=%v err=%v", res.Routed, err)
	}
	inbox := filepath.Join(root, ".evolve", "inbox")
	if rec := itemRecord(t, inbox, "answered"); rec["route"] != "console-manual" {
		t.Errorf("the released item is back in the root, routed: %v", rec)
	}
	if entries, _ := os.ReadDir(filepath.Join(inbox, "processing", "cycle-7")); len(entries) != 0 {
		t.Errorf("nothing stays stranded in the claim dir: %d file(s)", len(entries))
	}
	if n, _ := inboxmover.ReadFailureCount(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "answered"); n != 0 {
		t.Errorf("a no-work lane worked nothing — no failure bump: failure_count=%d", n)
	}
}

func TestApplyNoWork_KeepsTheConsolesEvidenceAndSkipsWhatIsGone(t *testing.T) {
	root, ws := seedLane(t, []string{"routed"}, []string{"routed", "consumed"},
		`{"top_n":[],"escalate_block":[{"task_id":"routed","reason":"console-routed"},{"task_id":"consumed","reason":"console-routed"}]}`)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if _, err := inboxmover.RouteConsole(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "routed", "protected surface: go/internal/core/cyclerun.go", 3); err != nil {
		t.Fatal(err)
	}
	c, events := recordingCenter()
	res, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}.WithSignals(c))
	if err != nil || len(res.Routed) != 0 {
		t.Fatalf("nothing new to route: routed=%v err=%v", res.Routed, err)
	}
	if rec := itemRecord(t, inbox, "routed"); rec["routed_reason"] != "protected surface: go/internal/core/cyclerun.go" {
		t.Errorf("the console's original evidence is kept: %v", rec["routed_reason"])
	}
	for _, e := range *events {
		if string(e.Code) == "INBOX_ROUTE_NOT_FOUND" {
			t.Errorf("a consumed answered id is not a route failure: %+v", e)
		}
	}
}
