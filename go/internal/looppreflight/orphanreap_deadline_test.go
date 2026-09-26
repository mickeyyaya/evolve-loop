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

func TestPreflight_OrphanReapIsDeadlineBounded(t *testing.T) {
	opts := goodPipelineOptions(t)

	// A lease-less run with a registered session forces the sweep to call the killer.
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
