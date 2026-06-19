// Command cmd_swarm.go — `evolve swarm status|reap`. Operator surface for the
// swarm harness (ADR-0032): inspect the per-cycle session manifest and reap
// orphaned worker sessions after a crash. Read-only `status`; teardown `reap`.
//
// `reap` is the crash-safe backstop: the in-process dispatcher reaps on normal
// exit, but a hard SIGKILL of the orchestrator leaves orphaned tmux sessions +
// process groups that only the on-disk manifest can recover. It NEVER does a
// broad `pkill` — it kills exactly the pgids/sessions the manifest recorded.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func runSwarm(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var sub string
	if len(args) > 0 {
		sub = args[0]
		args = args[1:]
	}
	switch sub {
	case "status":
		return runSwarmStatus(args, stdout, stderr)
	case "reap":
		return runSwarmReap(args, stdout, stderr)
	case "reap-orphans":
		return runSwarmReapOrphans(args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "usage: evolve swarm <status|reap|reap-orphans> [options]\n")
		return 2
	}
}

func runSwarmReapOrphans(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("swarm reap-orphans", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evolveDir := fs.String("evolve-dir", ".evolve", "path to the .evolve directory")
	dryRun := fs.Bool("dry-run", false, "report orphaned sessions without killing them")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	kill := swarm.ExecTmuxKill
	if *dryRun {
		kill = func(context.Context, string) error { return nil }
	}
	rep, err := sessionreaper.ReapOrphans(context.Background(), *evolveDir, sessionreaper.Options{Kill: kill})
	if err != nil {
		fmt.Fprintf(stderr, "swarm reap-orphans: %v\n", err)
		return 1
	}
	killed, skipped, failures := 0, 0, 0
	for _, orphan := range rep.Orphaned {
		killed += orphan.Report.Killed
		skipped += orphan.Report.Skipped
		failures += orphan.Report.Errors
	}
	fmt.Fprintf(stdout, "orphan runs=%d live-skipped=%d sessions=%d unsafe-skipped=%d errors=%d dry-run=%t\n",
		len(rep.Orphaned), rep.LiveRunsSkipped, killed, skipped, failures, *dryRun)
	return 0
}

// manifestPath resolves the per-cycle swarm manifest the registry writes:
// <evolveDir>/runs/cycle-<N>/.swarm/sessions.json.
func manifestPath(evolveDir string, cycle int) string {
	return filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle), ".swarm", "sessions.json")
}

func swarmFlags(args []string, stderr io.Writer) (evolveDir string, cycle int, ok bool) {
	fs := flag.NewFlagSet("swarm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&evolveDir, "evolve-dir", ".evolve", "path to the .evolve directory")
	fs.IntVar(&cycle, "cycle", 0, "cycle number (required)")
	if err := fs.Parse(args); err != nil {
		return "", 0, false
	}
	if cycle <= 0 {
		fmt.Fprintln(stderr, "swarm: --cycle N is required")
		return "", 0, false
	}
	return evolveDir, cycle, true
}

func runSwarmStatus(args []string, stdout, stderr io.Writer) int {
	evolveDir, cycle, ok := swarmFlags(args, stderr)
	if !ok {
		return 2
	}
	_, phase, pid, sessions, err := swarm.LoadManifest(manifestPath(evolveDir, cycle))
	if err != nil {
		fmt.Fprintf(stderr, "swarm status: %v\n", err)
		return 1
	}
	if len(sessions) == 0 {
		fmt.Fprintf(stdout, "no swarm sessions recorded for cycle %d\n", cycle)
		return 0
	}
	fmt.Fprintf(stdout, "cycle %d phase=%s owner-pid=%d:\n", cycle, phase, pid)
	for _, s := range sessions {
		fmt.Fprintf(stdout, "  %-6s %-8s pgid=%-7d tmux=%s branch=%s\n",
			s.WorkerID, s.Status, s.PGID, s.TmuxSession, s.Branch)
	}
	return 0
}

func runSwarmReap(args []string, stdout, stderr io.Writer) int {
	evolveDir, cycle, ok := swarmFlags(args, stderr)
	if !ok {
		return 2
	}
	path := manifestPath(evolveDir, cycle)
	c, phase, pid, sessions, err := swarm.LoadManifest(path)
	if err != nil {
		fmt.Fprintf(stderr, "swarm reap: %v\n", err)
		return 1
	}
	if len(sessions) == 0 {
		fmt.Fprintf(stdout, "no swarm sessions to reap for cycle %d\n", cycle)
		return 0
	}
	// Rebuild a registry over the loaded manifest so Reap can mark + persist.
	reg := swarm.NewSessionRegistry(path, c, phase, pid)
	for _, s := range sessions {
		_ = reg.Register(s)
		if s.Status == swarm.StatusReaped {
			_ = reg.MarkReaped(s.WorkerID)
		}
	}
	killer := swarm.ExecSessionKiller{
		KillGroup: func(pgid int) error {
			// Defense in depth (ExecSessionKiller already gates pgid>1): never
			// signal group 0 (caller's own group) or 1 (everything) — that would
			// kill the reaper / whole session, not the orphaned worker.
			if pgid <= 1 {
				return fmt.Errorf("refusing to kill process group %d", pgid)
			}
			return syscall.Kill(-pgid, syscall.SIGKILL)
		},
		KillTmux: swarm.ExecTmuxKill,
	}
	rep := swarm.Reap(context.Background(), reg, killer)
	fmt.Fprintf(stdout, "reaped %d session(s) for cycle %d\n", len(rep.Killed), cycle)
	for _, e := range rep.Errors {
		fmt.Fprintf(stderr, "  warn: %s\n", e)
	}
	return 0
}
