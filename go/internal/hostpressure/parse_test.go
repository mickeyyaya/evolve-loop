package hostpressure

import (
	"errors"
	"os"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return string(raw)
}

const meminfoFixture = "MemTotal:       16000000 kB\nMemFree:          100000 kB\nMemAvailable:    4000000 kB\nSwapTotal:       2097152 kB\nSwapFree:        1048576 kB\n"

func TestParseMeminfo_UsedAvailableAndSwap(t *testing.T) {
	m, err := ParseMeminfo(meminfoFixture)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.UsedMB(), 12000000.0/1024; got != want {
		t.Errorf("UsedMB = %v, want %v", got, want)
	}
	if got := m.SwapUsedMB(); got != 1024 {
		t.Errorf("SwapUsedMB = %v, want 1024", got)
	}
	if got := m.AvailablePct(); got != 25 {
		t.Errorf("AvailablePct = %v, want 25", got)
	}
}

func TestParseMeminfo_BadInputFailsLoudly(t *testing.T) {
	missing := "MemTotal: 1 kB\nMemAvailable: 1 kB\nSwapTotal: 1 kB\n"
	if _, err := ParseMeminfo(missing); !errors.Is(err, ErrMissingField) {
		t.Errorf("missing SwapFree: err = %v, want ErrMissingField", err)
	}
	zeroTotal := "MemTotal: 0 kB\nMemAvailable: 0 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"
	if _, err := ParseMeminfo(zeroTotal); !errors.Is(err, ErrMissingField) {
		t.Errorf("zero MemTotal: err = %v, want ErrMissingField", err)
	}
	if _, err := ParseMeminfo("MemTotal: x kB\n"); err == nil {
		t.Error("non-numeric MemTotal: want error")
	}
}

func TestParseVMStat_UsedIsActivePlusWiredPlusCompressor(t *testing.T) {
	got, err := ParseVMStat(fixture(t, "vm_stat.txt"))
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
		if _, err := ParseVMStat(text); !errors.Is(err, ErrMacParse) {
			t.Errorf("case %d: err = %v, want ErrMacParse", i, err)
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
		got, err := ParseSwapUsage(in)
		if err != nil || got != want {
			t.Errorf("%q: got %v err %v want %v", in, got, err, want)
		}
	}
}

func TestParseSwapUsage_BadInputFailsLoudly(t *testing.T) {
	for _, in := range []string{"", "total = 1M free = 1M", "total = 1M used = xM free = 1M", "total = 1M used = 5T free = 1M", "used = "} {
		if _, err := ParseSwapUsage(in); !errors.Is(err, ErrMacParse) {
			t.Errorf("%q: err = %v, want ErrMacParse", in, err)
		}
	}
}

func TestParseSwapUsage_RealCapture(t *testing.T) {
	got, err := ParseSwapUsage(fixture(t, "swapusage.txt"))
	if err != nil || got != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestParseMemoryLevel(t *testing.T) {
	if got, err := ParseMemoryLevel("93\n"); err != nil || got != 93 {
		t.Fatalf("got %v err %v, want 93", got, err)
	}
	for _, in := range []string{"", "x", "-1", "101"} {
		if _, err := ParseMemoryLevel(in); !errors.Is(err, ErrMacParse) {
			t.Errorf("%q: err = %v, want ErrMacParse", in, err)
		}
	}
}

func TestMeminfo_ParsedFieldsKeepTheKilobyteReadings(t *testing.T) {
	m, err := ParseMeminfo(meminfoFixture)
	if err != nil {
		t.Fatal(err)
	}
	want := Meminfo{TotalKB: 16000000, AvailableKB: 4000000, SwapTotalKB: 2097152, SwapFreeKB: 1048576}
	if m != want {
		t.Errorf("Meminfo = %+v, want %+v", m, want)
	}
}
