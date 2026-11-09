//go:build acs

package cycle2234

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const stalledCode signalcenter.Code = "LIVENESS_PHASE_STALLED"

var frozenAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

type stallRecorder struct {
	mu      sync.Mutex
	center  *signalcenter.Center
	stalls  []signalcenter.Event
	firstAt time.Time
	startAt time.Time
}

func newStallRecorder() *stallRecorder {
	r := &stallRecorder{center: signalcenter.New(), startAt: time.Now()}
	r.center.Subscribe(func(e signalcenter.Event) {
		if e.Code != stalledCode {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if len(r.stalls) == 0 {
			r.firstAt = time.Now()
		}
		r.stalls = append(r.stalls, e)
	})
	return r
}

func (r *stallRecorder) snapshot() []signalcenter.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]signalcenter.Event(nil), r.stalls...)
}

func (r *stallRecorder) firstElapsed() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.firstAt.Sub(r.startAt)
}

func policyFromJSON(t *testing.T, raw string) policy.ObserverPolicy {
	t.Helper()
	var p policy.ObserverPolicy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("observer policy %s: %v", raw, err)
	}
	return p
}

func frozenPaneSnapshot(t *testing.T, workspace string, shells int) {
	t.Helper()
	body := map[string]any{
		"session": "evolve-bridge-claude-c2234-build-pid1-n1-1", "cli": "claude-tmux", "agent": "build", "cycle": 2234,
		"busy": false, "progress_hash": "frozen", "progress_at": frozenAt.Format(time.RFC3339Nano),
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano), "writer_pid": os.Getpid(), "shells": shells,
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "build-pane-watch.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func startObserved(t *testing.T, observerPolicy policy.ObserverPolicy, shells int) *stallRecorder {
	t.Helper()
	ws := t.TempDir()
	frozenPaneSnapshot(t, ws, shells)
	rec := newStallRecorder()
	a := observer.NewCoreAdapter(observerPolicy)
	a.Signals = func() *signalcenter.Center { return rec.center }
	stop := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 2234, RunID: "run-2234"})
	t.Cleanup(stop)
	return rec
}

func waitForStall(rec *stallRecorder, limit time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for len(rec.snapshot()) == 0 {
		select {
		case <-ctx.Done():
			return len(rec.snapshot()) > 0
		case <-tick.C:
		}
	}
	return true
}

func runGoTest(t *testing.T, pkg string, names ...string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", pattern, pkg)
	if runErr != nil || code != 0 {
		t.Fatalf("RED: go test %s -run %s exit=%d err=%v\n%s", pkg, pattern, code, runErr, acsassert.OutputTail(out+errOut, 30))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2234_001_FooterShellCountIsParsedOnlyFromTheFooter(t *testing.T) {
	runGoTest(t, "./internal/bridge/panestream", "TestRunningShells_ReadsTheCountFromTheFooter")
}

func TestC2234_002_BridgePublishesTheShellCountThroughTheProductionWaitLoop(t *testing.T) {
	runGoTest(t, "./internal/bridge",
		"TestRunTmuxREPL_PublishesTheRunningShellCountFromTheFooter",
		"TestRunTmuxREPL_PublishesNoShellsWhenTheFooterShowsNone")
}

func TestC2234_003_DefaultShellLimitIsLongerThanTheDefaultStallLimit(t *testing.T) {
	resolved := policy.Policy{}.ObserverConfig()
	raw, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	shell, hasShell := fields["shell_stall_s"].(float64)
	stall, hasStall := fields["stall_s"].(float64)
	if !hasShell || !hasStall {
		t.Fatalf("RED: resolved observer policy %s has no shell_stall_s beside stall_s", raw)
	}
	if shell <= stall {
		t.Errorf("RED: shell_stall_s=%v must exceed stall_s=%v", shell, stall)
	}
}

func TestC2234_004_PaneWithRunningShellRaisesNoStallBeforeTheShellLimit(t *testing.T) {
	t.Parallel()
	rec := startObserved(t, policyFromJSON(t, `{"stall_s":1,"poll_s":1,"shell_stall_s":3600}`), 1)
	if waitForStall(rec, 4*time.Second) {
		t.Fatalf("RED: a pane whose footer shows a running shell raised LIVENESS_PHASE_STALLED at the plain stall limit: %+v", rec.snapshot())
	}
}

func TestC2234_005_PaneWithRunningShellStallsAtTheLongerLimitAndSaysItWaitsOnAShell(t *testing.T) {
	t.Parallel()
	rec := startObserved(t, policyFromJSON(t, `{"stall_s":1,"poll_s":1,"shell_stall_s":4}`), 2)
	if !waitForStall(rec, 12*time.Second) {
		t.Fatal("RED: a pane with a running shell never raised LIVENESS_PHASE_STALLED at the shell limit")
	}
	if elapsed := rec.firstElapsed(); elapsed < 3500*time.Millisecond {
		t.Errorf("RED: shell pane stalled after %s, before the longer shell limit of 4s", elapsed)
	}
	first := rec.snapshot()[0]
	if !strings.Contains(strings.ToLower(first.Reason), "shell") {
		t.Errorf("RED: stall reason %q does not say the seat waits on its own shell", first.Reason)
	}
	if first.Fields["shells"] != "2" {
		t.Errorf("RED: stall fields %v lack shells=2", first.Fields)
	}
}

func TestC2234_006_PaneWithNoShellAndNoTranscriptStillRaisesTheStall(t *testing.T) {
	t.Parallel()
	rec := startObserved(t, policyFromJSON(t, `{"stall_s":1,"poll_s":1,"shell_stall_s":3600}`), 0)
	if !waitForStall(rec, 6*time.Second) {
		t.Fatal("RED: a frozen pane with no running shell raised no LIVENESS_PHASE_STALLED")
	}
	first := rec.snapshot()[0]
	if strings.Contains(strings.ToLower(first.Reason), "shell") {
		t.Errorf("RED: no-shell stall reason %q wrongly blames a shell", first.Reason)
	}
}
