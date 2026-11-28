package inboxcmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
)

func editLines(t *testing.T, root string) int {
	t.Helper()
	count := 0
	for _, line := range lifecycleLines(t, root) {
		if line.Action == "edit" {
			count++
		}
	}
	return count
}

func TestCmd_InboxEdit_RefusesAPriorityChangeWithoutAReasonAndChangesNothing(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"weight":                 {"edit", "cli-inbox-show", "--set", "weight=0.7"},
		"priority class":         {"edit", "cli-inbox-show", "--set", "priority_class=stability"},
		"blank reason":           {"edit", "cli-inbox-show", "--set", "weight=0.7", "--reason", "  "},
		"mixed with a title":     {"edit", "cli-inbox-show", "--set", "title=a new title", "--set", "weight=0.7"},
		"class with blank":       {"edit", "cli-inbox-show", "--set", "priority_class=stability", "--reason", ""},
		"weight and class mixed": {"edit", "cli-inbox-show", "--set", "weight=0.7", "--set", "priority_class=stability"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			root, path := fileCLIItem(t)
			before, _ := os.ReadFile(path)
			editsBefore := editLines(t, root)
			var stdout, stderr bytes.Buffer

			rc := runInboxCmd(append([]string{"edit", "--project-root", root}, args[1:]...), nil, &stdout, &stderr)

			if rc != 10 || !strings.Contains(stderr.String(), "needs a reason") {
				t.Errorf("rc = %d stderr = %q, want rc 10 naming the missing reason", rc, stderr.String())
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
				t.Errorf("the item changed:\n%s", after)
			}
			if got := editLines(t, root); got != editsBefore {
				t.Errorf("edit ledger lines = %d, want %d", got, editsBefore)
			}
		})
	}
}

func TestCmd_InboxEdit_APriorityChangeWithAReasonRecordsItInTheOneEditLine(t *testing.T) {
	t.Parallel()
	for _, set := range []string{"weight=0.7", "priority_class=stability"} {
		root, path := fileCLIItem(t)
		editsBefore := editLines(t, root)
		var stdout, stderr bytes.Buffer

		rc := runInboxCmd([]string{"edit", "--project-root", root, "cli-inbox-show", "--set", set, "--reason", "operator review"}, nil, &stdout, &stderr)

		if rc != 0 {
			t.Fatalf("%s: rc = %d stderr = %q", set, rc, stderr.String())
		}
		if inboxtest.ItemFields(t, path)[strings.Split(set, "=")[0]] == nil {
			t.Errorf("%s: field missing after the edit", set)
		}
		lines := lifecycleLines(t, root)
		last := lines[len(lines)-1]
		if editLines(t, root) != editsBefore+1 || last.Action != "edit" || !strings.Contains(last.Message, "set "+set) || !strings.Contains(last.Message, "reason: operator review") {
			t.Errorf("%s: ledger tail = %+v, want the single edit line carrying the reason", set, last)
		}
	}
}

func TestCmd_InboxEdit_ARejectedPriorityClassIsRefusedAndChangesNothing(t *testing.T) {
	t.Parallel()
	root, path := fileCLIItem(t)
	before, _ := os.ReadFile(path)
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"edit", "--project-root", root, "cli-inbox-show", "--set", "priority_class=not-a-class", "--reason", "typo"}, nil, &stdout, &stderr)

	if rc != 1 {
		t.Errorf("rc = %d stderr = %q, want 1", rc, stderr.String())
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("the item changed:\n%s", after)
	}
	if editLines(t, root) != 0 {
		t.Errorf("a refused class left an edit ledger line")
	}
}

func TestCmd_InboxEdit_AReasonlessPriorityEditIsRefusedBeforeTheQueue(t *testing.T) {
	t.Parallel()
	root := inboxtest.EmptyRoot(t)
	queued := false
	d := queuedDeps()
	d.QueueGate = func(string, string, []string, bool, io.Writer, io.Writer) (int, bool) {
		queued = true
		return 7, true
	}

	rc, _, stderr := runWith(d, root, "edit", "x", "--set", "weight=0.4")

	if queued || rc != 10 || !strings.Contains(stderr, "needs a reason") {
		t.Errorf("queued = %v rc = %d stderr %q, want a refusal with rc 10 and nothing queued", queued, rc, stderr)
	}
}

func TestCmd_InboxEdit_AnUnreadablePolicyFailsALoudlyWithoutChangingTheItem(t *testing.T) {
	t.Parallel()
	root, path := fileCLIItem(t)
	before, _ := os.ReadFile(path)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	rc := runInboxCmd([]string{"edit", "--project-root", root, "cli-inbox-show", "--set", "weight=0.7", "--reason", "operator review"}, nil, &stdout, &stderr)

	if rc != 2 || !strings.Contains(stderr.String(), "inbox edit:") {
		t.Errorf("rc = %d stderr = %q, want rc 2 naming the policy fault", rc, stderr.String())
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("the item changed")
	}
}
