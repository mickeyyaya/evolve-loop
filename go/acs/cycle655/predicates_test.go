//go:build acs

// Package cycle655 encodes the acceptance criteria for the COMPLETION of
// builder-task-binding-topn-gate (8th recurrence of the wrong-task-build
// disease: cycles 282, 310, 522, 575, 577, 599, 640, 645). Cycle-652 built the
// gate correctly (audit PASS 0.94, adversarial PASS, its 4 ACS predicates
// green) but FAILed ship on ONE missing chore: the new package
// go/internal/topngate was never graduated into go/.apicover-enforce with an
// apicover_named_test.go, so the repo-wide completeness regression
// (TestApicoverEnforce_CoversEveryInternalPackage) would go RED once the
// package landed. This is the 3rd recurrence of the new-package-graduation gap
// (cycle 575 binaryguard, cycle 587 ciwatch, cycle 652 topngate).
//
// These predicates are BEHAVIORAL: 001-004 shell `go test` against the real
// go/internal/topngate package (the system under test) and 005 shells the real
// repo-wide apicover completeness gate, rather than grepping source — so a
// predicate greens only when the gate actually blocks/approves the right builds
// AND the package is genuinely graduated, not when a magic string is present.
// The white-box unit suite the Builder carries forward (verbatim from the
// cycle-652 worktree) lives at
// go/internal/topngate/{gate_test.go,reviewer_test.go,builder_authority_test.go}.
package cycle655

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTest runs `go test <args...>` inside the worktree's go/ module dir and
// returns combined output plus the exit code. Exit 0 == the named tests passed.
// Requires the go toolchain (always present in this repo).
func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	full := append([]string{"test"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go test failed to run: %v\n%s", err, out)
	return string(out), -1
}

// TestC655_001_OutOfLaneBuildBlocked re-verifies AC-1: a build report claiming a
// slug outside triage ## top_n is BLOCKED at the build->audit transition with a
// recorded abort_reason (table-driven: in-lane passes, out-of-lane blocks).
// Exercised by the real white-box gate suite, which drives
// topNBindingGate.check and NewReviewer(config.StageEnforce).Review over the
// in-lane / out-of-lane / multi-member / fail-open table. RED until the Builder
// carries the topngate package forward into this worktree.
func TestC655_001_OutOfLaneBuildBlocked(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "-run", "TestTopNBindingGate|TestNewReviewer_Enforce", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-1: the out-of-lane build->audit block suite must PASS; got exit=%d\n%s", code, out)
	}
}

// TestC655_002_BuilderPromptNamesTopNSoleAuthority re-verifies AC-2:
// agents/evolve-builder.md names triage-report.md's ## top_n as the SOLE task
// authority and demotes scout-report.md to background context. Exercised by the
// real TestBuilderPromptNamesTopNAsSoleTaskAuthority assertion, which checks for
// triage-report.md + top_n + a sole/authoritative/exclusive claim + an explicit
// scout-report demotion — not a single greppable token.
func TestC655_002_BuilderPromptNamesTopNSoleAuthority(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestBuilderPromptNamesTopNAsSoleTaskAuthority", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-2: builder-prompt task-authority suite must PASS; got exit=%d\n%s", code, out)
	}
}

// TestC655_003_ReplayCycle640ShapeBlocksBeforeAudit re-verifies AC-3: replaying
// the cycle-640 shape (triage=statefile task, build=token-resolver task) blocks
// at the build->audit transition instead of consuming audit/ship phases, with
// an abort_reason naming the wrong-task slug. Exercised by the real
// TestReplayCycle640Shape regression, which asserts Review returns
// Approve=false with the wrong slug in Reason.
func TestC655_003_ReplayCycle640ShapeBlocksBeforeAudit(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestReplayCycle640Shape", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-3: cycle-640-replay regression must PASS (blocked before audit); got exit=%d\n%s", code, out)
	}
}

// TestC655_004_TouchedPackageRaceClean binds the -race half of AC-4: `go test
// -race` on the touched package passes (race-clean). Running the whole package
// under -race also proves the carried-forward shadow-vs-enforce config gating
// (no feature flag), the gate's phase scoping, AND the newly-added
// apicover_named_test.go compile and behave. apicover COMPLETENESS is the
// separate repo-wide gate asserted by TestC655_005.
func TestC655_004_TouchedPackageRaceClean(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "./internal/topngate/...")
	if code != 0 {
		t.Errorf("AC-4(race): `go test -race ./internal/topngate/...` must PASS; got exit=%d\n%s", code, out)
	}
	if strings.Contains(out, "DATA RACE") {
		t.Errorf("AC-4(race): race detector flagged a data race:\n%s", out)
	}
}

// TestC655_005_TopngateApicoverGraduated binds the apicover half of AC-4 — the
// REMAINING GAP this completion cycle closes. Three assertions, all must hold:
//
//	(a) go/.apicover-enforce lists ./internal/topngate. This is an inherent
//	    config-presence check (membership in the enforce SSOT), waived below.
//	(b) go/internal/topngate/apicover_named_test.go exists and names NewReviewer
//	    — the package's only exported symbol (scout-verified). The graduation
//	    artifact CI's "api-coverage enforce" step exercises for correctness.
//	(c) THE LOAD-BEARING BEHAVIORAL ASSERTION: the real repo-wide completeness
//	    gate TestApicoverEnforce_CoversEveryInternalPackage PASSES. This shells
//	    the actual gate that failed ship in cycle-652 — it enumerates every
//	    ./internal/... package via `go list` and fails if any (including a
//	    carried-forward-but-ungraduated topngate) is absent from the SSOT, or
//	    if a stale/typo line was added. (c) is what makes (a) meaningful: you
//	    cannot green this predicate by pasting a garbage line — the package must
//	    genuinely exist AND be enumerated AND match the SSOT.
//
// RED at TDD time because (a) and (b) are absent in the fresh worktree; GREEN
// once the Builder carries the package forward and adds both artifacts.
func TestC655_005_TopngateApicoverGraduated(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// (a) config-presence: the enforce SSOT lists the package.
	// acs-predicate: config-check — membership in .apicover-enforce is an
	// inherent config-presence assertion (the SSOT is a static allow-list).
	enforcePath := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.FileContains(t, enforcePath, "./internal/topngate") {
		t.Errorf("AC-4(apicover-a): go/.apicover-enforce must list `./internal/topngate` (graduate the new package into the enforce SSOT)")
	}

	// (b) graduation artifact: the named test exists and names the sole exported
	// symbol NewReviewer by identifier.
	namedTest := filepath.Join(root, "go", "internal", "topngate", "apicover_named_test.go")
	if !acsassert.FileExists(t, namedTest) {
		t.Errorf("AC-4(apicover-b): go/internal/topngate/apicover_named_test.go must exist (public-API DoD)")
	} else if !acsassert.FileContains(t, namedTest, "NewReviewer") {
		t.Errorf("AC-4(apicover-b): apicover_named_test.go must name NewReviewer (topngate's only exported symbol) by identifier")
	}

	// (c) load-bearing behavioral gate: the real completeness regression passes.
	out, code := goTest(t, "-tags", "acs", "-count=1", "-run", "TestApicoverEnforce_CoversEveryInternalPackage", "./acs/regression/apicover/")
	if code != 0 {
		t.Errorf("AC-4(apicover-c): repo-wide apicover completeness gate must PASS (topngate graduated, no stale entries); got exit=%d\n%s", code, out)
	}
}
