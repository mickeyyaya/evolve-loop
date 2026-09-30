package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cliInboxItem = `{"id":"cli-inbox-show","kind":"feature","priority_class":"debuggability","weight":0.5,` +
	`"title":"evolve inbox show prints one item","files":["go/cmd/evolve/cmd_inbox.go"],` +
	`"summary":"No verb prints one item.","fix":"Add evolve inbox show <id>.",` +
	`"acceptance":["evolve inbox show <id> prints the item"]}`

func emptyInboxRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCmd_InboxAdd_FilesAnItemFromStdinThatBatchesLists(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"add"}, strings.NewReader(cliInboxItem), &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "cli-inbox-show.json") || !strings.Contains(stdout.String(), "lane-dispatchable") {
		t.Fatalf("rc = %d stdout = %q stderr = %q", rc, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if rc := runInbox([]string{"batches"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "cli-inbox-show") {
		t.Errorf("evolve inbox batches: rc = %d stdout = %q, want the filed item listed", rc, stdout.String())
	}
}

func TestCmd_InboxAdd_ReadsAFileAndSaysWhenTheItemIsConsoleOwned(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	item := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(item, []byte(strings.Replace(cliInboxItem, `"kind":"feature"`, `"kind":"pipeline-repair"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"add", "--file", item}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "console-owned") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want a filed, console-owned item", rc, stdout.String(), stderr.String())
	}
}

func TestCmd_InboxAdd_RefusesAnInvalidOrDuplicateItem(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Fatalf("first filing: rc = %d stderr = %q", rc, stderr.String())
	}
	for name, tc := range map[string]struct{ body, why string }{
		"a duplicate id": {cliInboxItem, "already filed"},
		"no summary":     {strings.Replace(cliInboxItem, `"summary":"No verb prints one item.",`, ``, 1), "summary"},
	} {
		stderr.Reset()
		if rc := runInbox([]string{"add"}, strings.NewReader(tc.body), &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), tc.why) {
			t.Errorf("%s: rc = %d stderr = %q, want 1 naming %q", name, rc, stderr.String(), tc.why)
		}
	}
}

func TestCmd_InboxAdd_RefusesAMalformedRequestOrAnUnreadableFile(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	for name, tc := range map[string]struct {
		args []string
		want int
	}{
		"an extra argument":     {[]string{"add", "cli-inbox-show"}, 10},
		"--file with no path":   {[]string{"add", "--file"}, 10},
		"a file that is absent": {[]string{"add", "--file", filepath.Join(t.TempDir(), "absent.json")}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(tc.args, strings.NewReader(""), &stdout, &stderr); rc != tc.want {
			t.Errorf("%s: rc = %d, want %d (stderr=%q)", name, rc, tc.want, stderr.String())
		}
	}
}

func TestCmd_InboxAdd_AFilingFaultExits2(t *testing.T) {
	root := emptyInboxRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"add"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 2 {
		t.Errorf("rc = %d, want 2 for an inbox that cannot be written (stderr=%q)", rc, stderr.String())
	}
}

func TestCmd_InboxAdd_JudgesWithTheLaneForbiddenPredicate(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", emptyInboxRoot(t))
	body := strings.Replace(cliInboxItem, `"files":["go/cmd/evolve/cmd_inbox.go"]`, `"files":["go/internal/guards/phase.go"]`, 1)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"add"}, strings.NewReader(body), &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "console-owned: protected fix surface") {
		t.Errorf("rc = %d stdout = %q, want the lane-forbidden predicate's verdict", rc, stdout.String())
	}
}

func TestCmd_InboxAdd_WarnsOnAMonotonicBinaryTargetButStillFiles(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	item := strings.Replace(cliInboxItem, `"acceptance":["evolve inbox show <id> prints the item"]`,
		`"class":"task-contract-design","acceptance":["reduce the backlog to at most 25 items","evolve inbox show <id> prints the item"]`, 1)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"add"}, strings.NewReader(item), &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "cli-inbox-show.json") {
		t.Fatalf("the lint is advisory, the item must still file; rc = %d stdout = %q stderr = %q", rc, stdout.String(), stderr.String())
	}
	if got := strings.Count(stderr.String(), "binary absolute target"); got != 1 {
		t.Errorf("want exactly one warning naming the absolute-count criterion, got %d in %q", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "reduce the backlog to at most 25 items") {
		t.Errorf("the warning must quote the offending criterion; got %q", stderr.String())
	}
}

func TestCmd_InboxAdd_NoMonotonicWarningForAOneShotClass(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	item := strings.Replace(cliInboxItem, `"acceptance":["evolve inbox show <id> prints the item"]`,
		`"class":"feature","acceptance":["reduce the backlog to at most 25 items"]`, 1)
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"add"}, strings.NewReader(item), &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if strings.Contains(stderr.String(), "binary absolute target") {
		t.Errorf("an absolute target is the right contract for one-shot work; got warning %q", stderr.String())
	}
}
