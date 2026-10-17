package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	bytesPerMB = 1 << 20
	mbPerGB    = 1024.0
)

var (
	pageSizeRe = regexp.MustCompile(`page size of ([0-9]+) bytes`)
	swapUsedRe = regexp.MustCompile(`used = ([0-9.]+)([KMG])`)
	vmPageRe   = regexp.MustCompile(`^(Pages active|Pages wired down|Pages occupied by compressor):\s+([0-9]+)\.`)
)

var errMacParse = errors.New("unparsable macOS counter output")

var toolTimeout = 800 * time.Millisecond

func execOutput(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	return sysexec.Command(ctx, name, args...).Output()
}

func readTool(run func(string, ...string) ([]byte, error), parse func(string) (float64, error), name string, args ...string) (float64, bool) {
	out, err := run(name, args...)
	if err != nil {
		return 0, false
	}
	v, err := parse(string(out))
	return v, err == nil
}

func readMacOS(run func(string, ...string) ([]byte, error), now func() time.Time) Counters {
	c := Counters{At: now()}
	c.MemUsedMB, c.HasMem = readTool(run, parseVMStat, "vm_stat")
	c.SwapUsedMB, c.HasSwap = readTool(run, parseSwapUsage, "sysctl", "-n", "vm.swapusage")
	return c
}

func parseVMStat(text string) (float64, error) {
	m := pageSizeRe.FindStringSubmatch(text)
	if m == nil {
		return 0, fmt.Errorf("vm_stat page size: %w", errMacParse)
	}
	pageSize, _ := strconv.ParseFloat(m[1], 64)
	var pages float64
	found := 0
	for _, line := range strings.Split(text, "\n") {
		pm := vmPageRe.FindStringSubmatch(line)
		if pm == nil {
			continue
		}
		n, _ := strconv.ParseFloat(pm[2], 64)
		pages += n
		found++
	}
	if found != 3 {
		return 0, fmt.Errorf("vm_stat pages: %w", errMacParse)
	}
	return pages * pageSize / bytesPerMB, nil
}

func parseSwapUsage(text string) (float64, error) {
	m := swapUsedRe.FindStringSubmatch(text)
	if m == nil {
		return 0, fmt.Errorf("vm.swapusage: %w", errMacParse)
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	switch m[2] {
	case "G":
		return v * mbPerGB, nil
	case "K":
		return v / kbPerMB, nil
	}
	return v, nil
}
