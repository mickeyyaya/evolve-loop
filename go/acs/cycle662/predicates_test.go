//go:build acs

// Package cycle662 materialises the cycle-662 acceptance criteria for the single
// triage-committed (`## top_n`) task: chronicle-s1-recurrence-index (weight 0.93).
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  chronicle-s1-recurrence-index — C662_001..007
//	The scout report proposed echo-veto-wiring-completion, but triage `## top_n`
//	(the sole task authority) committed chronicle-s1-recurrence-index instead, so
//	the predicates bind to THAT. Every non-committed item gets ZERO predicates.
//
// FEATURE CONTEXT — cycle 661 (395d8b7e) landed go/internal/recurrence
// (Ledger/RecordClosure/Escalator/Autofiler/EscalationPolicy + escalateRetroReason
// in core + `evolve lessons recurrence`) but the core shipped UNWIRED. THREE gaps
// make it dormant, closed this cycle:
//
//	(G1) PRODUCTION WIRING  — RecordClosure has zero call sites; nothing writes
//	     .evolve/recurrence-ledger.json, Count()==0 forever. Wire it at the
//	     deterministic retro-closeout seam (writeDeterministicLearning).
//	(G2) HISTORICAL BACKFILL — the ledger starts empty; a two-shape-tolerant scan
//	     over .evolve/instincts/lessons/*.yaml seeds the 267-lesson history with a
//	     SkippedFiles diagnostic for malformed files.
//	(G3) GENERIC CLASSIFICATION — operator-reset(96)+loop-fatal(62)=59% noise;
//	     a per-pattern Generic flag (denylist + pattern==errorCategory echo) keeps
//	     escalation and the CLI report on NON-generic patterns only.
//
// PREDICATE QUALITY (cycle-85): every predicate EXERCISES the SUT. Each shells
// `go test -race -v -run <name>` against the real package and asserts the named
// TDD-authored behavioral test actually RAN and PASSED (the `--- PASS: <name>`
// marker) — a package that compiles but lacks the test prints "no tests to run"
// (exit 0) with NO marker, so a bare exit check would vacuously green. C662_007
// is the apicover config-check (source-level, waived).
//
// TEST-NAME CONTRACT — these behavioral tests are authored by the TDD engineer
// (RED now; Builder must NOT modify them, only add production code to green them):
//
//	internal/recurrence : TestC662_BackfillCountsRecurrenceAcrossLessonShapes
//	                      TestC662_BackfillSkipsMalformedYAMLWithoutError
//	                      TestC662_BackfillTaskBindingChainCountsAtLeastSix
//	                      TestC662_MarksClassificationEchoPatternsGeneric
//	internal/core       : TestC662_RetroCloseoutRecordsClosureInLedger
//	                      TestC662_EscalateRetroReasonIgnoresGenericPatterns
//	cmd/evolve          : TestC662_RenderExcludesGenericPatterns
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C662_001 wiring writes a ledger keyed by the FAIL cycle.
//   - Positive : C662_002 recurrence counted across both lesson shapes.
//   - Negative/Edge : C662_003 malformed YAML skipped-not-fatal (SkippedFiles).
//   - Replay : C662_004 the historical task-binding chain counts >= 6.
//   - Semantic : C662_005 echo/denylist patterns marked Generic; specific ones not.
//   - Semantic/Negative : C662_006 escalation ignores generic (both directions).
//   - Semantic : C662_007 CLI report excludes generic noise.
//   - Config : C662_008 new exported symbols graduated in .apicover-enforce.
package cycle662

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	recurrencePkg = "./internal/recurrence/..."
	corePkg       = "./internal/core/..."
	cmdEvolvePkg  = "./cmd/evolve/..."
)

// goDir returns the go module directory for `go test -C <dir>` subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

// runNamedTest runs `go test -race -v -run <name> <pkg>` and reports whether the
// named test actually RAN and PASSED. The `--- PASS: <name>` marker guards the
// "-run matches nothing -> exit 0" false-green.
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

// TestC662_001_RetroCloseoutWiresLedger — G1/AC1. The retro-closeout seam must
// write .evolve/recurrence-ledger.json keyed by the FAIL cycle.
func TestC662_001_RetroCloseoutWiresLedger(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestC662_RetroCloseoutRecordsClosureInLedger")
	if !passed {
		t.Fatalf("RED: TestC662_RetroCloseoutRecordsClosureInLedger did not run+PASS.\n"+
			"Builder must call recurrence.RecordClosure at the deterministic retro-closeout\n"+
			"seam (writeDeterministicLearning) so a FAIL cycle writes recurrence-ledger.json\n"+
			"keyed by the cycle number.\nOutput:\n%s", out)
	}
}

