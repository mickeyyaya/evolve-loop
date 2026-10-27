//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestLoopDetach_JSONPrintsThePidTheLogAndRunningAndNoText(t *testing.T) {
	p := newDetachProject(t)
	probe := stubDetachChild(t, "lease", p)
	rc, stdout, stderr := runDetach(t, p.args("--json"))
	var got detachedLoopResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || rc != cliExitOK {
		t.Fatalf("rc %d stdout %q (%v) stderr %q, want exit 0 and one JSON result", rc, stdout, err, stderr)
	}
	if got.PID != probe.onlyPID(t) || got.Log != p.log || !got.Running {
		t.Errorf("result %+v, want the child pid, the log %s and running", got, p.log)
	}
	if strings.Contains(stdout, "loop: detached pid") {
		t.Errorf("the text must not reach stdout in JSON mode: %q", stdout)
	}
	for _, argv := range probe.argv {
		for _, a := range argv {
			if a == "--json" {
				t.Errorf("the child argv %q must not carry --json", argv)
			}
		}
	}
	requireChildTicking(t, p.ticks)
}

func TestRunSyncMain_AFailedFetchExitsOne(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rc := runSyncMain([]string{"--project-root", repo.Dir}, nil, &stdout, &stderr)
	if rc != cliExitFailed || !strings.Contains(stderr.String(), "git fetch origin") {
		t.Fatalf("rc %d stderr %q, want exit 1 naming the failed fetch", rc, stderr.String())
	}
}
