package main

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

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
	c.MemUsedMB, c.HasMem = readTool(run, hostpressure.ParseVMStat, "vm_stat")
	c.SwapUsedMB, c.HasSwap = readTool(run, hostpressure.ParseSwapUsage, "sysctl", "-n", "vm.swapusage")
	return c
}

func readMacOSHost(run func(string, ...string) ([]byte, error)) HostInfo {
	h := HostInfo{OS: runtime.GOOS}
	if out, err := run("sysctl", "-n", "hw.ncpu"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 {
			h.CPUs = n
		}
	}
	if out, err := run("sysctl", "-n", "hw.memsize"); err == nil {
		if m, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil && m > 0 {
			h.MemBytes = m
		}
	}
	return h
}