// TestC662_002_BackfillCountsAcrossShapes — G2/AC2a. Two-shape-tolerant backfill
// accumulates a per-pattern recurrence count across cycles.
func TestC662_002_BackfillCountsAcrossShapes(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_BackfillCountsRecurrenceAcrossLessonShapes")
	if !passed {
		t.Fatalf("RED: TestC662_BackfillCountsRecurrenceAcrossLessonShapes did not run+PASS.\n"+
			"Builder must add BackfillFromLessons that parses BOTH lesson shapes and counts\n"+
			"recurrence per pattern across cycles.\nOutput:\n%s", out)
	}
}

// TestC662_003_BackfillSkipsMalformedWithoutError — G2/AC2b (negative/edge). A
// malformed lesson is recorded in SkippedFiles and never aborts the scan.
func TestC662_003_BackfillSkipsMalformedWithoutError(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_BackfillSkipsMalformedYAMLWithoutError")
	if !passed {
		t.Fatalf("RED: TestC662_BackfillSkipsMalformedYAMLWithoutError did not run+PASS.\n"+
			"Builder must skip malformed YAML (return its basename in the SkippedFiles\n"+
			"diagnostic) without a fatal error, still counting the valid neighbors.\nOutput:\n%s", out)
	}
}

// TestC662_004_TaskBindingChainCountsAtLeastSix — G2/AC3 (replay). The historical
// task-binding chain must be visible with count >= 6 after backfill.
func TestC662_004_TaskBindingChainCountsAtLeastSix(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_BackfillTaskBindingChainCountsAtLeastSix")
	if !passed {
		t.Fatalf("RED: TestC662_BackfillTaskBindingChainCountsAtLeastSix did not run+PASS.\n"+
			"Builder's backfill must surface the historical task-binding recurrence chain\n"+
			"(>= 6 same-pattern closeouts across cycles).\nOutput:\n%s", out)
	}
}

// TestC662_005_MarksClassificationEchoGeneric — G3/AC4a (semantic). Echo/denylist
// patterns are Generic; a specific defect (pattern != errorCategory) is not.
func TestC662_005_MarksClassificationEchoGeneric(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_MarksClassificationEchoPatternsGeneric")
	if !passed {
		t.Fatalf("RED: TestC662_MarksClassificationEchoPatternsGeneric did not run+PASS.\n"+
			"Builder must add Entry.Generic + IsGeneric (denylist + pattern==errorCategory\n"+
			"echo) + Ledger.IsGenericPattern, and set the flag during backfill.\nOutput:\n%s", out)
	}
}

// TestC662_006_EscalationIgnoresGeneric — G3/AC4b (semantic/negative, both
// directions). escalateRetroReason must not escalate a generic pattern even at
// count>=2, but must still escalate a non-generic one. The load-bearing anchor is
// that internal/core BUILDS against the new recurrence surface (a magic string
// cannot compile core) AND the named test runs+passes.
func TestC662_006_EscalationIgnoresGeneric(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/core/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: internal/core does not build against the recurrence Generic surface (exit=%d): %v\n%s",
			code, err, errOut)
	}
	passed, out := runNamedTest(t, corePkg, "TestC662_EscalateRetroReasonIgnoresGenericPatterns")
	if !passed {
		t.Fatalf("RED: TestC662_EscalateRetroReasonIgnoresGenericPatterns did not run+PASS.\n"+
			"Builder must gate escalateRetroReason on led.IsGenericPattern: a generic pattern\n"+
			"stays 'proceed' at count>=2; a non-generic one still escalates to 'adapt'.\nOutput:\n%s", out)
	}
}

// TestC662_007_CLIExcludesGenericPatterns — G3/AC5 (semantic). `evolve lessons
// recurrence` render excludes generic noise, keeps specific defects.
func TestC662_007_CLIExcludesGenericPatterns(t *testing.T) {
	passed, out := runNamedTest(t, cmdEvolvePkg, "TestC662_RenderExcludesGenericPatterns")
	if !passed {
		t.Fatalf("RED: TestC662_RenderExcludesGenericPatterns did not run+PASS.\n"+
			"Builder must make renderRecurrenceReport skip Generic entries so the report\n"+
			"surfaces the de-noised non-generic top patterns.\nOutput:\n%s", out)
	}
}

// TestC662_008_NewExportsGraduatedInApicover — AC6 (config-check, waived). The new
// exported symbols must be named in go/.apicover-enforce.
//
// acs-predicate: config-check
func TestC662_008_NewExportsGraduatedInApicover(t *testing.T) {
	enforce := repoFile(t, "go/.apicover-enforce")
	for _, sym := range []string{"BackfillFromLessons", "IsGeneric", "IsGenericPattern"} {
		if !acsassert.FileContains(t, enforce, sym) {
			t.Errorf("RED: go/.apicover-enforce does not name new exported symbol %q — "+
				"the backfill/generic surface must be graduated in the same commit", sym)
		}
	}
}
