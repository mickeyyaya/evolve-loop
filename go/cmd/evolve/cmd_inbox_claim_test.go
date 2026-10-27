package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func claimableRoot(t *testing.T, item string) string {
	t.Helper()
	root := emptyInboxRoot(t)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-", "--project-root", root}, strings.NewReader(item), &stdout, &stderr); rc != 0 {
		t.Fatalf("file the item: rc %d %s", rc, stderr.String())
	}
	return root
}

func TestInboxClaim_MovesTheItemIntoTheCycleAndPrintsTheClaim(t *testing.T) {
	t.Parallel()
	root := claimableRoot(t, cliInboxItem)
	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"claim", "cli-inbox-show", "--cycle", "7", "--json", "--project-root", root}, nil, &stdout, &stderr)
	if rc != cliExitOK {
		t.Fatalf("rc = %d, stderr = %s", rc, stderr.String())
	}
	var got inboxClaimResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not one claim document: %v\n%s", err, stdout.String())
	}
	if got.ID != "cli-inbox-show" || got.Cycle != 7 || filepath.Dir(got.Path) != filepath.Join(root, ".evolve", "inbox", "processing", "cycle-7") {
		t.Errorf("claim = %+v, want cli-inbox-show in processing/cycle-7", got)
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Errorf("the claimed file is not at %s: %v", got.Path, err)
	}
	if _, err := os.Stat(got.From); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the root copy %s is still there: %v", got.From, err)
	}
}

func TestInboxClaim_TheTextFormNamesTheClaim(t *testing.T) {
	t.Parallel()
	root := claimableRoot(t, cliInboxItem)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"claim", "cli-inbox-show", "--cycle", "7", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d, stderr = %s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "inbox claim: cli-inbox-show claimed for cycle-7") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestInboxClaim_AWrongCallIsAUsageError(t *testing.T) {
	t.Parallel()
	root := claimableRoot(t, cliInboxItem)
	for _, args := range [][]string{
		{"claim", "cli-inbox-show"},
		{"claim", "cli-inbox-show", "--cycle"},
		{"claim", "cli-inbox-show", "--cycle", "x"},
		{"claim", "cli-inbox-show", "--cycle", "0"},
		{"claim", "--cycle", "7"},
		{"claim", "a", "b", "--cycle", "7"},
		{"claim", "cli-inbox-show", "--cycle", "7", "--bogus"},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(append(args, "--project-root", root), nil, &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), "inbox claim <id> --cycle N") {
			t.Errorf("%v: rc = %d stderr = %q, want 10 and the usage", args, rc, stderr.String())
		}
	}
}

func TestInboxClaim_EachRefusalHasItsExitCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		root func(t *testing.T) string
		id   string
		want int
	}{
		{"absent", func(t *testing.T) string { return claimableRoot(t, cliInboxItem) }, "absent", cliExitFailed},
		{"waiting on a dependency", blockedRoot, "blocked", cliExitRefused},
		{"console routed", consoleRoutedRoot, "cli-inbox-show", cliExitRefused},
		{"the move fails", unmovableRoot, "cli-inbox-show", cliExitIO},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if rc := runInbox([]string{"claim", c.id, "--cycle", "7", "--project-root", c.root(t)}, nil, &stdout, &stderr); rc != c.want {
				t.Errorf("rc = %d, want %d; stderr = %s", rc, c.want, stderr.String())
			}
		})
	}
}

func blockedRoot(t *testing.T) string {
	t.Helper()
	root := claimableRoot(t, cliInboxItem)
	blocked := strings.Replace(strings.Replace(cliInboxItem, `"cli-inbox-show"`, `"blocked"`, 1), `"acceptance"`, `"deps":["cli-inbox-show"],"acceptance"`, 1)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"add", "--file", "-", "--project-root", root}, strings.NewReader(blocked), &stdout, &stderr); rc != 0 {
		t.Fatalf("file the blocked item: rc %d %s", rc, stderr.String())
	}
	return root
}

func consoleRoutedRoot(t *testing.T) string {
	t.Helper()
	root := claimableRoot(t, cliInboxItem)
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"route-console", "cli-inbox-show", "operator owns it", "0", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("route the item to the console: %s", stderr.String())
	}
	return root
}

func unmovableRoot(t *testing.T) string {
	t.Helper()
	root := claimableRoot(t, cliInboxItem)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "processing"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReportInboxClaimError_AnUnknownErrorIsAnIOError(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if rc := reportInboxClaimError(errors.New("disk gone"), &stderr); rc != cliExitIO || !strings.Contains(stderr.String(), "disk gone") {
		t.Errorf("rc = %d stderr = %q, want 2 and the cause", rc, stderr.String())
	}
	if rc := reportInboxClaimError(inboxmover.ErrBadArgs, &stderr); rc != cliExitUsage {
		t.Errorf("a bad argument: rc = %d, want 10", rc)
	}
}
