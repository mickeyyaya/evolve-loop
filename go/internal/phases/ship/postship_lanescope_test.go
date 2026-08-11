package ship

// postship_lanescope_test.go — RED contract for the PASS half of the
// stable-failure-identity asymmetry (inbox consumption-rides-landing-ship
// 0.88; PR #439 fixed the FAIL half). Continuation/lane cycles carry NO
// triage-decision.json, so a PASS ship promoted NOTHING: the landed item
// stayed open and a full bookkeeping cycle (~25-30 min) was later spent
// moving one JSON file ("the crosspoll PASS consumed its item in-ship while
// the egps PASS did not"). When the decision file is ABSENT, the lane-scope
// pin's todo_ids are the committed set — same file-absent-only rule as the
// FAIL side (a PRESENT decision that committed zero ids keeps the declined
// menu unpromoted).

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeDrainLaneScope(t *testing.T, root string, cycleID int, ids ...string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"todo_ids":["` + strings.Join(ids, `","`) + `"],"goal_hash":"h"}`
	if err := os.WriteFile(filepath.Join(dir, "lane-scope.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPromoteInbox_LaneScopeFallbackPromotesOnPass(t *testing.T) {
	root := t.TempDir()
	writeDrainCycleState(t, root, 51)
	// NO triage-decision.json — the continuation/lane shape.
	writeDrainLaneScope(t, root, 51, "salvage-lane")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "salvage-lane")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "processed", "cycle-51", "salvage-lane.json")); err != nil {
		t.Fatalf("landed lane item not promoted to processed/ — the PASS-side bookkeeping-cycle class stays live (logs:\n%s)", strings.Join(res.Logs, "\n"))
	}
}

// The cycle-598 landing gate must bind the FALLBACK entry path exactly as it
// binds the triage path (diff-review MEDIUM): an unlanded ship with a
// lane-scope committed set promotes nothing and drains with the retry reason.
func TestPromoteInbox_UnlandedShipSkipsLaneFallbackPromotion(t *testing.T) {
	root := t.TempDir() // not a git repo → isLanded fails closed (not landed)
	writeDrainCycleState(t, root, 53)
	writeDrainLaneScope(t, root, 53, "salvage-lane")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "salvage-lane")

	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "processed", "cycle-53", "salvage-lane.json")); !os.IsNotExist(err) {
		t.Fatal("an UNLANDED ship promoted a lane-fallback item — the cycle-598 class reopened through the fallback path")
	}
	logs := strings.Join(res.Logs, "\n")
	if !strings.Contains(logs, "unlanded") {
		t.Errorf("unlanded skip must be loud:\n%s", logs)
	}
}

func TestPromoteInbox_EmptyCommittedDeclinedMenuStaysOpen(t *testing.T) {
	root := t.TempDir()
	writeDrainCycleState(t, root, 52)
	writeDrainTriageDecision(t, root, 52) // present, zero committed ids
	writeDrainLaneScope(t, root, 52, "declined-item")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "declined-item")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "declined-item.json")); err != nil {
		t.Fatalf("a declined-menu item left the open inbox on an empty-committed PASS — file-absent-only rule violated (logs:\n%s)", strings.Join(res.Logs, "\n"))
	}
}
