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

func TestParseVMStat_UsedIsActivePlusWiredPlusCompressor(t *testing.T) {
	got, err := parseVMStat(fixture(t, "vm_stat.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := float64(1225522+212195) * 16384 / (1 << 20)
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseVMStat_BadInputFailsLoudly(t *testing.T) {
	bad := []string{
		"",
		"Pages active: 1.\nPages wired down: 1.\nPages occupied by compressor: 1.\n",
		"Mach Virtual Memory Statistics: (page size of x bytes)\nPages active: 1.\nPages wired down: 1.\nPages occupied by compressor: 1.\n",
		"Mach Virtual Memory Statistics: (page size of 4096 bytes)\nPages active: 1.\n",
		"Mach Virtual Memory Statistics: (page size of 4096 bytes)\nPages active: x.\nPages wired down: 1.\nPages occupied by compressor: 1.\n",
	}
	for i, text := range bad {
		if _, err := parseVMStat(text); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestParseSwapUsage_Units(t *testing.T) {
	cases := map[string]float64{
		"total = 2048.00M  used = 1163.25M  free = 884.75M  (encrypted)": 1163.25,
		"total = 4.00G  used = 1.50G  free = 2.50G  (encrypted)":         1536,
		"total = 0.00K  used = 2048.00K  free = 0.00K":                   2,
		"total = 0.00M  used = 0.00M  free = 0.00M":                      0,
	}
	for in, want := range cases {
		got, err := parseSwapUsage(in)
		if err != nil || got != want {
			t.Errorf("%q: got %v err %v want %v", in, got, err, want)
		}
	}
}

func TestParseSwapUsage_BadInputFailsLoudly(t *testing.T) {
	for _, in := range []string{"", "total = 1M free = 1M", "total = 1M used = xM free = 1M", "total = 1M used = 5T free = 1M", "used = "} {
		if _, err := parseSwapUsage(in); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
}

func TestParseSwapUsage_RealCapture(t *testing.T) {
	got, err := parseSwapUsage(fixture(t, "swapusage.txt"))
	if err != nil || got != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
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
