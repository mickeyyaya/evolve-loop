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
)

const startTicksField = 19

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
