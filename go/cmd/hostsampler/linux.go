package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	kbPerMB          = 1024.0
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
	want := map[string]float64{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		key := strings.TrimSuffix(f[0], ":")
		switch key {
		case "MemTotal", "MemAvailable", "SwapTotal", "SwapFree":
			v, perr := strconv.ParseFloat(f[1], 64)
			if perr != nil {
				return 0, 0, perr
			}
			want[key] = v
		}
	}
	if len(want) != 4 {
		return 0, 0, fmt.Errorf("/proc/meminfo: %w", errMissingField)
	}
	return (want["MemTotal"] - want["MemAvailable"]) / kbPerMB, (want["SwapTotal"] - want["SwapFree"]) / kbPerMB, nil
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
