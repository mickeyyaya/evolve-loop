package inboxcmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const cliInboxItem = `{"id":"cli-inbox-show","kind":"feature","priority_class":"debuggability","weight":0.5,` +
	`"title":"evolve inbox show prints one item","files":["go/cmd/evolve/cmd_inbox.go"],` +
	`"summary":"No verb prints one item.","fix":"Add evolve inbox show <id>.",` +
	`"acceptance":["evolve inbox show <id> prints the item"]}`

func TestCmd_InboxAdd_FilesAnItemFromStdinThatBatchesLists(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(cliInboxItem), &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "cli-inbox-show.json") || !strings.Contains(stdout.String(), "lane-dispatchable") {
		t.Fatalf("rc = %d stdout = %q stderr = %q", rc, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if rc := runInboxCmd([]string{"batches"}, nil, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "cli-inbox-show") {
		t.Errorf("evolve inbox batches: rc = %d stdout = %q, want the filed item listed", rc, stdout.String())
	}
}

func TestCmd_InboxAdd_ReadsAFileAndSaysWhenTheItemIsConsoleOwned(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	item := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(item, []byte(strings.Replace(cliInboxItem, `"kind":"feature"`, `"kind":"pipeline-repair"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"add", "--file", item}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "console-owned") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want a filed, console-owned item", rc, stdout.String(), stderr.String())
	}
}

func TestCmd_InboxAdd_RefusesAnInvalidOrDuplicateItem(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer
	if rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 0 {
		t.Fatalf("first filing: rc = %d stderr = %q", rc, stderr.String())
	}
	for name, tc := range map[string]struct{ body, why string }{
		"a duplicate id": {cliInboxItem, "already filed"},
		"no summary":     {strings.Replace(cliInboxItem, `"summary":"No verb prints one item.",`, ``, 1), "summary"},
	} {
		stderr.Reset()
		if rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(tc.body), &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), tc.why) {
			t.Errorf("%s: rc = %d stderr = %q, want 1 naming %q", name, rc, stderr.String(), tc.why)
		}
	}
}

func TestCmd_InboxAdd_RefusesAMalformedRequestOrAnUnreadableFile(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", inboxtest.EmptyRoot(t))
	for name, tc := range map[string]struct {
		args []string
		want int
	}{
		"an extra argument":     {[]string{"add", "cli-inbox-show"}, 10},
		"--file with no path":   {[]string{"add", "--file"}, 10},
		"a file that is absent": {[]string{"add", "--file", filepath.Join(t.TempDir(), "absent.json")}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInboxCmd(tc.args, strings.NewReader(""), &stdout, &stderr); rc != tc.want {
			t.Errorf("%s: rc = %d, want %d (stderr=%q)", name, rc, tc.want, stderr.String())
		}
	}
}

func TestCmd_InboxAdd_AFilingFaultExits2(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stdout, stderr bytes.Buffer

	if rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(cliInboxItem), &stdout, &stderr); rc != 2 {
		t.Errorf("rc = %d, want 2 for an inbox that cannot be written (stderr=%q)", rc, stderr.String())
	}
}

func TestCmd_InboxAdd_JudgesWithTheLaneForbiddenPredicate(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", inboxtest.EmptyRoot(t))
	body := strings.Replace(cliInboxItem, `"files":["go/cmd/evolve/cmd_inbox.go"]`, `"files":["go/internal/guards/phase.go"]`, 1)
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(body), &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "console-owned: protected fix surface") {
		t.Errorf("rc = %d stdout = %q, want the lane-forbidden predicate's verdict", rc, stdout.String())
	}
}

func TestCmd_InboxAdd_WarnsOnAMonotonicBinaryTargetButStillFiles(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	item := strings.Replace(cliInboxItem, `"acceptance":["evolve inbox show <id> prints the item"]`,
		`"class":"task-contract-design","acceptance":["reduce the backlog to at most 25 items","evolve inbox show <id> prints the item"]`, 1)
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(item), &stdout, &stderr)

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
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	item := strings.Replace(cliInboxItem, `"acceptance":["evolve inbox show <id> prints the item"]`,
		`"class":"feature","acceptance":["reduce the backlog to at most 25 items"]`, 1)
	var stdout, stderr bytes.Buffer

	if rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(item), &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d stderr = %q", rc, stderr.String())
	}
	if strings.Contains(stderr.String(), "binary absolute target") {
		t.Errorf("an absolute target is the right contract for one-shot work; got warning %q", stderr.String())
	}
}

func TestCmd_InboxAdd_RefusesAPriorityClassThePolicyOrderLacks(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	urgent := strings.Replace(cliInboxItem, `"priority_class":"debuggability"`, `"priority_class":"urgent"`, 1)
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(urgent), &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), `priority_class "urgent"`) || !strings.Contains(stderr.String(), "correctness, stability, performance, debuggability, feature, maintainability, hygiene, security") {
		t.Fatalf("rc = %d stderr = %q, want 1 naming the class and the compiled class order", rc, stderr.String())
	}
	inboxtest.WritePolicy(t, root, `{"inbox_priority": {"class_order": ["urgent", "security"]}}`)
	stderr.Reset()
	if rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(urgent), &stdout, &stderr); rc != 0 {
		t.Errorf("a class the policy's own order names: rc = %d stderr = %q, want it filed", rc, stderr.String())
	}
}

func TestCmd_InboxAdd_AMalformedPolicyIsAFaultAndFilesNothing(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	inboxtest.WritePolicy(t, root, `{"inbox_priority": {"class_ordr": ["security"]}}`)
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"add", "--file", "-"}, strings.NewReader(cliInboxItem), &stdout, &stderr)

	if rc != 2 || !strings.Contains(stderr.String(), "class_ordr") {
		t.Errorf("rc = %d stderr = %q, want 2 naming the bad key", rc, stderr.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(root, ".evolve", "inbox")); len(entries) != 0 {
		t.Errorf("a policy fault filed %d entries", len(entries))
	}
}

func TestCmd_InboxAdd_WiresSignalCenterForConsoleOwnedItems(t *testing.T) {
	root := inboxtest.EmptyRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var events []signalcenter.Event
	deps := testDeps()
	deps.NewSignalCenter = func(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
		c := signalcenter.New()
		c.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
		return c, c.Flush
	}
	consoleItem := strings.Replace(cliInboxItem, `"kind":"feature"`, `"kind":"pipeline-repair"`, 1)
	var stdout, stderr bytes.Buffer

	if rc := Run([]string{"add", "--file", "-"}, strings.NewReader(consoleItem), &stdout, &stderr, deps); rc != 0 {
		t.Fatalf("console filing rc = %d stderr = %q", rc, stderr.String())
	}
	if len(events) != 1 || events[0].Code != signalcenter.Code("INBOX_ITEM_FILED_CONSOLE") || events[0].Fields["class"] != "kind" {
		t.Fatalf("events = %+v, want one INBOX_ITEM_FILED_CONSOLE class=kind", events)
	}

	events = nil
	laneItem := `{"id":"cli-inbox-lane","kind":"feature","priority_class":"debuggability","weight":0.5,` +
		`"title":"lane dispatchable item","files":["go/internal/zebra/zebra.go"],` +
		`"summary":"Separate lane item.","fix":"Add lane helper.",` +
		`"acceptance":["lane helper works"]}`
	if rc := Run([]string{"add", "--file", "-"}, strings.NewReader(laneItem), &stdout, &stderr, deps); rc != 0 {
		t.Fatalf("lane filing rc = %d stderr = %q", rc, stderr.String())
	}
	if len(events) != 0 {
		t.Errorf("lane filing emitted %+v, want none", events)
	}
}
