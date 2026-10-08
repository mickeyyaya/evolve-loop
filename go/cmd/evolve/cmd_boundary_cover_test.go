package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const coverRunID = "20261008T142501Z"

func TestDispatchBoundaryVerb_RoutesAKnownVerbAndRefusesAnUnknownOne(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	if rc := dispatchBoundaryVerb("teleport", nil, &stdout, &stderr); rc != exitIO || !strings.Contains(stderr.String(), `evolve boundary run: no handler for step "teleport"`) {
		t.Errorf("unknown verb: rc=%d stderr=%q, want exit %d and the no-handler line", rc, stderr.String(), exitIO)
	}
	stderr.Reset()
	if rc := dispatchBoundaryVerb(boundaryLogVerb, []string{"--run-id", "x"}, &stdout, &stderr); rc != exitUsage || !strings.Contains(stderr.String(), boundaryLogPrefix) {
		t.Errorf("boundary-log: rc=%d stderr=%q, want the boundary-log handler to refuse the run id", rc, stderr.String())
	}
}

func TestRunBoundaryLog_ALiveRunRefusesTheLegacyMove(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, ".evolve", boundaryLoopLog)
	writeBoundaryFile(t, legacy, "old loop output\n")
	var stdout, stderr bytes.Buffer

	rc := runBoundaryLog([]string{"--project-root", root, "--run-id", coverRunID}, nil, &stdout, &stderr)

	if rc != exitRefused || !strings.Contains(stderr.String(), "a run is live") || readText(t, legacy) != "old loop output\n" {
		t.Errorf("rc=%d stderr=%q, want exit %d, the live-run refusal and the legacy log kept", rc, stderr.String(), exitRefused)
	}
}

func TestRunBoundaryLog_AnUnmakeableLogDirIsAnIOError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeBoundaryFile(t, filepath.Join(root, ".evolve", "logs"), "")
	var stdout, stderr bytes.Buffer

	rc := runBoundaryLog([]string{"--project-root", root, "--run-id", coverRunID}, nil, &stdout, &stderr)

	if rc != exitIO || !strings.Contains(stderr.String(), "make the run log dir") || stdout.Len() != 0 {
		t.Errorf("rc=%d stdout=%q stderr=%q, want exit %d and the make-dir error", rc, stdout.String(), stderr.String(), exitIO)
	}
}

func TestPrepareBoundaryLog_AnUnswitchableCurrentLinkIsAnError(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeBoundaryFile(t, filepath.Join(evolveDir, "logs", "current", "keep"), "")

	err := prepareBoundaryLog(evolveDir, coverRunID, noLiveRun)

	if err == nil || !strings.Contains(err.Error(), "switch "+filepath.Join(evolveDir, "logs", "current")) {
		t.Errorf("prepareBoundaryLog = %v, want the switch error of logs/current", err)
	}
	if _, serr := os.Lstat(filepath.Join(evolveDir, boundaryLoopLog)); !os.IsNotExist(serr) {
		t.Errorf("the loop log link exists after a failed current switch: %v", serr)
	}
}

func TestMoveLegacyLoopLog_AnUnmakeableLegacyDirKeepsTheLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	link := filepath.Join(dir, boundaryLoopLog)
	writeBoundaryFile(t, link, "old\n")
	writeBoundaryFile(t, filepath.Join(dir, "logs"), "")

	err := moveLegacyLoopLog(link, filepath.Join(dir, "logs", coverRunID+legacyLogSuffix), noLiveRun)

	if err == nil || !strings.HasPrefix(err.Error(), "make the legacy log dir: ") || readText(t, link) != "old\n" {
		t.Errorf("moveLegacyLoopLog = %v, want the make-dir error and the log kept", err)
	}
}

func TestMoveLegacyLoopLog_AnUnremovableLegacyLogIsAnErrorAfterTheCopy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes from a read-only directory")
	}
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".evolve")
	link := filepath.Join(dir, boundaryLoopLog)
	writeBoundaryFile(t, link, "old\n")
	legacyDir := filepath.Join(t.TempDir(), coverRunID+legacyLogSuffix)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := moveLegacyLoopLog(link, legacyDir, noLiveRun)

	if err == nil || !strings.HasPrefix(err.Error(), "remove the legacy loop log after its copy to ") {
		t.Fatalf("moveLegacyLoopLog = %v, want the remove error", err)
	}
	if readText(t, filepath.Join(legacyDir, "loop.log")) != "old\n" || readText(t, link) != "old\n" {
		t.Errorf("want the copy made and the original kept, so a rerun finishes the move")
	}
}

func TestReplaceSymlink_EachFailedStepNamesItsPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stuck := filepath.Join(dir, "stuck")
	writeBoundaryFile(t, filepath.Join(stuck+".tmp-"+strconv.Itoa(os.Getpid()), "keep"), "")
	full := filepath.Join(dir, "full")
	writeBoundaryFile(t, filepath.Join(full, "keep"), "")
	cases := []struct {
		path, want string
	}{
		{stuck, "clear " + stuck + ".tmp-"},
		{filepath.Join(dir, "absent", "link"), "link " + filepath.Join(dir, "absent", "link") + ": "},
		{full, "switch " + full + ": "},
	}
	for _, tc := range cases {
		err := replaceSymlink("target", tc.path)
		if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("replaceSymlink(%s) = %v, want an error starting %q", tc.path, err, tc.want)
		}
	}
	if _, err := os.Lstat(full + ".tmp-" + strconv.Itoa(os.Getpid())); !os.IsNotExist(err) {
		t.Errorf("a failed switch leaves its temp link: %v", err)
	}
}
