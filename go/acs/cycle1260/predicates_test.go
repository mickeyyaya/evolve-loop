//go:build acs

// Package cycle1260 materialises the cycle-1260 acceptance criteria for the one
// fleet-scoped, triage-committed task:
//
//	egps-regression-tia-shadow-wiring  (inbox P1 0.91, 3rd live instance)
//
// The item asks for deterministic test-impact selection over the EGPS Go
// regression corpus, staged off → shadow → enforce via .evolve/policy.json,
// with shadow logging would-skip counts before anything is ever skipped.
//
// CONTROL-PLANE DEVIATION (recorded loudly; see test-report.md §Deviation).
// scout-report targeted go/internal/acssuite/acssuite.go. That path is
// PROTECTED CONTROL PLANE — guards.ProtectedSurfaceManifest carries
// {"/go/internal/acssuite/", "the gate runner"} — so NO phase of any cycle may
// write it (verified live: the role guard DENIED + alarmed this phase's write).
// Honoring the boundary yields the better design: the shadow stage changes
// nothing about what the gate runs, so it needs no code in the gate runner. It
// is observability computed beside the suite by the suite's own production
// caller, the audit phase (internal/phases/audit/audit.go:638 generateACSVerdict).
// Scout's Task-1 acceptance intent is preserved verbatim: off/absent stays
// byte-identical, shadow logs would-skip evidence, nothing is skipped yet.
//
// Predicate strategy — every predicate EXECUTES the system under test through a
// scoped subprocess and asserts on its exit code (the cycle-85 degenerate-
// predicate ban: no predicate's load-bearing assertion is a source grep).
// Each invocation names exactly ONE package and narrows it with -run, per the
// flaky-predicate-shape rules (no ./... sweeps, no whole-repo staleness checks,
// no wall-clock bounds, no literal PIDs, cmd.Dir always set explicitly).
//
//   - 001 the policy config surface (off default, closed vocabulary, typo ⇒ off)
//   - 002 selection semantics + the fail-safes that keep selection from ever
//     hiding a regression class
//   - 003 the ImporterClosure wiring proof — the cycle-1250 router/routingtest
//     reproducer, run against the REAL repository import graph
//   - 004 the shadow decision is computed and emitted as a readable artifact
//   - 005 the CRUX reachability proof: the decision is emitted from the real
//     audit-phase path, so the selection logic is not dead code (the exact
//     shape ImporterClosure sat in since cycle-1253 — GREEN, zero callers)
//   - 006 new-package graduation: the repo-wide apicover gate's dual edit
package cycle1260

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTestTimeout bounds one scoped package run. Derived from the invocation, not
// from wall-clock measurement of the system under test: nothing here asserts on
// elapsed time, so contention slows a predicate but can never false-RED it.
const goTestTimeout = 10 * time.Minute

