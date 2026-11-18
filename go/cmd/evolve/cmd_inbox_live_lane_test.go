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
		if rc := runInbox(args, nil, &stdout, &stderr); rc != cliExitRefused || !strings.Contains(stderr.String(), "a loop lane is live") || !strings.Contains(stderr.String(), "cycle-1790") {
			t.Errorf("%v: rc = %d stderr = %q, want 3 naming the live lane", args, rc, stderr.String())
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
