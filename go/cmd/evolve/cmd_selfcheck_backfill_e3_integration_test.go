//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunSelfcheck_AnUnreadableReportCannotBeJudgedAndExitsTwo(t *testing.T) {
	dir := bfRealDir(t, t.TempDir())
	bfChdir(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "build-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	floorRan := false
	orig := buildHandoffFloorFor
	t.Cleanup(func() { buildHandoffFloorFor = orig })
	buildHandoffFloorFor = func(string) core.BuildHandoffFloor { floorRan = true; return nil }
	var out, errw strings.Builder

	rc := runSelfcheck([]string{"build"}, nil, &out, &errw)

	if rc != 2 {
		t.Fatalf("rc = %d, want 2 when the deliverable cannot be read; stdout %q", rc, out.String())
	}
	if !strings.Contains(errw.String(), "[selfcheck] the build handoff cannot be judged: deliverable: read build-report.md") {
		t.Errorf("stderr = %q", errw.String())
	}
	if !strings.Contains(errw.String(), filepath.Join(dir, ".evolve", "cycle-state.json")) {
		t.Errorf("with no --worktree the check must bind the working directory %s: %q", dir, errw.String())
	}
	if out.Len() != 0 || floorRan {
		t.Errorf("an unjudgeable handoff printed %q / ran the floor=%v", out.String(), floorRan)
	}
}

func TestRunSelfcheck_RefusesAnUnknownFlag(t *testing.T) {
	t.Parallel()
	var out, errw strings.Builder

	rc := runSelfcheck([]string{"build", "--nope"}, nil, &out, &errw)

	if rc != 2 || !strings.Contains(errw.String(), "flag provided but not defined: -nope") || out.Len() != 0 {
		t.Fatalf("rc %d stdout %q stderr %q", rc, out.String(), errw.String())
	}
}
