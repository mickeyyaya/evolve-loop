//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestLoopDetach_AnUnwritableWriterPidFileWarnsAndTheLaunchGoesOn(t *testing.T) {
	p := newDetachProject(t)
	if err := os.Mkdir(p.log+gcpolicy.LogWriterPIDSuffix, 0o755); err != nil {
		t.Fatal(err)
	}
	probe := stubDetachChild(t, "boom", p)

	rc, stdout, stderr := runDetach(t, p.args())

	if !strings.Contains(stderr, "evolve loop: WARN: --detach: record the log writer pid in "+p.log+gcpolicy.LogWriterPIDSuffix) || !strings.Contains(stderr, "gc can delete this log while the loop writes it") {
		t.Errorf("stderr=%q, want the writer-pid WARN", stderr)
	}
	if rc != 1 || !strings.Contains(stdout, "loop: detached pid") || len(probe.cmds) != 1 {
		t.Errorf("rc=%d stdout=%q, want the launch to go on to the boot wait (the boom child exits 1)", rc, stdout)
	}
}

func TestLoopDetach_AnUnopenableLogIsALaunchFailure(t *testing.T) {
	p := newDetachProject(t)
	p.log = filepath.Join(p.root, "absent", "detach.log")
	probe := stubDetachChild(t, "lease", p)

	rc, _, stderr := runDetach(t, p.args())

	if rc != 1 || !strings.Contains(stderr, "evolve loop: --detach: open log "+p.log) || len(probe.cmds) != 0 {
		t.Errorf("rc=%d stderr=%q started=%d, want exit 1, the open-log error and no child", rc, stderr, len(probe.cmds))
	}
}

func TestRecordLogWriter_ReplacesAMarkerSymlinkInsteadOfWritingThroughIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "loop-x.log")
	elsewhere := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := logPath + gcpolicy.LogWriterPIDSuffix
	if err := os.Symlink(elsewhere, marker); err != nil {
		t.Fatal(err)
	}

	if err := recordLogWriter(logPath, 4242); err != nil {
		t.Fatalf("recordLogWriter: %v", err)
	}

	if got, _ := os.ReadFile(elsewhere); string(got) != "keep\n" {
		t.Errorf("the symlink target = %q, want it untouched: the pid file is replaced by rename, not written in place", got)
	}
	if got, _ := os.ReadFile(marker); string(got) != "4242\n" {
		t.Errorf("marker = %q, want 4242", got)
	}
	if info, err := os.Lstat(marker); err != nil || !info.Mode().IsRegular() {
		t.Errorf("marker is not a regular file: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("dir entries = %d, want 2 (the target and the marker, no temp file)", len(entries))
	}
}
