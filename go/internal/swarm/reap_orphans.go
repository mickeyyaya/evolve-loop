package swarm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var orphanNamespaces = []string{"evolve-bridge-", "evolve-recipe-"}

var pidTokenRE = regexp.MustCompile(`-pid(\d+)(?:-|$)`)

var testSessionRE = regexp.MustCompile(`^evolve-bridge-it-[A-Za-z0-9._]+-(\d+)$`)

type SessionLister func(ctx context.Context) ([]string, error)

type PidLiveness func(pid int) bool

type OrphanReapReport struct {
	Killed             []string
	SkippedLive        int
	SkippedForeign     int
	SkippedUnparseable int
	Errors             []string
}

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
		pid, ok := sessionOwner(s)
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

var tmuxListRun = func(ctx context.Context, args ...string) ([]byte, error) {
	return sysexec.Command(ctx, "tmux", args...).Output()
}

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

func ExecReapOrphans(ctx context.Context) OrphanReapReport {
	return ReapOrphanSessions(ctx, ExecListBridgeSessions, ExecPidAlive, ExecTmuxKill)
}

var ownerPidSocketRE = regexp.MustCompile(`^evolve-bridge-[pt](\d+)$`)

type OrphanSocketReport struct {
	Killed      []string
	SkippedLive int
	Errors      []string
}

type SocketLister func() ([]string, error)

type ServerKiller func(ctx context.Context, socket string) error

func ReapOrphanSockets(ctx context.Context, list SocketLister, alive PidLiveness, killServer ServerKiller) OrphanSocketReport {
	var rep OrphanSocketReport
	socks, err := list()
	if err != nil {
		rep.Errors = append(rep.Errors, fmt.Sprintf("list sockets: %v", err))
		return rep
	}
	for _, s := range socks {
		m := ownerPidSocketRE.FindStringSubmatch(s)
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

var socketGlob = func() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(bridge.TmuxSocketDir(), "evolve-bridge-[pt]*"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	return names, nil
}

func ExecListBridgeSockets() ([]string, error) { return socketGlob() }

func ExecKillServer(ctx context.Context, socket string) error {
	if socket == "" {
		return fmt.Errorf("refusing kill-server with an empty socket name")
	}
	_ = tmuxRun(ctx, "-L", socket, "kill-server")
	if err := os.Remove(filepath.Join(bridge.TmuxSocketDir(), socket)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove dead socket %s: %w", socket, err)
	}
	return nil
}

func ExecReapOrphanSockets(ctx context.Context) OrphanSocketReport {
	return ReapOrphanSockets(ctx, ExecListBridgeSockets, ExecPidAlive, ExecKillServer)
}

func sessionOwner(session string) (int, bool) {
	if pid, ok := SessionPID(session); ok {
		return pid, true
	}
	m := testSessionRE.FindStringSubmatch(session)
	if m == nil {
		return 0, false
	}
	pid, err := strconv.Atoi(m[1])
	return pid, err == nil && pid > 1
}
