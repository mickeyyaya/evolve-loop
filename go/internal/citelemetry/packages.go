package citelemetry

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

const (
	actionPass  = "pass"
	actionFail  = "fail"
	actionSkip  = "skip"
	actionPause = "pause"

	limitUnknown   = "unknown"
	limitMemory    = "memory"
	limitSaturated = "cpu-saturation"
	limitSyscalls  = "subprocesses-and-syscalls"
	limitWait      = "wait"
	limitCPU       = "cpu"

	swapLimitMB          = 1024
	saturatedBusyPct     = 90
	syscallRuleMinCPUS   = 0.5
	waitEfficiencyFloor  = 0.5
	unitSerial           = "serial"
	unitParallel         = "parallel"
	subtestSeparator     = "/"
	packagePathSeparator = "/"
)

type Rusage struct {
	WallS     float64 `json:"wall_s"`
	UserS     float64 `json:"user_s"`
	SystemS   float64 `json:"system_s"`
	PeakRSSMB float64 `json:"peak_rss_mb"`
	ExitCode  int     `json:"exit_code"`
}

type rusageRecord struct {
	Rusage
	Package string `json:"package"`
}

type Sampler struct {
	Samples       int     `json:"samples"`
	CPUBusyMaxPct float64 `json:"cpu_busy_max_pct"`
	SwapUsedMaxMB float64 `json:"swap_used_max_mb"`
	cpuBusyMin    float64
}

type sample struct {
	Ts         time.Time `json:"ts"`
	CPUBusyPct float64   `json:"cpu_busy_pct"`
	SwapUsedMB float64   `json:"swap_used_mb"`
}

type event struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
}

type Package struct {
	Package string   `json:"package"`
	Result  string   `json:"result"`
	WallS   float64  `json:"wall_s"`
	Limit   string   `json:"limit"`
	Rusage  *Rusage  `json:"rusage,omitempty"`
	Sampler *Sampler `json:"sampler,omitempty"`
}

type testRun struct {
	name     string
	seconds  float64
	parallel bool
}

type packageRun struct {
	name   string
	start  time.Time
	end    time.Time
	result string
	wall   float64
	tests  []testRun
	paused map[string]bool
}

func isResult(action string) bool {
	return action == actionPass || action == actionFail || action == actionSkip
}

func collectPackages(events []event) []*packageRun {
	var order []*packageRun
	byName := map[string]*packageRun{}
	for _, e := range events {
		p, seen := byName[e.Package]
		if !seen {
			p = &packageRun{name: e.Package, start: e.Time, paused: map[string]bool{}}
			byName[e.Package] = p
			order = append(order, p)
		}
		switch {
		case e.Test == "" && isResult(e.Action):
			p.result, p.wall, p.end = e.Action, e.Elapsed, e.Time
		case e.Test == "" || strings.Contains(e.Test, subtestSeparator):
		case e.Action == actionPause:
			p.paused[e.Test] = true
		case isResult(e.Action):
			p.tests = append(p.tests, testRun{name: e.Test, seconds: e.Elapsed, parallel: p.paused[e.Test]})
		}
	}
	return order
}

func (p *packageRun) testPath() (PathStep, PathStep) {
	var serialSum float64
	var slowSerial, slowParallel testRun
	for _, t := range p.tests {
		if t.parallel {
			if t.seconds >= slowParallel.seconds {
				slowParallel = t
			}
			continue
		}
		serialSum += t.seconds
		if t.seconds >= slowSerial.seconds {
			slowSerial = t
		}
	}
	if serialSum >= slowParallel.seconds {
		return PathStep{Level: levelTestMode, Unit: unitSerial, Seconds: serialSum}, testStep(slowSerial)
	}
	return PathStep{Level: levelTestMode, Unit: unitParallel, Seconds: slowParallel.seconds}, testStep(slowParallel)
}

func testStep(t testRun) PathStep {
	return PathStep{Level: levelTest, Unit: t.name, Seconds: t.seconds}
}

func modulePrefix(records []rusageRecord, runs []*packageRun) string {
	votes := map[string]int{}
	for _, r := range records {
		for _, p := range runs {
			if prefix, ok := strings.CutSuffix(p.name, packagePathSeparator+r.Package); ok {
				votes[prefix]++
			}
		}
	}
	best, count := "", 0
	for prefix, n := range votes {
		if n > count || n == count && prefix < best {
			best, count = prefix, n
		}
	}
	return best
}

func joinRusage(records []rusageRecord, runs []*packageRun) map[string]*Rusage {
	prefix := modulePrefix(records, runs)
	joined := map[string]*Rusage{}
	for _, r := range records {
		usage := r.Rusage
		joined[prefix+packagePathSeparator+r.Package] = &usage
	}
	return joined
}

func summarize(samples []sample, from, to time.Time) *Sampler {
	var s *Sampler
	for _, x := range samples {
		if x.Ts.Before(from) || x.Ts.After(to) {
			continue
		}
		if s == nil {
			s = &Sampler{cpuBusyMin: x.CPUBusyPct}
		}
		s.Samples++
		s.CPUBusyMaxPct = max(s.CPUBusyMaxPct, x.CPUBusyPct)
		s.cpuBusyMin = min(s.cpuBusyMin, x.CPUBusyPct)
		s.SwapUsedMaxMB = max(s.SwapUsedMaxMB, x.SwapUsedMB)
	}
	return s
}

func limitOf(r *Rusage, s *Sampler) string {
	if r == nil {
		return limitUnknown
	}
	cpu := r.UserS + r.SystemS
	switch {
	case s != nil && s.SwapUsedMaxMB > swapLimitMB:
		return limitMemory
	case s != nil && s.cpuBusyMin > saturatedBusyPct:
		return limitSaturated
	case cpu >= syscallRuleMinCPUS && r.SystemS > r.UserS:
		return limitSyscalls
	case cpu < waitEfficiencyFloor*r.WallS:
		return limitWait
	}
	return limitCPU
}

func withPackages(rep Report, bound artifacts) (Report, error) {
	events, err := decodeLines[event](bound.testJSON)
	if err != nil {
		return Report{}, err
	}
	records, err := decodeLines[rusageRecord](bound.rusage)
	if err != nil {
		return Report{}, err
	}
	samples, err := decodeLines[sample](bound.samples)
	if err != nil {
		return Report{}, err
	}
	runs := collectPackages(events)
	usage := joinRusage(records, runs)
	var finished []*packageRun
	for _, p := range runs {
		if p.result == "" {
			rep.Incomplete = append(rep.Incomplete, p.name)
			continue
		}
		finished = append(finished, p)
		sampler := summarize(samples, p.start, p.end)
		rep.Packages = append(rep.Packages, Package{
			Package: p.name, Result: p.result, WallS: p.wall,
			Limit: limitOf(usage[p.name], sampler), Rusage: usage[p.name], Sampler: sampler,
		})
	}
	slices.SortStableFunc(rep.Packages, func(a, b Package) int {
		return cmp.Or(-cmp.Compare(a.WallS, b.WallS), cmp.Compare(a.Package, b.Package))
	})
	if len(finished) == 0 {
		return rep, nil
	}
	slowest := slices.MaxFunc(finished, func(a, b *packageRun) int { return cmp.Compare(a.wall, b.wall) })
	rep.CriticalPath = append(rep.CriticalPath, PathStep{Level: levelPackage, Unit: slowest.name, Seconds: slowest.wall})
	if len(slowest.tests) == 0 {
		return rep, nil
	}
	mode, test := slowest.testPath()
	rep.CriticalPath = append(rep.CriticalPath, mode, test)
	return rep, nil
}
