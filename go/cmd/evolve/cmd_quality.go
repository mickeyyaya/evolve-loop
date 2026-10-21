package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeshape"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	qualityUsage      = "usage: evolve quality report --scope wave|tree|judges|replay --project-root P | evolve quality compare --index PQ1|PQ4 --a WINDOW --b WINDOW [--paired] --project-root P (WINDOW: wave:N or waves:A-B)"
	qualityPolicyPath = ".evolve/" + paths.PolicyFile
)

type qualityReader struct {
	index string
	read  func(root string) ([]qualitypanel.Reading, error)
}

type qualityEnv struct {
	scopes    map[string][]qualityReader
	newCenter func(root, evolveDir string, console io.Writer) (*signalcenter.Center, func())
}

var qualityScopes = map[string][]qualityReader{"wave": waveQualityReaders(), "tree": treeQualityReaders(), "judges": nil, "replay": nil}

func waveQualityReaders() []qualityReader {
	return []qualityReader{
		pipelineReader("PQ1", func(cs []pipelinerates.Cycle, _ string) ([]qualitypanel.Reading, error) {
			return pipelinerates.ShipRate(cs), nil
		}),
		pipelineReader("PQ4", func(cs []pipelinerates.Cycle, _ string) ([]qualitypanel.Reading, error) {
			return pipelinerates.DeadlineRate(cs), nil
		}),
		pipelineReader("CQ2", func(cs []pipelinerates.Cycle, root string) ([]qualitypanel.Reading, error) {
			return pipelinerates.PackagesPerShip(cs, pipelinerates.GitShipPaths(root))
		}),
		shapeReader("CQ1", func(t codeshape.Tree, _ string) ([]qualitypanel.Reading, error) {
			return []qualitypanel.Reading{codeshape.TreeConformance(t)}, nil
		}),
		pipelineReader("CQ1", func(cs []pipelinerates.Cycle, root string) ([]qualitypanel.Reading, error) {
			return codeshape.ChangedConformance(root, cs)
		}),
	}
}

func treeQualityReaders() []qualityReader {
	return []qualityReader{
		shapeReader("CQ4", func(t codeshape.Tree, root string) ([]qualitypanel.Reading, error) {
			readings := []qualitypanel.Reading{codeshape.InterfaceDepth(t), codeshape.TreeDuplication(t)}
			cycles, err := pipelinerates.LoadCycles(root)
			if err != nil {
				return readings, err
			}
			r, err := codeshape.DiffDuplication(root, t, cycles)
			if err != nil {
				return readings, err
			}
			return append(readings, r), nil
		}),
		{index: "CQ5", read: func(root string) ([]qualitypanel.Reading, error) {
			lines, files, err := codeshape.RuleLines(root)
			return []qualitypanel.Reading{codeshape.JudgmentSurface(lines, files)}, err
		}},
	}
}

func shapeReader(index string, read func(codeshape.Tree, string) ([]qualitypanel.Reading, error)) qualityReader {
	return qualityReader{index: index, read: func(root string) ([]qualitypanel.Reading, error) {
		tree, err := codeshape.Scan(codeshape.GoRoot(root))
		if err != nil {
			return nil, err
		}
		return read(tree, root)
	}}
}

func pipelineReader(index string, read func([]pipelinerates.Cycle, string) ([]qualitypanel.Reading, error)) qualityReader {
	return qualityReader{index: index, read: func(root string) ([]qualitypanel.Reading, error) {
		cycles, err := pipelinerates.LoadCycles(root)
		if err != nil {
			return nil, err
		}
		return read(cycles, root)
	}}
}

func runQuality(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runQualityWith(qualityEnv{scopes: qualityScopes, newCenter: newQualityCenter}, args, stdout, stderr)
}

func newQualityCenter(root, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	return newRoleSignalCenter("quality", root, evolveDir, console)
}

func runQualityWith(env qualityEnv, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "compare" {
		return runQualityCompare(env, args[1:], stdout, stderr)
	}
	scope, root, err := parseQualityArgs(args, env.scopes)
	if err != nil {
		fmt.Fprintf(stderr, "quality: %v\n%s\n", err, qualityUsage)
		return exitUsage
	}
	readers := env.scopes[scope]
	if len(readers) == 0 {
		fmt.Fprintf(stdout, "quality report: scope %s has no index readers yet (ADR-0153 components Q2 to Q10 add them)\n", scope)
	}
	center, closeCenter := env.newCenter(root, paths.EvolveDirOf(root), stderr)
	defer closeCenter()
	t := collectQuality(center, root, readers)
	fmt.Fprintf(stdout, "quality report: scope %s: %d reading(s), %d breach(es), %d warning(s)\n", scope, t.readings, t.breaches, t.warnings)
	return 0
}

func parseQualityArgs(args []string, scopes map[string][]qualityReader) (string, string, error) {
	if len(args) == 0 || args[0] != "report" {
		return "", "", errors.New("want the report verb")
	}
	var scope, root string
	operands, err := (cliFlags{values: map[string]*string{"--scope": &scope, "--project-root": &root}}).parse(args[1:])
	switch {
	case err != nil:
		return "", "", err
	case len(operands) > 0:
		return "", "", fmt.Errorf("unexpected argument %q", operands[0])
	case root == "":
		return "", "", errors.New("--project-root is required")
	}
	if _, ok := scopes[scope]; !ok {
		return "", "", fmt.Errorf("scope %q is not wave, tree, judges or replay", scope)
	}
	return scope, paths.AbsoluteRoot("--project-root", root, nil), nil
}

type qualityTally struct {
	center                       *signalcenter.Center
	readings, breaches, warnings int
}

func collectQuality(center *signalcenter.Center, root string, readers []qualityReader) qualityTally {
	t := qualityTally{center: center}
	stages := t.stages(root)
	for _, rd := range readers {
		readings, err := rd.read(root)
		if err != nil {
			t.emit(readerWarning(rd.index, err))
		}
		t.publish(readings, stages)
	}
	return t
}

func (t *qualityTally) stages(root string) map[string]qualitypanel.Stage {
	stages, err := policy.QualityStagesFor(root)
	if err != nil {
		t.emit(qualitypanel.SourceWarning("", qualitypanel.CodeSourceUnreadable, qualityPolicyPath, err.Error()))
	}
	return stages
}

func (t *qualityTally) publish(readings []qualitypanel.Reading, stages map[string]qualitypanel.Stage) {
	for _, r := range readings {
		for _, e := range qualitypanel.Events(r, qualitypanel.StageFor(r.Index, stages)) {
			t.emit(e)
		}
	}
}

func (t *qualityTally) emit(e signalcenter.Event) {
	switch e.Kind {
	case signalcenter.KindQualityReading:
		t.readings++
	case signalcenter.KindQualityBreach:
		t.breaches++
	default:
		t.warnings++
	}
	t.center.Emit(e)
}

func readerWarning(index string, err error) signalcenter.Event {
	var pathErr *fs.PathError
	if errors.Is(err, fs.ErrNotExist) && errors.As(err, &pathErr) {
		return qualitypanel.SourceWarning(index, qualitypanel.CodeSourceAbsent, pathErr.Path, err.Error())
	}
	return qualitypanel.SourceWarning(index, qualitypanel.CodeSourceUnreadable, "", err.Error())
}
