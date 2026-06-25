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

// reap_orphans.go — crash-recovery orphan GC.
//
// The per-run registry reaper (reap_runsessions.go) is the CLEAN-PATH teardown:
// it kills only the sessions a run recorded in its own file, so it is
// structurally incapable of touching another run's sessions (the 2026-06-11
// killer-B protection). But that guarantee has a hole: a SIGKILL'd loop never
// runs its teardown, and the NEXT loop can't reap the corpse because the dead
// run's sessions aren't in the new run's registry. Orphans then accumulate
// across crashes until the shared tmux server starves the machine.
//
// This GC closes the hole by reaping on a DIFFERENT axis — process liveness.
// Every auto-generated session name bakes in its creator's PID
// (bridge.resolveSession: "...-pid<PID>-n<nonce>-<ts>"). A session whose PID is
// dead can only be a corpse, so it is safe to kill. Crucially this preserves
// the killer-B guarantee by construction: a LIVE concurrent run's PID is alive,
// so its sessions are skipped, never killed. The failure modes are all
// fail-safe — an unparseable name, a foreign name, or a recycled-and-now-live
// PID all SKIP (leak), never mis-kill. The worst case is a leak the next sweep
// catches; it is never a wrong kill.
//
// LIMITATION: only sessions carrying a -pid<N> token are GC-eligible. Sessions
// without one — named sessions (bridge.NamedSessionName, "evolve-bridge-named-
// <name>") and ad-hoc test-harness sessions — are counted SkippedUnparseable and
// left for their creator or an operator to reap. All auto-generated bridge/recipe
// phase sessions (the ones that accumulate from crashed runs) DO carry the token,
// so this covers the production accumulation case; closing the named-session gap
// would mean baking a creator PID into NamedSessionName (a separate change).

// orphanNamespaces are the session-name prefixes this GC is allowed to touch.
// Anything else (a user's own tmux session sharing the socket) is foreign and
// never killed — defense in depth atop the PID check.
var orphanNamespaces = []string{"evolve-bridge-", "evolve-recipe-"}

// pidTokenRE extracts the creator PID from an auto-generated session name. The
// "-pid" prefix (with the leading dash) is required so substrings like "rapid7"
// never match. The trailing "-" or end-anchor stops at the nonce segment.
var pidTokenRE = regexp.MustCompile(`-pid(\d+)(?:-|$)`)

// SessionLister returns the names of every session on the bridge tmux server.
// Injected so the unit suite never touches a real server.
type SessionLister func(ctx context.Context) ([]string, error)

// PidLiveness reports whether a PID is currently alive. Injected for tests; the
// production impl (ExecPidAlive) probes with signal 0.
type PidLiveness func(pid int) bool

// OrphanReapReport summarizes one liveness sweep. The Skipped* counts are split
// so the operator can tell a healthy "nothing to do" sweep from one that left
// live concurrent runs untouched.
type OrphanReapReport struct {
	Killed             []string // sessions reaped (dead creator PID)
	SkippedLive        int      // creator PID still alive (concurrent run — left alone)
	SkippedForeign     int      // empty or outside the evolve namespace — never touched
	SkippedUnparseable int      // no -pid<N> token — liveness unknown, left alone
	Errors             []string // per-session killer errors (best-effort; sweep continues)
}

// SessionPID extracts the creator PID baked into a session name. ok is false
// when the name carries no usable -pid<N> token (PID 0 is refused — it would
// mean "the caller's own group" to a signaller).
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

// ReapOrphanSessions lists the bridge server and kills every session whose
// creator PID is dead, within the evolve namespace. It is idempotent and
// crash-recoverable: running it at the start of any loop reclaims every prior
// crashed run's sessions, and running it per-cycle catches mid-run orphans —
// all without ever touching a live concurrent run (its PIDs are alive).
//
// Best-effort throughout: a list error kills nothing (degrade to leak, never a
// blind sweep); a per-session kill error is recorded and the sweep continues.
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

// tmuxListRun is the exec seam for ExecListBridgeSessions, overridden in tests
// so the unit suite never shells out to tmux.
var tmuxListRun = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "tmux", args...).Output()
}

