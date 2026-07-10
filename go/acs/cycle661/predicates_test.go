//go:build acs

// Package cycle661 materialises the cycle-661 acceptance criteria for the single
// triage-committed (`## top_n`) task: recurrence-ledger-weight-escalation.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  recurrence-ledger-weight-escalation (weight 0.93) — C661_001..005
//	Every `## deferred` item (chronicle-s1-recurrence-index,
//	echo-veto-wiring-completion, new-package-graduation-buildentry-gate) gets
//	ZERO predicates here.
//
// FEATURE CONTEXT
//
//	The system NOTICED every recurrence (retros wrote "6th occurrence,
//	confidence 0.97" chains) but noticing lived in an advisory channel with no
//	write access to the priority queue. This cycle mints a deterministic
//	internal/recurrence ledger:
//	  (1) LEDGER   — on retro closeout, upsert {pattern_key -> cycles[], count,
//	      last_seen, fix_item_id, fix_landed_sha} into .evolve/recurrence-ledger.json
//	      (flock + atomic write).
//	  (2) ESCALATION — a pure policy formula bumps the linked OPEN inbox item's
//	      weight (min(0.99, base + 0.03*(count-1))), idempotent per cycle; when
//	      no open item exists the pattern is handed to the autofile seam once.
//	  (3) RETRO-DECISION PARITY — RetroDecision consults the ledger and must NOT
//	      emit bare "proceed" while the current lesson pattern has count>=2.
//	  (4) `evolve lessons recurrence` CLI report (patterns by count + fix status).
//	internal/recurrence is a NEW leaf package and MUST be graduated into
//	go/.apicover-enforce in the same commit (new-package-graduation class,
//	4th recurrence: 575/587/652/661).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it runs `go test -race` against the real packages and asserts the NAMED
// builder-authored behavioral test actually RAN and PASSED (checks the
// `--- PASS: <name>` marker in `-v` output). `go test -run X` on a package with
// no matching test exits 0 with "no tests to run" — a bare exit-code check would
// vacuously green, so every behavioral predicate below asserts the PASS marker,
// not merely exit==0. The only source-level assertions are the config-check
// apicover-enforce entry (C661_005, waived) and the core-consult wiring pin
// (C661_003), whose LOAD-BEARING anchor is that internal/core BUILDS against the
// new recurrence dependency (a magic string cannot make core compile).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : C661_001 three same-pattern retros => count=3 + one idempotent bump.
//   - Negative/Edge : C661_002 count>=2 with NO open item => autofile seam fires
//     exactly ONCE (an implementation that double-fires, or never fires, FAILS).
//   - Semantic  : C661_003 RetroDecision with an Nth-occurrence lesson forces
//     "adapt"-with-escalation, never bare "proceed".
//   - Semantic  : C661_004 `evolve lessons recurrence` sorts patterns by count
//     and shows fix-item status.
//   - Config    : C661_005 internal/recurrence graduated into .apicover-enforce.
//
// BUILDER TEST-NAME CONTRACT — the predicates target these exact test names;
// Builder must author them (do NOT rename without updating this file):
//
//	internal/recurrence : TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce
//	                      TestLedger_CountGE2NoOpenItemAutofilesOnce
//	internal/core       : TestDecideAfterRetro_NthOccurrenceForcesAdapt
//	cmd/evolve          : TestLessonsRecurrence_SortedByCountWithFixStatus
package cycle661

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the go module directory for `go -C <dir>` subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

// runNamedTest runs `go test -race -v -run <name> <pkg>` and returns whether the
// named test actually RAN and PASSED. The `--- PASS: <name>` marker guards the
// "-run matches nothing -> exit 0" false-green: a package that compiles but lacks
// the named test prints "no tests to run" (exit 0) with NO PASS marker.
func runNamedTest(t *testing.T, pkg, name string) (passed bool, out string) {
	t.Helper()
	dir := goDir(t)
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-race", "-count=1", "-v",
		"-run", "^"+name+"$", pkg,
	)
	out = stdout + "\n" + stderr
	return strings.Contains(out, "--- PASS: "+name), out
}

// TestC661_001_ThreeSamePatternRetrosCountThreeAndBumpOnce is the behavioral core
// (AC1): three fixture retro closeouts carrying the same pattern key must produce
// ledger count=3 and exactly one deterministic weight bump on the linked open
// inbox item, idempotent across a re-run of the same cycle's closeout.
//
// RED today: internal/recurrence does not exist, so `go test` fails to build and
// the named test never emits a PASS marker. GREEN once Builder implements the
// ledger upsert + escalation and authors
// TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce.
func TestC661_001_ThreeSamePatternRetrosCountThreeAndBumpOnce(t *testing.T) {
	passed, out := runNamedTest(t, "./internal/recurrence/...",
		"TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce")
	if !passed {
		t.Fatalf("RED: TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce did not run+PASS.\n"+
			"Builder must create leaf package internal/recurrence whose ledger upsert makes three\n"+
			"same-pattern retro closeouts yield count=3 and ONE idempotent weight bump on the linked\n"+
			"open inbox item.\nOutput:\n%s", out)
	}
}

