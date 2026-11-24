package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const repoContractFloorDeadline = 120 * time.Second

type RepoContractPackFn func(ctx context.Context, projectRoot, root string, changed []string) (reds []string, diagnostic string, err error)

func RepoContractFloorChecks(run RepoContractPackFn) BuildFloorCheckFn {
	return FailOpen("repo-contract", RepoContractFloorJudge(run))
}

type RepoContractStage struct {
	Name      string
	Subject   string
	Refusal   string
	FixBefore string
	Rerun     string
}

func BuildFloorStage() RepoContractStage {
	return RepoContractStage{
		Name:      "build-floor",
		Subject:   "floor",
		Refusal:   "ship runs these repo-wide suites before it pushes and refuses a red one",
		FixBefore: "handoff",
		Rerun:     "ship's gate runs the pack again before it pushes",
	}
}

func CommitGateStage() RepoContractStage {
	return RepoContractStage{
		Name:      "commit-gate",
		Subject:   "gate",
		Refusal:   "the commit gate refuses a commit while one is red",
		FixBefore: "the commit",
		Rerun:     "re-run the commit gate once the pack is green or the host is quiet",
	}
}

func RepoContractFloorJudge(run RepoContractPackFn) FloorJudge {
	return RepoContractJudge(BuildFloorStage(), run)
}

func RepoContractJudge(stage RepoContractStage, run RepoContractPackFn) FloorJudge {
	return func(ctx context.Context, in ReviewInput) ([]string, error) {
		if in.Worktree == "" {
			return nil, nil
		}
		runs, note := repocontract.PackRuns(repoContractGate(in, stage), in.Worktree)
		if note != "" {
			fmt.Fprintf(in.errOut(), "[%s] repo-contract %s: %s\n", stage.Name, stage.Subject, note)
		}
		if !runs {
			return nil, nil
		}
		runCtx, cancel := context.WithTimeout(ctx, repoContractFloorDeadline)
		defer cancel()
		started := time.Now()
		reds, diagnostic, err := run(runCtx, in.ProjectRoot, in.Worktree, changedOrUnknown(ctx, in, stage))
		if len(reds) == 0 && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, stage.deadlineExit(runCtx.Err(), diagnostic, time.Since(started))
		}
		if len(reds) == 0 {
			return nil, stage.unnamedPackExit(err, diagnostic, time.Since(started))
		}
		return in.repoContractLines(ctx, stage, reds, diagnostic), nil
	}
}

func changedOrUnknown(ctx context.Context, in ReviewInput, stage RepoContractStage) []string {
	paths, err := floorPaths(ctx, in)
	if err != nil {
		fmt.Fprintf(in.errOut(), "[%s] repo-contract %s: %v; the pack runs the whole by-name set\n", stage.Name, stage.Subject, err)
		return repocontract.UnknownChange()
	}
	return paths
}

type PackExitError struct {
	Stage      RepoContractStage
	Cause      error
	Elapsed    time.Duration
	Diagnostic string
}

func (e *PackExitError) Error() string {
	return fmt.Sprintf("scanner pack exited nonzero naming no test (%v; the pack ran %s; the %s's deadline is %s); %s:\n%s", e.Cause, e.Elapsed.Round(time.Millisecond), e.Stage.Subject, repoContractFloorDeadline, e.Stage.Rerun, floorFailureDiagnostic(e.Diagnostic))
}

func (e *PackExitError) Unwrap() error { return e.Cause }

type PackDeadlineError struct {
	Stage      RepoContractStage
	Cause      error
	Elapsed    time.Duration
	Diagnostic string
}

func (e *PackDeadlineError) Error() string {
	return fmt.Sprintf("scanner pack did not finish inside the %s deadline (%v; the pack ran %s), so no test result is known; %s:\n%s", repoContractFloorDeadline, e.Cause, e.Elapsed.Round(time.Millisecond), e.Stage.Rerun, floorFailureDiagnostic(e.Diagnostic))
}

func (e *PackDeadlineError) Unwrap() error { return e.Cause }

func (stage RepoContractStage) unnamedPackExit(err error, diagnostic string, elapsed time.Duration) error {
	if err == nil {
		return nil
	}
	return &PackExitError{Stage: stage, Cause: err, Elapsed: elapsed, Diagnostic: diagnostic}
}

func (stage RepoContractStage) deadlineExit(cause error, diagnostic string, elapsed time.Duration) error {
	return &PackDeadlineError{Stage: stage, Cause: cause, Elapsed: elapsed, Diagnostic: diagnostic}
}

func (in ReviewInput) repoContractLines(ctx context.Context, stage RepoContractStage, reds []string, diagnostic string) []string {
	r := in.rule(ctx, baseattr.KeysOf(reds))
	if r.clear() {
		return nil
	}
	named := reds
	if !r.lane.Unnamed {
		named = laneNames(reds, r.lane.Keys)
	}
	head := fmt.Sprintf("repo-contract scanner pack RED: %s — %s. Reproduce each from go/ with `go test -count=1 <package>` (for a named test add `-run '^<Test>$'`) and fix the change before %s:\n%s", strings.Join(named, ", "), stage.Refusal, stage.FixBefore, floorFailureDiagnostic(diagnostic))
	return []string{strings.Join(append([]string{head}, r.notes()...), "\n")}
}

func laneNames(names []string, lane []baseattr.Key) []string {
	var out []string
	for _, n := range names {
		if k := baseattr.KeysOf([]string{n}).Keys; len(k) == 1 && slices.Contains(lane, k[0]) {
			out = append(out, n)
		}
	}
	return out
}

func repoContractGate(in ReviewInput, stage RepoContractStage) string {
	root := in.ProjectRoot
	if root == "" {
		root = in.Worktree
	}
	p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(in.errOut(), "[%s] WARN repo-contract %s could not read the policy (%v); it runs the pack as enforce\n", stage.Name, stage.Subject, err)
	}
	return p.GatesConfig().RepoContractGate
}
