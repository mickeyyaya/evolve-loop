package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// cmd_gc.go — `evolve gc`: the operator surface for the crash-recovery tmux
// session GC. The same liveness sweep runs automatically at loop startup and
// after every cycle (see gcOrphanSessions); this command exposes it for manual
// cleanup after a crash and for inspection via --dry-run.
//
// SAFETY: reaps only sessions in the evolve namespace whose creator PID is dead.
// A live concurrent run's sessions (live PIDs) are never touched — the same
// killer-B guarantee the per-run registry reaper provides, enforced here by
// process liveness instead of file scoping.

// runGC implements `evolve gc [--dry-run]`.
func runGC(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve gc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "list orphan sessions that WOULD be reaped, without killing any")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	// Bound the sweep so a wedged tmux socket can't hang the command.
	ctx, cancel := context.WithTimeout(context.Background(), orphanGCTimeout)
	defer cancel()

	var rep swarm.OrphanReapReport
	if *dryRun {
		// A no-op killer turns the sweep into a preview: the report's Killed
		// list is exactly what a real run would reap.
		noop := func(_ context.Context, _ string) error { return nil }
		rep = swarm.ReapOrphanSessions(ctx, swarm.ExecListBridgeSessions, swarm.ExecPidAlive, noop)
		fmt.Fprintf(stdout, "evolve gc --dry-run: %d orphan session(s) would be reaped\n", len(rep.Killed))
		for _, s := range rep.Killed {
			fmt.Fprintf(stdout, "  WOULD-REAP %s\n", s)
		}
	} else {
		rep = swarm.ExecReapOrphans(ctx)
		fmt.Fprintf(stdout, "evolve gc: reaped %d orphan session(s)\n", len(rep.Killed))
		for _, s := range rep.Killed {
			fmt.Fprintf(stdout, "  reaped %s\n", s)
		}
	}
	fmt.Fprintf(stdout, "skipped: live=%d foreign=%d no-pid=%d; errors=%d\n",
		rep.SkippedLive, rep.SkippedForeign, rep.SkippedUnparseable, len(rep.Errors))
	for _, e := range rep.Errors {
		fmt.Fprintf(stderr, "evolve gc: error: %s\n", e)
	}

	// F6: also sweep whole per-run tmux sockets a crashed loop left behind.
	var srep swarm.OrphanSocketReport
	if *dryRun {
		noopKill := func(_ context.Context, _ string) error { return nil }
		srep = swarm.ReapOrphanSockets(ctx, swarm.ExecListBridgeSockets, swarm.ExecPidAlive, noopKill)
		fmt.Fprintf(stdout, "evolve gc --dry-run: %d dead per-run socket(s) would be reaped\n", len(srep.Killed))
	} else {
		srep = swarm.ExecReapOrphanSockets(ctx)
		fmt.Fprintf(stdout, "evolve gc: reaped %d dead per-run socket(s)\n", len(srep.Killed))
	}
	for _, s := range srep.Killed {
		fmt.Fprintf(stdout, "  socket %s\n", s)
	}
	for _, e := range srep.Errors {
		fmt.Fprintf(stderr, "evolve gc: socket error: %s\n", e)
	}

	if len(rep.Errors) > 0 || len(srep.Errors) > 0 {
		return 1
	}
	return 0
}
