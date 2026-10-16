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

func packageHasProductionGoFiles(worktree, pkg string) bool {
	return ciparity.PackageDirHasProductionGoFiles(codequality.ModuleDir(worktree), pkg)
}

func packageNewThisCycle(ctx context.Context, worktree, pkg string) bool {
	rel := "go/" + strings.TrimPrefix(pkg, "./")
	out, code, err := gitCapture(ctx, worktree, "ls-tree", "--name-only", "HEAD", "--", rel)
	if err != nil || code != 0 {
		return false
	}
	return strings.TrimSpace(out) == ""
}
