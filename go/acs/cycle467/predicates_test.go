//go:build acs

// Package cycle467 materialises the cycle-467 acceptance criteria for the
// single triage-committed task (## top_n only, operator priority override):
//
//	fleet-s3-guards (go/internal/fleet/preflight.go+quota,
//	go/cmd/evolve/cmd_loop_wave.go ctx threading, wave-level disjointness
//	pin) → C467_001..006
//
// FLEET-AS-POLICY S3: (a) dirty-control-plane wave preflight via
// guards.IsProtectedSurface — helper OUTSIDE internal/guards, which is itself
// protected surface; (b) quota-aware Count shrink off clihealth.Store.Active()
// (min 1, WARN naming family+reason); (c) wave-level file-disjointness
// regression pin on PlanWaves/PlanFromTriage output specs; (d) PR #298
// reviewer note — thread the loop's cancellable ctx through wavePlanFn and
// kill the context.Background() mint at cmd_loop_wave.go:116.
//
// 1:1 AC-materialization: 6 predicates + 0 manual+checklist + 0 removed = 6
// ACs total (see .evolve/evals/fleet-s3-guards.md), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"):
// go/internal/fleet fails to COMPILE (preflight_test.go / quota_test.go
// reference PreflightControlPlane / QuotaAwareCount, which do not exist yet)
// and go/cmd/evolve fails to COMPILE (cmd_loop_wave_s3_test.go pins the
// post-S3 dispatchIteration/wavePlanFn signatures) — so C467_001..004 and
// C467_006 are red on those subprocess compile failures. C467_005 is
// additionally red on its own direct assertion: context.Background() is
// still PRESENT in cmd_loop_wave.go. The two wave-disjointness pins
// (C467_004's inner tests) passed standalone BEFORE the contract files
// landed — they are pre-existing-GREEN regression pins by design (AC4 pins
// existing behavior at a previously-unpinned level).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C467_001 (dirty control plane MUST refuse, naming file +
//	            remediation, launcher/planFn never invoked — kills a
//	            preflight that always passes or fires after launch),
//	            C467_005's cancelled-ctx leg (cancellation must surface,
//	            never a silent launch)
//	Edge/OOD:   C467_002's not-a-git-repo fail-loud leg, C467_003's min-1
//	            clamp (more benches than count; count already 1),
//	            C467_004's duplicate-id collapse
//	Semantic:   C467_001 vs C467_002 (refusing dirt is DISTINCT from not
//	            false-positiving on clean/innocent dirt — a guard that
//	            refuses everything passes 001 but fails 002); C467_003's
//	            shrink vs no-bench pass-through
package cycle467

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fleetPkg = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	cmdPkg   = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

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

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
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

// TestC467_001_DirtyControlPlaneRefusedActionably (AC1, negative): a
// modified tracked .evolve/policy.json or an untracked skills/audit/
// addition must refuse the wave, the refusal must name the offending path
// AND the `evolve ship --class manual` remediation, and at the dispatch seam
// a refusal must fire BEFORE planFn/launcher run.
func TestC467_001_DirtyControlPlaneRefusedActionably(t *testing.T) {
	fleetOut, fleetCode := runGoTest(t, "TestPreflightControlPlane_DirtyPolicyRefusedWithActionableMessage|TestPreflightControlPlane_UntrackedControlPlaneAdditionRefused", true, fleetPkg)
	requireTestsRan(t, fleetOut, 2)
	if fleetCode != 0 {
		t.Errorf("PreflightControlPlane dirty-refusal contract is red (exit=%d)\n%s", fleetCode, fleetOut)
	}
	cmdOut, cmdCode := runGoTest(t, "TestDispatchIteration_PreflightRefusalNeverPlansNorLaunches", true, cmdPkg)
	requireTestsRan(t, cmdOut, 1)
	if cmdCode != 0 {
		t.Errorf("dispatchIteration preflight-refusal seam contract is red (exit=%d)\n%s", cmdCode, cmdOut)
	}
}

// TestC467_002_CleanAndInnocentTreesNeverFalsePositive (AC2): a clean tree
// and non-control-plane dirt must pass the preflight; an unverifiable
// (non-git) root must fail LOUD; the sequential (Count==1) path must never
// even run the preflight; and a passing preflight must leave wave dispatch
// byte-identical to the pre-guard behavior.
func TestC467_002_CleanAndInnocentTreesNeverFalsePositive(t *testing.T) {
	fleetOut, fleetCode := runGoTest(t, "TestPreflightControlPlane_CleanTreePasses|TestPreflightControlPlane_NonControlPlaneDirtIgnored|TestPreflightControlPlane_NotAGitRepoErrors", true, fleetPkg)
	requireTestsRan(t, fleetOut, 3)
	if fleetCode != 0 {
		t.Errorf("PreflightControlPlane no-false-positive/fail-loud contract is red (exit=%d)\n%s", fleetCode, fleetOut)
	}
	cmdOut, cmdCode := runGoTest(t, "TestDispatchIteration_PreflightCleanWaveProceeds|TestDispatchIteration_SequentialPathNeverRunsPreflight", true, cmdPkg)
	requireTestsRan(t, cmdOut, 2)
	if cmdCode != 0 {
		t.Errorf("dispatchIteration clean-preflight/sequential-skip contract is red (exit=%d)\n%s", cmdCode, cmdOut)
	}
}

