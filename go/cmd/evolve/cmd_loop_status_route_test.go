package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func installRecordingLoopLauncher(t *testing.T) *[]loopConfig {
	t.Helper()
	launches := &[]loopConfig{}
	prev := loopLaunchFn
	t.Cleanup(func() { loopLaunchFn = prev })
	loopLaunchFn = func(cfg loopConfig, _ io.Reader, _, _ io.Writer) int {
		*launches = append(*launches, cfg)
		return 0
	}
	return launches
}

func newLoopStatusRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoopStatus_AnExtraWordNeverStartsALoop(t *testing.T) {
	root := newLoopStatusRoot(t)
	cases := [][]string{
		{"status", "foo"},
		{"status", "of", "the", "fleet"},
		{"status", "--json", "foo"},
		{"--project-root", root, "status", "foo"},
		{"--project-root", root, "--strategy", "harden", "status"},
		{"--cli", "auditor=claude", "status", "x"},
		{"3", "status", "foo"},
		{"harden", "status", "foo"},
		{"stop", "now"},
		{"Plan", "the", "refactor"},
		{"help", "me"},
		{"watch", "it"},
	}
	for _, args := range cases {
		launches := installRecordingLoopLauncher(t)
		var stdout, stderr bytes.Buffer
		rc := runLoop(args, nil, &stdout, &stderr)
		if rc != 10 || len(*launches) != 0 {
			t.Errorf("evolve loop %q: rc=%d launches=%d want rc=10 and no launch\nstderr:\n%s", args, rc, len(*launches), stderr.String())
		}
	}
}

func TestLoopStatus_AfterLeadingFlagsPrintsTheReport(t *testing.T) {
	root := newLoopStatusRoot(t)
	for _, args := range [][]string{
		{"--project-root", root, "status"},
		{"--project-root=" + root, "status", "--json"},
	} {
		launches := installRecordingLoopLauncher(t)
		var stdout, stderr bytes.Buffer
		if rc := runLoop(args, nil, &stdout, &stderr); rc != 0 || len(*launches) != 0 || stdout.Len() == 0 {
			t.Errorf("evolve loop %q: rc=%d launches=%d stdout=%q want the report\nstderr:\n%s", args, rc, len(*launches), stdout.String(), stderr.String())
		}
	}
}

func TestLoopStatus_GoalTextStatusStillStartsALoop(t *testing.T) {
	root := newLoopStatusRoot(t)
	launches := installRecordingLoopLauncher(t)
	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--project-root", root, "--goal-text", "status"}, nil, &stdout, &stderr)
	if rc != 0 || len(*launches) != 1 || (*launches)[0].GoalText != "status" {
		t.Fatalf("--goal-text status: rc=%d launches=%+v want one launch with goal status\nstderr:\n%s", rc, *launches, stderr.String())
	}
}

func TestLoopStatusArgs_KeepsLeadingFlagsAndDropsTheVerb(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want []string
		ok   bool
	}{
		{[]string{"status"}, []string{}, true},
		{[]string{"--project-root", "/p", "status", "--json"}, []string{"--project-root", "/p", "--json"}, true},
		{[]string{"--", "status"}, []string{"--"}, true},
		{[]string{"Status"}, nil, false},
		{[]string{"--goal-text", "status"}, nil, false},
		{[]string{"-h"}, nil, false},
		{nil, nil, false},
	}
	for _, tc := range cases {
		if got, ok := loopStatusArgs(tc.args); ok != tc.ok || !slices.Equal(got, tc.want) {
			t.Errorf("loopStatusArgs(%q) = (%q, %v) want (%q, %v)", tc.args, got, ok, tc.want, tc.ok)
		}
	}
}
