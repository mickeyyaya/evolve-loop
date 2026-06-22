package core

// phase_bindings_selfcheck.go — deterministic post-build self-check (the
// false-green backstop). A builder can delete an env read, break a pre-existing
// UNIT test in a changed package, "pass" its own ACS check (which does not run
// that package's unit tests), and hand off a green build-report — the regression
// then only surfaces at audit, two attempts later (cycle-2 / w1-config-singletons
// M1). Running the changed packages' unit tests here, deterministically, records
// ground-truth so the builder's self-report cannot lie and the audit/retro have
// the exact failing tests. UNIT tests only (no -tags integration): the
// env-dependent tmux/REPL integration tests are intentionally excluded so a real
// regression fails the check while a flaky live-launch test does not.
//
// Like build-gofmt and build-derived-regen this is deterministic work that must
// not depend on the LLM builder remembering; best-effort and NEVER aborts —
// audit stays the verdict authority (build's only legal successor is audit).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

// selfCheckFailure is one changed package whose unit tests failed, with the
// captured output for builder/debugger feedback.
type selfCheckFailure struct {
	Pkg    string `json:"pkg"`
	Output string `json:"output"`
}

// buildSelfCheckRunner runs a package's unit tests in moduleDir and reports the
// combined output and whether they passed. A package-level seam so tests drive
// the pass/fail branches without spawning `go test`.
var buildSelfCheckRunner = realGoUnitTest

// changedGoTestPackages maps the cycle's changed repo paths to the unique,
// sorted set of go-module package patterns to unit-test (e.g.
// "go/internal/bridge/x.go" → "./internal/bridge"). Only paths under the go/
// module that end in .go contribute; everything else is skipped.
func changedGoTestPackages(paths []string) []string {
	seen := map[string]bool{}
	var pkgs []string
	for _, p := range paths {
		if !strings.HasPrefix(p, "go/") || !strings.HasSuffix(p, ".go") {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path.Dir(p), "go"), "/")
		pkg := "./" + rel
		if !seen[pkg] {
			seen[pkg] = true
			pkgs = append(pkgs, pkg)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

// runBuildSelfCheck runs each package's unit tests through run and returns only
// the failures. Pure over its inputs (the runner is injected) so the collection
// logic is unit-tested without git or `go test`.
func runBuildSelfCheck(ctx context.Context, moduleDir string, pkgs []string, run func(context.Context, string, string) (string, bool)) []selfCheckFailure {
	var fails []selfCheckFailure
	for _, pkg := range pkgs {
		if out, ok := run(ctx, moduleDir, pkg); !ok {
			fails = append(fails, selfCheckFailure{Pkg: pkg, Output: out})
		}
	}
	return fails
}

// realGoUnitTest runs `go test` (UNIT only — no integration tag) for one package
// in moduleDir. passed == (exit 0), EXCEPT a package whose files are all excluded
// by build tags is "nothing to unit-test" rather than a failure (see
// goTestExcludedByBuildTags). A bounded timeout keeps a wedged test from hanging
// the build phase.
func realGoUnitTest(ctx context.Context, moduleDir, pkg string) (output string, passed bool) {
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-timeout", "120s", pkg)
	cmd.Dir = moduleDir
	out, err := cmd.CombinedOutput()
	s := string(out)
	return s, err == nil || goTestExcludedByBuildTags(s)
}

// goTestExcludedByBuildTags reports whether a non-zero `go test` result is the
// "the package under test has no Go files under the current (untagged) build
// constraints" setup condition rather than a real unit-test failure. This check
// runs untagged by design (no integration tag — see realGoUnitTest), but every
// cycle materializes a //go:build-gated acceptance package (acs/cycleN); an
// untagged `go test` of one reports "build constraints exclude all Go files …
// [setup failed]" — nothing to unit-test here, NOT a regression. Flagging it
// would WARN on every cycle and bury real signal.
//
// The "imports " guard keeps this narrow: when a transitive *dependency* (not the
// tested package) is the excluded one, `go test` prints an "imports <dep>:" chain
// and the tested package's own build genuinely failed — that must still be
// reported. A real compile error ("[build failed]") or assertion ("--- FAIL")
// also carries different text and is reported.
func goTestExcludedByBuildTags(output string) bool {
	return strings.Contains(output, "build constraints exclude all Go files") &&
		!strings.Contains(output, "imports ")
}

// buildSelfCheck runs the changed packages' unit tests after the build phase and
// records ground-truth when any fail. Best-effort: no changed Go packages → a
// no-op; failures WARN and write a `.evolve/build-selfcheck.json` artifact for
// the audit/retro to consume. NEVER aborts the cycle.
func (o *Orchestrator) buildSelfCheck(ctx context.Context, worktree string) {
	if worktree == "" {
		return
	}
	pkgs := changedGoTestPackages(changedWorktreePaths(ctx, worktree))
	if len(pkgs) == 0 {
		return
	}
	fails := runBuildSelfCheck(ctx, codequality.ModuleDir(worktree), pkgs, buildSelfCheckRunner)
	if len(fails) == 0 {
		return
	}
	names := make([]string, len(fails))
	for i, f := range fails {
		names[i] = f.Pkg
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-selfcheck: %d changed package(s) FAIL unit tests before audit — the build self-report is not green; audit is the backstop: %s\n", len(fails), strings.Join(names, ", "))
	writeBuildSelfCheckArtifact(worktree, fails)
}

// writeBuildSelfCheckArtifact records the failing packages under the worktree's
// .evolve dir so the audit and the next attempt can read the exact failures.
// Best-effort: a write error is non-fatal (the WARN already surfaced it).
func writeBuildSelfCheckArtifact(worktree string, fails []selfCheckFailure) {
	dst := filepath.Join(worktree, ".evolve", "build-selfcheck.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(fails, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(dst, append(data, '\n'), 0o644)
}
