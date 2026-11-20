package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func fakeRunner(outputs map[string]string, fail string) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		if name == fail {
			return nil, errors.New("exec failed")
		}
		return []byte(outputs[name]), nil
	}
}

func TestReadMacOS_MemorySwapAndNoCPU(t *testing.T) {
	at := time.Unix(9, 0)
	run := fakeRunner(map[string]string{"vm_stat": fixture(t, "vm_stat.txt"), "sysctl": "total = 1M  used = 2.00M  free = 1M"}, "")
	got := readMacOS(run, func() time.Time { return at })
	if got.HasCPU || got.HasRunQueue || got.DiskCount != 0 || !got.HasSwap || got.SwapUsedMB != 2 || !got.At.Equal(at) {
		t.Fatalf("got %+v", got)
	}
	if !got.HasMem || got.MemUsedMB <= 0 {
		t.Fatalf("mem=%v", got.MemUsedMB)
	}
}

func TestReadMacOS_ToolFailureSkipsOnlyThatField(t *testing.T) {
	vm := fixture(t, "vm_stat.txt")
	cases := []struct {
		name              string
		outputs           map[string]string
		fail              string
		wantMem, wantSwap bool
	}{
		{"vm_stat exec fails", map[string]string{"sysctl": "used = 1M"}, "vm_stat", false, true},
		{"vm_stat unparsable", map[string]string{"vm_stat": "junk", "sysctl": "used = 1M"}, "", false, true},
		{"sysctl exec fails", map[string]string{"vm_stat": vm}, "sysctl", true, false},
		{"sysctl unparsable", map[string]string{"vm_stat": vm, "sysctl": "junk"}, "", true, false},
	}
	for _, c := range cases {
		got := readMacOS(fakeRunner(c.outputs, c.fail), time.Now)
		if got.HasMem != c.wantMem || got.HasSwap != c.wantSwap {
			t.Errorf("%s: got %+v", c.name, got)
		}
	}
}

func TestExecOutput_RunsRealCommandAndReportsFailure(t *testing.T) {
	out, err := execOutput("echo", "hi")
	if err != nil || string(out) != "hi\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if _, err := execOutput("/nonexistent/binary"); err == nil {
		t.Fatal("want error")
	}
}

func TestExecOutput_DeadlineKillsSlowTool(t *testing.T) {
	old := toolTimeout
	t.Cleanup(func() { toolTimeout = old })
	toolTimeout = 50 * time.Millisecond
	start := time.Now()
	if _, err := execOutput("sleep", "5"); err == nil {
		t.Fatal("want deadline error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("deadline not enforced")
	}
}

func TestToolTimeout_IsShorterThanDefaultInterval(t *testing.T) {
	if toolTimeout >= time.Second {
		t.Fatalf("toolTimeout=%v", toolTimeout)
	}
}

func TestReadMacOSHost_ReadsNCPUAndMemsize(t *testing.T) {
	run := func(name string, args ...string) ([]byte, error) {
		if name == "sysctl" && len(args) == 2 && args[0] == "-n" {
			switch args[1] {
			case "hw.ncpu":
				return []byte("5\n"), nil
			case "hw.memsize":
				return []byte("17179869184\n"), nil
			}
		}
		return nil, errors.New("unexpected")
	}
	got := readMacOSHost(run)
	if got.CPUs != 5 || got.MemBytes != 17179869184 || got.OS == "" {
		t.Fatalf("got %+v", got)
	}
}

func TestReadMacOSHost_IgnoresFailedOrMalformedProbes(t *testing.T) {
	run := func(name string, args ...string) ([]byte, error) {
		return []byte("bad"), nil
	}
	got := readMacOSHost(run)
	if got.CPUs != 0 || got.MemBytes != 0 {
		t.Fatalf("got %+v", got)
	}
}
