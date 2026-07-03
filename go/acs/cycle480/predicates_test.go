//go:build acs

// Package cycle480 materialises the cycle-480 acceptance criteria.
//
// TRIAGE COMMITTED TWO ## top_n TASKS, but only ONE is materializable this
// cycle:
//
//	universal-envelope-floor        (go/internal/router/model_routing_clamp.go)
//	  → C480_001, C480_002, C480_003
//	egps-timeout-loud-diagnostic    (go/internal/acssuite/acssuite.go) — DROPPED
//	  → the fix locus is the PROTECTED CONTROL-PLANE SURFACE
//	    `/go/internal/acssuite/` (the EGPS gate runner; see
//	    go/internal/guards/integrity_surface.go). A cycle may not edit the gate
//	    that grades it (ADR-0064). Builder would be denied identically, so no
//	    predicate can bind to it this cycle. Its ACs are dispositioned
//	    `unverifiable-remove` (cycle-scoped) in test-report.md with the
//	    recommendation to route it via `evolve ship --class manual` OUTSIDE a
//	    cycle. Predicates bind ONLY to buildable committed work (R9.3).
//
// Task 1 root cause (scout Key Finding 3): the operator low-model floor at
// model_routing_clamp.go:52 gates the clamp-up entirely on
// `prof.ModelTierEnvelope != nil`. 72/91 profiles declare NO envelope, so a
// below-floor `tier:fast` proposal against a nil-envelope profile falls through
// to policy.ValidatePin, which (by design B2) treats "no envelope configured" as
// a PREFERENCE — and is never clamped up to the balanced floor. The fix
// substitutes a compiled-default envelope {min:balanced, max:deep} at the clamp
// site when the profile declares none, so the floor is UNIVERSAL.
//
// 1:1 AC-materialization (Task 1): 4 predicate ACs + 1 CI-parity AC = 5 ACs,
// none double-counted (see .evolve/evals/universal-envelope-floor.md).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:  C480_002's ExplicitEnvelopeNotOverridden — a fix that applies the
//	           default UNCONDITIONALLY (clobbering an explicit envelope that
//	           permits fast) must NOT survive.
//	Edge/OOD:  C480_002's WithinCeilingPassesThrough — a nil-envelope proposal
//	           already within the default ceiling ("deep") must pass unclamped.
//	Anti-game: C480_001's AppliesAcrossPhases — the floor must be universal, not
//	           hardcoded to the single phase name AC1 uses.
//	Semantic:  the clamp-UP surface (C480_001) is distinct from the no-over-clamp
//	           surface (C480_002); satisfying one must not silently satisfy the
//	           other.
//
// RED strategy (verified in test-report.md "RED Run Output"): C480_001 is RED
// because model_routing_clamp.go has no universal floor today — a nil-envelope
// fast proposal stays fast (zero clamps). C480_003 (full-package CI-parity) is
// RED for the same reason (it runs the new RED unit tests). C480_002's two legs
// are pre-existing-correct boundary pins (GREEN today) that a plausible buggy
// fix would break — declared as such per the AC-Materialization Contract.
package cycle480

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"

// runGoTest runs `go test` on an internal package under -race and returns the
// combined output + exit code. Behavioral predicates invoke the system under
// test (no source-grep gaming).
func runGoTest(t *testing.T, runFilter string, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-race", "-v"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with no
// matching test exits 0 with "no tests to run", which would green a predicate on
// unwritten (or renamed) work.
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

// TestC480_001_NilEnvelopeFloorClampsUpUniversally (Task1-AC1 behavioral +
// AC4 anti-gaming): a below-floor tier proposed against a profile with NO
// declared envelope must be clamped UP to the universal balanced floor, and the
// floor must apply across EVERY phase (not a hardcoded phase name). Exercises
// the SUT (ClampPlanModelRouting). RED today — no universal floor exists.
func TestC480_001_NilEnvelopeFloorClampsUpUniversally(t *testing.T) {
	out, code := runGoTest(t,
		"TestClampPlanModelRouting_NilEnvelopeFloorClampsUp|TestClampPlanModelRouting_NilEnvelopeFloorAppliesAcrossPhases",
		routerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("universal-envelope-floor clamp-up is red (exit=%d) — a nil-envelope profile still lets a below-floor tier through\n%s", code, out)
	}
}

// TestC480_002_EnvelopeFloorDoesNotOverClamp (Task1-AC2 negative + AC3 edge):
// the two boundary pins. The compiled default must apply ONLY when no envelope
// is declared — an explicit envelope that permits fast is honored (negative:
// a fix that clobbers explicit envelopes turns this RED), and a nil-envelope
// proposal already within the default ceiling passes through unclamped (edge).
// Both are correct today; the pin's value is surviving this cycle's edit.
func TestC480_002_EnvelopeFloorDoesNotOverClamp(t *testing.T) {
	out, code := runGoTest(t,
		"TestClampPlanModelRouting_ExplicitEnvelopeNotOverriddenByDefault|TestClampPlanModelRouting_NilEnvelopeWithinCeilingPassesThrough",
		routerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("envelope-floor over-clamp guard is red (exit=%d) — the default floor either overrode an explicit envelope or clamped a within-ceiling tier\n%s", code, out)
	}
}

// TestC480_003_RouterCIParity (Task1-AC5 CI-parity + boundary): the full
// internal/router package must pass under -race, go vet must be clean, and
// apicover -enforce over internal/router must stay clean (the cycle adds only
// test files + an unexported clamp-site change — zero new exported symbols — so
// apicover must not regress; guards the cycle-413 WARN-ship class). Mirrors the
// exact repo-wide CI on the touched package.
func TestC480_003_RouterCIParity(t *testing.T) {
	out, code := runGoTest(t, "", routerPkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/router is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./internal/router/...")
	if vetCode != 0 {
		t.Errorf("go vet ./internal/router/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	apicoverCmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.router480.txt ./internal/router/ >/dev/null && " +
		"go tool cover -func=coverage.router480.txt > coverage.router480.func.txt && " +
		"bin/apicover -enforce -cover coverage.router480.func.txt $(go list -f '{{.Dir}}' ./internal/router)"
	apiOut, _, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/router is red (exit=%d)\n%s", apiCode, apiOut)
	}
}
