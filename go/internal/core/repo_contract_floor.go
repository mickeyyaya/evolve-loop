package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type RepoContractPackFn func(ctx context.Context, root string) (reds []string, err error)

func RepoContractFloorChecks(run RepoContractPackFn) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" || repoContractGateOff(in) {
			return nil
		}
		reds, err := run(ctx, in.Worktree)
		if err == nil {
			return nil
		}
		if len(reds) == 0 {
			fmt.Fprintf(os.Stderr, "[build-floor] WARN repo-contract scanner pack exited nonzero naming no test (%v); ship's gate runs the pack again before it pushes\n", err)
			return nil
		}
		return []string{fmt.Sprintf("repo-contract scanner pack RED: %s — ship runs these repo-wide suites before it pushes and refuses a red one. Reproduce each from go/ with `go test -count=1 <package>` (for a named test add `-run '^<Test>$'`) and fix the change before handoff", strings.Join(reds, ", "))}
	}
}

func repoContractGateOff(in ReviewInput) bool {
	root := in.ProjectRoot
	if root == "" {
		root = in.Worktree
	}
	p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN repo-contract floor could not read the policy (%v); it runs the pack as enforce\n", err)
	}
	return p.GatesConfig().RepoContractGate == "off"
}
