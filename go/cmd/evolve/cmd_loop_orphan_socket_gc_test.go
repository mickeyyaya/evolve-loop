package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// deadBridgePID is above every OS's pid_max, so the liveness probe always
// reports its socket's owner dead.
const deadBridgePID = 999999999

// TestRunLoopBatch_OrphanSocketGCReapsOnlyDeadSocketsInTheEnvNamedDir pins that
// isolating the loop tests from the host's tmux socket dir redirects the loop's
// orphan-socket sweep instead of disabling it: the sweep a batch runs must still
// reap a dead-pid per-run socket, and spare a live-pid one, in the socket dir
// TMUX_TMPDIR names.
func TestRunLoopBatch_OrphanSocketGCReapsOnlyDeadSocketsInTheEnvNamedDir(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	_, cycleRan := installGCHookSpy(t, evolveDir)

	storage := &fixtures.FakeStorage{}
	defer installStubDeps(t, storage, newFakeLedger())()

	prevOrch := loopOrchOverride
	loopOrchOverride = &batchEndOrch{ran: cycleRan}
	defer func() { loopOrchOverride = prevOrch }()

	tmuxTmp := t.TempDir()
	t.Setenv("TMUX_TMPDIR", tmuxTmp)
	sockDir := filepath.Join(tmuxTmp, fmt.Sprintf("tmux-%d", os.Getuid()))
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dead := filepath.Join(sockDir, fmt.Sprintf("evolve-bridge-p%d", deadBridgePID))
	// The parent is alive for the whole test, and unlike os.Getpid() it never
	// names the loop's own per-run socket, which the batch's teardown removes.
	live := filepath.Join(sockDir, fmt.Sprintf("evolve-bridge-p%d", os.Getppid()))
	for _, p := range []string{dead, live} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "orphan socket gc goal",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc=%d, want 0 (clean max_cycles completion); stderr=%s", rc, stderr.String())
	}
	if !*cycleRan {
		t.Fatal("the batch never ran its cycle, so its post-cycle sweep was never exercised")
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Errorf("dead-pid socket %s survived the batch (stat err=%v): the loop's orphan-socket sweep no longer reaps in the TMUX_TMPDIR socket dir, so the host isolation disabled the sweep instead of redirecting it; stderr=%s", dead, err, stderr.String())
	}
	if _, err := os.Stat(live); err != nil {
		t.Errorf("live-pid socket %s was removed by the batch (stat err=%v): the sweep must spare a socket whose owner is alive", live, err)
	}
}
