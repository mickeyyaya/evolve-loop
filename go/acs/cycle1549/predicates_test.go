//go:build acs

package cycle1549

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

	fullPathTest = "TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting"
	negativeTest = "TestJudgmentLessonFullPath_NoLessonWithoutAWellFormedSentinelFAIL"
)

func judgmentLessonTestFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "judgment_lesson_test.go")
}

func runCoreSubtest(t *testing.T, parent, sub string) {
	t.Helper()
	pattern := "^" + parent + "$/^" + sub + "$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, corePkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+parent+"/"+sub) {
		t.Errorf("subtest %s/%s did not report PASS — it is absent, renamed, skipped, or was filtered out "+
			"(exit 0 with no test run is NOT proof of anything).\nstdout:\n%s", parent, sub, stdout)
	}
}

func TestC1549_001_real_sentinel_parse_produces_the_fail(t *testing.T) {
	runCoreSubtest(t, fullPathTest, "real_sentinel_parse_produces_the_FAIL")

	file := judgmentLessonTestFile(t)
	parsed, err := acsassert.CountInGoFunc(file, fullPathTest, "ParseVerdictSentinel")
	if err != nil {
		t.Fatalf("CountInGoFunc(%s, %s): %v", file, fullPathTest, err)
	}
	if parsed < 1 {
		t.Errorf("%s never calls phasecontract.ParseVerdictSentinel* — the FAIL must be PRODUCED by the "+
			"production parser, not asserted about a hand-built PhaseResponse; that is the whole "+
			"composition this cycle exists to prove", fullPathTest)
	}
	rendered, err := acsassert.CountInGoFunc(file, fullPathTest, "RenderVerdictSentinel")
	if err != nil {
		t.Fatalf("CountInGoFunc(%s, %s): %v", file, fullPathTest, err)
	}
	if rendered < 1 {
		t.Errorf("%s never calls phasecontract.RenderVerdictSentinel* — a hand-typed sentinel string is a "+
			"SECOND grammar that drifts away from the one the classifier reads; render the fixture with "+
			"the production renderer so producer and parser stay in lockstep", fullPathTest)
	}
}

func TestC1549_002_objection_reaches_next_cycle_planner_context(t *testing.T) {
	runCoreSubtest(t, fullPathTest, "objection_reaches_next_cycle_planner_context")
}

func TestC1549_003_failed_at_unchanged_and_continuation_non_halting(t *testing.T) {
	runCoreSubtest(t, fullPathTest, "failed_at_unchanged_and_continuation_non_halting")
}

func TestC1549_004_no_lesson_for_pass_malformed_or_absent_sentinel(t *testing.T) {
	for _, sub := range []string{"stated_PASS", "malformed_sentinel", "absent_sentinel"} {
		runCoreSubtest(t, negativeTest, sub)
	}
}
