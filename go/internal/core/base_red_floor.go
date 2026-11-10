package core

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBuildBaseRedAttributed signalcenter.Code = "BUILD_BASE_RED_ATTRIBUTED"
	CodeBuildBaseRedUnjudged   signalcenter.Code = "BUILD_BASE_RED_UNJUDGED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildBaseRedAttributed, "a build floor red was rerun on the lane base and every red test is red there too, so the floor does not refuse the lane; fields.keys (package.Test of each base-red test), main_red (the keys that are red on main too), took")
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildBaseRedUnjudged, "a build floor red could not be rerun on the lane base, so the lane is blamed as before; fields.keys (package.Test of each red test), cause")
}

type FloorJudge func(ctx context.Context, in ReviewInput) ([]string, error)

type MainRedSink func(projectRoot string, v baseattr.Verdict) (opened bool, err error)

func FailOpen(name string, judge FloorJudge) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		lines, err := judge(ctx, in)
		if err != nil {
			fmt.Fprintf(in.errOut(), "[build-floor] WARN %s %v\n", name, err)
			return nil
		}
		return lines
	}
}

func floorPaths(ctx context.Context, in ReviewInput) ([]string, error) {
	if in.Worktree == "" {
		return nil, nil
	}
	paths, err := worktreePathsSince(ctx, in.Worktree, floorBase(in))
	if err != nil {
		return nil, fmt.Errorf("the changed paths of %s cannot be read: %w", in.Worktree, err)
	}
	return paths, nil
}

func UnitTestFloorJudge(ctx context.Context, in ReviewInput) ([]string, error) {
	paths, err := floorPaths(ctx, in)
	if err != nil {
		return nil, err
	}
	return changedPackageFloorChecks(ctx, in, paths), nil
}

func CoverChangedFloorJudge(ctx context.Context, in ReviewInput) ([]string, error) {
	paths, err := floorPaths(ctx, in)
	if err != nil {
		return nil, err
	}
	return coverChangedFailures(ctx, in, paths), nil
}

func (in ReviewInput) attributor() baseattr.Attributor {
	a := BaseRedAttributor(cmp.Or(in.ProjectRoot, in.Worktree), in.baseRunner(), in.errOut())
	a.Serialize = func(context.Context) func() { return in.acquireFloorRetakeLock() }
	return a
}

func BaseRedAttributor(root string, run baseattr.Runner, warn io.Writer) baseattr.Attributor {
	gates := policy.LoadOrDefaults(filepath.Join(root, ".evolve", "policy.json"), warn).GatesConfig()
	return baseattr.New(gitexec.Git{Dir: root, Exec: gitRunner}, run, gates.BaseRedRerun, nil)
}

func (in ReviewInput) baseRunner() baseattr.Runner {
	if in.baseRun != nil {
		return in.baseRun
	}
	return baseattr.GoTestRunner(ipcenv.GoTestEnv())
}

func (in ReviewInput) laneReds(ctx context.Context, reds baseattr.Reds) (baseattr.Reds, error) {
	v, err := in.attributor().Attribute(ctx, floorBase(in), reds)
	switch {
	case err != nil:
		in.reportUnjudged(reds, err)
		return reds, err
	case v.Blamed():
		return baseattr.Reds{Keys: v.LaneOnly, Unnamed: reds.Unnamed || len(v.LaneOnly) == 0, Flags: reds.Flags}, nil
	}
	in.reportAttributed(v)
	return baseattr.Reds{}, nil
}

type ruling struct {
	lane  baseattr.Reds
	info  []baseattr.Key
	cause error
}

func (in ReviewInput) rule(ctx context.Context, reds baseattr.Reds) ruling {
	lane, err := in.laneReds(ctx, reds)
	var info []baseattr.Key
	for _, k := range reds.Keys {
		if !slices.Contains(lane.Keys, k) {
			info = append(info, k)
		}
	}
	return ruling{lane: lane, info: info, cause: err}
}

