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

// changedWorktreePathsSince lists paths changed relative to baseSHA (committed
// work included — the build-floor reviewer's axis) plus untracked additions.
func changedWorktreePathsSince(ctx context.Context, worktree, baseSHA string) []string {
	var out []string
	if diff, code, err := gitCapture(ctx, worktree, "diff", baseSHA, "--name-only"); err == nil && code == 0 {
		for _, l := range strings.Split(diff, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
	}
	if oth, code, err := gitCapture(ctx, worktree, "ls-files", "--others", "--exclude-standard"); err == nil && code == 0 {
		for _, l := range strings.Split(oth, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
	}
	return out
}

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
// goTestExcludedByBuildTags). The subprocess env is sanitized (see sanitizeEnv)
// so the campaign's runtime flags don't flip env-sensitive tests. A bounded
// timeout keeps a wedged test from hanging the build phase.
func realGoUnitTest(ctx context.Context, moduleDir, pkg string) (output string, passed bool) {
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-timeout", "120s", pkg)
	cmd.Dir = moduleDir
	cmd.Env = sanitizeEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	s := string(out)
	return s, err == nil || goTestExcludedByBuildTags(s)
}

// sanitizeEnv drops the loop's per-run EVOLVE_* runtime flags from an env so the
// self-check's `go test` runs tests in their default (CI-like) configuration.
// The self-check runs as a subprocess of a cycle, which sets EVOLVE_FLEET=1 (and
// other phase env); inheriting that flips env-sensitive tests into false failures
// — e.g. internal/bridge's fleet-mode worktree guard returns exit 10 ("explicit
// worktree required") under EVOLVE_FLEET, though the package passes cleanly in
// CI. Prefix match on "EVOLVE_"; everything else (PATH, HOME, GOFLAGS, …) is kept
// so the toolchain still works. What we strip is only the campaign's *ambient*
// runtime config: a test in the package under test that needs an EVOLVE_ var sets
// it at runtime via t.Setenv inside the go-test subprocess, which runs after this
// sanitization and is unaffected.
func sanitizeEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
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
	// Clear any prior artifact FIRST so a passing (re)build never inherits a stale
	// failure. The toolchain gate (go/acs/regression/buildselfcheck) reads this
	// artifact at audit and HARD-FAILS on it; without the clear, a retry that fixes
	// the build would still see the previous attempt's failures and loop forever.
	removeBuildSelfCheckArtifact(worktree)
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

// removeBuildSelfCheckArtifact deletes any prior build-selfcheck artifact so a
// passing rebuild does not leave a stale failure for the toolchain gate to read.
// Best-effort: a missing file (the common case) is not an error.
func removeBuildSelfCheckArtifact(worktree string) {
	_ = os.Remove(filepath.Join(worktree, ".evolve", "build-selfcheck.json"))
}

// writeBuildSelfCheckArtifact records the failing packages under the worktree's
// .evolve dir so the audit and the next attempt can read the exact failures.
// Best-effort: a write error is non-fatal (the WARN already surfaced it).
func writeBuildSelfCheckArtifact(worktree string, fails []selfCheckFailure) {
	dir := filepath.Join(worktree, ".evolve")
	dst := filepath.Join(dir, "build-selfcheck.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[selfcheck] WARN could not persist build-selfcheck artifact %s: %v\n", dst, err)
		return
	}
	data, err := json.MarshalIndent(fails, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[selfcheck] WARN could not encode build-selfcheck artifact %s: %v\n", dst, err)
		return
	}
	if err := os.WriteFile(dst, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[selfcheck] WARN could not write build-selfcheck artifact %s: %v\n", dst, err)
	}
}
