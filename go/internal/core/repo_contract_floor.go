package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const repoContractFloorDeadline = 120 * time.Second

type RepoContractPackFn func(ctx context.Context, projectRoot, root string) (reds []string, diagnostic string, err error)

func RepoContractFloorChecks(run RepoContractPackFn) BuildFloorCheckFn {
	return FailOpen("repo-contract", RepoContractFloorJudge(run))
}

func RepoContractFloorJudge(run RepoContractPackFn) FloorJudge {
	return func(ctx context.Context, in ReviewInput) ([]string, error) {
		if in.Worktree == "" {
			return nil, nil
		}
		runs, note := repocontract.PackRuns(repoContractGate(in), in.Worktree)
		if note != "" {
			fmt.Fprintf(in.errOut(), "[build-floor] repo-contract floor: %s\n", note)
		}
		if !runs {
			return nil, nil
		}
		runCtx, cancel := context.WithTimeout(ctx, repoContractFloorDeadline)
		defer cancel()
		reds, diagnostic, err := run(runCtx, in.ProjectRoot, in.Worktree)
		if len(reds) == 0 {
			return nil, unnamedPackExit(err, diagnostic)
		}
		return in.repoContractLines(ctx, reds, diagnostic), nil
	}
}

func unnamedPackExit(err error, diagnostic string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("scanner pack exited nonzero naming no test (%v; the floor's deadline is %s); ship's gate runs the pack again before it pushes:\n%s", err, repoContractFloorDeadline, floorFailureDiagnostic(diagnostic))
}

func (in ReviewInput) repoContractLines(ctx context.Context, reds []string, diagnostic string) []string {
	r := in.rule(ctx, baseattr.KeysOf(reds))
	if r.clear() {
		return nil
	}
	named := reds
	if !r.lane.Unnamed {
		named = laneNames(reds, r.lane.Keys)
	}
	head := fmt.Sprintf("repo-contract scanner pack RED: %s — ship runs these repo-wide suites before it pushes and refuses a red one. Reproduce each from go/ with `go test -count=1 <package>` (for a named test add `-run '^<Test>$'`) and fix the change before handoff:\n%s", strings.Join(named, ", "), floorFailureDiagnostic(diagnostic))
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

func repoContractGate(in ReviewInput) string {
	root := in.ProjectRoot
	if root == "" {
		root = in.Worktree
	}
	p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN repo-contract floor could not read the policy (%v); it runs the pack as enforce\n", err)
	}
	return p.GatesConfig().RepoContractGate
}
