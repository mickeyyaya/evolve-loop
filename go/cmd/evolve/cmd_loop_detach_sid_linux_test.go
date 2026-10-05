package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func detachSessionID(pid int) (int, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	stat := string(raw)
	fields := strings.Fields(stat[strings.LastIndexByte(stat, ')')+1:])
	if len(fields) < 4 {
		return 0, fmt.Errorf("short /proc/%d/stat: %q", pid, stat)
	}
	return strconv.Atoi(fields[3])
}