func (r ruling) clear() bool {
	return !r.lane.Unnamed && len(r.lane.Keys) == 0
}

func (r ruling) notes() []string {
	var out []string
	if r.cause != nil {
		out = append(out, fmt.Sprintf("base-red attribution could not run (%v), so the lane is blamed for each red", r.cause))
	}
	if len(r.info) > 0 {
		out = append(out, "information: red on the base too, not refused: "+baseattr.Names(r.info))
	}
	return out
}

func (r ruling) of(reds baseattr.Reds) ruling {
	if !reds.Attributable() {
		return ruling{lane: baseattr.Reds{Keys: reds.Keys, Unnamed: true}}
	}
	part := ruling{info: r.info, cause: r.cause}
	for _, k := range reds.Keys {
		if slices.Contains(r.lane.Keys, k) {
			part.lane.Keys = append(part.lane.Keys, k)
		}
	}
	return part
}

func (in ReviewInput) laneFailures(ctx context.Context, fails []selfCheckFailure) []selfCheckFailure {
	parsed := make([]baseattr.Reds, len(fails))
	var asked baseattr.Reds
	for i, f := range fails {
		parsed[i] = baseattr.ParseGoTest(f.Output)
		if parsed[i].Attributable() {
			asked.Keys = append(asked.Keys, parsed[i].Keys...)
		}
	}
	var all ruling
	if asked.Attributable() {
		all = in.rule(ctx, asked)
	}
	var kept []selfCheckFailure
	for i, f := range fails {
		part := all.of(parsed[i])
		if part.clear() {
			continue
		}
		f.Output = strings.Join(append([]string{f.Output}, part.notes()...), "\n")
		kept = append(kept, f)
	}
	return kept
}

func (in ReviewInput) coverRedLines(ctx context.Context, out string, flags []string) []string {
	reds := baseattr.ParseGoTest(out)
	reds.Flags = flags
	r := in.rule(ctx, reds)
	if r.clear() {
		return nil
	}
	return append([]string{floorFailureDiagnostic(out)}, r.notes()...)
}

func (in ReviewInput) reportAttributed(v baseattr.Verdict) {
	keys := baseattr.Names(v.BaseRed)
	reason := fmt.Sprintf("build floor: each red test is red on the base %s too, so the floor does not refuse it: %s (attributed in %s)", v.Base, keys, v.Took)
	fmt.Fprintf(in.errOut(), "[build-floor] NOTE %s\n", reason)
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.reportAttributed", Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityInfo, Code: CodeBuildBaseRedAttributed, Reason: reason,
		Fields: map[string]string{"keys": keys, "main_red": baseattr.Names(v.MainRed), "took": v.Took.String()},
	})
	in.openMainRedStop(v)
}

func (in ReviewInput) openMainRedStop(v baseattr.Verdict) {
	opening := stopOpening{projectRoot: in.ProjectRoot, verdict: v, cycle: in.Cycle, runID: in.RunID, phase: string(PhaseBuild), origin: "ReviewInput.openMainRedStop"}
	if err := opening.through(in.mainRedSink(), in.Signals); err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN the main-red stop was not recorded: %v\n", err)
	}
}

func (in ReviewInput) mainRedSink() MainRedSink {
	if in.MainRedStop != nil {
		return in.MainRedStop
	}
	return OpenMainRedStop
}

func (in ReviewInput) reportUnjudged(reds baseattr.Reds, cause error) {
	keys := baseattr.Names(reds.Keys)
	reason := fmt.Sprintf("build floor: the red test(s) %s could not be rerun on the base (%v), so the lane is blamed", keys, cause)
	fmt.Fprintf(in.errOut(), "[build-floor] WARN %s\n", reason)
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.reportUnjudged", Kind: signalcenter.KindGateRejected,
		Severity: signalcenter.SeverityWarn, Code: CodeBuildBaseRedUnjudged, Reason: reason,
		Fields: map[string]string{"keys": keys, "cause": cause.Error()},
	})
}
