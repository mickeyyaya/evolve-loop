package cycleoutcome

// nowork_route_test.go — F30's loop breaker. A fleet lane whose triage answered
// for its scoped item without committing it (a reasoned drop, a skip, an
// escalation) now ends as planned no-work instead of a claim failure — but the
// item is still pending in the inbox root, so the next wave would draw it into
// another lane and another no-work end. The no-work closeout hands every
// scoped item the lane answered for to the console, in place, with the lane's
// reason: the console confirms and retires it (an unshipped lane never retires
// work — the anti-laundering principle behind the closure-claim gate).

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// seedLane writes one inbox item per id, the lane pin and the triage decision.
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

	routed, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}.WithSignals(c))
	if err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	if len(routed) != 1 || routed[0] != "answered" {
		t.Fatalf("routed = %v, want exactly the lane's answered item", routed)
	}
	inbox := filepath.Join(root, ".evolve", "inbox")
	rec := itemRecord(t, inbox, "answered")
	if rec["route"] != "console-manual" || !strings.Contains(rec["routed_reason"].(string), "lane triage (cycle 7) dropped: stale: closed by #535") {
		t.Errorf("the answered item carries the console route and the lane's reason: %v", rec)
	}
	// An unanswered scoped item is not the lane's to hand over, and an id the
	// agent answered for OUTSIDE the lane's scope never widens what this
	// closeout may touch (the ApplyFailure containsID rule).
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

// TestApplyNoWork_RoutesNothingWithoutAPinOrAPendingItem: a sequential cycle
// (no lane pin) has no lane scope to hand over, and an answered item a ship
// already consumed is simply gone — neither is an error.
func TestApplyNoWork_RoutesNothingWithoutAPinOrAPendingItem(t *testing.T) {
	decision := `{"top_n":[],"skip_shipped":[{"task_id":"shipped","git_sha":"abc123"}]}`
	root, ws := seedLane(t, []string{"shipped"}, nil, decision)
	if routed, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err != nil || len(routed) != 0 {
		t.Fatalf("no lane pin ⇒ nothing to hand over: routed=%v err=%v", routed, err)
	}

	root, ws = seedLane(t, nil, []string{"shipped"}, decision)
	if routed, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err != nil || len(routed) != 0 {
		t.Fatalf("an answered item no longer in the inbox is not an error: routed=%v err=%v", routed, err)
	}
}

// TestApplyNoWork_AppendsTheRouteThroughTheInjectedLedger: like the failure
// walk (ADR-0101 S4a), the hand-off's lifecycle line goes through the root's
// observed ledger, never a self-constructed one over the same file.
func TestApplyNoWork_AppendsTheRouteThroughTheInjectedLedger(t *testing.T) {
	root, ws := seedLane(t, []string{"answered"}, []string{"answered"},
		`{"top_n":[],"escalate_block":[{"task_id":"answered","reason":"console-routed"}]}`)
	rec := &recordingLifecycleLedger{}
	in := NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}
	if routed, err := ApplyNoWork(in.WithLedger(rec)); err != nil || len(routed) != 1 {
		t.Fatalf("ApplyNoWork: routed=%v err=%v", routed, err)
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

// TestApplyNoWork_ReleasesWhatTheLaneClaimed (F30 architecture review C1):
// triage claims before it selects, so an answered scoped item may sit in
// processing/cycle-N/. The hand-off routes it there, in place, and then
// releases the cycle's claims to the root with no failure bump — the item
// arrives console-visible instead of stranded in a gitignored claim dir.
func TestApplyNoWork_ReleasesWhatTheLaneClaimed(t *testing.T) {
	root, ws := seedLane(t, []string{"answered"}, []string{"answered"},
		`{"top_n":[],"dropped":[{"id":"answered","reason":"stale: closed by #535"}]}`)
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "answered", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	routed, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard})
	if err != nil || len(routed) != 1 {
		t.Fatalf("ApplyNoWork: routed=%v err=%v", routed, err)
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

// TestApplyNoWork_KeepsTheConsolesEvidenceAndSkipsWhatIsGone (review m3, m4):
// an item already routed to the console keeps its original reason (the lane's
// "escalated: console-routed" would overwrite a protected-surface refusal with
// a circular one), and an answered id no longer in the inbox is simply gone —
// no route-not-found WARN on the stream.
func TestApplyNoWork_KeepsTheConsolesEvidenceAndSkipsWhatIsGone(t *testing.T) {
	root, ws := seedLane(t, []string{"routed"}, []string{"routed", "consumed"},
		`{"top_n":[],"escalate_block":[{"task_id":"routed","reason":"console-routed"},{"task_id":"consumed","reason":"console-routed"}]}`)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if _, err := inboxmover.RouteConsole(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "routed", "protected surface: go/internal/core/cyclerun.go", 3); err != nil {
		t.Fatal(err)
	}
	c, events := recordingCenter()
	routed, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}.WithSignals(c))
	if err != nil || len(routed) != 0 {
		t.Fatalf("nothing new to route: routed=%v err=%v", routed, err)
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
