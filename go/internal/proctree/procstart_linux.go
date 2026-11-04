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
	startTicksField     = 19
	clockTicksPerSecond = 100
)

var errProcStatMalformed = errors.New("procstart: stat is malformed")

func readProcStart(pid int) (string, error) {
	return startOfAt("/proc", pid)
}

func startOfAt(root string, pid int) (string, error) {
	stat, err := os.ReadFile(root + "/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return "", syscall.ESRCH
	}
	if err != nil {
		return "", err
	}
	ticks, err := startTicks(string(stat))
	if err != nil {
		return "", err
	}
	boot, err := os.ReadFile(root + "/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(boot)) + ":" + ticks, nil
}

func startTicks(stat string) (string, error) {
	end := strings.LastIndexByte(stat, ')')
	fields := strings.Fields(stat[end+1:])
	if end < 0 || len(fields) <= startTicksField {
		return "", fmt.Errorf("%w: %q", errProcStatMalformed, stat)
	}
	return fields[startTicksField], nil
}

func readProcStartTime(pid int) (time.Time, error) {
	return startTimeAt("/proc", pid)
}

func startTimeAt(root string, pid int) (time.Time, error) {
	stat, err := os.ReadFile(root + "/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, syscall.ESRCH
	}
	if err != nil {
		return time.Time{}, err
	}
	field, err := startTicks(string(stat))
	if err != nil {
		return time.Time{}, err
	}
	ticks, err := strconv.ParseInt(field, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %w", errProcStatMalformed, err)
	}
	boot, err := bootTime(root)
	if err != nil {
		return time.Time{}, err
	}
	return boot.Add(time.Duration(ticks) * time.Second / clockTicksPerSecond), nil
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
