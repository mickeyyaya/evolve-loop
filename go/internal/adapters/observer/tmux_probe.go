package observer

// tmux_probe.go — cycle-190 fix: a concrete LivenessProbe for tmux-driver
// phases. The auto-spawn observer's filesystem signals (stdout-log size,
// workspace mtime) both go flat while a tmux agent is in a long single
// "Incubating" turn — extended thinking plus one large tool call that commits
// no scrollback lines and writes no artifact until the turn ends. The live
// tmux pane is the only liveness signal in that window (its spinner /
// token-counter advances every second). This probe reads it.

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
	"strings"
)

// tmuxRunner runs a tmux subcommand and returns its stdout. Injectable so the
// probe is unit-testable without a real tmux server. Nil → realTmuxRunner.
type tmuxRunner func(args ...string) ([]byte, error)

func realTmuxRunner(args ...string) ([]byte, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, err
	}
	return exec.Command(path, args...).Output()
}

// newTmuxPaneProbe returns a LivenessProbe that reports the agent alive when
// the live tmux pane for this cycle/phase changed since the last call.
//
// It locates the bridge session by the deterministic infix "-c<cycle>-<phase>-"
// (sessions are named evolve-bridge-<cli>-c<cycle>-<phase>-pid<pid>-<ts>, e.g.
// evolve-bridge-agy-c190-build-pid90464-1780402698) and hashes `capture-pane
// -p`. A changed hash means the pane is animating (the agent is mid-turn, not
// hung); the first sighting returns true to grant one more window. No matching
// session, tmux absent, or a capture error → false: the probe makes no
// liveness claim and the caller's stall logic proceeds unchanged.
//
// The returned closure holds the last-seen hash, so it must be called from a
// single goroutine (the observer's Watch loop — which it is).
func newTmuxPaneProbe(cycle int, phase string, run tmuxRunner) func() bool {
	if run == nil {
		run = realTmuxRunner
	}
	infix := fmt.Sprintf("-c%d-%s-", cycle, phase)
	var lastHash string
	var observed bool
	return func() bool {
		session := findBridgeSession(run, infix)
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
			return true // first sighting: the pane exists; grant one window
		}
		changed := sum != lastHash
		lastHash = sum
		return changed
	}
}

// findBridgeSession returns the first tmux session whose name is an
// evolve-bridge session containing infix, or "" when none match / tmux errors.
func findBridgeSession(run tmuxRunner, infix string) string {
	out, err := run("ls", "-F", "#{session_name}")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "evolve-bridge-") && strings.Contains(s, infix) {
			return s
		}
	}
	return ""
}
