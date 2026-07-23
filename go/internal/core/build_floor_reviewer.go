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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
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

// DefaultBuildFloorChecks is the production deterministic engine: the
// changed-package selfcheck engine plus every check that must run REGARDLESS
// of the changed set. RemovalClaimFailures is deliberately composed OUTSIDE
// changedPackageFloorChecks: that engine returns early when the diff yields no
// Go test packages, and a build whose only claim is "I deleted X" derives
// exactly zero packages — the early return is the precise blind spot a false
// removal claim would hide behind (cycle-660).
func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {
	out := RemovalClaimFailures(ctx, in)
	return append(out, changedPackageFloorChecks(ctx, in)...)
}

// changedPackageFloorChecks reuses the EXACT selfcheck machinery the advisory
// post-build binding runs
// (changedWorktreePaths → changedGoTestPackages → runBuildSelfCheck with the
// real go-test runner) — the flip from advisory to rejecting is the whole
// change (the cycle-1008 smoking gun: the artifact recorded the failure and
// nothing acted on it). Returns one line per failing package. Any inability
// to run (no worktree, no packages) is GREEN — fail-open, downstream gates
// stay armed.
func changedPackageFloorChecks(ctx context.Context, in ReviewInput) []string {
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
	// Split the changed set: ENFORCED packages run once under the coverage-
	// instrumented pass inside apicoverNamingFailures (their test run doubles
	// as the selfcheck — reviewer MED: never run the same package's tests
	// twice per handoff); everything else takes the plain selfcheck.
	moduleDir := codequality.ModuleDir(in.Worktree)
	enforcedSet := map[string]bool{}
	if enforceBytes, err := os.ReadFile(filepath.Join(moduleDir, ".apicover-enforce")); err == nil {
		for _, p := range ciparity.IntersectEnforced(pkgs, enforceBytes) {
			enforcedSet[p] = true
		}
	}
	plain := make([]string, 0, len(pkgs))
	enforced := make([]string, 0, len(enforcedSet))
	for _, p := range pkgs {
		if enforcedSet[p] {
			enforced = append(enforced, p)
		} else {
			plain = append(plain, p)
		}
	}
	fails := runBuildSelfCheck(ctx, moduleDir, plain, buildSelfCheckRunner)
	// The apicover parity class (5 live instances: 3 main REDs, a console PR
	// red, and cycle-1022's invisible audit override): an ENFORCED changed
	// package with an unnamed export dies at HANDOFF, not at audit/CI.
	namingFails := apicoverNamingFailures(ctx, moduleDir, enforced, paths)
	// Persist the artifact for the ACS toolchain gate (same producer contract
	// as the advisory binding, which skips its duplicate run when the floor is
	// enforced — one go-test pass per build, not two).
	removeBuildSelfCheckArtifact(in.Worktree)
	if len(fails) > 0 {
		writeBuildSelfCheckArtifact(in.Worktree, fails)
	}
	out := make([]string, 0, len(fails)+len(namingFails))
	for _, f := range fails {
		head := f.Output
		if len(head) > 400 {
			head = head[:400] + "…"
		}
		out = append(out, fmt.Sprintf("%s: unit tests FAIL\n%s", f.Pkg, head))
	}
	out = append(out, namingFails...)
	return out
}

