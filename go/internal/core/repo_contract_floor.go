package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const repoContractFloorDeadline = 120 * time.Second

type RepoContractPackFn func(ctx context.Context, root string) (reds []string, diagnostic string, err error)

func RepoContractFloorChecks(run RepoContractPackFn) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" {
			return nil
		}
		runs, note := repocontract.PackRuns(repoContractGate(in), in.Worktree)
		if note != "" {
			fmt.Fprintf(os.Stderr, "[build-floor] repo-contract floor: %s\n", note)
		}
		if !runs {
			return nil
		}
		runCtx, cancel := context.WithTimeout(ctx, repoContractFloorDeadline)
		defer cancel()
		reds, diagnostic, err := run(runCtx, in.Worktree)
		if len(reds) == 0 {
			if err != nil {
				fmt.Fprintf(os.Stderr, "[build-floor] WARN repo-contract scanner pack exited nonzero naming no test (%v; the floor's deadline is %s); ship's gate runs the pack again before it pushes:\n%s\n", err, repoContractFloorDeadline, floorFailureDiagnostic(diagnostic))
			}
			return nil
		}
		return []string{fmt.Sprintf("repo-contract scanner pack RED: %s — ship runs these repo-wide suites before it pushes and refuses a red one. Reproduce each from go/ with `go test -count=1 <package>` (for a named test add `-run '^<Test>$'`) and fix the change before handoff:\n%s", strings.Join(reds, ", "), floorFailureDiagnostic(diagnostic))}
	}
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
