package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	readyItem   = `{"id":"ready-one","kind":"feature","title":"a lane may take this","weight":0.5,"summary":"the ready summary"}`
	consoleItem = `{"id":"console-one","kind":"pipeline-repair","title":"the console owns this","weight":0.4}`
	waitingItem = `{"id":"waiting-one","kind":"feature","title":"this waits on ready-one","weight":0.3,"deps":["ready-one"]}`
)

func curationRoot(t *testing.T, items map[string]string) string {
	t.Helper()
	root := emptyInboxRoot(t)
	seedCurationItems(t, root, items)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root
}

func seedCurationItems(t *testing.T, root string, items map[string]string) {
	t.Helper()
	for rel, body := range items {
		path := filepath.Join(root, ".evolve", "inbox", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func menuRoot(t *testing.T) string {
	return curationRoot(t, map[string]string{
		"2026-09-30T00-00-00Z-ready-one.json":     readyItem,
		"2026-09-30T00-00-00Z-console-one.json":   consoleItem,
		"2026-09-30T00-00-00Z-waiting-one.json":   waitingItem,
		"consumed/2026-09-01T00-00-00Z-gone.json": `{"id":"gone","kind":"feature"}`,
	})
}

type lifecycleLine struct {
	Kind    string `json:"kind"`
	Action  string `json:"action"`
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
	GitHead string `json:"git_head"`
}

func lifecycleLines(t *testing.T, root string) []lifecycleLine {
	t.Helper()
	f, err := os.Open(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatalf("the ledger: %v", err)
	}
	defer func() { _ = f.Close() }()
	var out []lifecycleLine
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 1<<20), 1<<20)
	for scan.Scan() {
		var line lifecycleLine
		if json.Unmarshal(scan.Bytes(), &line) == nil && line.Kind == "inbox-lifecycle" {
			out = append(out, line)
		}
	}
	return out
}

func TestCmd_InboxShow_PrintsTheItemAndItsLaneMenuPlace(t *testing.T) {
	menuRoot(t)
	for id, want := range map[string][]string{
		"ready-one":   {"ready", "2026-09-30T00-00-00Z-ready-one.json", "the ready summary"},
		"console-one": {"console", "pipeline-repair"},
		"waiting-one": {"waiting", "deps unmet: needs ready-one"},
	} {
		var stdout, stderr bytes.Buffer

		rc := runInbox([]string{"show", id}, nil, &stdout, &stderr)

		if rc != 0 {
			t.Fatalf("show %s: rc = %d stderr = %q", id, rc, stderr.String())
		}
		for _, w := range want {
			if !strings.Contains(stdout.String(), w) {
				t.Errorf("show %s: stdout lacks %q:\n%s", id, w, stdout.String())
			}
		}
	}
}

func TestCmd_InboxShow_JSONCarriesTheWholeItemAndItsPlace(t *testing.T) {
	menuRoot(t)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"show", "--json", "waiting-one"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	var doc struct {
		ID     string         `json:"id"`
		Path   string         `json:"path"`
		Status string         `json:"status"`
		Reason string         `json:"reason"`
		Item   map[string]any `json:"item"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	if doc.ID != "waiting-one" || doc.Path != ".evolve/inbox/2026-09-30T00-00-00Z-waiting-one.json" || doc.Status != "waiting" ||
		doc.Reason != "deps unmet: needs ready-one" || doc.Item["title"] != "this waits on ready-one" {
		t.Errorf("doc = %+v", doc)
	}
}

func TestCmd_InboxShow_AnIDNotPendingExits1NamingWhereItIs(t *testing.T) {
	menuRoot(t)
	for id, why := range map[string]string{"gone": "consumed", "never-filed": "no inbox item"} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox([]string{"show", id}, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), why) {
			t.Errorf("show %s: rc = %d stderr = %q, want 1 naming %q", id, rc, stderr.String(), why)
		}
	}
}

func TestCmd_InboxShow_RefusesAMalformedRequest(t *testing.T) {
	menuRoot(t)
	for _, args := range [][]string{{"show"}, {"show", "a", "b"}, {"show", "--bogus", "a"}, {"show", " "}} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 {
			t.Errorf("%v: rc = %d, want 10", args, rc)
		}
	}
}

func TestCmd_Inbox_HelpNamesEveryVerbAndAnUnknownVerbIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"--help"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	for _, synopsis := range []string{"inbox batches", "inbox list [--status", "inbox show <id>", "inbox add", "inbox edit <id|item-path>",
		"inbox verify <id> --evidence", "inbox withdraw <id> <reason>", "inbox route-console", "inbox route-lane", "inbox consume",
		"inbox quarantine", "inbox ack-fingerprint"} {
		if !strings.Contains(stdout.String(), synopsis) {
			t.Errorf("--help lacks %q:\n%s", synopsis, stdout.String())
		}
	}
	stdout.Reset()
	if rc := runInbox([]string{"rename"}, nil, &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), "inbox withdraw") {
		t.Errorf("an unknown verb: rc = %d stderr = %q, want 10 with the verb list", rc, stderr.String())
	}
}

func TestCmd_Inbox_EveryUsageLineIsItsVerbsSynopsis(t *testing.T) {
	for _, v := range inboxVerbs() {
		if got, want := inboxUsage(v.name), "usage: evolve inbox "+v.synopsis; got != want {
			t.Errorf("inboxUsage(%q) = %q, want %q", v.name, got, want)
		}
	}
	for verb, args := range map[string][]string{
		"show": {"show"}, "list": {"list", "extra"}, "edit": {"edit"}, "withdraw": {"withdraw"}, "verify": {"verify"},
		"add": {"add", "extra"}, "consume": {"consume"}, "route-console": {"route-console"}, "route-lane": {"route-lane"},
		"quarantine": {"quarantine"}, "ack-fingerprint": {"ack-fingerprint"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, strings.NewReader(""), &stdout, &stderr); rc != 10 || !strings.Contains(stderr.String(), inboxUsage(verb)) {
			t.Errorf("%v: rc = %d stderr = %q, want 10 with %q", args, rc, stderr.String(), inboxUsage(verb))
		}
	}
	if !strings.Contains(inboxUsage("list"), "--status "+strings.Join(menuStatusNames[:], "|")) {
		t.Errorf("list's synopsis = %q, want the statuses the placement names", inboxUsage("list"))
	}
}
