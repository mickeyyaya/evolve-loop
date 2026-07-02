//go:build acs

// Package cycle464 materialises the cycle-464 acceptance criteria for the
// two triage-committed tasks (## top_n only): S1 of the FLEET-AS-POLICY
// operator-priority goal (backlog deferred by explicit operator order —
// see scout-report.md Carryover Decisions).
//
//	fleet-policy-block (P1, go/internal/policy/policy.go)  → C464_001..005
//	fleet-policy-docs  (P2, docs-only, depends on P1)      → C464_006..009
//
// 1:1 AC-materialization: 9 predicates + 1 manual+checklist (the
// fleet-policy-docs [model] "Table accuracy" grader, disposed in
// test-report.md — no automated predicate stands in for a model grader)
// + 0 removed = 10 ACs total (5 in evals/fleet-policy-block.md,
// 5 in evals/fleet-policy-docs.md), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): C464_001-004
// fail because go/internal/policy/fleet_config_param_test.go references
// policy.FleetPolicy/FleetConfig/FleetConfig(), which do not exist yet —
// the internal/policy package fails to COMPILE, so every test that touches
// it (including the whole-package regression C464_003 and the apicover
// naming sweep C464_004, which needs a green coverage profile) is red for
// the same root cause. C464_005 is a PRE-EXISTING GREEN config-check (no
// production reader exists yet for the not-yet-invented env names — this
// predicate exists to prevent their future introduction, mirroring the
// cycle-22 dead-flag contract). C464_006-008 fail because neither
// docs/operations/runtime-reference.md nor docs/architecture/control-flags.md
// mentions the fleet block yet. C464_009 is PRE-EXISTING GREEN for the same
// reason as C464_005 (nothing to leak yet).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C464_002 (unknown plan_source must fail safe to "manual",
//	            not pass through unchanged — kills a vocab-blind getter),
//	            C464_005/C464_009 (no new EVOLVE_FLEET_* env var may appear
//	            in production code or docs — config-driven only)
//	Edge/OOD:   C464_001 (count 0/negative clamp; concurrency 0 follows the
//	            RESOLVED count, not the raw input — the zero/negative-lane
//	            edge cases a hardcoded-defaults getter cannot fake)
//	Semantic:   C464_006 vs C464_007 (the runtime-reference KEY TABLE is a
//	            distinct requirement from control-flags.md's CLOSED-VOCAB
//	            documentation — both must hold independently; a docs stub
//	            mentioning "fleet" once satisfies neither)
package cycle464

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X`
// with no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// ---- fleet-policy-block (P1) ----

// TestC464_001_FleetConfigResolutionDefaults (AC1): count/concurrency
// resolution table under -race — absent/empty/zero/negative Count clamps
// to 1 (never a zero- or negative-lane wave), Count overrides pass
// through, and Concurrency<=0 follows the RESOLVED Count. Shells the named
// RED unit test written this cycle so a rename or deletion can never
// silently green this predicate.
func TestC464_001_FleetConfigResolutionDefaults(t *testing.T) {
	out, code := runGoTest(t, "TestFleetConfig_Resolution", true, policyPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("FleetConfig resolution contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC464_002_FleetConfigPlanSourceClosedVocab (AC2, negative/OOD): an
// unknown plan_source (e.g. "yolo") must fail safe to "manual" WITH a
// surfaced warning — never pass through unchanged, and never silently fall
// back to the "triage" default the way the swarm/parallel_evaluate
// precedents fail back to THEIR default.
func TestC464_002_FleetConfigPlanSourceClosedVocab(t *testing.T) {
	out, code := runGoTest(t, "TestFleetConfig_PlanSourceClosedVocab", true, policyPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("FleetConfig plan_source closed-vocab contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC464_003_PolicyPackageRegression (AC3): the whole internal/policy
// package stays green under -race — no regression across the existing 27+
// policy test files once the fleet block lands.
func TestC464_003_PolicyPackageRegression(t *testing.T) {
	out, code := runGoTest(t, "", true, policyPkg)
	if code != 0 {
		t.Errorf("internal/policy package regression is red (exit=%d)\n%s", code, out)
	}
}

// TestC464_004_ApicoverNamingEnforced (AC4, CI-parity): mirrors
// .github/workflows/go.yml's "api-coverage enforce" step scoped to
// internal/policy, EXACTLY the eval fleet-policy-block.md grader command —
// every new exported symbol (FleetPolicy/FleetConfig/FleetConfig) must be
// named by a test AST AND show >0% executed coverage. Kills the cycle-413
// gaming class (a new exported symbol shipped without a naming test breaks
// main CI's repo-wide apicover -enforce).
func TestC464_004_ApicoverNamingEnforced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	cmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.fleetpolicy464.txt ./internal/policy/ >/dev/null && " +
		"go tool cover -func=coverage.fleetpolicy464.txt > coverage.fleetpolicy464.func.txt && " +
		"bin/apicover -enforce -cover coverage.fleetpolicy464.func.txt $(go list -f '{{.Dir}}' ./internal/policy)"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("apicover -enforce over internal/policy is red (exit=%d)\n%s", code, out)
	}
}

// TestC464_005_NoNewFleetEnvFlags (AC5, negative, config-check): the fleet
// block is config-driven ([[no_feature_flags_use_design_patterns]]) — no
// production Go file may read EVOLVE_FLEET_COUNT/CONCURRENCY/PLAN via
// os.Getenv. PRE-EXISTING GREEN today (the names don't exist yet); this
// predicate is the standing contract that prevents their future
// introduction (mirrors the cycle-22 dead-flag pattern).
//
// acs-predicate: config-check
func TestC464_005_NoNewFleetEnvFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, name := range []string{"EVOLVE_FLEET_COUNT", "EVOLVE_FLEET_CONCURRENCY", "EVOLVE_FLEET_PLAN"} {
		_, _, code, _ := acsassert.SubprocessOutput("grep", "-rln", name,
			filepath.Join(goDir, "internal"), filepath.Join(goDir, "cmd"))
		// grep exits 0 when a match is found (BAD: a new env flag leaked in).
		// grep exits 1 when no match is found (GOOD: config-only, as required).
		if code == 0 {
			t.Errorf("production Go code under go/internal or go/cmd references %q — "+
				"the fleet block must be policy.json-only, never a new env flag", name)
		}
	}
}

// ---- fleet-policy-docs (P2, depends on P1) ----

// TestC464_006_RuntimeReferenceFleetKeyTable (AC6): EXACTLY the eval
// fleet-policy-docs.md grader — from the fleet section onward,
// runtime-reference.md must name all three keys (count/concurrency/
// plan_source), not a stub mention.
func TestC464_006_RuntimeReferenceFleetKeyTable(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	cmd := "awk '/fleet/,0' " + doc + " | grep -c -E 'plan_source|concurrency|count'"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("fleet key-table grep is red (exit=%d, likely zero matches — the fleet section is absent)\n%s", code, out)
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Errorf("fleet key-table match count %q did not parse: %v", out, err)
		return
	}
	if n < 3 {
		t.Errorf("fleet key-table match count = %d, want >= 3 (count/concurrency/plan_source all present, not just a stub \"fleet\" mention)", n)
	}
}

// TestC464_007_ControlFlagsClosedVocabDocumented (AC7): EXACTLY the eval
// fleet-policy-docs.md grader — control-flags.md's plan_source line must
// name BOTH vocabulary values ("triage" and "manual"), so the fail-safe
// direction (unknown -> manual) is documented, not just the default.
func TestC464_007_ControlFlagsClosedVocabDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	cmd := "grep -E 'plan_source' " + doc + " | grep -E 'triage' | grep -c -E 'manual'"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("control-flags.md plan_source closed-vocab documentation is red (exit=%d, expected >= 1 line naming both triage and manual)\n%s", code, out)
	}
}

// TestC464_008_SequentialDefaultStated (AC8): EXACTLY the eval
// fleet-policy-docs.md grader — the byte-identical-sequential-by-default
// guarantee must be STATED in runtime-reference.md next to the fleet/count
// language, not merely implied.
func TestC464_008_SequentialDefaultStated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	cmd := "grep -rn -i 'sequential' " + doc + " | grep -c -i 'fleet\\|count'"
	out, _, code, _ := acsassert.SubprocessOutput("bash", "-c", cmd)
	if code != 0 {
		t.Errorf("sequential-default statement is red (exit=%d, expected >= 1 line pairing \"sequential\" with fleet/count)\n%s", code, out)
	}
}

// TestC464_009_NoNewFleetEnvFlagsInDocs (AC9, negative, config-check): the
// docs must never document a new EVOLVE_FLEET_* env var — the fleet block
// is policy.json-only. PRE-EXISTING GREEN today; standing contract against
// future drift.
//
// acs-predicate: config-check
func TestC464_009_NoNewFleetEnvFlagsInDocs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docsDir := filepath.Join(root, "docs")
	for _, name := range []string{"EVOLVE_FLEET_COUNT", "EVOLVE_FLEET_CONCURRENCY", "EVOLVE_FLEET_PLAN"} {
		_, _, code, _ := acsassert.SubprocessOutput("grep", "-rln", name, docsDir)
		if code == 0 {
			t.Errorf("docs reference %q — the fleet block must be policy.json-only, never a new env flag", name)
		}
	}
}
