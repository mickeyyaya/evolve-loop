package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const (
	secondDeadDispatch = "01R/1837/build/p998n1"
	laterDeadDispatch  = "01R/1838/build/p997n1"
	tagProofWarning    = "only the tag proof applies"
)

func corruptTreeFor(t *testing.T, root, id string) {
	t.Helper()
	if err := os.MkdirAll(proctree.TreeDir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proctree.TreeFile(proctree.TreeDir(root), id), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func warningsNaming(stderr, id string) int {
	count := 0
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, tagProofWarning) && strings.Contains(line, id) {
			count++
		}
	}
	return count
}

func TestGCDispatchReadOnce_RecordsCreatedAfterTheFirstPassWaitForTheNextRun(t *testing.T) {
	root := t.TempDir()
	member := taggedAt(root, 30469, "node", deadDispatch, "node", "mcp.js")
	original := saveTreeOf(t, root, deadDispatch, member)
	var lateFile string
	h := &fakeGCProcessHost{table: []proctree.Process{member}}
	withGCProcessHost(t, h)
	gcInjectedProcessHost.signal = func(pid int, _ syscall.Signal) error {
		h.sent = append(h.sent, pid)
		if lateFile == "" {
			lateFile = saveTreeOf(t, root, laterDeadDispatch)
		}
		return nil
	}
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)

	failed := r.dispatchProcesses(root, 24)

	removed := treePaths(stdout.String())
	if failed || lateFile == "" || len(removed) != 1 || removed[0] != original {
		t.Errorf("failed=%v removed=%v late=%q, want only the record read at the start of the run (%s)\nstdout=%s\nstderr=%s", failed, removed, lateFile, original, stdout.String(), stderr.String())
	}
	if !exists(lateFile) {
		t.Errorf("the record created after the first pass was removed: %s", lateFile)
	}
}

func TestGCDispatchReadOnce_TheTagProofWarningPrintsOncePerTaggedProcessInEveryMode(t *testing.T) {
	for _, dry := range []bool{true, false} {
		root := t.TempDir()
		first := taggedAt(root, 30469, "node", deadDispatch, "node", "a.js")
		second := taggedAt(root, 30470, "node", secondDeadDispatch, "node", "b.js")
		corruptTreeFor(t, root, deadDispatch)
		corruptTreeFor(t, root, secondDeadDispatch)
		h := &fakeGCProcessHost{table: []proctree.Process{first, second}}
		r, _, stderr := gcRunOver(t, h, root, dry)

		r.dispatchProcesses(root, 24)

		for _, id := range []string{deadDispatch, secondDeadDispatch} {
			if got := warningsNaming(stderr.String(), id); got != 1 {
				t.Errorf("dry=%v dispatch %s: %d tag-proof warnings, want 1\nstderr=%s", dry, id, got, stderr.String())
			}
		}
	}
}

func TestGCDispatchReadOnce_ATaggedProcessWithoutARecordWarnsNeverAndStillStops(t *testing.T) {
	root := t.TempDir()
	member := taggedAt(root, 30469, "node", deadDispatch, "node", "a.js")
	h := &fakeGCProcessHost{table: []proctree.Process{member}}
	r, stdout, stderr := gcRunOver(t, h, root, true)

	r.dispatchProcesses(root, 24)

	if strings.Contains(stderr.String(), tagProofWarning) || !strings.Contains(stdout.String(), "WOULD-STOP pid=30469") {
		t.Errorf("stdout=%s\nstderr=%s, want no warning and the tagged process planned for a stop", stdout.String(), stderr.String())
	}
}

func TestGCDispatchReadOnce_ACorruptRecordStillFallsBackToTheTagProofOnce(t *testing.T) {
	root := t.TempDir()
	member := taggedAt(root, 30469, "node", deadDispatch, "node", "a.js")
	corruptTreeFor(t, root, deadDispatch)
	h := &fakeGCProcessHost{table: []proctree.Process{member}}
	r, stdout, stderr := gcRunOver(t, h, root, true)

	failed := r.dispatchProcesses(root, 24)

	if failed || !strings.Contains(stdout.String(), "WOULD-STOP pid=30469") || warningsNaming(stderr.String(), deadDispatch) != 1 {
		t.Errorf("failed=%v stdout=%s\nstderr=%s, want the tag proof to plan the stop and one warning", failed, stdout.String(), stderr.String())
	}
}
