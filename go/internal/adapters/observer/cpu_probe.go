package observer

// cpu_probe.go — a CPU-delta LivenessProbe for HEADLESS phases. The tmux
// pane-hash probe (tmux_probe.go) covers tmux-driver agents, but a headless
// `claude -p` phase that thinks for minutes in one turn with no streamed output
// has no pane to probe and its stdout-log + workspace both go flat — the same
// false-stall shape as cycle-190, one layer over. This probe reads the agent
// process's accumulated CPU time: a computing agent (even silently thinking)
// accrues CPU; a deadlocked one does not. The bridge writes the agent PID to a
// per-phase file at launch (engine.go execRunner, gated by EVOLVE_BRIDGE_PIDFILE);
// this probe reads it.
//
// NOT absolute ground truth — it is a better PROXY than stdout-flatness, with
// its own blind spots: an agent blocked on a slow API response mid-turn accrues
// little CPU (false-negative), and a wedged busy-loop reads alive (false-pos).
// It fails SAFE: a no-claim (false) leaves the caller's existing stall logic
// unchanged, and the first sighting grants one window. The true fix remains a
// bridge wall-clock heartbeat envelope independent of agent output.

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// psRunner returns `ps` CPU-time output for pid. Injectable for tests (no real
// process required). Nil → realPSRunner.
type psRunner func(pid int) (string, error)

func realPSRunner(pid int) (string, error) {
	// `ps -o time=` prints cumulative CPU time (e.g. "0:23.45" / "01:23:45")
	// with no header; portable across macOS and Linux.
	out, err := exec.Command("ps", "-o", "time=", "-p", strconv.Itoa(pid)).Output()
	return string(out), err
}

// newProcessCPUProbe returns a LivenessProbe that reports the agent alive when
// its accumulated CPU time advanced since the previous call. The PID is read
// from pidFile (written by the bridge at launch). Missing pidfile, an
// unparseable PID, or a ps error → false (no liveness claim; the caller's stall
// logic proceeds unchanged). The first sighting returns true (grant one window),
// mirroring the tmux probe. Single-goroutine use only (the observer's Watch
// loop): it holds the last CPU sample.
func newProcessCPUProbe(pidFile string, run psRunner) func() bool {
	if run == nil {
		run = realPSRunner
	}
	var lastCPU string
	var observed bool
	return func() bool {
		pid, ok := readPID(pidFile)
		if !ok {
			return false
		}
		out, err := run(pid)
		if err != nil {
			return false
		}
		cpu := strings.TrimSpace(out)
		if cpu == "" {
			return false
		}
		if !observed {
			observed = true
			lastCPU = cpu
			return true // first sighting: grant one window
		}
		advanced := cpu != lastCPU
		lastCPU = cpu
		return advanced
	}
}

// readPID parses the integer PID written in pidFile, or (0,false) when the file
// is absent/empty/unparseable. A pidfile that appears LATER (lost a startup
// race, or a slow mount) is not a permanent trap: `observed` is set only after a
// successful read, so the first successful read still fires the first-sighting
// grant.
func readPID(pidFile string) (int, bool) {
	if pidFile == "" {
		return 0, false
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}
