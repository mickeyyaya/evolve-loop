package core

// build_floor_reviewer.go — the shift-left build handoff floor (operator
// directive 2026-07-21): deterministic checks move to the FRONT, as part of
// build-phase verification, while the judgment phases (audit, adversarial-
// review) still follow as the final verdict layer. Mounted in the E2
// DeliverableReviewer chain for phase==build only: a red deterministic
// self-check REJECTS the build deliverable, which the existing correction
// ladder converts into a bounded in-phase builder fix — closing the
// cycle-1008 class where the builder recorded ./cmd/evolve failing in
// build-selfcheck.json and handed off anyway, burning four downstream phases
// before the ACS toolchain gate refused ship.
//
// The reviewer owns POLICY only; the deterministic ENGINE is injected
// (production: the existing phase_bindings selfcheck/gofmt machinery via
// BuildFloorChecks). Fail-open floors: a nil engine or an engine that cannot
// run approves loudly — downstream deterministic gates (ACS toolchain,
// apicover, CI) stay armed, so the floor can never false-block a build over
// its own plumbing.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

// BuildFloorCheckFn runs the deterministic build-floor checks for a completed
// build and returns the failures (empty = green). Implementations must be
// deterministic and LLM-free.
type BuildFloorCheckFn func(ctx context.Context, in ReviewInput) []string

// buildFloorReviewer implements DeliverableReviewer for the build phase.
type buildFloorReviewer struct {
	checks BuildFloorCheckFn
}

// NewBuildFloorReviewer builds the reviewer around an injected deterministic
// check engine. A nil engine yields a fail-open reviewer (approve everything,
// WARN once per review) so composition roots can wire unconditionally.
func NewBuildFloorReviewer(checks BuildFloorCheckFn) DeliverableReviewer {
	return &buildFloorReviewer{checks: checks}
}

func (r *buildFloorReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	if r.checks == nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: no deterministic check engine wired — failing open (downstream gates stay armed)\n")
		return ReviewResult{Approve: true}
	}
	failures := r.checks(ctx, in)
	if len(failures) == 0 {
		return ReviewResult{Approve: true}
	}
	reason := fmt.Sprintf("build handoff floor: %d deterministic check failure(s) — fix these exactly before handoff:\n  %s",
		len(failures), strings.Join(failures, "\n  "))
	fmt.Fprintf(os.Stderr, "[build-floor] REJECT: %s\n", reason)
	return ReviewResult{Approve: false, Retry: true, Reason: reason}
}

// DefaultBuildFloorChecks is the production deterministic engine: it reuses
// the EXACT selfcheck machinery the advisory post-build binding runs
// (changedWorktreePaths → changedGoTestPackages → runBuildSelfCheck with the
// real go-test runner) — the flip from advisory to rejecting is the whole
// change (the cycle-1008 smoking gun: the artifact recorded the failure and
// nothing acted on it). Returns one line per failing package. Any inability
// to run (no worktree, no packages) is GREEN — fail-open, downstream gates
// stay armed.
func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	// Note: this runs BEFORE recordAndBranch's gofmt/derived-regen normalizes;
	// both are test-outcome-neutral today and the failure direction of any
	// future sensitivity is a spurious REJECT (one extra ladder round), never
	// a false approve.
	// Diff against the CYCLE BASE, not HEAD: the builder's mandated protocol
	// COMMITS its work, so at review time (before the post-record soft-reset)
	// `git diff HEAD` is empty and a HEAD-based floor approves vacuously —
	// the reviewer-caught near-no-op. Base-diff sees committed AND uncommitted
	// work; empty base falls back to the HEAD-based derivation (degraded
	// provisioning, where the builder could not have committed).
	var paths []string
	if in.WorktreeBaseSHA != "" {
		paths = changedWorktreePathsSince(ctx, in.Worktree, in.WorktreeBaseSHA)
	} else {
		paths = changedWorktreePaths(ctx, in.Worktree)
	}
	pkgs := changedGoTestPackages(paths)
	if len(pkgs) == 0 {
		return nil
	}
	fails := runBuildSelfCheck(ctx, codequality.ModuleDir(in.Worktree), pkgs, buildSelfCheckRunner)
	// Persist the artifact for the ACS toolchain gate (same producer contract
	// as the advisory binding, which skips its duplicate run when the floor is
	// enforced — one go-test pass per build, not two).
	removeBuildSelfCheckArtifact(in.Worktree)
	if len(fails) > 0 {
		writeBuildSelfCheckArtifact(in.Worktree, fails)
	}
	out := make([]string, 0, len(fails))
	for _, f := range fails {
		head := f.Output
		if len(head) > 400 {
			head = head[:400] + "…"
		}
		out = append(out, fmt.Sprintf("%s: unit tests FAIL\n%s", f.Pkg, head))
	}
	return out
}
