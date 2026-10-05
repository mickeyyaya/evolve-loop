package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func fileCLIItem(t *testing.T) (root, path string) {
	t.Helper()
	root = emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Fatalf("add: rc = %d stderr = %q", rc, stderr.String())
	}
	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-cli-inbox-show.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("the filed item: %v %v", matches, err)
	}
	return root, matches[0]
}

func itemFields(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestCmd_InboxEdit_SetWeightRewritesOnlyThatFieldAndRecordsIt(t *testing.T) {
	root, path := fileCLIItem(t)
	before := itemFields(t, path)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	after := itemFields(t, path)
	if after["weight"] != 0.4 {
		t.Errorf("weight = %v, want 0.4", after["weight"])
	}
	delete(before, "weight")
	delete(after, "weight")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("another field changed:\nbefore %v\nafter  %v", before, after)
	}
	lines := lifecycleLines(t, root)
	if last := lines[len(lines)-1]; last.Action != "edit" || last.TaskID != "cli-inbox-show" || last.Message != "set weight=0.4" {
		t.Errorf("ledger tail = %+v, want the edit line", last)
	}
	if !strings.Contains(stdout.String(), "cli-inbox-show") {
		t.Errorf("stdout names the edited item: %q", stdout.String())
	}
}

func TestCmd_InboxEdit_RefusesAnUnbackedDepOrAFiledIDAndLeavesTheFile(t *testing.T) {
	root, path := fileCLIItem(t)
	idless := filepath.Join(root, ".evolve", "inbox", "2026-09-27T00-00-00Z-idless.json")
	if err := os.WriteFile(idless, []byte(`{"title":"filed by hand","weight":0.2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"a dep naming no item": {"edit", "cli-inbox-show", "--add", "deps=no-such-item"},
		"an id already filed":  {"edit", idless, "--set", "id=cli-inbox-show"},
	} {
		target := path
		if args[1] == idless {
			target = idless
		}
		before, _ := os.ReadFile(target)
		var stdout, stderr bytes.Buffer

		rc := runInbox(args, nil, &stdout, &stderr)

		if rc != 1 {
			t.Errorf("%s: rc = %d, want 1 (stderr=%q)", name, rc, stderr.String())
		}
		if after, _ := os.ReadFile(target); !bytes.Equal(before, after) {
			t.Errorf("%s: the item changed:\n%s", name, after)
		}
	}
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"edit", idless, "--set", "id=hand-filed"}, nil, &stdout, &stderr); rc != 0 || itemFields(t, idless)["id"] != "hand-filed" {
		t.Errorf("stamping an id-less item named by its path: rc = %d stderr = %q", rc, stderr.String())
	}
}

func TestCmd_InboxEdit_RoutesLifecycleFieldsAndMalformedEditsAreUsageErrors(t *testing.T) {
	_, path := fileCLIItem(t)
	before, _ := os.ReadFile(path)
	for _, args := range [][]string{
		{"edit", "cli-inbox-show", "--set", "route=lane"},
		{"edit", "cli-inbox-show", "--set", "routed_reason=x"},
		{"edit", "cli-inbox-show", "--set", "failure_count=0"},
		{"edit", "cli-inbox-show", "--add", "weight=0.1"},
		{"edit", "cli-inbox-show", "--set", "weight"},
		{"edit", "cli-inbox-show", "--set", "=0.4"},
		{"edit", "cli-inbox-show"},
		{"edit", "--set", "weight=0.4"},
		{"edit", "cli-inbox-show", "--set", "weight=0.4", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 {
			t.Errorf("%v: rc = %d, want 10 (stderr=%q)", args, rc, stderr.String())
		}
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("a usage error changed the item:\n%s", after)
	}
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"edit", "no-such-item", "--set", "weight=0.4"}, nil, &stdout, &stderr); rc != 1 {
		t.Errorf("an unknown id: rc = %d, want 1", rc)
	}
}

func TestCmd_InboxEdit_AnUnwritableInboxExits2(t *testing.T) {
	root, _ := fileCLIItem(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4"}, nil, &stdout, &stderr); rc != 2 {
		t.Errorf("rc = %d, want 2 (stderr=%q)", rc, stderr.String())
	}
}

func TestCmd_InboxEdit_ASetValueKeepsEveryEqualsSignAfterTheFirst(t *testing.T) {
	_, path := fileCLIItem(t)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "fix=a=b"}, nil, &stdout, &stderr)

	if rc != 0 || itemFields(t, path)["fix"] != "a=b" {
		t.Errorf("rc = %d fix = %v stderr = %q, want field fix set to a=b", rc, itemFields(t, path)["fix"], stderr.String())
	}
}

func TestCmd_InboxEdit_RefusesToOpenAConsoleOwnedItemToLanes(t *testing.T) {
	root := curationRoot(t, map[string]string{"x.json": `{"id":"x","kind":"bug","weight":0.5,"files":["go/internal/guards/phase.go"]}`})
	path := filepath.Join(root, ".evolve", "inbox", "x.json")
	before, _ := os.ReadFile(path)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"edit", "x", "--remove", "files=go/internal/guards/phase.go"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "evolve inbox route-lane") {
		t.Errorf("rc = %d stderr = %q, want 1 pointing to route-lane", rc, stderr.String())
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("the item changed:\n%s", after)
	}
}

func TestCmd_InboxCurationVerbs_RefuseWhileALoopLaneIsLive(t *testing.T) {
	root, path := fileCLIItem(t)
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	for _, args := range [][]string{
		{"edit", "cli-inbox-show", "--set", "weight=0.4"},
		{"withdraw", "cli-inbox-show", "mistake"},
		{"verify", "cli-inbox-show", "--evidence", "checked"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), "a loop lane is live") || !strings.Contains(stderr.String(), "cycle-1790") {
			t.Errorf("%v: rc = %d stderr = %q, want 1 naming the live lane", args, rc, stderr.String())
		}
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("a refused verb changed the item:\n%s", after)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"edit", "cli-inbox-show", "--set", "weight=0.4"}, nil, &stdout, &stderr); rc != 0 {
		t.Errorf("a stale lease: rc = %d stderr = %q, want the edit written", rc, stderr.String())
	}
	var shown bytes.Buffer
	if rc := runInbox([]string{"show", "cli-inbox-show"}, nil, &shown, &stderr); rc != 0 {
		t.Errorf("show is read-only and never refused: rc = %d", rc)
	}
}