// runScoped runs `go test -count=1 -run <pattern> <pkg>` from the module dir and
// returns its combined output plus the exit code. cmd.Dir is set explicitly —
// never inherited from the process cwd, which differs between the main tree,
// the cycle worktree, and each fleet lane.
func runScoped(t *testing.T, pkg, runPattern string, extraArgs ...string) (string, int) {
	t.Helper()
	moduleDir := filepath.Join(acsassert.RepoRoot(t), "go")

	ctx, cancel := context.WithTimeout(context.Background(), goTestTimeout)
	defer cancel()

	args := append([]string{"test", "-count=1", "-run", runPattern}, extraArgs...)
	args = append(args, pkg)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.WaitDelay = 30 * time.Second
	out, err := cmd.CombinedOutput()

	code := 0
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return string(out), code
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

// TestC1260_001_regression_tia_policy_stage is the config surface: the staged
// rollout is config-as-code in .evolve/policy.json, resolved through an
// accessor whose default is "off" (the checked-in policy.json has no
// regression_tia block) and whose unknown-value fallback is "off" — a typo must
// never silently arm test selection.
func TestC1260_001_regression_tia_policy_stage(t *testing.T) {
	out, code := runScoped(t, "./internal/policy", "TestRegressionTIAConfig")
	if code != 0 {
		t.Errorf("policy.RegressionTIAConfig contract is not satisfied (exit %d).\n%s", code, tail(out))
	}
}

// TestC1260_002_selection_failsafes pins the selection semantics AND the
// fail-safes that make selection safe at all: an unknown scope or unresolvable
// dependency data must skip NOTHING. Selection that can hide a regression class
// is the failure mode this whole item exists to prevent.
func TestC1260_002_selection_failsafes(t *testing.T) {
	out, code := runScoped(t, "./internal/regressiontia", "TestSelect_")
	if code != 0 {
		t.Errorf("regressiontia.Select semantics/fail-safes are not satisfied (exit %d).\n%s", code, tail(out))
	}
}

// TestC1260_003_importer_closure_wired is the cycle-1250 reproducer: a change
// confined to internal/router MUST widen to internal/routingtest, which imports
// it and holds the keystone parity invariant. Forward-only scope would mark the
// routing regression package skippable — the miss that kept main red for 5
// commits. Runs against the real repository import graph.
func TestC1260_003_importer_closure_wired(t *testing.T) {
	out, code := runScoped(t, "./internal/regressiontia", "TestChangedScope_")
	if code != 0 {
		t.Errorf("reverse-dependency widening (changedpkgs.ImporterClosure) is not wired into the scope derivation (exit %d).\n%s", code, tail(out))
	}
}

// TestC1260_004_shadow_decision_emitted pins the evidence artifact: the shadow
// decision is computed only when armed, names the corpus it reasoned about, and
// round-trips as readable JSON — the operator's only view of what selection
// WOULD have done before enforce is ever considered.
func TestC1260_004_shadow_decision_emitted(t *testing.T) {
	out, code := runScoped(t, "./internal/regressiontia", "TestCompute_|TestEmit_")
	if code != 0 {
		t.Errorf("shadow decision compute/emit contract is not satisfied (exit %d).\n%s", code, tail(out))
	}
}

// TestC1260_005_audit_phase_reachability is the CRUX. A seam whose only caller
// is a test is dead code: changedpkgs.ImporterClosure shipped GREEN in
// cycle-1253 with ZERO callers, so the fix never executed once. This predicate
// requires the decision to be emitted from generateACSVerdict — the real
// audit-phase function that runs the EGPS suite — with off/absent policy
// leaving that path byte-identical and a broken evidence sink never failing the
// audit.
func TestC1260_005_audit_phase_reachability(t *testing.T) {
	out, code := runScoped(t, "./internal/phases/audit", "TestGenerateACSVerdict_")
	if code != 0 {
		t.Errorf("the shadow decision has no production caller in the audit phase (exit %d) — selection logic that never runs is the cycle-1253 dead-code shape repeated.\n%s", code, tail(out))
	}
}

// TestC1260_006_new_package_graduation is the repo-wide apicover gate's DUAL
// edit (ADR-0069, distinct from the per-cycle ACS coverage gate): a new
// go/internal/<pkg> must be enrolled in go/.apicover-enforce AND carry real
// assertions over every exported symbol. The enrollment line is an inherent
// config-presence check; the load-bearing half below EXECUTES the package and
// requires the gate's own >=85% line-coverage floor.
//
// acs-predicate: config-check
func TestC1260_006_new_package_graduation(t *testing.T) {
	enroll := filepath.Join(acsassert.RepoRoot(t), "go", ".apicover-enforce")
	raw, err := os.ReadFile(enroll)
	if err != nil {
		t.Fatalf("read %s: %v", enroll, err)
	}
	if !hasEnrollLine(string(raw), "./internal/regressiontia") {
		t.Errorf("go/.apicover-enforce does not enroll ./internal/regressiontia — an unenrolled new package aborts the build phase (cycle-1218: three lanes, one halt, same cause)")
	}

	out, code := runScoped(t, "./internal/regressiontia", "Test", "-cover")
	if code != 0 {
		t.Fatalf("internal/regressiontia does not build/pass under -cover (exit %d).\n%s", code, tail(out))
	}
	pct, ok := coveragePercent(out)
	if !ok {
		t.Fatalf("no coverage figure in `go test -cover ./internal/regressiontia` output:\n%s", tail(out))
	}
	if pct < 85.0 {
		t.Errorf("internal/regressiontia line coverage = %.1f%%, want >= 85%% (the apicover Phase-5 Definition of Done the enrollment line commits to)", pct)
	}
}

// hasEnrollLine reports whether pattern appears as its own non-comment line.
func hasEnrollLine(body, pattern string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == pattern {
			return true
		}
	}
	return false
}

var coverageRe = regexp.MustCompile(`coverage:\s+([0-9.]+)% of statements`)

func coveragePercent(out string) (float64, bool) {
	m := coverageRe.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return pct, true
}

// tail keeps the last ~40 lines of a failing run — where `go test` accumulates
// the `--- FAIL:` detail a reader needs.
func tail(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}
