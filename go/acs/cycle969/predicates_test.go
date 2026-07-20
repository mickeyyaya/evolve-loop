//go:build acs

// Package cycle969 materializes the cycle-969 acceptance criteria for the sole
// committed task of this fleet lane, wire-carryforward-prune-cli (triage
// top_n; fleet_scope pins this lane to the todo-id prune-superseded-orphans-
// lane, so per R9.3 no predicates bind to any other lane's items).
//
// DEFECT CLOSED. cycle-962 shipped two fully-implemented, unit-tested exports
// in go/internal/core — CarryforwardCandidateLandable (carryforward_filter.go)
// and PruneSupersededOrphans (prune_superseded_orphans.go) — with ZERO non-test
// production callers and no CLI surface to invoke either. This cycle adds an
// `evolve branches audit|prune` subcommand (new go/cmd/evolve/cmd_branches.go,
// registered in registry.go mirroring the `worktree` row) that dispatches to
// BOTH core functions, giving them their first live caller and letting an
// operator actually walk the stale orphan `cycle-*` ref backlog.
//
// SUT surface the Builder must add WITHOUT modifying this file:
//   - go/cmd/evolve/cmd_branches.go: func runBranches(args []string, stdin
//     io.Reader, stdout, stderr io.Writer) int — dispatches `audit` and `prune`
//     subcommands (mirrors runWorktree). `audit` is read-only and prints, per
//     local cycle-* branch, BOTH a supersession verdict (via
//     core.PruneSupersededOrphans / its refSuperseded screen) and a
//     carry-forward verdict (via core.CarryforwardCandidateLandable). `prune`
//     defaults to dry-run (deletes nothing) and deletes a superseded branch
//     ONLY under explicit --dry-run=false AND when hasOpenPR reports false.
//   - registry.go: a {Name:"branches", Run: runBranches} row.
//
// OUTPUT CONTRACT the predicates below bind (also in agent-mailbox.md):
//
//	audit                 → one line per branch: `<ref> superseded=<t|f> landable=<t|f>`
//	prune (dry-run)       → lists each superseded ref (`<ref> ... would-prune`), deletes nothing
//	prune --dry-run=false → deletes each superseded ref with no open PR (`<ref> ... pruned`)
//
// hasOpenPR DETERMINISM (required contract, surfaced per Core Rule 3): the
// prune fixtures below are git repos with NO configured remote. hasOpenPR MUST
// degrade to (false, nil) when there is no remote / `gh` is unavailable — a
// branch provably has no reachable remote PR, so a locally-superseded ref is
// safe to delete (this is exactly verify_remote_pr_before_branch_delete: we
// deleted only after confirming no open PR). A hasOpenPR that ERRORS on a
// no-remote repo leaves C969_004 RED — legitimate TDD pressure toward the
// graceful-degradation the tool needs to run in CI.
//
// PREDICATE STYLE (cycle-85 rule): every predicate BUILDS the real `evolve`
// binary and RUNS the new subcommand against a REAL git repo built in a temp
// dir, asserting on exit code + emitted stdout + the post-run branch list — no
// source-grep predicate exists here. A correct `evolve branches audit` output
// necessarily proves the CLI calls BOTH core functions (their verdicts appear
// in stdout), so these behavioral predicates ARE the caller-existence proof
// that closes the inert-API gap. RED before the Builder acts: `evolve branches`
// is an unknown subcommand → non-zero exit / empty stdout → every assertion
// fails for the right reason (the subcommand is absent). Each predicate folds
// exit==0 into its assertion so none can false-green on the pre-impl repo.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C969_001 (audit reports a superseded branch — anti-`always-false`
//	           for PruneSupersededOrphans), C969_002 (audit's landable column
//	           distinguishes superseded vs divergent — binds the SECOND core
//	           func with two distinct outcomes), C969_004 (--dry-run=false
//	           actually deletes — the anti-no-op for prune).
//	NEGATIVE → C969_003 (default prune is dry-run: a superseded branch SURVIVES
//	           — the strongest safety signal; a prune ignoring the dry-run
//	           default would delete it), C969_005 (--dry-run=false never deletes
//	           a NON-superseded divergent branch — anti-overreach).
//	EDGE     → superseded-via-ancestor vs divergent-clean fixtures exercise both
//	           supersession paths and the clean-merge landable path.
//	SEMANTIC → audit(read-only report) / prune-dry-run(report, no delete) /
//	           prune-force(delete) are DISTINCT behaviors, each asserted apart.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 subcommands exist, registered, dispatch to BOTH core funcs → C969_001 + C969_002
//	AC2 prune dry-run default; --dry-run=false deletes only non-open-PR   → C969_003 + C969_004 + C969_005
//	AC3 unit tests red-first + caller-existence for both funcs            → C969_001/002 (live callers proven behaviorally) + manual (unit suite)
//	AC4 go vet ./... and go test ./cmd/evolve/... green, no regression    → manual+checklist (Auditor CI-parity)
package cycle969

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns <repoRoot>/go — the module dir the evolve binary builds from.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// buildEvolve compiles the worktree's `evolve` binary once into a temp path and
// returns it. A build failure is a hard test error (never a silent pass): the
// predicates assert on the binary's runtime behavior, so it must exist.
func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-C", goDir(t), "-o", bin, "./cmd/evolve")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve binary: %v\n%s", err, out)
	}
	return bin
}

