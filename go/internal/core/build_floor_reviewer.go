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
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
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
// of the changed set. RemovalClaimFailures and personaBudgetFailures are
// deliberately composed OUTSIDE changedPackageFloorChecks: that engine returns
// early when the diff yields no Go test packages, and both of their triggering
// diffs derive exactly zero packages — a build whose only claim is "I deleted
// X" (cycle-660), and a lane whose only change is an agents/evolve-*.md
// persona doc (cycle-1101). The early return is the precise blind spot each
// would otherwise hide behind.
//
// The changed-path set is derived ONCE here and passed down: the two path-
// driven engines must adjudicate the same diff, and one `git diff` per handoff
// is the standing floor rule.
func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {
	out := RemovalClaimFailures(ctx, in)
	paths := changedFloorPaths(ctx, in)
	out = append(out, personaBudgetFailures(ctx, in.Worktree, paths)...)
	// ADR-0077 docs floor: WARN-only, so it rides the SAME derived change set
	// rather than returning a failure — an architecture change with no doc is a
	// finding for the auditor, never a handoff REJECT.
	docsFloorWarn(in, paths)
	return append(out, changedPackageFloorChecks(ctx, in, paths)...)
}

// docsFloorWarn evaluates the ADR-0077 documentation floor over the handoff's
// change set and prints its WARN to stderr (the same channel every other
// fail-open floor signal uses, so it lands in the phase log the auditor reads).
// Stage comes from .evolve/policy.json `docs_floor.stage`; an unreadable policy
// falls back to the compiled default, which the empty Config.Stage encodes.
func docsFloorWarn(in ReviewInput, paths []string) {
	var cfg docsfloor.Config
	if in.ProjectRoot != "" {
		if p, err := policy.Load(filepath.Join(in.ProjectRoot, ".evolve", "policy.json")); err == nil {
			cfg.Stage = p.DocsFloorConfig().Stage
		}
	}
	// Label with the blocking-grade classifier (docsfloor.IsArchitectureClass):
	// it drops test-only diffs — which document nothing and were the WARN's main
	// false positive — and picks up the trust-kernel, new-package and phase-spec
	// surfaces the broad label misses. The VERDICT stays WARN (ADR-0077): only
	// the precision of "is this architecture" improves here.
	v := docsfloor.Evaluate(cfg, docsfloor.Input{
		ArchitectureLabeled: docsfloor.IsArchitectureClass(paths),
		ChangedFiles:        paths,
	})
	if v.Status == docsfloor.StatusWarn {
		fmt.Fprintf(os.Stderr, "[docs-floor] WARN: %s\n", v.Reason)
	}
}

// changedFloorPaths derives the lane's changed repo paths for the floor.
//
// Diff against the CYCLE BASE, not HEAD: the builder's mandated protocol
// COMMITS its work, so at review time (before the post-record soft-reset)
// `git diff HEAD` is empty and a HEAD-based floor approves vacuously — the
// reviewer-caught near-no-op. Base-diff sees committed AND uncommitted work;
// an empty base falls back to the HEAD-based derivation (degraded
// provisioning, where the builder could not have committed).
func changedFloorPaths(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	if in.WorktreeBaseSHA != "" {
		return changedWorktreePathsSince(ctx, in.Worktree, in.WorktreeBaseSHA)
	}
	return changedWorktreePaths(ctx, in.Worktree)
}

// changedPackageFloorChecks reuses the EXACT selfcheck machinery the advisory
// post-build binding runs
// (changedWorktreePaths → changedGoTestPackages → runBuildSelfCheck with the
// real go-test runner) — the flip from advisory to rejecting is the whole
// change (the cycle-1008 smoking gun: the artifact recorded the failure and
// nothing acted on it). Returns one line per failing package. Any inability
// to run (no worktree, no packages) is GREEN — fail-open, downstream gates
// stay armed.
func changedPackageFloorChecks(ctx context.Context, in ReviewInput, paths []string) []string {
	if in.Worktree == "" {
		return nil
	}
	// Note: this runs BEFORE recordAndBranch's gofmt/derived-regen normalizes;
	// both are test-outcome-neutral today and the failure direction of any
	// future sensitivity is a spurious REJECT (one extra ladder round), never
	// a false approve.
	// paths comes from changedFloorPaths (cycle-base diff, HEAD fallback) —
	// see its doc for why the base axis is load-bearing.
	pkgs := changedGoTestPackages(paths)
	moduleDir := codequality.ModuleDir(in.Worktree)
	pkgs = buildTagVisiblePackages(ctx, moduleDir, pkgs)
	if len(pkgs) == 0 {
		return nil
	}
	// Split the changed set: ENFORCED packages run once under the coverage-
	// instrumented pass inside apicoverNamingFailures (their test run doubles
	// as the selfcheck — reviewer MED: never run the same package's tests
	// twice per handoff); everything else takes the plain selfcheck.
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
		out = append(out, fmt.Sprintf("%s: unit tests FAIL\n%s", f.Pkg, floorFailureDiagnostic(f.Output)))
	}
	out = append(out, namingFails...)
	return out
}

