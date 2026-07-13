package looppreflight

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// Cycle-769 boot-orphan-sweep-bounded-tombstone regression contract (preflight
// half). Incident: checks.go ran the boot orphan sweep with
// context.Background() — a wedged tmux server hangs loop boot silently and
// indefinitely, while the per-cycle sweep has been deadline-bounded
// (orphanGCTimeout, cmd_loop_control.go) since the same incident class.
//
// Contract: the preflight sweep's killer receives a context carrying a
// boot-scale deadline (≤30s from the call — the existing orphanGCTimeout is
// 15s; the exact home of the hoisted const is the implementer's choice), so a
// blocked tmux exec is abandoned instead of wedging boot. The killer is
// injectable via Options.OrphanKill (defaulting to swarm.ExecTmuxKill) —
// preflight is otherwise untestable without a real tmux server.
func TestPreflight_OrphanReapIsDeadlineBounded(t *testing.T) {
	opts := goodPipelineOptions(t)

	// A lease-less run with one registered session, so the sweep must invoke
	// the killer (an empty runs dir would green trivially without exercising
	// the deadline path at all).
	runDir := filepath.Join(opts.EvolveDir, "runs", "stale")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sessionrecord.Append(sessionrecord.PathIn(runDir), sessionrecord.Record{Session: "evolve-bridge-stale"}); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var sessions []string
	var deadlines []time.Time
	missingDeadline := false
	opts.OrphanKill = func(ctx context.Context, session string) error {
		mu.Lock()
		defer mu.Unlock()
		sessions = append(sessions, session)
		if d, ok := ctx.Deadline(); ok {
			deadlines = append(deadlines, d)
		} else {
			missingDeadline = true
		}
		return nil
	}

	start := time.Now()
	if _, err := Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sessions) == 0 {
		t.Fatal("injected OrphanKill was never invoked — the boot sweep did not go through the seam (stale run with a registry was present)")
	}
	if missingDeadline {
		t.Fatal("boot orphan sweep ran with an unbounded context (no deadline) — a wedged tmux hangs boot forever")
	}
	for _, d := range deadlines {
		if until := d.Sub(start); until <= 0 || until > 30*time.Second {
			t.Fatalf("boot sweep deadline is not boot-scale: %v from Run start (want (0, 30s], orphanGCTimeout discipline)", until)
		}
	}
}
