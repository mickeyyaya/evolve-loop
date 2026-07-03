//go:build acs

// Package cycle476 materialises the cycle-476 acceptance criteria for the two
// triage-committed tasks (## top_n only, operator priority override T2 —
// advisor tier-emission determinism):
//
//	advisor-real-persona-liveness-golden  (go/internal/core/
//	  phase_advisor_tier_elicitation_test.go) → C476_001, C476_002
//	advisor-persona-tier-example-harmonize (agents/evolve-router.md,
//	  config-only) → turns C476_001/C476_002 GREEN
//
// Root cause (scout): the operator's three literal deliverables already ship on
// main (Go {cli,tier} example #293, liveness golden, overlay-log goldens). The
// RESIDUAL intermittency is a competing bare existing-phase example INSIDE the
// shipped persona agents/evolve-router.md (frontmatter output-format :10 and the
// body example :35), which appears BEFORE and undermines the Go-appended
// {cli,tier} example. Every prior advisor-prompt test used a STUB persona
// (WithPersona("PERSONA BODY")) and was structurally blind to it. Task 1 adds the
// missing real-persona test class (RED today); Task 2 harmonizes the persona
// (config-only) to turn it GREEN.
//
// 1:1 AC-materialization: 4 predicates + 1 manual+checklist + 0 removed = 5 ACs
// total (see .evolve/evals/advisor-real-persona-liveness-golden.md and
// .evolve/evals/advisor-persona-tier-example-harmonize.md), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"):
// C476_001 and C476_002 are RED because the two real-persona goldens fail their
// assertions today — the persona body example :35 lacks {cli,tier} and the
// frontmatter output-format :10 omits cli/tier. C476_003 (overlay-log goldens)
// and the degrade/tier-confinement legs of C476_002 are pre-existing GREEN
// regression pins (they lock behavior Task 2 must NOT relax) — declared as such
// per the AC-Materialization Contract. C476_004 (CI-parity) greens once
// internal/core compiles and the goldens pass.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C476_001's >=2-examples floor — deleting the persona example
//	            (leaving only the Go one) must NOT green the golden; the persona
//	            itself must teach the tiered schema. C476_002's
//	            AbsentCLITierFieldsStayEmpty — a "add the schema text but break
//	            absent-field parsing" fake must not survive.
//	Edge/OOD:   C476_002's sanitizeAdvisorTier confinement (high/top/raw-model/
//	            empty all rejected — no clamp/floor relaxation widening the
//	            vocabulary).
//	Semantic:   C476_001's body-example surface vs the frontmatter output-format
//	            surface are DISTINCT — a fix to the body alone must not silently
//	            satisfy the frontmatter enumeration, and vice versa.
package cycle476

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
const runnerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"

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

// TestC476_001_RealPersonaComposedPromptCarriesTierAndCLI (AC1, behavioral +
// negative): the PRODUCTION composed plan prompt built with the REAL shipped
// agents/evolve-router.md must show {cli,tier} on EVERY existing-phase
// response-schema example, with NO surviving bare example. Exercises the SUT
// (composePlanPrompt over the real persona, not a stub). The golden's >=2-example
// floor is the anti-delete guard — the persona itself must teach the tiered
// schema, not omit its example. RED today (persona body :35 is bare); Task 2's
// config edit turns it GREEN.
func TestC476_001_RealPersonaComposedPromptCarriesTierAndCLI(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_RealPersonaExistingExampleCarriesTierAndCLI", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("real-persona composed-prompt liveness golden is red (exit=%d) — a bare existing-phase example survives in agents/evolve-router.md, competing with the Go {cli,tier} example\n%s", code, out)
	}
}

// TestC476_002_PersonaFrontmatterAndDegradeContract (AC2/AC3, semantic + negative
// + edge): three distinct surfaces bound together.
//   - Frontmatter output-format (:10) enumerates cli/tier (semantic surface
//     DISTINCT from the body example) — RED today, greened by Task 2.
//   - AbsentCLITierFieldsStayEmpty: a legacy-shape response (no cli/tier keys)
//     still parses to empty CLI/Tier so no overlay is proposed — the byte-
//     identical-degrade pin that a "fake the schema text, break absent-field
//     parsing" gaming edit must not defeat (pre-existing GREEN).
//   - sanitizeAdvisorTier confinement: high/top/raw-model/empty all rejected —
//     no clamp/floor relaxation widening the tier vocabulary (pre-existing GREEN).
func TestC476_002_PersonaFrontmatterAndDegradeContract(t *testing.T) {
	out, code := runGoTest(t,
		"TestRealPersonaFrontmatterOutputFormatEnumeratesTierCLI|TestParsePhasePlan_AbsentCLITierFieldsStayEmpty|TestSanitizeAdvisorTier_RejectsHighTopAndRawModel",
		true, corePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("frontmatter-enumeration + degrade + tier-confinement contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC476_003_OverlayLogGoldensRemainGreen (AC4, locked regression pin): the P3
// overlay-observability goldens — the `advisor overlay cli=.. tier=..` and `no
// advisor overlay (profile default)` log shapes — must stay GREEN. Task 2 is
// config-only and must not perturb the runner's overlay logging (pre-existing
// GREEN; the pin's value is surviving this cycle's edit and future changes).
func TestC476_003_OverlayLogGoldensRemainGreen(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_AdvisorOverlayAppliedLogsLine|TestRunner_NoAdvisorOverlayLogsProfileDefault", true, runnerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("overlay-log goldens are red (exit=%d) — the locked observability shapes changed\n%s", code, out)
	}
}

// TestC476_004_CIParityCoreRaceVetApicover (AC5, CI-parity + boundary): the full
// internal/core package must pass under -race, go vet must be clean, and apicover
// -enforce over internal/core must stay clean (the cycle adds only unexported
// test helpers — zero new exported symbols — so apicover must not regress; guards
// the cycle-413 WARN-ship class). Mirrors the exact repo-wide CI on the touched
// package.
func TestC476_004_CIParityCoreRaceVetApicover(t *testing.T) {
	out, code := runGoTest(t, "", true, corePkg)
	if code != 0 {
		t.Errorf("full-package -race regression on internal/core is red (exit=%d)\n%s", code, out)
	}
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	vetOut, _, vetCode, _ := acsassert.SubprocessOutput("bash", "-c", "cd "+goDir+" && go vet ./internal/core/...")
	if vetCode != 0 {
		t.Errorf("go vet ./internal/core/... is red (exit=%d)\n%s", vetCode, vetOut)
	}
	apicoverCmd := "cd " + goDir + " && " +
		"go build -o bin/apicover ./cmd/apicover && " +
		"go test -coverprofile=coverage.core476.txt ./internal/core/ >/dev/null && " +
		"go tool cover -func=coverage.core476.txt > coverage.core476.func.txt && " +
		"bin/apicover -enforce -cover coverage.core476.func.txt $(go list -f '{{.Dir}}' ./internal/core)"
	apiOut, _, apiCode, _ := acsassert.SubprocessOutput("bash", "-c", apicoverCmd)
	if apiCode != 0 {
		t.Errorf("apicover -enforce over internal/core is red (exit=%d)\n%s", apiCode, apiOut)
	}
}
