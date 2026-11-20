package hostpressure

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	loadavgPath = "/proc/loadavg"
	meminfoPath = "/proc/meminfo"
)

type Reading struct {
	Load1           float64
	CPUs            int
	MemAvailablePct float64
	SwapUsedMB      float64
}

func (r Reading) LoadPerCore() float64 { return r.Load1 / float64(max(r.CPUs, 1)) }

type Reader struct {
	ReadFile func(string) ([]byte, error)
	Run      func(name string, args ...string) ([]byte, error)
	CPUs     func() int
}

func (r Reader) Read() (Reading, error) {
	if raw, err := r.ReadFile(loadavgPath); err == nil {
		return r.readLinux(string(raw))
	}
	return r.readDarwin()
}

func (r Reader) readLinux(loadavg string) (Reading, error) {
	load, err := parseLoad(loadavg)
	if err != nil {
		return Reading{}, err
	}
	raw, err := r.ReadFile(meminfoPath)
	if err != nil {
		return Reading{}, fmt.Errorf("read %s: %w", meminfoPath, err)
	}
	mem, err := ParseMeminfo(string(raw))
	if err != nil {
		return Reading{}, err
	}
	return Reading{Load1: load, CPUs: r.CPUs(), MemAvailablePct: mem.AvailablePct(), SwapUsedMB: mem.SwapUsedMB()}, nil
}

func (r Reader) readDarwin() (Reading, error) {
	loadText, err := r.sysctl("vm.loadavg")
	if err != nil {
		return Reading{}, err
	}
	load, err := parseLoad(loadText)
	if err != nil {
		return Reading{}, err
	}
	pct, err := r.darwinAvailablePct()
	if err != nil {
		return Reading{}, err
	}
	swapText, err := r.sysctl("vm.swapusage")
	if err != nil {
		return Reading{}, err
	}
	swap, err := ParseSwapUsage(swapText)
	if err != nil {
		return Reading{}, err
	}
	return Reading{Load1: load, CPUs: r.CPUs(), MemAvailablePct: pct, SwapUsedMB: swap}, nil
}

func (r Reader) darwinAvailablePct() (float64, error) {
	if text, err := r.sysctl("kern.memorystatus_level"); err == nil {
		if pct, perr := ParseMemoryLevel(text); perr == nil {
			return pct, nil
		}
	}
	out, err := r.Run("vm_stat")
	if err != nil {
		return 0, fmt.Errorf("vm_stat: %w", err)
	}
	usedMB, err := ParseVMStat(string(out))
	if err != nil {
		return 0, err
	}
	sizeText, err := r.sysctl("hw.memsize")
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseFloat(strings.TrimSpace(sizeText), 64)
	if err != nil || size <= 0 {
		return 0, fmt.Errorf("hw.memsize %q: %w", strings.TrimSpace(sizeText), ErrMacParse)
	}
	return percent - usedMB/(size/bytesPerMB)*percent, nil
}

func (r Reader) sysctl(key string) (string, error) {
	out, err := r.Run("sysctl", "-n", key)
	if err != nil {
		return "", fmt.Errorf("sysctl %s: %w", key, err)
	}
	return string(out), nil
}

func parseLoad(text string) (float64, error) {
	fields := strings.Fields(strings.Trim(strings.TrimSpace(text), "{}"))
	if len(fields) == 0 {
		return 0, fmt.Errorf("load average: %w", ErrMissingField)
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("load average %q: %w", fields[0], ErrMissingField)
	}
	return load, nil
}