// floorFailureDiagnosticMax bounds one failing package's recorded output. The
// reason is not aesthetics: this text lands in the phase's failure reason and
// is what the next attempt's builder is handed as the whole story.
const floorFailureDiagnosticMax = 400

// floorFailureDiagnostic trims a failing package's `go test` output to the
// TAIL, not the head.
//
// Cycle-1268 is the record of why the direction matters: the floor kept
// output[:400], but `go test` writes its `--- FAIL` lines, panics and stack
// traces at the END, after whatever the package logged on the way. The recorded
// reason was therefore 400 bytes of repeated `[engine] WARN: Deps.TokenResolver
// is nil` and nothing else — the operator was handed noise from exactly the
// region where the diagnosis was not. Keeping the tail costs the same bytes and
// carries the verdict lines.
func floorFailureDiagnostic(output string) string {
	if len(output) <= floorFailureDiagnosticMax {
		return output
	}
	return "…" + output[len(output)-floorFailureDiagnosticMax:]
}

// buildTagVisiblePackages drops changed packages that have NO Go files under
// the default build tags — the ACS predicate packages, which are `//go:build
// acs`. `go test` on one is a SETUP failure ("build constraints exclude all Go
// files"), not a test failure, so without this filter a cycle whose diff
// carries an acs package red-lines its own build floor for a package that is
// green under `-tags acs`.
//
// Second line of defense, not the fix: the plain selfcheck path already
// tolerates this condition via goTestExcludedByBuildTags, and the primary fix
// is that modern acs packages are NOT enrolled in .apicover-enforce at all (see
// the rationale block there). This filter covers the legacy ./acs/cycle9..661
// enrollments, which would otherwise still reach the enforced coverage run.
//
// Fail-open by construction: any `go list` plumbing error returns the input
// unchanged, so a broken toolchain narrows nothing and the floor keeps judging.
func buildTagVisiblePackages(ctx context.Context, moduleDir string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return pkgs
	}
	args := append([]string{"list", "-e", "-f", "{{.Dir}}\t{{len .GoFiles}}\t{{len .TestGoFiles}}\t{{len .XTestGoFiles}}"}, pkgs...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = sanitizeEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: go list failed (%v) — build-tag visibility filter skipped this handoff\n", err)
		return pkgs
	}
	empty := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) != 4 {
			continue
		}
		if f[1] == "0" && f[2] == "0" && f[3] == "0" {
			empty[filepath.Clean(f[0])] = true
		}
	}
	if len(empty) == 0 {
		return pkgs
	}
	kept := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		dir := filepath.Clean(filepath.Join(moduleDir, strings.TrimPrefix(p, "./")))
		if empty[dir] {
			fmt.Fprintf(os.Stderr, "[build-floor] NOTE: %s has no Go files under the default build tags (build-tagged package) — excluded from the selfcheck run\n", p)
			continue
		}
		kept = append(kept, p)
	}
	return kept
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
	// ADR-0080 P1: the coverage run doubles as the enforced packages'
	// selfcheck — a full go-test execution, host-wide single-flight for the
	// same reason as the EGPS suite (batch-16 contention false-reds). A lock
	// failure degrades to unserialized, never to skipped verification.
	if release, lerr := verifylock.Acquire(ctx, filepath.Dir(moduleDir), os.Stderr); lerr == nil {
		defer release()
	} else {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: verification single-flight unavailable (%v) — running unserialized\n", lerr)
	}
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
