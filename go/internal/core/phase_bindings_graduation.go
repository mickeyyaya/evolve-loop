package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

// buildGraduationCheck reports the graduation abort reason for the cycle's
// worktree: non-empty iff a changed go/internal/<pkg> package is NEW this
// cycle (no committed files at HEAD — a modified pre-existing package is the
// enforce-ratchet's concern, and a deleted/renamed-away package is not new)
// and absent from go/.apicover-enforce. Fail-open ("") when the worktree is
// empty or the enforce file is unreadable.
func buildGraduationCheck(ctx context.Context, worktree string) string {
	if worktree == "" {
		return ""
	}
	enforceBytes, err := os.ReadFile(filepath.Join(codequality.ModuleDir(worktree), ".apicover-enforce"))
	if err != nil {
		return ""
	}
	changed := changedGoTestPackages(changedWorktreePaths(ctx, worktree))
	var fresh []string
	for _, pkg := range ciparity.NewUngraduatedPackages(changed, enforceBytes) {
		if packageNewThisCycle(ctx, worktree, pkg) {
			if !packageHasProductionGoFiles(worktree, pkg) {
				// Never silently skip: the vacuous-obligation skip is
				// announced, so a test-only mint that lands unenrolled is
				// visible in the cycle log rather than discovered at audit.
				fmt.Fprintf(os.Stderr, "[build-floor] graduation deferred: new package %s is test-only (no production .go surface) — abort suppressed; audit re-flags the package on any later change\n", pkg)
				continue
			}
			fresh = append(fresh, pkg)
		}
	}
	if len(fresh) == 0 {
		return ""
	}
	return fmt.Sprintf("new package(s) %s changed this cycle but are absent from go/.apicover-enforce — the repo-wide apicover unnamed-export gate (ADR-0069's SECOND gate; the per-cycle ACS coverage gate is a different one and needs no enrollment) therefore never inspects them. Make EXACTLY these edits in THIS handoff, then re-run `evolve selfcheck build`:\n%s",
		strings.Join(fresh, ", "), ciparity.GraduationPrescription(fresh))
}

// packageHasProductionGoFiles delegates to the SHARED graduation predicate —
// see ciparity.PackageDirHasProductionGoFiles for the test-only-package
// rationale. One predicate, two seams, no disagreement.
func packageHasProductionGoFiles(worktree, pkg string) bool {
	return ciparity.PackageDirHasProductionGoFiles(codequality.ModuleDir(worktree), pkg)
}

// packageNewThisCycle reports whether an enforce-list-form package pattern
// ("./internal/foo") has NO committed files at the worktree's HEAD — i.e. the
// package was introduced by this cycle's pending diff. A package present at
// HEAD is pre-existing (modified or deleted this cycle), which the graduation
// obligation does not cover: flagging a delete/rename would make graduation
// hygiene un-shippable. Fail-open on git error: a package we cannot
// prove new must not abort the build (audit stays the backstop).
func packageNewThisCycle(ctx context.Context, worktree, pkg string) bool {
	rel := "go/" + strings.TrimPrefix(pkg, "./")
	out, code, err := gitCapture(ctx, worktree, "ls-tree", "--name-only", "HEAD", "--", rel)
	if err != nil || code != 0 {
		return false
	}
	return strings.TrimSpace(out) == ""
}
