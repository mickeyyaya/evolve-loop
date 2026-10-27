// Package citelemetry joins the jobs, steps, test JSON, rusage and sampler records of one CI run into a critical-path report.
// See docs/architecture/packages/internal-citelemetry.md.
package citelemetry

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	levelWorkflowJob = "L5"
	levelStep        = "L4"
	levelPackage     = "L3"
	levelTestMode    = "L2"
	levelTest        = "L1"
	macosJobMark     = "macOS"
	ubuntuJobMark    = "ubuntu"
	testStepPrefix   = "test"
)

var (
	errNoJobs     = errors.New("the jobs document has no job")
	errUnknownJob = errors.New("no job has the name of an artifact binding")
)

type G1 struct {
	MacosJobS       float64 `json:"macos_job_s"`
	UbuntuTestStepS float64 `json:"ubuntu_test_step_s"`
}

type PathStep struct {
	Level   string  `json:"level"`
	Unit    string  `json:"unit"`
	Seconds float64 `json:"seconds"`
}

type Report struct {
	G1           G1         `json:"g1"`
	CriticalPath []PathStep `json:"critical_path"`
	Packages     []Package  `json:"packages"`
	Incomplete   []string   `json:"incomplete"`
}

type jobStep struct {
	name    string
	seconds float64
}

type job struct {
	name    string
	seconds float64
	steps   []jobStep
}

type artifacts struct {
	testJSON []byte
	rusage   []byte
	samples  []byte
}

type input struct {
	jobs      []job
	artifacts map[string]artifacts
}

func build(in input) (Report, error) {
	if err := validate(in); err != nil {
		return Report{}, err
	}
	rep := Report{G1: g1Of(in.jobs)}
	critical := criticalJob(in.jobs)
	rep.CriticalPath = []PathStep{{Level: levelWorkflowJob, Unit: critical.name, Seconds: critical.seconds}}
	step, ok := longestStep(critical)
	if !ok {
		return rep, nil
	}
	rep.CriticalPath = append(rep.CriticalPath, PathStep{Level: levelStep, Unit: step.name, Seconds: step.seconds})
	bound, ok := in.artifacts[critical.name]
	if !ok {
		return rep, nil
	}
	return withPackages(rep, bound)
}

func validate(in input) error {
	if len(in.jobs) == 0 {
		return errNoJobs
	}
	names := map[string]bool{}
	for _, j := range in.jobs {
		names[j.name] = true
	}
	for _, bound := range slices.Sorted(maps.Keys(in.artifacts)) {
		if !names[bound] {
			return fmt.Errorf("%w: %q", errUnknownJob, bound)
		}
	}
	return nil
}

func criticalJob(jobs []job) job {
	return slices.MaxFunc(jobs, func(a, b job) int { return cmp.Compare(a.seconds, b.seconds) })
}

func longestStep(j job) (jobStep, bool) {
	if len(j.steps) == 0 {
		return jobStep{}, false
	}
	return slices.MaxFunc(j.steps, func(a, b jobStep) int { return cmp.Compare(a.seconds, b.seconds) }), true
}

func g1Of(jobs []job) G1 {
	var g G1
	for _, j := range jobs {
		if strings.Contains(j.name, macosJobMark) {
			g.MacosJobS = max(g.MacosJobS, j.seconds)
		}
		if !strings.Contains(j.name, ubuntuJobMark) {
			continue
		}
		for _, s := range j.steps {
			if strings.HasPrefix(s.name, testStepPrefix) {
				g.UbuntuTestStepS = max(g.UbuntuTestStepS, s.seconds)
			}
		}
	}
	return g
}
