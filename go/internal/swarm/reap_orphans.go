package swarm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// orphanNamespaces bounds the GC: a user's own session on the same socket is foreign and never killed.
var orphanNamespaces = []string{"evolve-bridge-", "evolve-recipe-"}

// pidTokenRE requires the leading dash so a substring like "rapid7" never matches.
var pidTokenRE = regexp.MustCompile(`-pid(\d+)(?:-|$)`)

// SessionLister returns every session name on the bridge tmux server.
type SessionLister func(ctx context.Context) ([]string, error)

// PidLiveness reports whether a PID is alive.
type PidLiveness func(pid int) bool

// OrphanReapReport summarizes one liveness sweep, with skips split by reason.
type OrphanReapReport struct {
	Killed             []string
	SkippedLive        int // creator PID alive: a concurrent run
	SkippedForeign     int // empty, or outside the evolve namespace
	SkippedUnparseable int // no -pid<N> token, so liveness is unknown
	Errors             []string
}

// SessionPID extracts the creator PID from a session name, refusing PID 0, which a signaller reads as its own group.
func SessionPID(session string) (pid int, ok bool) {
	m := pidTokenRE.FindStringSubmatch(session)
	if m == nil {
		return 0, false
	}
	p, err := strconv.Atoi(m[1])
	if err != nil || p <= 0 {
		return 0, false
	}
	return p, true
}

func inEvolveNamespace(session string) bool {
	for _, p := range orphanNamespaces {
		if strings.HasPrefix(session, p) {
			return true
		}
	}
	return false
}

// ReapOrphanSessions kills every evolve-namespace session whose creator PID is dead; a list error kills nothing.
func ReapOrphanSessions(ctx context.Context, list SessionLister, alive PidLiveness, kill TmuxKiller) OrphanReapReport {
	var rep OrphanReapReport
	sessions, err := list(ctx)
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("list sessions: %v", err))
		return rep
	}
	for _, s := range sessions {
		if s == "" || !inEvolveNamespace(s) {
			rep.SkippedForeign++
			continue
		}
		pid, ok := SessionPID(s)
		if !ok {
			rep.SkippedUnparseable++
			continue
		}
		if alive(pid) {
			rep.SkippedLive++
			continue
		}
		if err := kill(ctx, s); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", s, err))
			continue
		}
		rep.Killed = append(rep.Killed, s)
	}
	return rep
}

// tmuxListRun is a test seam, so the unit suite never shells out to tmux.
var tmuxListRun = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "tmux", args...).Output()
}

// ExecListBridgeSessions is the production SessionLister; a stopped server with no output is an empty list, not an error.
func ExecListBridgeSessions(ctx context.Context) ([]string, error) {
	out, err := tmuxListRun(ctx, bridge.TmuxSocketArgs("list-sessions", "-F", "#{session_name}")...)
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w", err)
	}
	var names []string
	for _, line := range strings.Split(trimmed, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			names = append(names, l)
		}
	}
	return names, nil
}

// ExecReapOrphans runs ReapOrphanSessions with the production lister, liveness probe and killer.
func ExecReapOrphans(ctx context.Context) OrphanReapReport {
	return ReapOrphanSessions(ctx, ExecListBridgeSessions, ExecPidAlive, ExecTmuxKill)
}

// socketPidRE matches only per-run sockets, so the shared default socket is never killed.
var socketPidRE = regexp.MustCompile(`^evolve-bridge-p(\d+)$`)

// OrphanSocketReport summarizes one per-run-socket sweep.
type OrphanSocketReport struct {
	Killed      []string
	SkippedLive int
	Errors      []string
}

// SocketLister returns the bridge per-run socket names on the host.
type SocketLister func() ([]string, error)

// ServerKiller kills the tmux server on one socket.
type ServerKiller func(ctx context.Context, socket string) error

// ReapOrphanSockets kills the tmux server of every per-run bridge socket whose owner PID is dead; a list error kills nothing.
func ReapOrphanSockets(ctx context.Context, list SocketLister, alive PidLiveness, killServer ServerKiller) OrphanSocketReport {
	var rep OrphanSocketReport
	socks, err := list()
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("list sockets: %v", err))
		return rep
	}
	for _, s := range socks {
		m := socketPidRE.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		pid, perr := strconv.Atoi(m[1])
		if perr != nil || pid <= 0 {
			continue
		}
		if alive(pid) {
			rep.SkippedLive++
			continue
		}
		if err := killServer(ctx, s); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", s, err))
			continue
		}
		rep.Killed = append(rep.Killed, s)
	}
	return rep
}

// tmuxSocketDir mirrors where tmux keeps -L socket files.
func tmuxSocketDir() string {
	base := os.Getenv("TMUX_TMPDIR")
	if base == "" {
		base = "/tmp"
	}
	return filepath.Join(base, fmt.Sprintf("tmux-%d", os.Getuid()))
}

// socketGlob is a test seam over the tmux socket directory.
var socketGlob = func() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(tmuxSocketDir(), "evolve-bridge-p*"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	return names, nil
}

// ExecListBridgeSockets is the production SocketLister.
func ExecListBridgeSockets() ([]string, error) { return socketGlob() }

// ExecKillServer is the production ServerKiller: it refuses an empty socket name, and a missing server counts as success.
func ExecKillServer(ctx context.Context, socket string) error {
	if socket == "" {
		return fmt.Errorf("refusing kill-server with an empty socket name")
	}
	_ = tmuxRun(ctx, "-L", socket, "kill-server")
	return nil
}

// ExecReapOrphanSockets runs ReapOrphanSockets with the production socket lister, liveness probe and server killer.
func ExecReapOrphanSockets(ctx context.Context) OrphanSocketReport {
	return ReapOrphanSockets(ctx, ExecListBridgeSockets, ExecPidAlive, ExecKillServer)
}
