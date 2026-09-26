package observer

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// anyProbe reports alive if any probe does. Every probe runs on every call, with
// no short-circuit, so each stateful probe keeps its last sample current. Nil
// probes are skipped.
func anyProbe(probes ...func() bool) func() bool {
	return func() bool {
		alive := false
		for _, p := range probes {
			if p != nil && p() {
				alive = true
			}
		}
		return alive
	}
}

// tmuxRunner runs a tmux subcommand and returns its stdout; tests inject it.
type tmuxRunner func(args ...string) ([]byte, error)

func realTmuxRunner(args ...string) ([]byte, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, err
	}
	return exec.Command(path, args...).Output()
}

// socketTmuxRunner points every call at the bridge's isolated socket
// (bridge.TmuxSocket). A default-socket query never sees the agent panes, so
// the probe would report every agent dead.
func socketTmuxRunner(run tmuxRunner) tmuxRunner {
	return func(args ...string) ([]byte, error) {
		return run(bridge.TmuxSocketArgs(args...)...)
	}
}

// newTmuxPaneProbe returns a LivenessProbe that reports the agent alive when its
// tmux pane changed since the last call. It finds the bridge session by the
// "-c<cycle>-<phase>-" infix and hashes `capture-pane -p`; the first sighting
// grants one window. No session, no tmux or a capture error makes no liveness
// claim. The closure keeps the last hash, so only one goroutine may call it.
func newTmuxPaneProbe(cycle int, phase, runID string, run tmuxRunner) func() bool {
	if run == nil {
		run = socketTmuxRunner(realTmuxRunner)
	}
	infix := fmt.Sprintf("-c%d-%s-", cycle, phase)
	// A known run id also requires the run token: matching another run's session
	// would keep a dead agent's stall clock fresh. An empty runID matches the infix alone.
	runInfix := ""
	if runID != "" {
		runInfix = "-" + sessionrecord.RunScopeToken(runID) + "-"
	}
	var lastHash string
	var observed bool
	return func() bool {
		session := findBridgeSession(run, infix, runInfix)
		if session == "" {
			return false
		}
		pane, err := run("capture-pane", "-t", session, "-p")
		if err != nil {
			return false
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(pane))
		if !observed {
			observed = true
			lastHash = sum
			return true
		}
		changed := sum != lastHash
		lastHash = sum
		return changed
	}
}

// findBridgeSession returns the first evolve-bridge session whose name contains
// infix and, when set, runInfix; it returns "" when none match or tmux fails.
func findBridgeSession(run tmuxRunner, infix, runInfix string) string {
	out, err := run("ls", "-F", "#{session_name}")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		s := strings.TrimSpace(line)
		if !strings.HasPrefix(s, "evolve-bridge-") || !strings.Contains(s, infix) {
			continue
		}
		if runInfix != "" && !strings.Contains(s, runInfix) {
			continue
		}
		return s
	}
	return ""
}