// TestC467_003_QuotaAwareCountShrinksWithMinOneClamp (AC3): a benched
// required family shrinks the effective lane count (WARN names family +
// reason), the clamp never drops below 1, and zero benches pass through
// silently. Supplementary wiring pin: the loop's wave path must actually
// consult QuotaAwareCount (the pure function alone greening while cmd_loop
// ignores it would be a no-op ship).
func TestC467_003_QuotaAwareCountShrinksWithMinOneClamp(t *testing.T) {
	out, code := runGoTest(t, "TestQuotaAwareCount", true, fleetPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("QuotaAwareCount shrink/clamp/pass-through contract is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	cmdLoop := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go")
	cmdLoopWave := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_wave.go")
	if !acsassert.FileContainsAny(cmdLoop, "QuotaAwareCount") && !acsassert.FileContainsAny(cmdLoopWave, "QuotaAwareCount") {
		t.Errorf("neither cmd_loop.go nor cmd_loop_wave.go references QuotaAwareCount — the shrink is not wired into the live wave path")
	}
}

// TestC467_004_WaveLevelFileDisjointnessPinned (AC4, regression pin): the
// wave-level disjointness tests — overlapping file scopes never co-schedule
// within one wave's []CycleSpec on either PlanWaves or PlanFromTriage output
// — must exist and be green (they were verified pre-existing-GREEN
// standalone; the pin's value is surviving future planner changes).
func TestC467_004_WaveLevelFileDisjointnessPinned(t *testing.T) {
	out, code := runGoTest(t, "TestPlanWaves_WaveLevelFileDisjoint|TestPlanFromTriage_WaveLevelFileDisjoint", true, fleetPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("wave-level file-disjointness regression pin is red (exit=%d)\n%s", code, out)
	}
}

// TestC467_005_CtxThreadedThroughPlanPath (AC5, PR #298 reviewer note): the
// caller's ctx must reach the plan function (context-value probe) and a
// cancelled ctx must be observable there and surface errors.Is-matchably —
// AND the context.Background() mint must be GONE from cmd_loop_wave.go
// (absence via FileNotContains, the verifiableBy `grep -c == 0` clause).
func TestC467_005_CtxThreadedThroughPlanPath(t *testing.T) {
	out, code := runGoTest(t, "TestDispatchIteration_CtxReachesPlanFn|TestDispatchIteration_CancelledCtxObservableInPlanFn", true, cmdPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("ctx-threading contract is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	acsassert.FileNotContains(t, filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_wave.go"), "context.Background()")
}

// TestC467_006_RaceVetApicoverCleanWithZeroGuardsEdits (AC6, CI-parity +
// boundary): full -race regression on the two touched packages, go vet
// clean, apicover -enforce on internal/fleet (new exported symbols
// PreflightControlPlane/QuotaAwareCount must be named by tests AND executed
// — kills the cycle-413 WARN-ship class), and ZERO edits under
// go/internal/guards/ (the preflight must IMPORT the protected predicate,
// never modify its package): neither uncommitted nor committed-since-main.
func TestC467_006_RaceVetApicoverCleanWithZeroGuardsEdits(t *testing.T) {
	out, code := runGoTest(t, "", true, fleetPkg, cmdPkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/fleet + cmd/evolve is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./cmd/evolve/... ./internal/fleet/...")
	if vetCode != 0 {
		t.Errorf("go vet ./cmd/evolve/... ./internal/fleet/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	apicoverCmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.s3guards467.txt ./internal/fleet/ >/dev/null && " +
		"go tool cover -func=coverage.s3guards467.txt > coverage.s3guards467.func.txt && " +
		"bin/apicover -enforce -cover coverage.s3guards467.func.txt $(go list -f '{{.Dir}}' ./internal/fleet)"
	apiOut, _, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/fleet is red (exit=%d)\n%s", apiCode, apiOut)
	}
	statusOut, statusErr, statusCode, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--", "go/internal/guards/")
	if statusCode != 0 {
		t.Errorf("git status on go/internal/guards/ failed (exit=%d)\n%s", statusCode, statusErr)
	} else if strings.TrimSpace(statusOut) != "" {
		t.Errorf("uncommitted edits under go/internal/guards/ (protected surface — the cycle must never touch it):\n%s", statusOut)
	}
	diffOut, diffErr, diffCode, _ := acsassert.SubprocessOutput("bash", "-c",
		`cd `+root+` && git diff --name-only "$(git merge-base main HEAD)" HEAD -- go/internal/guards/`)
	if diffCode != 0 {
		t.Errorf("git diff merge-base check on go/internal/guards/ failed (exit=%d)\n%s", diffCode, diffErr)
	} else if strings.TrimSpace(diffOut) != "" {
		t.Errorf("committed edits under go/internal/guards/ since main (protected surface):\n%s", diffOut)
	}
}
