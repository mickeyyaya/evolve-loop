package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func liveLaneLease(t *testing.T, root string, ownerAge time.Duration) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1790")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}, time.Now().Add(-ownerAge)); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_InboxRouteVerbs_RefuseWhileALoopLaneIsLiveAndLeaveTheItemByteIdentical(t *testing.T) {
	for _, args := range [][]string{
		{"route-console", "cli-inbox-show", "operator owns this", "1790"},
		{"route-lane", "cli-inbox-show", "lane may take it"},
	} {
		root, path := fileCLIItem(t)
		liveLaneLease(t, root, 0)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		rc := runInbox(args, nil, &stdout, &stderr)
		if rc != 1 || !strings.Contains(stderr.String(), "a loop lane is live") || !strings.Contains(stderr.String(), "cycle-1790") || !strings.Contains(stderr.String(), args[0]) {
			t.Errorf("%v: rc = %d stderr = %q, want 1 naming the verb and the live lane", args, rc, stderr.String())
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Errorf("%v: a refused verb changed the item: %v\n%s", args, err, after)
		}
		entries, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*"))
		if len(entries) != 1 {
			t.Errorf("%v: a refused verb left %v in the inbox, want only the original item", args, entries)
		}
	}
}

func TestCmd_InboxRouteVerbs_ARefusalNeverMasksUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"route-console", "cli-inbox-show", "reason", "not-a-cycle"},
		{"route-console", "cli-inbox-show"},
		{"route-lane", "cli-inbox-show"},
	} {
		root, _ := fileCLIItem(t)
		liveLaneLease(t, root, 0)
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 10 {
			t.Errorf("%v: rc = %d stderr = %q, want 10 (usage is checked before the lane gate)", args, rc, stderr.String())
		}
	}
}

func TestCmd_InboxRouteVerbs_AStaleLeaseDoesNotRefuse(t *testing.T) {
	for _, args := range [][]string{
		{"route-console", "cli-inbox-show", "operator owns this", "1790"},
		{"route-lane", "cli-inbox-show", "lane may take it"},
	} {
		root, _ := fileCLIItem(t)
		liveLaneLease(t, root, time.Hour)
		var stdout, stderr bytes.Buffer
		if rc := runInbox(args, nil, &stdout, &stderr); rc != 0 {
			t.Errorf("%v with a stale lease: rc = %d stderr = %q, want the route applied", args, rc, stderr.String())
		}
	}
}
