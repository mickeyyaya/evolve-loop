package core

// build_persona_budget_check.go — the in-lane half of the persona line-budget
// gate (inbox item `persona-budget-inlane-gate`; third instance of the
// "per-cycle-gate ≠ repo-wide-gate" class after warnship-apicover-ci-gap and
// acs-predicate-compile-gate-at-build-exit).
//
// The gap: `changedPackageFloorChecks` derives its test set from
// `changedGoTestPackages`, which keeps ONLY paths matching `go/**.go`. A lane
// that grows `agents/evolve-*.md` past the 751-line budget pinned by
// go/internal/prompts' TestPersonaStopCriterionDedupe_CombinedLineCountReduced
// therefore yields ZERO packages, hits that engine's `len(pkgs) == 0 → nil`
// early return, and hands off green — the breach lands on main's CI after
// ship, reddening the build for every concurrent lane on the branch (observed
// twice on 2026-07-23). The post-ship visibility half was already closed by
// adding `agents/**` to the go CI path filter (0a732192); this closes the
// pre-handoff half.
//
// Shape follows RemovalClaimFailures deliberately: composed OUTSIDE
// changedPackageFloorChecks in DefaultBuildFloorChecks, for exactly the reason
// that file's comment already documents — a check that must run regardless of
// what changedGoTestPackages derives cannot live behind its early return.
//
// The gate is the CONJUNCTION (a persona doc changed AND the prompts package is
// red), never path-presence alone: touching a persona doc is not itself a
// violation, and a floor that rejected on the path would false-block every
// legitimate persona edit. Fail-open on plumbing (no worktree, no changed
// paths) per this floor's documented policy — downstream gates stay armed.

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

const (
	// personaDocPrefix + personaDocSuffix match the persona docs the
	// internal/prompts budget test reads off disk (agents/evolve-{scout,
	// builder,auditor}.md today; the glob covers any future sibling).
	personaDocPrefix = "agents/evolve-"
	personaDocSuffix = ".md"

	// personaBudgetPkg is the go-module package pattern whose tests own the
	// persona budget. The literal doubles as the operator-facing marker in the
	// failure line: a reject must say WHICH gate fired.
	personaBudgetPkg = "./internal/prompts"
)

// personaDocTouched reports whether any changed repo path is a persona doc.
// Paths are repo-relative and slash-separated (git's `--name-only` output).
func personaDocTouched(paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, personaDocPrefix) && strings.HasSuffix(p, personaDocSuffix) {
			return true
		}
	}
	return false
}

// personaBudgetFailures runs the persona line-budget package for a lane whose
// diff touches a persona doc, and returns one failure line when it is red.
//
// Skips when changedGoTestPackages already derived the same package (the diff
// touched go/internal/prompts/*.go too): changedPackageFloorChecks runs it in
// that case, and this floor's standing rule is one `go test` pass per package
// per handoff.
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
