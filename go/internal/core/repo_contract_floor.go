package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type RepoContractPackFn func(ctx context.Context, moduleDir string) (reds []string, err error)

func RepoContractFloorChecks(run RepoContractPackFn) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" || repoContractGateOff(in.ProjectRoot) {
			return nil
		}
		reds, err := run(ctx, codequality.ModuleDir(in.Worktree))
		if err == nil {
			return nil
		}
		if len(reds) == 0 {
			fmt.Fprintf(os.Stderr, "[build-floor] WARN repo-contract scanner pack exited nonzero naming no test (%v); ship's gate runs the pack again before it pushes\n", err)
			return nil
		}
		return []string{fmt.Sprintf("repo-contract scanner pack RED: %s — ship runs these repo-wide suites before it pushes and refuses a red one. Reproduce each from go/ with `go test -count=1 -run '^<Test>$' <package>` and fix the change before handoff", strings.Join(reds, ", "))}
	}
}

func repoContractGateOff(projectRoot string) bool {
	gate := policy.Policy{}.GatesConfig().RepoContractGate
	if projectRoot != "" {
		if p, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
			gate = p.GatesConfig().RepoContractGate
		}
	}
	return gate == "off"
}
