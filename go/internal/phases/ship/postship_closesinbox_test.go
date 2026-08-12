package ship

// postship_closesinbox_test.go — RED contract for the wiring half of
// consumption-rides-landing-ship (cycle 1452): a PASS ship whose diff closes an
// inbox item consumes that item in the SAME landing, even when triage never
// named the id.
//
// The live instance: schema-aligned-salvage-layer landed in #453, nothing
// consumed its item, and wave cycle-1448 re-picked already-shipped work as live
// scope. The defect is structural — consumption is a separate act from landing,
// so forgetting is always possible.
//
// What these tests freeze (doNotModifyTests):
//   - marker ids join the committed set through the ONE existing lifecycle seam;
//   - they ride the EXACT cycle-598 landing gate (`isLanded`) — no new, parallel,
//     or weaker gate: an unlanded ship consumes nothing;
//   - a marker works with NO triage decision present (the continuation/lane
//     shape that produced the live instance);
//   - absence of build-report.md is not an error and does not disturb the
//     triage-sourced promotion;
//   - a landed ship WITHOUT a marker consumes only what triage named (the
//     anti-over-consumption half — a diff-inference implementation fails here).
//
// Fixture helpers (writeDrainCycleState / writeDrainTriageDecision /
// writeDrainInboxItem) are shared with the sibling postship drain tests.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeClosesBuildReport drops the build-report.md promoteInbox must read for
// the current cycle, carrying a line-anchored Closes-Inbox marker for each id.
// With no ids it writes a marker-free report (the negative fixture).
func writeClosesBuildReport(t *testing.T, root string, cycleID int, ids ...string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Build Report — cycle " + strconv.Itoa(cycleID) + "\n\nImplemented the task; suite green.\n\n"
	for _, id := range ids {
		body += "Closes-Inbox: " + id + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// promotedPath is where a consumed item must land for the cycle.
func promotedPath(root string, cycleID int, id string) string {
	return filepath.Join(root, ".evolve", "inbox", "processed", "cycle-"+strconv.Itoa(cycleID), id+".json")
}

// TestPromoteInbox_ClosesInboxMarkerConsumesUnnamedItemOnLandedShip — the
// load-bearing case. Triage committed to ONE id; the landing also closed a
// second item the Builder marked. Both must be consumed by this ship, not by a
// future bookkeeping cycle. CommitSHA is empty so the landing gate fails OPEN
// (the pre-existing "no commit recorded" contract), isolating this test to the
// committed-set change.
func TestPromoteInbox_ClosesInboxMarkerConsumesUnnamedItemOnLandedShip(t *testing.T) {
	root := t.TempDir()
	const cid = 1452
	writeDrainCycleState(t, root, cid)
	writeDrainTriageDecision(t, root, cid, "named-by-triage")
	writeClosesBuildReport(t, root, cid, "closed-by-marker")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "named-by-triage")
	writeDrainInboxItem(t, inbox, "closed-by-marker")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	logs := strings.Join(res.Logs, "\n")
	if _, err := os.Stat(promotedPath(root, cid, "named-by-triage")); err != nil {
		t.Fatalf("regression: the triage-named item was not promoted (logs:\n%s)", logs)
	}
	if _, err := os.Stat(promotedPath(root, cid, "closed-by-marker")); err != nil {
		t.Fatalf("a Closes-Inbox-marked item survived its own landing ship — consumption is still a separate act from landing, so the next wave re-picks shipped work as live scope (logs:\n%s)", logs)
	}
}

// TestPromoteInbox_ClosesInboxMarkerConsumesWithNoTriageDecision — the exact
// live-instance shape: a continuation/lane cycle carries NO triage-decision.json
// and no lane-scope pin, so the promotion branch was never entered at all. A
// marker alone must be a sufficient committed set on a landed ship.
func TestPromoteInbox_ClosesInboxMarkerConsumesWithNoTriageDecision(t *testing.T) {
	root := t.TempDir()
	const cid = 1453
	writeDrainCycleState(t, root, cid)
	// No triage-decision.json, no triage-report.md to project from, no lane-scope.json.
	writeClosesBuildReport(t, root, cid, "schema-aligned-salvage-layer")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "schema-aligned-salvage-layer")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(promotedPath(root, cid, "schema-aligned-salvage-layer")); err != nil {
		t.Fatalf("a decision-less lane cycle did not consume its marked item — this is the #453 live instance verbatim (logs:\n%s)", strings.Join(res.Logs, "\n"))
	}
}

// TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip — the cycle-598 gate
// must bind the marker path exactly as it binds the triage and lane-scope paths.
// A non-git TempDir with a real-looking SHA makes isLanded fail closed.
func TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip(t *testing.T) {
	root := t.TempDir()
	const cid = 1454
	writeDrainCycleState(t, root, cid)
	writeClosesBuildReport(t, root, cid, "closed-by-marker")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "closed-by-marker")

	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(promotedPath(root, cid, "closed-by-marker")); !os.IsNotExist(err) {
		t.Fatal("an UNLANDED ship consumed a marker-closed item — the marker path introduced a second, weaker gate and reopened the cycle-598 class")
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "unlanded") {
		t.Errorf("unlanded skip is not disclosed in the ship logs:\n%s", logs)
	}
}

// TestPromoteInbox_LandedShipWithoutMarkerConsumesOnlyTriageNamedItems — the
// anti-over-consumption half. A build-report with no marker must leave every
// unnamed item open; an implementation that infers closure from the diff, the
// cycle dir, or `connects_to` proximity fails here.
func TestPromoteInbox_LandedShipWithoutMarkerConsumesOnlyTriageNamedItems(t *testing.T) {
	root := t.TempDir()
	const cid = 1455
	writeDrainCycleState(t, root, cid)
	writeDrainTriageDecision(t, root, cid, "named-by-triage")
	writeClosesBuildReport(t, root, cid) // report present, NO marker
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "named-by-triage")
	writeDrainInboxItem(t, inbox, "untouched-item")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if _, err := os.Stat(promotedPath(root, cid, "named-by-triage")); err != nil {
		t.Fatalf("regression: the triage-named item was not promoted (logs:\n%s)", strings.Join(res.Logs, "\n"))
	}
	if _, err := os.Stat(promotedPath(root, cid, "untouched-item")); !os.IsNotExist(err) {
		t.Fatal("an item nobody claimed was consumed — closure must be an explicit marker, never inferred (false-positive consumption is silent data loss)")
	}
}

// TestPromoteInbox_AbsentBuildReportIsNotAnError — degrade cleanly: cycles that
// skip the build phase have no build-report.md at all, and the triage-sourced
// promotion must be untouched by the new read.
func TestPromoteInbox_AbsentBuildReportIsNotAnError(t *testing.T) {
	root := t.TempDir()
	const cid = 1456
	writeDrainCycleState(t, root, cid)
	writeDrainTriageDecision(t, root, cid, "named-by-triage")
	// No build-report.md written at all.
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "named-by-triage")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("absent build-report.md must not fail ship: %v", err)
	}
	if _, err := os.Stat(promotedPath(root, cid, "named-by-triage")); err != nil {
		t.Fatalf("regression: triage-sourced promotion broke when build-report.md is absent (logs:\n%s)", strings.Join(res.Logs, "\n"))
	}
}
