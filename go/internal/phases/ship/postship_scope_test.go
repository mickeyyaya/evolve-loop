package ship

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeScopeGuardDecision(t *testing.T, root string, cycleID int, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func heldPath(inbox string, cycleID int, id string) string {
	return filepath.Join(inbox, "processing", "cycle-"+strconv.Itoa(cycleID), id+".json")
}

func assertWarnNames(t *testing.T, logs []string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		named := false
		for _, line := range logs {
			if strings.HasPrefix(line, "[ship] WARN:") && strings.Contains(line, id) {
				named = true
				break
			}
		}
		if !named {
			t.Errorf("no [ship] WARN line names the refused id %q:\n%s", id, strings.Join(logs, "\n"))
		}
	}
}

func assertInPlace(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("item left its place (%s): %v", path, err)
	}
}

func TestPromoteInbox_LaneNamingAnotherLanesItemShipsOwnAndLeavesTheOtherInPlace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 61)
	writeDrainLaneScope(t, root, 61, "own-item")
	writeScopeGuardDecision(t, root, 61, `{"top_n":[{"id":"own-item"},{"id":"foreign-top"}],"skip_shipped":[{"task_id":"foreign-skip"}],"superseded":["foreign-superseded"]}`)
	writeClosesBuildReport(t, root, 61, "foreign-closed")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-61"), "own-item")
	for _, id := range []string{"foreign-top", "foreign-skip", "foreign-superseded", "foreign-closed"} {
		writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-62"), id)
	}

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	assertInPlace(t, promotedPath(root, 61, "own-item"))
	for _, id := range []string{"foreign-top", "foreign-skip", "foreign-superseded", "foreign-closed"} {
		assertInPlace(t, heldPath(inbox, 62, id))
		if _, err := os.Stat(promotedPath(root, 61, id)); !os.IsNotExist(err) {
			t.Errorf("lane 61 retired %q, an item lane 62 holds (logs:\n%s)", id, strings.Join(res.Logs, "\n"))
		}
	}
}

func TestPromoteInbox_RefusalWarnNamesEachRefusedID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 63)
	writeDrainLaneScope(t, root, 63, "own-item")
	writeScopeGuardDecision(t, root, 63, `{"top_n":[{"id":"own-item"},{"id":"foreign-top"}],"superseded":["foreign-superseded"]}`)
	writeClosesBuildReport(t, root, 63, "foreign-closed")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-63"), "own-item")
	for _, id := range []string{"foreign-top", "foreign-superseded", "foreign-closed"} {
		writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-64"), id)
	}

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	assertWarnNames(t, res.Logs, "foreign-top", "foreign-superseded", "foreign-closed")
	for _, line := range res.Logs {
		if strings.HasPrefix(line, "[ship] WARN:") && strings.Contains(line, "own-item") {
			t.Errorf("the lane's own item is named as refused: %s", line)
		}
	}
}

func TestPromoteInbox_CycleWithoutLanePinCannotRetireAnotherCyclesHeldItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 65)
	writeDrainTriageDecision(t, root, 65, "own-item", "foreign-item")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, inbox, "own-item")
	writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-66"), "foreign-item")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	assertInPlace(t, promotedPath(root, 65, "own-item"))
	assertInPlace(t, heldPath(inbox, 66, "foreign-item"))
	assertWarnNames(t, res.Logs, "foreign-item")
}

func TestPromoteInbox_LanePinRefusesAnUnheldInboxItemOutsideThePin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 67)
	writeDrainLaneScope(t, root, 67, "own-item")
	writeScopeGuardDecision(t, root, 67, `{"top_n":[{"id":"own-item"},{"id":"stranger"}]}`)
	writeClosesBuildReport(t, root, 67, "stranger-closed")
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-67"), "own-item")
	writeDrainInboxItem(t, inbox, "stranger")
	writeDrainInboxItem(t, inbox, "stranger-closed")

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	assertInPlace(t, promotedPath(root, 67, "own-item"))
	assertInPlace(t, filepath.Join(inbox, "stranger.json"))
	assertInPlace(t, filepath.Join(inbox, "stranger-closed.json"))
	assertWarnNames(t, res.Logs, "stranger", "stranger-closed")
}

func TestPromoteInbox_LanePromotesItsPinnedAndHeldItemsWithoutRefusals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDrainCycleState(t, root, 68)
	writeDrainLaneScope(t, root, 68, "pinned-item")
	writeScopeGuardDecision(t, root, 68, `{"top_n":[{"id":"pinned-item"},{"id":"held-item"}]}`)
	writeClosesBuildReport(t, root, 68, "marker-item")
	inbox := filepath.Join(root, ".evolve", "inbox")
	for _, id := range []string{"pinned-item", "held-item", "marker-item"} {
		writeDrainInboxItem(t, filepath.Join(inbox, "processing", "cycle-68"), id)
	}

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	for _, id := range []string{"pinned-item", "held-item", "marker-item"} {
		assertInPlace(t, promotedPath(root, 68, id))
	}
	for _, line := range res.Logs {
		if strings.Contains(line, "refused") {
			t.Errorf("a legitimate lane run logged a refusal: %s", line)
		}
	}
}
