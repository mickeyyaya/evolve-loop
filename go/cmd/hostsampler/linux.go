package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
)

const (
	cpuLineMinFields = 9
	cpuIdleIndex     = 4
	cpuIOWaitIndex   = 5
	cpuSumFields     = 8
	diskTicksIndex   = 12
	diskNameIndex    = 2
)

var wholeDisk = regexp.MustCompile(`^(sd[a-z]+|vd[a-z]+|xvd[a-z]+|nvme[0-9]+n[0-9]+|mmcblk[0-9]+)$`)

var errMissingField = errors.New("missing field")

func readLinux(root string, now func() time.Time) (Counters, error) {
	stat, err := os.ReadFile(filepath.Join(root, "stat"))
	if err != nil {
		return Counters{}, err
	}
	busy, total, running, err := parseStat(string(stat))
	if err != nil {
		return Counters{}, err
	}
	meminfo, err := os.ReadFile(filepath.Join(root, "meminfo"))
	if err != nil {
		return Counters{}, err
	}
	mem, swap, err := parseMeminfo(string(meminfo))
	if err != nil {
		return Counters{}, err
	}
	diskstats, err := os.ReadFile(filepath.Join(root, "diskstats"))
	if err != nil {
		return Counters{}, err
	}
	ticks, disks, err := parseDiskstats(string(diskstats))
	if err != nil {
		return Counters{}, err
	}
	return Counters{
		At: now(), HasCPU: true, CPUBusy: busy, CPUTotal: total, HasRunQueue: true, RunQueue: running,
		HasMem: true, MemUsedMB: mem, HasSwap: true, SwapUsedMB: swap, DiskTicksMs: ticks, DiskCount: disks,
	}, nil
}

func readLinuxHost(root string, run func(string, ...string) ([]byte, error)) HostInfo {
	h := HostInfo{OS: runtime.GOOS}
	if out, err := run("nproc"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 {
			h.CPUs = n
		}
	}
	if meminfo, err := os.ReadFile(filepath.Join(root, "meminfo")); err == nil {
		if memBytes, err := parseMemTotalBytes(string(meminfo)); err == nil {
			h.MemBytes = memBytes
		}
	}
	return h
}

func parseMemTotalBytes(text string) (uint64, error) {
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "MemTotal:" {
			kb, err := strconv.ParseUint(f[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return kb * 1024, nil
		}
	}
	return 0, fmt.Errorf("/proc/meminfo MemTotal: %w", errMissingField)
}

func parseStat(text string) (busy, total uint64, running int, err error) {
	cpuSeen, runSeen := false, false
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch {
		case f[0] == "cpu":
			if busy, total, err = cpuTicks(f); err != nil {
				return 0, 0, 0, err
			}
			cpuSeen = true
		case f[0] == "procs_running" && len(f) > 1:
			if running, err = strconv.Atoi(f[1]); err != nil {
				return 0, 0, 0, err
			}
			runSeen = true
		}
	}
	if !cpuSeen || !runSeen {
		return 0, 0, 0, fmt.Errorf("/proc/stat: %w", errMissingField)
	}
	return busy, total, running, nil
}

func cpuTicks(f []string) (busy, total uint64, err error) {
	if len(f) < cpuLineMinFields {
		return 0, 0, fmt.Errorf("/proc/stat cpu line: %w", errMissingField)
	}
	var idle uint64
	for i := 1; i <= cpuSumFields; i++ {
		v, perr := strconv.ParseUint(f[i], 10, 64)
		if perr != nil {
			return 0, 0, perr
		}
		total += v
		if i == cpuIdleIndex || i == cpuIOWaitIndex {
			idle += v
		}
	}
	return total - idle, total, nil
}

func parseMeminfo(text string) (usedMB, swapMB float64, err error) {
	m, err := hostpressure.ParseMeminfo(text)
	if err != nil {
		return 0, 0, err
	}
	return m.UsedMB(), m.SwapUsedMB(), nil
}

func parseDiskstats(text string) (ticks uint64, disks int, err error) {
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) <= diskTicksIndex || !wholeDisk.MatchString(f[diskNameIndex]) {
			continue
		}
		v, perr := strconv.ParseUint(f[diskTicksIndex], 10, 64)
		if perr != nil {
			return 0, 0, perr
		}
		ticks += v
		disks++
	}
	return ticks, disks, nil
}
