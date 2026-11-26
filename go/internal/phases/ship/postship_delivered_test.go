package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestPromoteInbox_DeliveredShipMovesItemsToDeliveredAndNeverReleases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 611
	const id = "train-delivered-item"
	writeInboxFixture(t, root, cid, id)
	opts := &Options{ProjectRoot: root, Runner: landingScriptedRunner(1).runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Delivered: true}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	inbox := filepath.Join(root, ".evolve", "inbox")
	if _, err := os.Stat(filepath.Join(inbox, "delivered", id+".json")); err != nil {
		t.Errorf("a delivered ship must move the item to delivered/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, id+".json")); err == nil {
		t.Error("a delivered ship must not release the item to the inbox root")
	}
	if !anyContains(res.Logs, "delivered: commit") || anyContains(res.Logs, "promotion skipped: unlanded") {
		t.Errorf("logs = %v, want the delivered line and no unlanded warning", res.Logs)
	}
	if state := inboxmover.ResolveDispatchState(inboxmover.Options{ProjectRoot: root}, id).State; state != inboxmover.StateDelivered {
		t.Errorf("state = %q, want delivered", state)
	}
}

func TestScopedCommittedIDs_LogsEachRefusal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const id = "held-elsewhere"
	writeInboxFixture(t, root, 700, id)
	cycleDir := filepath.Join(root, ".evolve", "runs", "cycle-700")
	body, err := os.ReadFile(filepath.Join(cycleDir, "triage-decision.json"))
	if err != nil {
		t.Fatal(err)
	}
	opts := &Options{ProjectRoot: root, Runner: landingScriptedRunner(1).runner(), Stderr: io.Discard}
	res := &RunResult{}

	ids := cycleInbox{opts: opts, mv: inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, res: res, dir: cycleDir, cid: 609, body: body}.scopedCommittedIDs()

	if len(ids) != 0 || !anyContains(res.Logs, "held by cycle 700") {
		t.Errorf("ids = %v logs = %v, want the refusal logged and nothing scoped", ids, res.Logs)
	}
}

func TestShortSHA_NeedsEightCharacters(t *testing.T) {
	t.Parallel()
	if got := shortSHA("0123456789"); got != "01234567" {
		t.Errorf("shortSHA = %q", got)
	}
	if got := shortSHA("abc"); got != "" {
		t.Errorf("shortSHA of a short value = %q, want empty", got)
	}
}

func TestPromoteInbox_FailsWhenTheCycleStateIsUnreadable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "cycle-state.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := &Options{ProjectRoot: root, Runner: landingScriptedRunner(1).runner(), Stderr: io.Discard}

	if err := promoteInbox(context.Background(), opts, &RunResult{Delivered: true}); err == nil {
		t.Error("an unreadable cycle state must fail")
	}
}

func TestPromoteInbox_AFailedDeliveryNamesTheHoldNotARetriage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 612
	writeInboxFixture(t, root, cid, "blocked-item")
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "delivered"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := &Options{ProjectRoot: root, Runner: landingScriptedRunner(1).runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Delivered: true}

	_ = promoteInbox(context.Background(), opts, res)

	if !anyContains(res.Logs, "ends in quarantine/ or stays in processing/") || anyContains(res.Logs, "re-triage") {
		t.Errorf("logs = %v, want the hold named and no re-triage claim", res.Logs)
	}
}
