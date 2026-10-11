//go:build integration

package observer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var psCPUTime = regexp.MustCompile(`^\s*([0-9]+-)?[0-9]+(:[0-9]{2})+(\.[0-9]+)?\s*$`)

func reapedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run true: %v", err)
	}
	return cmd.ProcessState.Pid()
}

func burnCPU(d time.Duration) int {
	n := 0
	for start := time.Now(); time.Since(start) < d; n++ {
	}
	return n
}

func TestRealPSRunner_ReportsCumulativeCPUTimeOfALiveProcess(t *testing.T) {
	out, err := realPSRunner(os.Getpid())

	if err != nil {
		t.Fatalf("realPSRunner(self): %v", err)
	}
	if strings.Contains(out, "TIME") || !psCPUTime.MatchString(out) {
		t.Fatalf("realPSRunner(self) = %q, want one headerless CPU-time value", out)
	}
}

func TestRealPSRunner_GonePIDIsAnExitErrorWithNoTime(t *testing.T) {
	out, err := realPSRunner(reapedPID(t))

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a ps exit error for a reaped pid", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("out = %q, want no CPU time for a gone pid", out)
	}
}

func TestRealPSRunner_MissingPSIsAnError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	out, err := realPSRunner(os.Getpid())

	if !errors.Is(err, exec.ErrNotFound) || out != "" {
		t.Fatalf("realPSRunner = %q, %v; want empty output and exec.ErrNotFound", out, err)
	}
}

func TestNewProcessCPUProbe_DefaultRunnerSeesABusyProcessAdvance(t *testing.T) {
	probe := newProcessCPUProbe(writePIDFile(t, strconv.Itoa(os.Getpid())), nil)

	first := probe()
	second := false
	for deadline := time.Now().Add(10 * time.Second); !second && time.Now().Before(deadline); {
		burnCPU(200 * time.Millisecond)
		second = probe()
	}

	if !first || !second {
		t.Fatalf("probe first=%v second=%v, want the grant then an advance after burning CPU", first, second)
	}
}

func TestNewProcessCPUProbe_DefaultRunnerMakesNoClaimForAGonePID(t *testing.T) {
	if newProcessCPUProbe(writePIDFile(t, strconv.Itoa(reapedPID(t))), nil)() {
		t.Fatal("probe claimed liveness for a reaped pid")
	}
}
