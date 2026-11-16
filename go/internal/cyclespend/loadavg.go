package cyclespend

import (
	"strconv"
	"strings"
)

func ReadLoadAverage(readFile func(string) ([]byte, error), sysctl func() (string, error)) float64 {
	if raw, err := readFile("/proc/loadavg"); err == nil {
		return ParseLoadAverage(string(raw))
	}
	raw, err := sysctl()
	if err != nil {
		return 0
	}
	return ParseLoadAverage(raw)
}

func ParseLoadAverage(raw string) float64 {
	fields := strings.Fields(strings.Trim(strings.TrimSpace(raw), "{}"))
	if len(fields) == 0 {
		return 0
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return load
}
