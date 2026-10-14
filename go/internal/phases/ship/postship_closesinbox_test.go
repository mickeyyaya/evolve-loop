package ship

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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

func promotedPath(root string, cycleID int, id string) string {
	return filepath.Join(root, ".evolve", "inbox", "processed", "cycle-"+strconv.Itoa(cycleID), id+".json")
}

func TestPromoteInbox_ClosesInboxMarkerConsumesUnnamedItemOnLandedShip(t *testing.T) {
	t.Parallel()
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

func TestPromoteInbox_ClosesInboxMarkerConsumesWithNoTriageDecision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 1453
	writeDrainCycleState(t, root, cid)
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

func TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip(t *testing.T) {
	t.Parallel()
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

func TestPromoteInbox_LandedShipWithoutMarkerConsumesOnlyTriageNamedItems(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 1455
	writeDrainCycleState(t, root, cid)
	writeDrainTriageDecision(t, root, cid, "named-by-triage")
	writeClosesBuildReport(t, root, cid)
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

func TestPromoteInbox_AbsentBuildReportIsNotAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 1456
	writeDrainCycleState(t, root, cid)
	writeDrainTriageDecision(t, root, cid, "named-by-triage")
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
