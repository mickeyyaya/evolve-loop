package qualitypanel

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type RevertOutcome string

const (
	RevertTestsPassed RevertOutcome = "tests_passed"
	RevertTestsFailed RevertOutcome = "tests_failed"
	RevertNotCompiled RevertOutcome = "not_compiled"
)

type Hunk struct {
	Package string
	File    string
	Patch   string
}

type RevertRunner interface {
	Revert(scratchDir string, h Hunk) (RevertOutcome, error)
}

type RevertSkip struct {
	Hunk   Hunk
	Reason string
}

type RevertInput struct {
	Hunks      []Hunk
	PlaneRoot  string
	MainRoot   string
	ScratchDir string
	Window     string
	Source     string
	Cycle      int
}

type RevertResult struct {
	TQ2               Reading
	Sensitive         int
	Insensitive       int
	InsensitiveHunks  []Hunk
	UncompilableHunks []Hunk
	Excluded          []RevertSkip
}

func MeasureRevertSensitivity(in RevertInput, runner RevertRunner) (RevertResult, error) {
	if err := checkScratchDirs("revert", in.ScratchDir, in.PlaneRoot, in.MainRoot); err != nil {
		return RevertResult{}, err
	}
	var res RevertResult
	scratch := filepath.Clean(in.ScratchDir)
	for _, h := range in.Hunks {
		if guard := killGuardPathOf(h.File); guard != "" {
			res.Excluded = append(res.Excluded, RevertSkip{Hunk: h, Reason: "the path " + guard + " is a kill guard"})
			continue
		}
		outcome, err := runner.Revert(scratch, h)
		if err != nil {
			return RevertResult{}, fmt.Errorf("revert runner: %w", err)
		}
		if err := res.record(h, outcome); err != nil {
			return RevertResult{}, err
		}
	}
	n := res.Sensitive + res.Insensitive
	res.TQ2 = Reading{
		Index: "TQ2", Interval: Wilson(res.Sensitive, n), Method: MethodWilson95, N: n,
		Window: in.Window, Source: in.Source, Cycle: in.Cycle,
		Extra: map[string]string{"uncompilable": strconv.Itoa(len(res.UncompilableHunks))},
	}
	return res, nil
}

func (r *RevertResult) record(h Hunk, outcome RevertOutcome) error {
	switch outcome {
	case RevertTestsFailed:
		r.Sensitive++
	case RevertNotCompiled:
		r.Sensitive++
		r.UncompilableHunks = append(r.UncompilableHunks, h)
	case RevertTestsPassed:
		r.Insensitive++
		r.InsensitiveHunks = append(r.InsensitiveHunks, h)
	default:
		return fmt.Errorf("revert outcome %q of %s is not tests_passed, tests_failed or not_compiled", outcome, h.File)
	}
	return nil
}

func (r RevertResult) Events(stage Stage) []signalcenter.Event {
	return Events(r.TQ2, stage)
}
