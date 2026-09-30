//go:build acs

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

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

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

func TestC662_001_RetroCloseoutWiresLedger(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestC662_RetroCloseoutRecordsClosureInLedger")
	if !passed {
		t.Fatalf("RED: TestC662_RetroCloseoutRecordsClosureInLedger did not run+PASS.\n"+
			"Builder must call recurrence.RecordClosure at the deterministic retro-closeout\n"+
			"seam (writeDeterministicLearning) so a FAIL cycle writes recurrence-ledger.json\n"+
			"keyed by the cycle number.\nOutput:\n%s", out)
	}
}

func TestC662_002_BackfillCountsAcrossShapes(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_BackfillCountsRecurrenceAcrossLessonShapes")
	if !passed {
		t.Fatalf("RED: TestC662_BackfillCountsRecurrenceAcrossLessonShapes did not run+PASS.\n"+
			"Builder must add BackfillFromLessons that parses BOTH lesson shapes and counts\n"+
			"recurrence per pattern across cycles.\nOutput:\n%s", out)
	}
}

func TestC662_003_BackfillSkipsMalformedWithoutError(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_BackfillSkipsMalformedYAMLWithoutError")
	if !passed {
		t.Fatalf("RED: TestC662_BackfillSkipsMalformedYAMLWithoutError did not run+PASS.\n"+
			"Builder must skip malformed YAML (return its basename in the SkippedFiles\n"+
			"diagnostic) without a fatal error, still counting the valid neighbors.\nOutput:\n%s", out)
	}
}

func TestC662_004_TaskBindingChainCountsAtLeastSix(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_BackfillTaskBindingChainCountsAtLeastSix")
	if !passed {
		t.Fatalf("RED: TestC662_BackfillTaskBindingChainCountsAtLeastSix did not run+PASS.\n"+
			"Builder's backfill must surface the historical task-binding recurrence chain\n"+
			"(>= 6 same-pattern closeouts across cycles).\nOutput:\n%s", out)
	}
}

func TestC662_005_MarksClassificationEchoGeneric(t *testing.T) {
	passed, out := runNamedTest(t, recurrencePkg, "TestC662_MarksClassificationEchoPatternsGeneric")
	if !passed {
		t.Fatalf("RED: TestC662_MarksClassificationEchoPatternsGeneric did not run+PASS.\n"+
			"Builder must add Entry.Generic + IsGeneric (denylist + pattern==errorCategory\n"+
			"echo) + Ledger.IsGenericPattern, and set the flag during backfill.\nOutput:\n%s", out)
	}
}

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

func TestC662_007_CLIExcludesGenericPatterns(t *testing.T) {
	passed, out := runNamedTest(t, cmdEvolvePkg, "TestC662_RenderExcludesGenericPatterns")
	if !passed {
		t.Fatalf("RED: TestC662_RenderExcludesGenericPatterns did not run+PASS.\n"+
			"Builder must make renderRecurrenceReport skip Generic entries so the report\n"+
			"surfaces the de-noised non-generic top patterns.\nOutput:\n%s", out)
	}
}

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
