//go:build acs

package cycle2247

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTest(t *testing.T, pkg string, names ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	args := []string{"-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-tags", "integration"}
	if len(names) > 0 {
		args = append(args, "-run", "^("+strings.Join(names, "|")+")$")
	}
	args = append(args, pkg)
	out, errOut, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil || code != 0 {
		t.Fatalf("go test %s exit=%d: %v\n%s", pkg, code, err, acsassert.OutputTail(out+errOut, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2247_001_RequiredWorkflowDeclaresADailyScheduleTrigger(t *testing.T) {
	goTest(t, "./internal/ciparity/",
		"TestRequiredWorkflow_RunsTheFullGraphOnceADayOnTheDefaultBranch",
		"TestRequiredWorkflow_ScheduleLeavesTheResultAndRoutingGraphUnchanged",
		"TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies",
	)
}

func TestC2247_002_ScheduleGroupHasTheRunIDAndCancelsNothing(t *testing.T) {
	goTest(t, "./internal/ciparity/",
		"TestCIConcurrency_ScheduleGroupsByRunIDAndNeverCancels",
		"TestCIConcurrency_RequiredAndGoWorkflowsAreEvaluatedForEveryEvent",
		"TestCIConcurrency_DistinctPullRequestsRefsAndRunsNeverShareAGroup",
		"TestCIConcurrency_PullRequestGroupsByNumberAndCancels",
		"TestCIConcurrency_PushGroupsByRefAndNeverCancels",
		"TestRequiredWorkflow_ConcurrencyCancelsOnlySupersededPullRequestRuns",
	)
}

func TestC2247_003_RealChangesScriptSelectsEverySuiteForSchedule(t *testing.T) {
	goTest(t, "./internal/ciparity/",
		"TestRequiredRouting_ScheduleSelectsEverySuiteWhateverTheDiff",
		"TestRequiredRouting_ScheduleRoutesLikeAManualRun",
		"TestRequiredRouting_MissingEvidenceCannotBecomeDocumentationSkip",
	)
}

func TestC2247_004_MergeWindowParserAcceptsTheScheduleArm(t *testing.T) {
	goTest(t, "./internal/mergewindow/",
		"TestParse_TheLiveRoutingAcceptsAScheduledRunAsAFullRun",
		"TestClassify_TheLiveCIRoutingTableDecidesEachPath",
	)
}

func TestC2247_005_BothPackagesPassTheirFullIntegrationSuites(t *testing.T) {
	goTest(t, "./internal/ciparity/")
	goTest(t, "./internal/mergewindow/")
}

// acs-predicate: config-check — inherently a documentation presence assertion.
func TestC2247_006_PlanRecordsDecisionD28(t *testing.T) {
	root := acsassert.RepoRoot(t)
	plan := filepath.Join(root, "docs", "plans", "ci-wall-time-2026-10.md")
	if !acsassert.LineContainsAll(plan, "| D28 |", "schedule", "run id") {
		t.Errorf("%s has no table row for decision D28 that names the schedule trigger and the run id group", plan)
	}
	if !acsassert.LineContainsAll(plan, "| D28 |", "ci-main-daily-full-run") && !acsassert.LineContainsAll(plan, "D28", "ci-main-daily-full-run") {
		t.Errorf("%s does not tie D28 to the inbox item ci-main-daily-full-run", plan)
	}
}
