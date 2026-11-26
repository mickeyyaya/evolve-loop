//go:build linux

package proctree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	ppidField           = 1
	pgrpField           = 2
	startTicksField     = 19
	clockTicksPerSecond = 100
)

var errProcStatMalformed = errors.New("procstart: stat is malformed")

func readProcStart(pid int) (string, error) {
	return startOfAt("/proc", pid)
}

func startOfAt(root string, pid int) (string, error) {
	parsed, err := statAt(root, pid)
	if err != nil {
		return "", err
	}
	boot, err := os.ReadFile(root + "/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(boot)) + ":" + parsed.ticks, nil
}

type procStat struct {
	comm       string
	ppid, pgrp int
	ticks      string
}

func parseStat(stat string) (procStat, error) {
	open, end := strings.IndexByte(stat, '('), strings.LastIndexByte(stat, ')')
	if open < 0 || end < open {
		return procStat{}, fmt.Errorf("%w: %q", errProcStatMalformed, stat)
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) <= startTicksField {
		return procStat{}, fmt.Errorf("%w: %q", errProcStatMalformed, stat)
	}
	ppid, ppidErr := strconv.Atoi(fields[ppidField])
	pgrp, pgrpErr := strconv.Atoi(fields[pgrpField])
	if err := errors.Join(ppidErr, pgrpErr); err != nil {
		return procStat{}, fmt.Errorf("%w: %w", errProcStatMalformed, err)
	}
	return procStat{comm: stat[open+1 : end], ppid: ppid, pgrp: pgrp, ticks: fields[startTicksField]}, nil
}

func readProcStartTime(pid int) (time.Time, error) {
	return startTimeAt("/proc", pid)
}

func startTimeAt(root string, pid int) (time.Time, error) {
	parsed, err := statAt(root, pid)
	if err != nil {
		return time.Time{}, err
	}
	ticks, err := parsed.startTicks()
	if err != nil {
		return time.Time{}, err
	}
	boot, err := bootTime(root)
	if err != nil {
		return time.Time{}, err
	}
	return startedAt(boot, ticks), nil
}

func statAt(root string, pid int) (procStat, error) {
	stat, err := os.ReadFile(root + "/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return procStat{}, syscall.ESRCH
	}
	if err != nil {
		return procStat{}, err
	}
	return parseStat(string(stat))
}

func (s procStat) startTicks() (int64, error) {
	ticks, err := strconv.ParseInt(s.ticks, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errProcStatMalformed, err)
	}
	return ticks, nil
}

func startedAt(boot time.Time, ticks int64) time.Time {
	return boot.Add(time.Duration(ticks) * time.Second / clockTicksPerSecond)
}

func bootTime(root string) (time.Time, error) {
	body, err := os.ReadFile(root + "/stat")
	if err != nil {
		return time.Time{}, err
	}
	for _, line := range strings.Split(string(body), "\n") {
		value, found := strings.CutPrefix(line, "btime ")
		if !found {
			continue
		}
		sec, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: btime: %w", errProcStatMalformed, err)
		}
		return time.Unix(sec, 0), nil
	}
	return time.Time{}, fmt.Errorf("%w: no btime line", errProcStatMalformed)
}
