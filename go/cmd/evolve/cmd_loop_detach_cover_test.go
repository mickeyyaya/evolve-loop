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
