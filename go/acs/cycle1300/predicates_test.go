//go:build acs

// Package cycle1300 materialises the cycle-1300 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox contract-block-cli-escalation):
//
//   - breaker-neutral-salvage-retry-when-no-escalation-family-exists
//   - demotion-ledger-records-salvage-attempted-vs-no-remedy-possible
//
// Predicate strategy. The contract this cycle adds lives entirely on unexported
// seams of internal/core (contractSalvageRetryDirectiveHeading, contractDispatch,
// formatContractGateDemotionWarn) reached only through Orchestrator.RunCycle, so
// the predicates DRIVE the RED contract tests as a subprocess rather than
// grepping source: each named test builds a real .evolve/profiles/*.json on
// disk, runs a real cycle, and asserts on the directives/ledger the production
// ladder actually emitted. A no-op implementation cannot pass them — the
// negative tests (no heading when an escalation target exists, none on the first
// block, none on a Blocks==0 rejection) fail a blanket "always re-prompt" too.
//
// Flaky-shape compliance: ONE named package, always narrowed with an anchored
// -run so the 40s+ whole-core suite never runs; no wall-clock bounds, no literal
// PIDs, no un-reaped load generators; `go test -C <worktree>/go` pins the working
// directory so a fleet lane's cwd cannot change which tree is measured.
package cycle1300

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// corePkg is the single package under test. Every invocation below narrows it
// with an anchored -run: the unnarrowed core suite is a known 40s+ sweep and is
// the regression suite's job, never a cycle predicate's.
const corePkg = "./internal/core/"

// escalationDoc is the live architecture doc the demotion-record task must keep
// truthful (constraint 4's documentation pattern).
const escalationDoc = "docs/architecture/contract-block-cli-escalation.md"

// goDir is the worktree's go module root — predicates read the CYCLE's source,
// not main's (worktree isolation; acsassert.RepoRoot resolves the worktree).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runContract runs `go test -C <worktree>/go -count=1 -v -run ^(names...)$
// ./internal/core/` and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal; a compile failure in
//     the target package — the expected RED signal before Builder implements the
//     seams — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test was
//     deleted or skipped, not that it passed. That is reported as a miss.
func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-v",
		"-run", "^("+strings.Join(names, "|")+")$", corePkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s: code=%d err=%v\n%s", corePkg, code, err, tail(out, 30))
	}
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			missing = append(missing, n)
		}
	}
	return code == 0 && len(missing) == 0, missing, out
}

// tail returns the last n lines — diagnostics stay readable in the verdict.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// report renders a uniform RED message naming both halves of a failure: the
// contract that did not hold, and any test that vanished rather than passing.
func report(t *testing.T, what string, missing []string, out string) {
	t.Helper()
	if len(missing) > 0 {
		t.Errorf("RED: %s — and these contract tests did not report PASS (deleted or skipped): %v\n%s",
			what, missing, tail(out, 40))
		return
	}
	t.Errorf("RED: %s\n%s", what, tail(out, 40))
}

// TestC1300_001_SalvageRetryFiresWhenNoEscalationFamily pins task 1's crux plus
// its precision guard. Positive: a single-family chain (no escalation target)
// must turn the block-2 correction into a structured re-prompt carrying the
// verbatim validator reason, on the SAME CLI, without adding a dispatch
// (breaker-neutral). Negative/anti-no-op: a chain that DOES have a cross-family
// target must escalate exactly as before and must NOT also re-prompt.
func TestC1300_001_SalvageRetryFiresWhenNoEscalationFamily(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestContractEscalation_SalvageRetry_WhenNoOtherFamily",
		"TestContractEscalation_SalvageRetry_NotWhenEscalationTargetExists")
	if !ok {
		report(t, "a contract-blocked phase whose whole dispatch chain is one CLI family still falls through unremedied — the block-2 correction must become a breaker-neutral structured re-prompt (same CLI, verbatim reason, no extra dispatch), and must stay disjoint from CLI escalation where a target exists", missing, out)
	}
}

// TestC1300_002_SalvageRetryRespectsTriggerScoping pins the two boundaries the
// remedy inherits from the escalation it stands in for: it starts at the SECOND
// consecutive contract block (one bad turn is not a CLI verdict), and only a
// CONTRACT block earns it (Blocks==0 rejections from evalgate / topngate /
// triagecap are task-binding failures, not format failures). This is the edge /
// OOD axis: a blanket "always re-prompt" implementation fails here.
func TestC1300_002_SalvageRetryRespectsTriggerScoping(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestContractEscalation_SalvageRetry_NotOnFirstBlock",
		"TestContractEscalation_SalvageRetry_NotOnNonContractRejection")
	if !ok {
		report(t, "the salvage retry fired outside its trigger window — it must start at the same block as escalation would (block 2) and never on a Blocks==0 non-contract rejection", missing, out)
	}
}

// TestC1300_003_DemotionRecordDistinguishesSalvageAttempt pins task 2 end to
// end: the ledger entry emitted by a real demoted cycle must carry
// salvage_attempted=true (while keeping escalated=false — a re-prompt is not an
// escalation), and the operator-facing WARN must stop claiming no remedy ran
// when one did.
func TestC1300_003_DemotionRecordDistinguishesSalvageAttempt(t *testing.T) {
	ok, missing, out := runContract(t,
		"TestContractEscalation_SalvageRetry_LedgerRecordsSalvageAttempted",
		"TestContractEscalation_SalvageRetry_WarnDistinguishesAttemptFromNoRemedy")
	if !ok {
		report(t, "a gate demotion still cannot be told apart from one where no remedy was possible — the ledger Action must record salvage_attempted, and the WARN must not claim 'did NOT run' after a salvage retry was attempted", missing, out)
	}
}

// TestC1300_004_EscalationDocNamesSalvageOutcome keeps the live architecture doc
// truthful about the mechanism the ladder now runs — the doc is marked
// "Status: live" and is what an operator reads before touching this ladder.
//
// acs-predicate: config-check — this criterion IS a documentation-content
// criterion (there is no runtime behaviour to invoke); the behavioural half of
// task 2 is covered by 003 above.
func TestC1300_004_EscalationDocNamesSalvageOutcome(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), escalationDoc)
	if !acsassert.FileExists(t, doc) {
		return // FileExists already reported its own failure
	}
	for _, want := range []string{"salvage_attempted", "structured re-prompt"} {
		if !acsassert.FileContains(t, doc, want) {
			t.Errorf("RED: %s does not document %q — the doc is Status: live and is the operator's map of this ladder; a remedy that exists only in code is a remedy nobody knows to look for", escalationDoc, want)
		}
	}
}
