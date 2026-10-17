package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fixtureStat = `cpu  100 10 50 800 40 5 5 10 0 0
cpu0 50 5 25 400 20 2 3 5 0 0
intr 12345
procs_running 3
procs_blocked 0
`

const fixtureMeminfo = `MemTotal:       16384000 kB
MemFree:         1000000 kB
MemAvailable:    8192000 kB
SwapTotal:       2048000 kB
SwapFree:        1024000 kB
`

const fixtureDiskstats = `   7       0 loop0 1 0 2 0 0 0 0 0 0 100 0 0 0 0 0 0 0
   8       0 sda 100 0 200 50 10 0 20 5 0 400 55 0 0 0 0 0 0
   8       1 sda1 100 0 200 50 10 0 20 5 0 390 55 0 0 0 0 0 0
 259       0 nvme0n1 5 0 10 1 1 0 2 1 0 600 2 0 0 0 0 0 0
 259       1 nvme0n1p1 5 0 10 1 1 0 2 1 0 590 2 0 0 0 0 0 0
`

func writeProc(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func fullProc(t *testing.T) string {
	return writeProc(t, map[string]string{"stat": fixtureStat, "meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats})
}

func TestReadLinux_ParsesAllSources(t *testing.T) {
	at := time.Unix(50, 0)
	got, err := readLinux(fullProc(t), func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	if got.CPUTotal != 1020 || got.CPUBusy != 1020-800-40 {
		t.Errorf("cpu busy=%d total=%d", got.CPUBusy, got.CPUTotal)
	}
	if !got.HasRunQueue || got.RunQueue != 3 {
		t.Errorf("runqueue=%d has=%v", got.RunQueue, got.HasRunQueue)
	}
	if got.MemUsedMB != float64(16384000-8192000)/1024 {
		t.Errorf("mem=%v", got.MemUsedMB)
	}
	if got.SwapUsedMB != 1000 {
		t.Errorf("swap=%v", got.SwapUsedMB)
	}
	if got.DiskTicksMs != 1000 || got.DiskCount != 2 {
		t.Errorf("disk ticks=%d count=%d", got.DiskTicksMs, got.DiskCount)
	}
	if !got.At.Equal(at) {
		t.Errorf("at=%v", got.At)
	}
}

func TestReadLinux_MissingOrBadFilesFailLoudly(t *testing.T) {
	cases := map[string]map[string]string{
		"no stat":        {"meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats},
		"no meminfo":     {"stat": fixtureStat, "diskstats": fixtureDiskstats},
		"no diskstats":   {"stat": fixtureStat, "meminfo": fixtureMeminfo},
		"stat garbage":   {"stat": "nothing here\n", "meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats},
		"stat short":     {"stat": "cpu 1 2\nprocs_running 1\n", "meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats},
		"stat nonnum":    {"stat": "cpu a b c d e f g h\nprocs_running 1\n", "meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats},
		"no running":     {"stat": "cpu  1 1 1 1 1 1 1 1\n", "meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats},
		"running nonnum": {"stat": "cpu  1 1 1 1 1 1 1 1\nprocs_running x\n", "meminfo": fixtureMeminfo, "diskstats": fixtureDiskstats},
		"mem garbage":    {"stat": fixtureStat, "meminfo": "MemTotal: 5 kB\n", "diskstats": fixtureDiskstats},
		"mem nonnum":     {"stat": fixtureStat, "meminfo": "MemTotal: x kB\nMemAvailable: 1 kB\nSwapTotal: 1 kB\nSwapFree: 1 kB\n", "diskstats": fixtureDiskstats},
		"disk nonnum":    {"stat": fixtureStat, "meminfo": fixtureMeminfo, "diskstats": "8 0 sda 1 0 2 0 0 0 0 0 0 zz 0\n"},
	}
	for name, files := range cases {
		if _, err := readLinux(writeProc(t, files), time.Now); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestParseDiskstats_SkipsShortLinesPartitionsAndVirtualDevices(t *testing.T) {
	ticks, count, err := parseDiskstats("short\n" + fixtureDiskstats + "253 0 vda 1 0 2 0 0 0 0 0 0 7 0\n253 1 xvdb 1 0 2 0 0 0 0 0 0 3 0\n179 0 mmcblk0 1 0 2 0 0 0 0 0 0 5 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if ticks != 1000+7+3+5 || count != 5 {
		t.Fatalf("ticks=%d count=%d", ticks, count)
	}
}
