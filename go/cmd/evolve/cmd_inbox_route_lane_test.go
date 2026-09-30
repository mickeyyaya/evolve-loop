package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pipelineRepairInbox(t *testing.T, taskID, body string) string {
	t.Helper()
	d := t.TempDir()
	inbox := filepath.Join(d, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, taskID+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCmd_InboxRouteLane_ALaneCanNowClaimAPipelineRepairItem(t *testing.T) {
	d := pipelineRepairInbox(t, "task-1", `{"id":"task-1","kind":"pipeline-repair","files":["go/internal/cycleclassify/classify.go"]}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer
	if rc := runInboxMover([]string{"claim", "task-1", "1779"}, nil, &stdout, &stderr); rc != 3 {
		t.Fatalf("claim before the route: rc = %d, want 3 (pipeline-repair work is console-owned by default)", rc)
	}
	stdout.Reset()
	stderr.Reset()

	rc := runInbox([]string{"route-lane", "task-1", "  operator: batch pipeline repairs through the loop  "}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	if got := readRouted(t, filepath.Join(d, ".evolve", "inbox", "task-1.json")); got.Route != "lane" || got.RoutedReason != "operator: batch pipeline repairs through the loop" {
		t.Errorf("item = %+v; want route lane with the trimmed reason", got)
	}
	if !strings.Contains(stdout.String(), "task-1") {
		t.Errorf("stdout names the routed item: %q", stdout.String())
	}
	if rc := runInboxMover([]string{"claim", "task-1", "1780"}, nil, &stdout, &stderr); rc != 0 {
		t.Errorf("claim after the route: rc = %d, want 0; a lane may now take the item\nstderr=%s", rc, stderr.String())
	}
}

func TestCmd_InboxRouteLane_RefusesAnAgentAutofiledPipelineItem(t *testing.T) {
	d := pipelineRepairInbox(t, "task-1", `{"id":"task-1","kind":"pipeline-repair","injected_by":"loop-halt"}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"route-lane", "task-1", "operator"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "agent-autofiled") {
		t.Errorf("rc = %d stderr = %q; want 1 naming why the item stays operator-owned", rc, stderr.String())
	}
}

func TestCmd_InboxRouteLane_RefusesAMalformedOrUnknownRequest(t *testing.T) {
	d := pipelineRepairInbox(t, "task-1", `{"id":"task-1","kind":"pipeline-repair"}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"no reason", []string{"route-lane", "task-1"}, 10},
		{"a blank reason", []string{"route-lane", "task-1", "  "}, 10},
		{"an unknown id", []string{"route-lane", "no-such-task", "operator"}, 1},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(tc.args, nil, &stdout, &stderr); rc != tc.want {
			t.Errorf("%s: rc = %d, want %d (stderr=%q)", tc.name, rc, tc.want, stderr.String())
		}
	}
}

func TestCmd_InboxRouteLane_RefusesADeclaredProtectedFile(t *testing.T) {
	d := pipelineRepairInbox(t, "task-1", `{"id":"task-1","kind":"bug","files":["go/internal/guards/phase.go"]}`)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"route-lane", "task-1", "operator"}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "protected fix surface") {
		t.Errorf("rc = %d stderr = %q; want 1 naming the protected fix surface", rc, stderr.String())
	}
}

func TestCmd_InboxRouteLane_AnUnreadableInboxExits2(t *testing.T) {
	d := pipelineRepairInbox(t, "task-1", `{"id":"task-1","kind":"pipeline-repair"}`)
	inbox := filepath.Join(d, ".evolve", "inbox")
	if err := os.Chmod(inbox, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"route-lane", "task-1", "operator"}, nil, &stdout, &stderr); rc != 2 {
		t.Errorf("rc = %d, want 2 for an inbox that cannot be read (stderr=%q)", rc, stderr.String())
	}
}
