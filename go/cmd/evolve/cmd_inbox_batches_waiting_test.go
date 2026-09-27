package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func waitingInboxRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a.json": `{"id":"lane-work","weight":0.9}`,
		"b.json": `{"id":"operator-work","weight":0.96,"route":"console-manual"}`,
		"c.json": `{"id":"blocked-work","weight":0.95,"deps":["operator-work"]}`,
	} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInboxBatches_HoldsBackItemsWaitingOnADependency(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", waitingInboxRoot(t))
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"batches", "--json"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	var doc struct {
		Batches           json.RawMessage `json:"batches"`
		DependencyBlocked []struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
			Reason string `json:"reason"`
		} `json:"dependency_blocked"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if len(doc.DependencyBlocked) != 1 || doc.DependencyBlocked[0].Item.ID != "blocked-work" || doc.DependencyBlocked[0].Reason != "deps unmet: needs operator-work" {
		t.Errorf("the waiting item is its own bucket, shaped like console_routed: %+v", doc.DependencyBlocked)
	}
	if strings.Contains(string(doc.Batches), "blocked-work") {
		t.Errorf("a waiting item is never a batch a lane may draw: %s", stdout.String())
	}
	stdout.Reset()
	if rc := runInbox([]string{"batches"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "1 item(s) waiting on a dependency") {
		t.Errorf("the text worklist names the waiting item loudly: rc=%d\n%s", rc, stdout.String())
	}
}
