package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	cpuRatioLimit   = 1.35
	cpuFloorSeconds = 15.0
	referenceCPU    = 2.0
	minReferences   = 3
	cpuHeading      = "## Package CPU against the recorded baseline"
)

type rusageRecord struct {
	Package string  `json:"package"`
	UserS   float64 `json:"user_s"`
	SystemS float64 `json:"system_s"`
}

func loadRusage(path string) (map[string]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("rusage %s: %w", path, err)
	}
	defer f.Close()
	cpu := map[string]float64{}
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 16*1024*1024)
	for line := 1; sc.Scan(); line++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var rec rusageRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return nil, fmt.Errorf("rusage %s: line %d: %w", path, line, err)
		}
		cpu[shortPkg(rec.Package)] += rec.UserS + rec.SystemS
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("rusage %s: %w", path, err)
	}
	return cpu, nil
}

type cpuVerdict struct {
	Pkg        string
	Baseline   float64
	Run        float64
	Index      float64
	References int
	Judged     bool
}

func (v cpuVerdict) normalized() float64 { return v.Run / v.Index }

func (v cpuVerdict) failed() bool {
	n := v.normalized()
	return v.Judged && n > cpuRatioLimit*v.Baseline && n-v.Baseline > cpuFloorSeconds
}

func median(values []float64) float64 {
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func (rf *ratchetFile) judgeCPU(usage map[string]float64, goos string) []cpuVerdict {
	ratios := map[string]float64{}
	var names []string
	for name, rec := range rf.Packages {
		base, run := rec.CPU[goos], usage[name]
		if base <= 0 || run <= 0 {
			continue
		}
		names = append(names, name)
		if base >= referenceCPU {
			ratios[name] = run / base
		}
	}
	sort.Strings(names)
	verdicts := make([]cpuVerdict, 0, len(names))
	for _, name := range names {
		var others []float64
		for ref, ratio := range ratios {
			if ref != name {
				others = append(others, ratio)
			}
		}
		v := cpuVerdict{Pkg: name, Baseline: rf.Packages[name].CPU[goos], Run: usage[name], References: len(others)}
		if len(others) >= minReferences {
			v.Index, v.Judged = median(others), true
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}

func cpuFailures(verdicts []cpuVerdict) []string {
	var out []string
	for _, v := range verdicts {
		if v.failed() {
			out = append(out, fmt.Sprintf("%s used %.2fs of CPU, %.2fs after the speed index %.2f, above %.2fx its baseline of %.2fs", v.Pkg, v.Run, v.normalized(), v.Index, cpuRatioLimit, v.Baseline))
		}
	}
	return out
}

func writeCPUSection(w io.Writer, verdicts []cpuVerdict) {
	fmt.Fprintf(w, "\n%s\n\n", cpuHeading)
	var rows []cpuVerdict
	for _, v := range verdicts {
		if v.failed() || !v.Judged {
			rows = append(rows, v)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, "_(none)_ of %d packages with a baseline\n", len(verdicts))
		return
	}
	fmt.Fprint(w, "| Package | Baseline CPU (s) | Run CPU (s) | Speed index | Verdict |\n|---|--:|--:|--:|---|\n")
	for _, v := range rows {
		verdict := fmt.Sprintf("reported only: %d reference packages, want %d", v.References, minReferences)
		if v.Judged {
			verdict = fmt.Sprintf("FAIL: %.2fs after the index", v.normalized())
		}
		fmt.Fprintf(w, "| %s | %.2f | %.2f | %.2f | %s |\n", v.Pkg, v.Baseline, v.Run, v.Index, verdict)
	}
}
