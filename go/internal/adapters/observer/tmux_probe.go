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

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// anyProbe composes liveness probes: the agent is alive if ANY sub-probe says
// so. ALL sub-probes are consulted every call (no short-circuit) so each keeps
// its internal last-sample state consistent across calls. Nil sub-probes are
// skipped; an all-nil/empty set yields a probe that always returns false. Lives
// here (not in cpu_probe.go) because it is a generic combinator over the probe
// type, not CPU-specific.
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
func newTmuxPaneProbe(cycle int, phase, runID string, run tmuxRunner) func() bool {
	if run == nil {
		run = realTmuxRunner
	}
	infix := fmt.Sprintf("-c%d-%s-", cycle, phase)
	// CB.6: a probe that knows its run id asserts the run token before
	// claiming liveness — matching ANOTHER run's session would keep a dead
	// agent's stall clock fresh (the cross-run false-liveness class).
	// runID="" (legacy single-driver dispatch) keeps the infix-only match.
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
			return true // first sighting: the pane exists; grant one window
		}
		changed := sum != lastHash
		lastHash = sum
		return changed
	}
}

// findBridgeSession returns the first tmux session whose name is an
// evolve-bridge session containing infix (and runInfix, when non-empty — the
// CB.6 run-ownership assertion), or "" when none match / tmux errors.
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
			continue // another run's session: never a liveness claim for ours
		}
		return s
	}
	return ""
}
