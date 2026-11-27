//go:build acs

package cycle2657

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func integrationTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/integration",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func shipTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/phases/ship",
		Tags:    "integration",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2657_001_RowRaASecondDeliveryWithTheSamePatchIDsIsANoOpAtTheWriter(t *testing.T) {
	integrationTests(t,
		"TestWriteOnce_RowRa_ASecondDeliveryWithTheSamePatchIDsIsANoOp",
		"TestWriteOnce_ASecondDeliveryReadsTheRecordFileNotTheWaveMember")
}

func TestC2657_002_RowRbNewPatchIDsAreRefusedWithATypedErrorNamingTheMemberAndTheRecordStaysUntouched(t *testing.T) {
	integrationTests(t,
		"TestWriteOnce_RowRb_ASecondDeliveryWithNewPatchIDsIsRefusedWithATypedErrorNamingTheMember",
		"TestWriteOnce_PatchIDsAreComparedAsAWholeSetNotByPrefix")
}

func TestC2657_003_TheDriverNeverReplacesAMemberOfAnyStatusWithNewPatchesAtEnforceAndAMergedMemberLandsOnce(t *testing.T) {
	integrationTests(t,
		"TestDriverAdmits_RowRb_AtEnforceANewPatchSetNeverReplacesAMemberInAnyStatus",
		"TestDriverAdmits_RowRc_AMergedMemberLandsAtMostOnce")
}

func TestC2657_004_Row10bTheGuardTableHoldsForEveryShipPathAndBothRaceRows(t *testing.T) {
	integrationTests(t,
		"TestWriteOnce_GuardTable_EveryShipPathAtEnforce",
		"TestWriteOnce_GuardTable_ARaceUnderExclusiveCreateKeepsTheFirstRecord")
}

func TestC2657_005_AShipRetryAfterADeliveryReadsAsDeliveredThroughTheProductionShipRunEvenWhenTheClockMoved(t *testing.T) {
	shipTests(t,
		"TestDeliverWriteOnce_AShipRetryUnderANewSHAWithTheSamePatchesReadsAsDelivered",
		"TestDeliverWriteOnce_AResumedShipOfTheSameBuildReadsAsDelivered")
}

func TestC2657_006_ADeliveryWithNewPatchesIsAPermanentRefusalAtEnforceAndShadowStillReplaces(t *testing.T) {
	shipTests(t,
		"TestDeliverWriteOnce_ADeliveryWithNewPatchesIsAPermanentRefusalThatLeavesMainAndTheRecordAlone",
		"TestDeliverWriteOnce_AShadowRetryStillReplacesTheRecord")
}

func TestC2657_007_ShadowKeepsReplaceAndRow10eStillComposesTheNewPatchIDs(t *testing.T) {
	integrationTests(t,
		"TestWriteOnce_TheReplacePolicyStillReplacesAfterTheDefence",
		"TestDriverAdmits_Row10e_AtShadowAPendingMemberDeliveredAgainIsComposedWithTheNewPatchIDs")
}

func TestC2657_008_Row10dARetryAfterTheLaneRefFailedLandsOnceMovesTheRefAndEjectsNothing(t *testing.T) {
	integrationTests(t, "TestDriverAdmits_Row10d_AtEnforceAPendingMemberDeliveredAgainUnderANewSHALandsOnceAndEjectsNothing")
	shipTests(t, "TestDeliverWriteOnce_ARetryAfterTheLaneRefFailedMovesTheRefAndKeepsTheRecord")
}

func TestC2657_009_EveryFunctionOfTheChangedFilesAndTheConsumeStampSiteIsFullyCovered(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	for _, run := range []struct {
		pkg, tags string
		files     []string
		funcs     []string
	}{
		{"./internal/integration", "", []string{"integration/deliverable.go", "integration/driver.go"}, nil},
		{"./internal/phases/ship", "integration", []string{"ship/deliverable.go"}, []string{"stage"}},
	} {
		profile := filepath.Join(t.TempDir(), "cover.out")
		args := []string{"test", "-count=1", "-coverprofile=" + profile}
		if run.tags != "" {
			args = append(args, "-tags", run.tags)
		}
		testOut, testErr, code, err := acsassert.SubprocessOutputIn(goDir, "go", append(args, run.pkg)...)
		if err != nil || code != 0 {
			t.Fatalf("go test %s exit=%d err=%v\n%s", run.pkg, code, err, acsassert.OutputTail(testOut+testErr, 40))
		}
		funcs, coverErr, code, err := acsassert.SubprocessOutputIn(goDir, "go", "tool", "cover", "-func="+profile)
		if err != nil || code != 0 {
			t.Fatalf("go tool cover exit=%d err=%v %s", code, err, coverErr)
		}
		for _, file := range run.files {
			seen := 0
			for _, line := range strings.Split(funcs, "\n") {
				fields := strings.Fields(line)
				if len(fields) < 3 || !strings.Contains(fields[0], "/"+file+":") {
					continue
				}
				seen++
				if fields[len(fields)-1] != "100.0%" {
					t.Errorf("%s is not fully covered: %s", file, strings.TrimSpace(line))
				}
			}
			if seen == 0 {
				t.Errorf("the cover output lists no function of %s", file)
			}
		}
		for _, name := range run.funcs {
			pct, err := acsassert.FuncCoverage(funcs, name)
			if err != nil || pct != 100 {
				t.Errorf("%s coverage = %.1f%% (err=%v), want 100%%", name, pct, err)
			}
		}
	}
}

func TestC2657_010_TheChangedSourceFilesHoldNoComment(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	directive := regexp.MustCompile(`^//(go:|line )`)
	for _, rel := range []string{
		"internal/integration/deliverable.go",
		"internal/integration/driver.go",
		"internal/phases/ship/deliverable.go",
		"internal/phases/ship/consume.go",
		"internal/integration/write_once_defence_test.go",
		"internal/phases/ship/deliverable_write_once_integration_test.go",
	} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(goDir, rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if !directive.MatchString(c.Text) {
					t.Errorf("%s holds the comment %q", rel, c.Text)
				}
			}
		}
	}
}
