package qualitypanel

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type MutantStatus string

const (
	MutantKilled     MutantStatus = "killed"
	MutantLived      MutantStatus = "lived"
	MutantNotCovered MutantStatus = "not_covered"
	MutantTimedOut   MutantStatus = "timed_out"
	MutantNotViable  MutantStatus = "not_viable"
)

type Mutant struct {
	ID       string
	Status   MutantStatus
	KilledBy []string
}

type MutationReport struct {
	Mutants     []Mutant
	TestSeconds map[string]float64
}

type MutationTarget struct {
	Package   string
	File      string
	Name      string
	StartLine int
	EndLine   int
}

type ChangedFunction struct {
	Package string
	File    string
	Name    string
	Source  []byte
}

type MutationRunner interface {
	Run(scratchDir string, targets []MutationTarget) (MutationReport, error)
}

type MutationSkip struct {
	Function ChangedFunction
	Reason   string
}

type MutationInput struct {
	Functions  []ChangedFunction
	PlaneRoot  string
	MainRoot   string
	ScratchDir string
	Window     string
	Source     string
	Cycle      int
}

type MutationResult struct {
	TQ1             Reading
	TQ3             Reading
	Killed          int
	Lived           int
	NotCovered      int
	ZeroUniqueTests []string
	Excluded        []MutationSkip
	Refused         []MutationSkip
}

func MeasureMutation(in MutationInput, runner MutationRunner) (MutationResult, error) {
	if err := checkScratch(in); err != nil {
		return MutationResult{}, err
	}
	plan := planMutation(in.Functions)
	var report MutationReport
	if len(plan.targets) > 0 {
		var err error
		if report, err = runner.Run(filepath.Clean(in.ScratchDir), plan.targets); err != nil {
			return MutationResult{}, fmt.Errorf("mutation runner: %w", err)
		}
	}
	res := scoreMutation(report)
	res.Excluded, res.Refused = plan.excluded, plan.refused
	res.TQ1 = mutationReading("TQ1", res.TQ1, in)
	res.TQ3 = mutationReading("TQ3", res.TQ3, in)
	res.TQ1.Extra = map[string]string{"not_covered": strconv.Itoa(res.NotCovered)}
	return res, nil
}

func (r MutationResult) Events(stage Stage) []signalcenter.Event {
	return append(Events(r.TQ1, stage), Events(r.TQ3, stage)...)
}

func mutationReading(index string, r Reading, in MutationInput) Reading {
	r.Index, r.Window, r.Source, r.Cycle = index, in.Window, in.Source, in.Cycle
	return r
}

func checkScratch(in MutationInput) error {
	for label, path := range map[string]string{"scratch": in.ScratchDir, "plane": in.PlaneRoot, "main": in.MainRoot} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("mutation %s path %q is not absolute", label, path)
		}
	}
	scratch := filepath.Clean(in.ScratchDir)
	for label, root := range map[string]string{"plane": in.PlaneRoot, "main": in.MainRoot} {
		if within(filepath.Clean(root), scratch) {
			return fmt.Errorf("mutation scratch %q is inside the %s tree %q", scratch, label, root)
		}
	}
	return nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
