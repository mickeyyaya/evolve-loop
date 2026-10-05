package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmd_InboxWithdraw_RemovesAPendingItemAndAddFilesTheIDAgain(t *testing.T) {
	root, path := fileCLIItem(t)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"withdraw", "cli-inbox-show", "filed under the wrong scope"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the item is still on disk: %v", err)
	}
	lines := lifecycleLines(t, root)
	if last := lines[len(lines)-1]; last.Action != "withdraw" || last.TaskID != "cli-inbox-show" || !strings.Contains(last.Message, "filed under the wrong scope") {
		t.Errorf("ledger tail = %+v, want the withdraw line", last)
	}
	if rc := runInbox([]string{"add"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Errorf("filing the withdrawn id again: rc = %d stderr = %q", rc, stderr.String())
	}
}

func TestCmd_InboxWithdraw_RefusesAClaimedRoutedOrConsumedItem(t *testing.T) {
	root := curationRoot(t, map[string]string{
		"processing/cycle-12/held.json": `{"id":"held"}`,
		"routed.json":                   `{"id":"routed","route":"console-manual","routed_reason":"a lane found a protected surface","routed_cycle":12}`,
		"consumed/gone.json":            `{"id":"gone"}`,
	})
	for _, id := range []string{"held", "routed", "gone"} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox([]string{"withdraw", id, "mistake"}, nil, &stdout, &stderr); rc != 1 {
			t.Errorf("%s: rc = %d, want 1 (stderr=%q)", id, rc, stderr.String())
		}
	}
	for _, rel := range []string{"processing/cycle-12/held.json", "routed.json", "consumed/gone.json"} {
		if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", rel)); err != nil {
			t.Errorf("%s: %v, want it left in place", rel, err)
		}
	}
}

func TestCmd_InboxWithdraw_RefusesAMalformedRequest(t *testing.T) {
	fileCLIItem(t)
	for _, args := range [][]string{{"withdraw"}, {"withdraw", "cli-inbox-show"}, {"withdraw", "cli-inbox-show", "  "}, {"withdraw", " ", "why"}} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 {
			t.Errorf("%v: rc = %d, want 10", args, rc)
		}
	}
}
