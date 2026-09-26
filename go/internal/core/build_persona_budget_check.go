package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

const (
	personaDocPrefix = "agents/evolve-"
	personaDocSuffix = ".md"

	// personaBudgetPkg also names the gate in the failure line.
	personaBudgetPkg = "./internal/prompts"
)

// personaDocTouched takes repo-relative, slash-separated paths.
func personaDocTouched(paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, personaDocPrefix) && strings.HasSuffix(p, personaDocSuffix) {
			return true
		}
	}
	return false
}

// personaBudgetFailures fails only on the conjunction of a touched persona doc
// and a red prompts package; touching a persona doc is not itself a violation.
// It skips when changedPackageFloorChecks already runs the package, so each
// package gets one `go test` pass per handoff.
func personaBudgetFailures(ctx context.Context, worktree string, paths []string) []string {
	if worktree == "" || !personaDocTouched(paths) {
		return nil
	}
	for _, pkg := range changedGoTestPackages(paths) {
		if pkg == personaBudgetPkg {
			return nil
		}
	}
	out, passed := buildSelfCheckRunner(ctx, codequality.ModuleDir(worktree), personaBudgetPkg)
	if passed {
		return nil
	}
	head := out
	if len(head) > 400 {
		head = head[:400] + "…"
	}
	return []string{fmt.Sprintf(
		"%s: persona line-budget tests FAIL for a lane that changed agents/evolve-*.md — fix the persona docs before handoff (CI on main would RED for every concurrent lane):\n%s",
		personaBudgetPkg, head)}
}
