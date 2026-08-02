package core

// phase_bindings_graduation.go — deterministic build-entry graduation guard
// (inbox new-package-graduation-buildentry-gate, 3rd recurrence: cycles
// 575/587/652). A package NEW this cycle cannot be in go/.apicover-enforce yet,
// so the touched∩enforced apicover gate never inspects it — the recurring
// warnship_apicover_ci_gap blind spot. The audit-side half
// (apicoverNewPackageGraduationDefault) landed 2026-07-07; this is the
// build-entry half: the same predicate at the post-build seam, but
// abort-capable — unlike buildSelfCheck (WARN-only, NEVER aborts, see
// phase_bindings_selfcheck.go), an ungraduated new package FAILS the build
// phase with an explicit abort_reason, because graduation is a hard shipping
// obligation the builder itself must satisfy, not a diagnostic for audit to
// re-discover two attempts later.

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
// worktree: non-empty iff a changed go/internal/<pkg> package is NEW this cycle
// (no committed files at HEAD — a modified pre-existing package is the
// enforce-ratchet's concern, and a deleted/renamed-away package is not new) and
// absent from go/.apicover-enforce. Fail-open ("" — mirroring the audit
// default) when the worktree is empty or the enforce file is unreadable:
// with no enforce list there is nothing to graduate against. Detection reuses
// ciparity.NewUngraduatedPackages over the same changed-path derivation the
// WARN-only self-check uses, so the two seams cannot disagree on scope.
func buildGraduationCheck(ctx context.Context, worktree string) string {
	if worktree == "" {
		return ""
	}
	enforceBytes, err := os.ReadFile(filepath.Join(codequality.ModuleDir(worktree), ".apicover-enforce"))
	if err != nil {
		return "" // no enforce list → nothing to graduate against (fail-open)
	}
	changed := changedGoTestPackages(changedWorktreePaths(ctx, worktree))
	var fresh []string
	for _, pkg := range ciparity.NewUngraduatedPackages(changed, enforceBytes) {
		if packageNewThisCycle(ctx, worktree, pkg) && packageHasProductionGoFiles(worktree, pkg) {
			fresh = append(fresh, pkg)
		}
	}
	if len(fresh) == 0 {
		return ""
	}
	return fmt.Sprintf("new package(s) %s changed this cycle but are absent from go/.apicover-enforce — the repo-wide apicover unnamed-export gate (ADR-0069's SECOND gate; the per-cycle ACS coverage gate is a different one and needs no enrollment) therefore never inspects them. Make EXACTLY these edits in THIS handoff, then re-run `evolve selfcheck build`:\n%s",
		strings.Join(fresh, ", "), graduationPrescription(fresh))
}

// graduationPrescription renders the exact two edits per ungraduated package.
//
// The floor already NAMED the offending packages and still cost batch-21 three
// build phases and a halt at cycle-1218: naming the class is not remediation
// when the class is a house rule the agent never inherited. So the abort reason
// emits the pattern line to append and the test path to create,
// copy-pasteable, derived from the same "./internal/<pkg>" enforce-list form
// the detector matched on so the prescription cannot name a package the check
// did not flag.
//
// WHO READS THIS (review MEDIUM — do not overclaim): the reason reaches the
// operator log, the phase outcome's abort_reason (phasetiming/failure_learning)
// and the resume path — NOT an in-cycle builder re-dispatch, which the
// correction ladder does not carry. The builder learns the rule from its
// PERSONA (agents/evolve-builder.md); this text is what makes the failure
// self-serving for whoever picks it up next, human or resumed cycle.
func graduationPrescription(pkgs []string) string {
	var b strings.Builder
	for _, pkg := range pkgs {
		fmt.Fprintf(&b, "  %s:\n", pkg)
		fmt.Fprintf(&b, "    1. append this line to go/.apicover-enforce:  %s\n", pkg)
		// A recursive pattern names no single directory, so prescribing
		// "<pattern>/apicover_named_test.go" would hand the builder an invalid
		// path (review LOW). Name the obligation without the bogus path.
		if strings.Contains(pkg, "...") {
			b.WriteString("    2. add an apicover_named_test.go in EACH package the pattern covers, naming every exported symbol in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)\n")
			continue
		}
		dir := "go/" + strings.TrimPrefix(pkg, "./")
		fmt.Fprintf(&b, "    2. create %s/apicover_named_test.go naming every exported symbol of the package in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)\n", dir)
	}
	return strings.TrimRight(b.String(), "\n")
}

// packageHasProductionGoFiles delegates to the SHARED graduation predicate —
// see ciparity.PackageDirHasProductionGoFiles for the test-only-package
// rationale (cycles 1223/1224/1228). One predicate, two seams, no disagreement.
func packageHasProductionGoFiles(worktree, pkg string) bool {
	return ciparity.PackageDirHasProductionGoFiles(codequality.ModuleDir(worktree), pkg)
}

// packageNewThisCycle reports whether an enforce-list-form package pattern
// ("./internal/foo") has NO committed files at the worktree's HEAD — i.e. the
// package was introduced by this cycle's pending diff. A package present at
// HEAD is pre-existing (modified or deleted this cycle), which the graduation
// obligation does not cover: flagging a delete/rename would make graduation
// hygiene un-shippable (AC3). Fail-open on git error: a package we cannot
// prove new must not abort the build (audit stays the backstop).
func packageNewThisCycle(ctx context.Context, worktree, pkg string) bool {
	rel := "go/" + strings.TrimPrefix(pkg, "./")
	out, code, err := gitCapture(ctx, worktree, "ls-tree", "--name-only", "HEAD", "--", rel)
	if err != nil || code != 0 {
		return false
	}
	return strings.TrimSpace(out) == ""
}
