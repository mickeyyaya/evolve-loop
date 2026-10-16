package observer

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type psRunner func(pid int) (string, error)

func realPSRunner(pid int) (string, error) {
	out, err := sysexec.Command(context.Background(), "ps", "-o", "time=", "-p", strconv.Itoa(pid)).Output()
	return string(out), err
}

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
			return true
		}
		advanced := cpu != lastCPU
		lastCPU = cpu
		return advanced
	}
}

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
