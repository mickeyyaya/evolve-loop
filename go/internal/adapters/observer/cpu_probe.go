package observer

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// psRunner returns `ps` CPU-time output for pid; tests inject it.
type psRunner func(pid int) (string, error)

func realPSRunner(pid int) (string, error) {
	// `ps -o time=` prints cumulative CPU time with no header, on macOS and Linux alike.
	out, err := exec.Command("ps", "-o", "time=", "-p", strconv.Itoa(pid)).Output()
	return string(out), err
}

// newProcessCPUProbe returns a LivenessProbe that reports the agent alive when
// its cumulative CPU time advanced since the last call. The PID comes from the
// bridge's pidfile. Any read or ps failure makes no liveness claim, and the
// first sighting grants one window. The closure keeps the last sample, so only
// one goroutine may call it.
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
		// observed flips only after a successful read, so a late pidfile still earns the grant.
		if !observed {
			observed = true
			lastCPU = cpu
			return true
		}
		advanced := cpu != lastCPU
		lastCPU = cpu
		return advanced
	}
}

// readPID parses the PID in pidFile, reporting false when it is absent, empty or not a positive integer.
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
