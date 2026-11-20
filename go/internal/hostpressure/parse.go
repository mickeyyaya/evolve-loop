// Package hostpressure reads host load, memory and swap and decides an admission cap from them. See docs/architecture/packages/internal-hostpressure.md.
package hostpressure

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	kbPerMB    = 1024.0
	bytesPerMB = 1 << 20
	mbPerGB    = 1024.0
	percent    = 100.0
)

var (
	pageSizeRe = regexp.MustCompile(`page size of ([0-9]+) bytes`)
	swapUsedRe = regexp.MustCompile(`used = ([0-9.]+)([KMG])`)
	vmPageRe   = regexp.MustCompile(`^(Pages active|Pages wired down|Pages occupied by compressor):\s+([0-9]+)\.`)
)

var (
	ErrMissingField = errors.New("missing field")
	ErrMacParse     = errors.New("unparsable macOS counter output")
)

type Meminfo struct {
	TotalKB     float64
	AvailableKB float64
	SwapTotalKB float64
	SwapFreeKB  float64
}

func (m Meminfo) UsedMB() float64 { return (m.TotalKB - m.AvailableKB) / kbPerMB }

func (m Meminfo) SwapUsedMB() float64 { return (m.SwapTotalKB - m.SwapFreeKB) / kbPerMB }

func (m Meminfo) AvailablePct() float64 { return m.AvailableKB / m.TotalKB * percent }

func ParseMeminfo(text string) (Meminfo, error) {
	want := map[string]float64{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch key := strings.TrimSuffix(f[0], ":"); key {
		case "MemTotal", "MemAvailable", "SwapTotal", "SwapFree":
			v, err := strconv.ParseFloat(f[1], 64)
			if err != nil {
				return Meminfo{}, err
			}
			want[key] = v
		}
	}
	if len(want) != 4 || want["MemTotal"] <= 0 {
		return Meminfo{}, fmt.Errorf("/proc/meminfo: %w", ErrMissingField)
	}
	return Meminfo{TotalKB: want["MemTotal"], AvailableKB: want["MemAvailable"], SwapTotalKB: want["SwapTotal"], SwapFreeKB: want["SwapFree"]}, nil
}

func ParseVMStat(text string) (float64, error) {
	m := pageSizeRe.FindStringSubmatch(text)
	if m == nil {
		return 0, fmt.Errorf("vm_stat page size: %w", ErrMacParse)
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
		return 0, fmt.Errorf("vm_stat pages: %w", ErrMacParse)
	}
	return pages * pageSize / bytesPerMB, nil
}

func ParseSwapUsage(text string) (float64, error) {
	m := swapUsedRe.FindStringSubmatch(text)
	if m == nil {
		return 0, fmt.Errorf("vm.swapusage: %w", ErrMacParse)
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

func ParseMemoryLevel(text string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || v < 0 || v > percent {
		return 0, fmt.Errorf("kern.memorystatus_level %q: %w", strings.TrimSpace(text), ErrMacParse)
	}
	return v, nil
}