// apicoverNamingFailures runs the coverage-backed apicover enforce check over
// the enforced changed packages — the same naming floor CI applies, shifted
// to build handoff. The coverage test run DOUBLES as those packages'
// selfcheck (a test failure is returned as a floor failure, never silently
// dropped), and every fail-open plumbing branch WARNs loudly (reviewer MED:
// silence here would let a coverage-run flake vanish the naming check).
func apicoverNamingFailures(ctx context.Context, moduleDir string, enforced []string, changedPaths []string) []string {
	if len(enforced) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(enforced))
	for _, p := range enforced {
		dirs = append(dirs, filepath.Join(moduleDir, strings.TrimPrefix(p, "./")))
	}
	// Diff-scope (cycle-1048): only violations in files THIS change touched
	// hard-fail; a touched package's pre-existing debt WARNs in the report.
	changedByDir := changedFileBasenamesByDir(moduleDir, dirs, changedPaths)
	// apicover's enforce contract is named-AND-executed — it needs a coverage
	// profile or every named export reads as false-green. Generate one scoped
	// to the enforced changed packages (their single test run this handoff).
	coverFunc, testOut, status := scopedCoverFunc(ctx, moduleDir, enforced)
	if coverFunc != "" {
		defer func() { _ = os.RemoveAll(filepath.Dir(coverFunc)) }() // reviewer HIGH: no temp leak
	}
	switch status {
	case coverStatusTestsFailed:
		head := testOut
		if len(head) > 600 {
			head = head[:600] + "…"
		}
		return []string{fmt.Sprintf("enforced package tests FAIL (coverage run doubles as their selfcheck):\n%s", head)}
	case coverStatusPlumbingError:
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: scoped coverage generation failed (%s) — apicover naming check skipped this handoff; audit/CI gates stay armed\n", testOut)
		return nil
	}
	var buf strings.Builder
	code, rerr := apicover.Run(ctx, apicover.Config{Enforce: true, Dirs: dirs, CoverPath: coverFunc, ChangedFilesByDir: changedByDir}, &buf)
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: apicover measurement failed (%v) — naming check skipped this handoff\n", rerr)
		return nil
	}
	if code == 0 {
		return nil
	}
	report := buf.String()
	if len(report) > 800 {
		report = report[:800] + "…"
	}
	return []string{fmt.Sprintf("apicover naming floor: %d enforced changed package(s) carry unnamed exports — name+exercise them (CI api-coverage-enforce would FAIL):\n%s", len(enforced), report)}
}

const (
	coverStatusOK = iota
	coverStatusTestsFailed
	coverStatusPlumbingError
)

// scopedCoverFunc runs `go test -coverprofile` over pkgs and converts it to
// `go tool cover -func` output. Returns the func-file path (caller owns the
// temp dir cleanup via its parent), the combined test output, and a status
// distinguishing TEST failures (a real floor finding) from PLUMBING errors
// (fail-open, loudly). The per-invocation -timeout mirrors realGoUnitTest's
// defense-in-depth so one hung package cannot wedge the whole check beyond
// the ambient ctx.
func scopedCoverFunc(ctx context.Context, moduleDir string, pkgs []string) (path, output string, status int) {
	tmpDir, err := os.MkdirTemp("", "buildfloor-cover-*")
	if err != nil {
		return "", err.Error(), coverStatusPlumbingError
	}
	profile := filepath.Join(tmpDir, "cover.out")
	args := append([]string{"test", "-count=1", "-timeout", "300s", "-coverprofile", profile}, pkgs...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = sanitizeEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return tmpDir + "/", string(out), coverStatusTestsFailed
	}
	funcOut := filepath.Join(tmpDir, "cover.func.txt")
	cmd2 := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profile)
	cmd2.Dir = moduleDir
	cmd2.Env = sanitizeEnv(os.Environ())
	fo, err := cmd2.Output()
	if err != nil {
		return tmpDir + "/", "go tool cover: " + err.Error(), coverStatusPlumbingError
	}
	if err := os.WriteFile(funcOut, fo, 0o644); err != nil {
		return tmpDir + "/", err.Error(), coverStatusPlumbingError
	}
	return funcOut, "", coverStatusOK
}

// changedFileBasenamesByDir maps each enforced package dir to the basenames of
// the changed .go files inside it — the diff-scope filter apicover consumes.
// changedPaths are worktree-relative; dirs are absolute under moduleDir's tree.
func changedFileBasenamesByDir(moduleDir string, dirs []string, changedPaths []string) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(dirs))
	worktree := filepath.Dir(moduleDir) // moduleDir = <worktree>/go
	for _, d := range dirs {
		out[d] = map[string]bool{}
	}
	for _, p := range changedPaths {
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		abs := filepath.Join(worktree, p)
		d := filepath.Dir(abs)
		if set, ok := out[d]; ok {
			set[filepath.Base(p)] = true
		}
	}
	return out
}
