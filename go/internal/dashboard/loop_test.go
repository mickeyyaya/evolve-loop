package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// writeCycleState writes .evolve/cycle-state.json exactly as the kernel does
// and returns the cycle's run workspace path.
func writeCycleState(t *testing.T, root string, cs cyclestate.CycleState) string {
	t.Helper()
	ws := core.RunWorkspacePath(root, cs.CycleID)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	cs.WorkspacePath = ws
	buf, err := json.Marshal(cs)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, core.ResolveCycleStatePath(filepath.Join(root, ".evolve")), string(buf))
	return ws
}

func TestReadLoop_FreshLeaseIsRunning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	ws := writeCycleState(t, root, cyclestate.CycleState{CycleID: 1606, Phase: "tdd",
		PhaseStartedAt: now.Add(-3 * time.Minute).Format(time.RFC3339), AuditDispatches: 2, ActiveWorktree: "/wt"})
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN"}, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	writeNDJSON(t, filepath.Join(ws, "llm-calls.ndjson"),
		`{"ts":"2026-09-02T11:50:00Z","agent":"tdd","phase":"tdd","cli":"claude-tmux","model":"balanced","attempt":1}`)

	ls, warns := readLoop(root, now)
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if !ls.Running || ls.BrakeEngaged || ls.CycleID != 1606 || ls.Phase != "tdd" || ls.AuditRounds != 2 {
		t.Fatalf("LoopStatus = %+v", ls)
	}
	if ls.CLI != "claude-tmux" || ls.Model != "balanced" || ls.ActiveWorktree != "/wt" {
		t.Fatalf("dispatch fields = %+v", ls)
	}
	if ls.LeaseHeartbeat.IsZero() || ls.PhaseStartedAt.IsZero() {
		t.Fatalf("timestamps not parsed: %+v", ls)
	}
}

func TestReadLoop_StaleLeaseAndBrake(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	ws := writeCycleState(t, root, cyclestate.CycleState{CycleID: 1606, Phase: "tdd"})
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN"}, now.Add(-runlease.DefaultTTL-time.Minute)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".evolve", "loop-stop"), "")
	ls, _ := readLoop(root, now)
	if ls.Running || !ls.BrakeEngaged {
		t.Fatalf("stale lease + brake: %+v", ls)
	}
}

func TestReadLoop_StoppedLoopFallsBackToNewestRunJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	// No cycle-state.json (clean stop); two run workspaces, the newer one checkpointed mid-repair.
	for _, id := range []int{1605, 1606} {
		ws := core.RunWorkspacePath(root, id)
		buf, _ := json.Marshal(cyclestate.CycleState{CycleID: id, Phase: "tdd", WorkspacePath: ws,
			PhaseStartedAt: "2026-09-02T02:57:05Z", AuditDispatches: 2})
		writeFile(t, filepath.Join(ws, core.RunStateFile), string(buf))
	}
	if err := runlease.Write(core.RunWorkspacePath(root, 1606), runlease.Lease{RunID: "r"}, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	ls, warns := readLoop(root, now)
	if len(warns) != 0 || ls.Running || !ls.Checkpointed || ls.CycleID != 1606 || ls.Phase != "tdd" || ls.AuditRounds != 2 {
		t.Fatalf("stopped-loop fallback = %+v warns=%v", ls, warns)
	}
}

func TestReadLoop_NoCycleStateIsQuiet(t *testing.T) {
	t.Parallel()
	ls, warns := readLoop(t.TempDir(), time.Now())
	if ls.Running || ls.CycleID != 0 || len(warns) != 0 {
		t.Fatalf("empty root: %+v warns=%v", ls, warns)
	}
}

func TestReadLoop_MalformedCycleStateWarns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, core.ResolveCycleStatePath(filepath.Join(root, ".evolve")), "{")
	_, warns := readLoop(root, time.Now())
	if len(warns) != 1 {
		t.Fatalf("want one warning for a torn cycle-state, got %v", warns)
	}
}
