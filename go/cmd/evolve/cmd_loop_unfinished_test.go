package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// cmd_loop_unfinished_test.go — F1 sibling: a fresh `evolve loop` that finds an
// unfinished cycle whose run lease is still FRESH must recognize the LIVE owner
// and steer the operator to --resume / wait — never to `evolve cycle reset`
// (which would refuse anyway) and never to `pkill`.
func TestRunLoop_UnfinishedCycle_FreshLeaseHaltsWithLiveOwnerMsg(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-395")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"dispatch":{"policy":"off"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Fresh lease AND a genuinely-alive owner pid ⇒ a live loop owns cycle 395.
	// Use this test process's own pid so the production pid-aware liveness fence
	// (runlease.OwnerLive, cycle-554) sees a real live owner — a fresh heartbeat
	// alone is no longer sufficient (a crashed owner's stale-but-fresh lease now
	// correctly falls through to the resume|reset guidance instead).
	livePID := os.Getpid()
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: livePID}, time.Now()); err != nil {
		t.Fatalf("write lease: %v", err)
	}

	storage := &fixtures.FakeStorage{
		State:      core.State{LastCycleNumber: 394},
		CycleState: core.CycleState{CycleID: 395, Phase: "build", WorkspacePath: ws},
	}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Fatalf("rc=%d, want 2 (halt on a live-owned unfinished cycle); stderr=%s", rc, stderr.String())
	}
	s := stderr.String()
	for _, want := range []string{"owned by a LIVE run", strconv.Itoa(livePID), "evolve loop --resume"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected live-owner halt message containing %q; got:\n%s", want, s)
		}
	}
	if strings.Contains(s, "seal & move on") {
		t.Errorf("must NOT steer to `evolve cycle reset` for a LIVE run; got:\n%s", s)
	}
}
