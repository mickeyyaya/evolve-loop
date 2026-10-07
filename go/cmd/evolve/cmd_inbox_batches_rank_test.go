package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestInboxBatches_ListsBatchesInRankOrderWithEachScore(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a.json": `{"id":"heavier-security","weight":0.6,"priority_class":"security"}`,
		"b.json": `{"id":"lighter-correctness","weight":0.55,"priority_class":"correctness"}`,
		"c.json": `{"id":"blocker","weight":0.5,"priority_class":"correctness"}`,
		"d.json": `{"id":"waits-on-blocker","weight":0.9,"priority_class":"stability","deps":["blocker"]}`,
	} {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"batches"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	want := "4 items -> 3 batches\n" +
		"- batch 1 (no shared signal): blocker\n  - blocker: score 0.4750, top factor base\n" +
		"- batch 2 (no shared signal): lighter-correctness\n  - lighter-correctness: score 0.4475, top factor base\n" +
		"- batch 3 (no shared signal): heavier-security\n  - heavier-security: score 0.2950, top factor base\n"
	if !strings.HasPrefix(stdout.String(), want) {
		t.Errorf("stdout =\n%s\nwant prefix\n%s", stdout.String(), want)
	}
}

func TestInboxBatches_RanksWithTheEvolveDirsPolicy(t *testing.T) {
	root := t.TempDir()
	fixtures.MustWrite(t, filepath.Join(root, ".evolve", "inbox", "a.json"), `{"id":"heavier-security","weight":0.6,"priority_class":"security"}`)
	fixtures.MustWrite(t, filepath.Join(root, ".evolve", "inbox", "b.json"), `{"id":"lighter-correctness","weight":0.55,"priority_class":"correctness"}`)
	fixtures.MustWrite(t, filepath.Join(root, ".evolve", "policy.json"), `{"inbox_priority":{"class_order":["security","stability","correctness"]}}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"batches"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "- batch 1 (no shared signal): heavier-security\n") {
		t.Errorf("the evolve dir's class_order puts security first:\n%s", stdout.String())
	}
}