// runEvolve runs the built binary with cwd=dir and returns stdout+stderr and
// the exit code (the branch subcommands take --project-root explicitly, but a
// stable cwd keeps git-relative resolution deterministic).
func runEvolve(t *testing.T, bin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run evolve %v: %v", args, err)
		}
	}
	return sout.String(), serr.String(), code
}

// git runs `git -C dir args...` as the TEST HARNESS (not the SUT) and fails on
// any non-zero exit — fixtures must build cleanly for the SUT assertion to mean
// anything.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// writeCommit writes content to name under dir and commits it with msg.
func writeCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

// fixtureRepo builds a fresh git repo (NO remote) on branch `main` with:
//
//	cycle-100 — SUPERSEDED: branched from an earlier main commit, then main
//	            advanced past it, so cycle-100 is a strict ancestor of main
//	            (refSuperseded true; CarryforwardCandidateLandable false).
//	cycle-200 — DIVERGENT: a unique, cleanly-mergeable commit not on main
//	            (refSuperseded false; CarryforwardCandidateLandable true).
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "acs@evolve.local")
	git(t, dir, "config", "user.name", "acs")
	git(t, dir, "config", "commit.gpgsign", "false")
	writeCommit(t, dir, "base.txt", "line1\n", "base commit")

	// cycle-100 at the current (soon-to-be-ancestor) tip.
	git(t, dir, "branch", "cycle-100")

	// cycle-200 diverges with a unique, non-conflicting file.
	git(t, dir, "checkout", "-q", "-b", "cycle-200")
	writeCommit(t, dir, "feature.txt", "new work\n", "cycle-200 unique commit")
	git(t, dir, "checkout", "-q", "main")

	// main advances past cycle-100, making cycle-100 a strict ancestor.
	writeCommit(t, dir, "base.txt", "line1\nline2\n", "advance main")
	return dir
}

// branchExists reports whether refs/heads/<name> is present in dir.
func branchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return cmd.Run() == nil
}