// TestC661_002_CountGE2NoOpenItemAutofilesExactlyOnce is the negative/edge axis
// (AC2): when a pattern reaches count>=2 but NO open inbox item exists to bump,
// the pattern must be handed to the retro-preventive autofile seam EXACTLY ONCE
// while it stays open — an implementation that double-fires OR never fires FAILS.
//
// RED today: package absent. GREEN once Builder authors
// TestLedger_CountGE2NoOpenItemAutofilesOnce.
func TestC661_002_CountGE2NoOpenItemAutofilesExactlyOnce(t *testing.T) {
	passed, out := runNamedTest(t, "./internal/recurrence/...",
		"TestLedger_CountGE2NoOpenItemAutofilesOnce")
	if !passed {
		t.Fatalf("RED: TestLedger_CountGE2NoOpenItemAutofilesOnce did not run+PASS.\n"+
			"Builder must make a count>=2 pattern with no open inbox item hand off to the autofile\n"+
			"seam exactly once (dedup guard) — never zero, never twice.\nOutput:\n%s", out)
	}
}

// TestC661_003_RetroDecisionConsultsLedgerForcesAdapt is the semantic axis (AC3):
// RetroDecision must consult the recurrence ledger — a lesson whose pattern has
// count>=2 forces "adapt"-with-escalation wording and MUST NOT coexist with a
// bare "proceed: no failures requiring adaptation". The load-bearing anchor is
// that internal/core BUILDS against the recurrence dependency (a magic string
// cannot compile core) AND the named core test runs+passes.
//
// RED today: recurrence package absent → core does not build → build fail.
func TestC661_003_RetroDecisionConsultsLedgerForcesAdapt(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/core/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: internal/core does not build with the recurrence-ledger consult (exit=%d): %v\n%s",
			code, err, errOut)
	}

	// decision_branch.go must actually reference the recurrence package — the
	// consult site. Build above proves it links; this pins WHERE.
	if !acsassert.FileContains(t, repoFile(t, "go/internal/core/decision_branch.go"), "recurrence.") {
		t.Errorf("RED: decision_branch.go does not reference recurrence.* — RetroDecision must consult " +
			"the ledger so count>=2 cannot emit bare 'proceed'")
	}

	passed, out := runNamedTest(t, "./internal/core/...",
		"TestDecideAfterRetro_NthOccurrenceForcesAdapt")
	if !passed {
		t.Fatalf("RED: TestDecideAfterRetro_NthOccurrenceForcesAdapt did not run+PASS.\n"+
			"Builder must gate the RetroDecision 'proceed' branch on a ledger lookup: an\n"+
			"Nth-occurrence (count>=2) pattern forces 'adapt: escalated <item> to <weight>'.\nOutput:\n%s", out)
	}
}

// TestC661_004_LessonsRecurrenceCLIReportSortedByCount is the CLI axis (AC4):
// `evolve lessons recurrence` reports patterns sorted by count with fix-item
// status. Exercised through the cmd-package behavioral test.
//
// RED today: the subcommand + its test do not exist. GREEN once Builder authors
// TestLessonsRecurrence_SortedByCountWithFixStatus.
func TestC661_004_LessonsRecurrenceCLIReportSortedByCount(t *testing.T) {
	passed, out := runNamedTest(t, "./cmd/evolve/...",
		"TestLessonsRecurrence_SortedByCountWithFixStatus")
	if !passed {
		t.Fatalf("RED: TestLessonsRecurrence_SortedByCountWithFixStatus did not run+PASS.\n"+
			"Builder must add `evolve lessons recurrence` printing patterns sorted by descending\n"+
			"count with each pattern's fix_item status.\nOutput:\n%s", out)
	}
}

// TestC661_005_RecurrenceGraduatedIntoApicoverEnforce is the config axis (AC5):
// the new leaf package internal/recurrence must be graduated into
// go/.apicover-enforce in the same commit (new-package-graduation class,
// 4th recurrence: 575/587/652/661).
//
// acs-predicate: config-check
//
// RED today: .apicover-enforce does not list internal/recurrence.
func TestC661_005_RecurrenceGraduatedIntoApicoverEnforce(t *testing.T) {
	enforce := repoFile(t, "go/.apicover-enforce")
	if !acsassert.FileContainsAny(enforce,
		"./internal/recurrence",
		"internal/recurrence",
	) {
		t.Errorf("RED: go/.apicover-enforce does not list internal/recurrence — the new leaf package " +
			"must be graduated into the enforced set in the same commit (new-package obligation, " +
			"4th-recurrence class)")
	}
	// Sanity: the enforce file still lists a known package, so a truncation can't
	// vacuously pass the absence check above.
	if !acsassert.FileContains(t, enforce, "./internal/adapters/statemap") {
		t.Errorf("apicover-enforce sanity: expected the file to still list ./internal/adapters/statemap")
	}
}