// ExecListBridgeSessions is the production SessionLister: it lists session names
// on the bridge's isolated -L socket. A stopped server (no sessions ever
// created) makes tmux exit non-zero with empty output — that is the desired
// "nothing to reap" state, not an error, so a clean machine is not a failure.
func ExecListBridgeSessions(ctx context.Context) ([]string, error) {
	out, err := tmuxListRun(ctx, bridge.TmuxSocketArgs("list-sessions", "-F", "#{session_name}")...)
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			return nil, nil // no server / no sessions
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

// ExecReapOrphans wires the production lister, liveness probe, and killer into
// ReapOrphanSessions — the one call sites use (loop startup, per-cycle, and the
// `evolve gc` command).
func ExecReapOrphans(ctx context.Context) OrphanReapReport {
	return ReapOrphanSessions(ctx, ExecListBridgeSessions, ExecPidAlive, ExecTmuxKill)
}

// ───────────────────────── per-run socket GC (F6) ─────────────────────────
// With per-run tmux sockets (bridge.DeriveRunSocket → evolve-bridge-p<looppid>),
// a crashed loop leaves its WHOLE socket server running with orphaned panes —
// reap-by-session-name on one socket (ReapOrphanSessions) can't see another
// socket's sessions. ReapOrphanSockets finds per-run socket files whose owner
// loop pid is dead and kills that socket's server outright. kill-server is safe
// here (unlike the shared default) precisely because the socket is the dead
// run's alone; a live owner's socket is skipped.

// socketPidRE matches a per-run socket name and captures the owner loop pid. The
// shared default ("evolve-bridge") and any other name never match → never killed.
var socketPidRE = regexp.MustCompile(`^evolve-bridge-p(\d+)$`)

// OrphanSocketReport summarizes one per-run-socket sweep.
type OrphanSocketReport struct {
	Killed      []string // per-run sockets whose dead-owner server was killed
	SkippedLive int      // owner pid still alive (a running loop — left alone)
	Errors      []string // per-socket kill errors (best-effort; sweep continues)
}

// SocketLister returns the bridge per-run socket names present on the host.
// Injected so the unit suite never touches the real tmux socket directory.
type SocketLister func() ([]string, error)

// ServerKiller kills the tmux server on one socket (kill-server -L socket).
// Injected for testability.
type ServerKiller func(ctx context.Context, socket string) error

// ReapOrphanSockets kills the tmux server of every per-run bridge socket whose
// owner pid is dead. Non-matching names (the shared default, probe sockets) are
// left untouched — kill-server is only ever aimed at a socket provably owned by
// a dead loop. Best-effort: a list error reaps nothing; a kill error is recorded
// and the sweep continues.
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
			continue // not a per-run socket — never kill-server
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

// tmuxSocketDir returns tmux's per-uid socket directory ($TMUX_TMPDIR or /tmp,
// subdir tmux-<uid>) — where -L socket files live.
func tmuxSocketDir() string {
	base := os.Getenv("TMUX_TMPDIR")
	if base == "" {
		base = "/tmp"
	}
	return filepath.Join(base, fmt.Sprintf("tmux-%d", os.Getuid()))
}

// socketGlob is the exec/fs seam for ExecListBridgeSockets, overridden in tests.
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

// ExecKillServer is the production ServerKiller: best-effort kill-server on the
// named socket (a missing server is the desired end state). Refuses an empty
// name (tmux would resolve -L "" oddly).
func ExecKillServer(ctx context.Context, socket string) error {
	if socket == "" {
		return fmt.Errorf("refusing kill-server with an empty socket name")
	}
	_ = tmuxRun(ctx, "-L", socket, "kill-server")
	return nil
}

// ExecReapOrphanSockets wires the production socket lister, liveness probe, and
// server killer into ReapOrphanSockets.
func ExecReapOrphanSockets(ctx context.Context) OrphanSocketReport {
	return ReapOrphanSockets(ctx, ExecListBridgeSockets, ExecPidAlive, ExecKillServer)
}
