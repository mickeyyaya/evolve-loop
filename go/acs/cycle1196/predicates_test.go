//go:build acs

// Package cycle1196 materialises the cycle-1196 acceptance criteria for the one
// triage-committed top_n task, `lane-base-fetch-origin-main` (fleet-scope todo
// `loop-must-base-lanes-on-origin-main-not-stale-local`).
//
// Defect: gitWorktree.Create bases every new lane branch on the LOCAL HEAD of
// projectRoot (`git worktree add -B <branch> <wt> HEAD`, go/internal/core/worktree.go:97)
// with zero fetch/origin interaction in the file. In a multi-lane fleet the local
// checkout drifts behind origin/main as sibling lanes land, so each new lane forks
// from a stale tip — silently reproducing already-fixed defects and inflating
// ship-time merge conflicts.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1098 precedent).
// gitWorktree and its gitRunner seam are UNEXPORTED, so an out-of-package
// predicate cannot call Create directly; each predicate therefore shells
// `go test -run` over the RED contract tests authored this cycle in
// go/internal/core/worktree_lanebase_test.go. Every one of those drives the real
// Create() through a scripted git seam and asserts on the recorded git argv,
// call ORDER, returned worktree path and returned error — none is a source-grep
// of production code (the cycle-85 degenerate-predicate ban).
package cycle1196

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// corePkg is the worktree provisioner's home package. gitWorktree/gitRunner are
// unexported, hence the `go test` subprocess form rather than a direct import.
const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package surfaces as a non-zero exit — the intended RED signal
// before Builder implements.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal)
	// must flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1196_001_LaneBasesOnFetchedOriginTip — AC1. With an origin remote
// configured, Create must fetch the upstream tip in projectRoot BEFORE
// `git worktree add`, and cut the lane branch from that fetched ref
// (origin/<default-branch> or FETCH_HEAD) rather than the literal local "HEAD".
// The driving test asserts on the recorded git argv and on call ORDER, so an
// implementation that fetches after basing (or fetches and still passes "HEAD")
// fails.
func TestC1196_001_LaneBasesOnFetchedOriginTip(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestGitWorktree_Create_FetchesOriginBeforeBasingLane")
	if !ok {
		t.Errorf("lane worktrees still fork from the stale local HEAD: Create does not fetch origin before `git worktree add`, or still passes \"HEAD\" as the start-ref\n%s", out)
	}
}

// TestC1196_002_NoOriginFallsBackToLocalHEAD — AC2 (edge/OOD). A repo with no
// origin remote (isolated local dev, several test fixtures) must still
// provision successfully via the explicit local-HEAD fallback, with the fetch
// skipped entirely. This kills the "fetch unconditionally" implementation that
// would break every remoteless repo.
func TestC1196_002_NoOriginFallsBackToLocalHEAD(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestGitWorktree_Create_NoOriginFallsBackToLocalHEAD")
	if !ok {
		t.Errorf("Create no longer provisions cleanly in a repo with no origin remote (must skip the fetch and base on local HEAD)\n%s", out)
	}
}

// TestC1196_003_FetchFailureIsFatal — AC3, the NEGATIVE criterion and the
// strongest anti-no-op signal. Origin exists but the fetch fails: Create must
// return an error naming the fetch and must NOT run `git worktree add` at all.
// A "best-effort fetch, continue from local HEAD" implementation reproduces the
// exact defect this task closes and is rejected here.
func TestC1196_003_FetchFailureIsFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestGitWorktree_Create_FetchFailureIsFatal")
	if !ok {
		t.Errorf("a failed origin fetch does not fail loudly: Create still provisions the lane from the stale local tip instead of returning a wrapped fetch error\n%s", out)
	}
}

// TestC1196_004_ExistingProvisioningContractsIntact — AC4 (regression). The
// idempotent-reuse path (worktree.go:74-90) must stay a no-op — no fetch, no
// re-`add` for a live lane, or a resumed cycle would lose in-progress work —
// and the absolute-base guards must still refuse relative paths. Runs the
// pre-existing Create tests together with the new reuse guard.
func TestC1196_004_ExistingProvisioningContractsIntact(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestGitWorktree_Create_ReuseSkipsFetch|TestGitWorktree_RelativeBaseRefused|TestGitWorktree_RelativeProjectRootRefused|TestOrchestrator_ProvisionsWorktree_PassesToSourcePhases")
	if !ok {
		t.Errorf("the fetch-before-base change regressed existing worktree provisioning contracts (reuse no-op / absolute-base guards / orchestrator provisioning)\n%s", out)
	}
}