// hasBranchLine reports whether stdout has a line mentioning ref that also
// contains every needle (the audit/prune row for that branch).
func hasBranchLine(stdout, ref string, needles ...string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		if !strings.Contains(line, ref) {
			continue
		}
		ok := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// --- C969_001 (AC1, POSITIVE): audit reports the superseded branch, read-only.
// Proves runBranches dispatches to core.PruneSupersededOrphans (the supersession
// verdict appears) and that audit never deletes. RED pre-impl: `branches` is an
// unknown subcommand → exit!=0 / no such line.
func TestC969_001_AuditReportsSupersededReadOnly(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "audit", "--project-root", dir, "--base", "main")
	if code != 0 {
		t.Errorf("RED: `evolve branches audit` exit=%d (want 0) — subcommand must exist and dispatch to core.PruneSupersededOrphans\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !hasBranchLine(stdout, "cycle-100", "superseded=true") {
		t.Errorf("RED: audit did not report cycle-100 superseded=true — PruneSupersededOrphans not wired\nstdout:\n%s", stdout)
	}
	if !branchExists(t, dir, "cycle-100") {
		t.Errorf("audit must be READ-ONLY: cycle-100 was deleted by an audit run")
	}
}

// --- C969_002 (AC1/AC3, POSITIVE/SEMANTIC): audit's landable column binds the
// SECOND core func (core.CarryforwardCandidateLandable) with two DISTINCT
// outcomes — superseded cycle-100 is landable=false, divergent-clean cycle-200
// is landable=true. A no-op that hardcodes one value cannot satisfy both.
func TestC969_002_AuditLandableColumnDistinguishes(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "audit", "--project-root", dir, "--base", "main")
	if code != 0 {
		t.Fatalf("RED: `evolve branches audit` exit=%d (want 0)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !hasBranchLine(stdout, "cycle-100", "landable=false") {
		t.Errorf("RED: audit did not report cycle-100 landable=false — CarryforwardCandidateLandable not wired (superseded ref must be non-landable)\nstdout:\n%s", stdout)
	}
	if !hasBranchLine(stdout, "cycle-200", "landable=true") {
		t.Errorf("RED: audit did not report cycle-200 landable=true — CarryforwardCandidateLandable not wired (divergent clean ref must be landable)\nstdout:\n%s", stdout)
	}
}

// --- C969_003 (AC2, NEGATIVE): default `prune` (no flags) is DRY-RUN — the
// superseded cycle-100 SURVIVES. Folds exit==0 into the assertion so it cannot
// false-green pre-impl (an unknown subcommand fails, leaving cycle-100 present
// too — the exit check rejects that).
func TestC969_003_PruneDryRunDefaultKeepsSuperseded(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "prune", "--project-root", dir, "--base", "main")
	if code != 0 {
		t.Errorf("RED: `evolve branches prune` (default) exit=%d (want 0) — must run a dry-run walk\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !branchExists(t, dir, "cycle-100") {
		t.Errorf("SAFETY: default prune deleted cycle-100 — prune MUST default to dry-run and delete nothing")
	}
	if !hasBranchLine(stdout, "cycle-100", "would-prune") {
		t.Errorf("RED: default prune did not flag cycle-100 as a would-prune candidate\nstdout:\n%s", stdout)
	}
}

// --- C969_004 (AC2, POSITIVE): `prune --dry-run=false` DELETES the superseded
// cycle-100 (no remote → no open PR → safe to delete). The anti-no-op for the
// actual prune path.
func TestC969_004_PruneForceDeletesSuperseded(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "prune", "--project-root", dir, "--base", "main", "--dry-run=false")
	if code != 0 {
		t.Fatalf("RED: `evolve branches prune --dry-run=false` exit=%d (want 0)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if branchExists(t, dir, "cycle-100") {
		t.Errorf("RED: prune --dry-run=false did not delete superseded cycle-100 (no remote → hasOpenPR must degrade to false)\nstdout:\n%s", stdout)
	}
}

// --- C969_005 (AC2, NEGATIVE): `prune --dry-run=false` NEVER deletes a
// NON-superseded (divergent) branch — cycle-200 SURVIVES. Anti-overreach:
// pruning must be gated on the supersession screen, not blanket-delete cycle-*.
func TestC969_005_PruneForceKeepsDivergent(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "prune", "--project-root", dir, "--base", "main", "--dry-run=false")
	if code != 0 {
		t.Fatalf("RED: `evolve branches prune --dry-run=false` exit=%d (want 0)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !branchExists(t, dir, "cycle-200") {
		t.Errorf("OVERREACH: prune --dry-run=false deleted divergent cycle-200 — only SUPERSEDED refs may be pruned")
	}
}
