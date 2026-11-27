//go:build acs

package cycle2653

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2653_001_RowRaASecondDeliveryWithTheSamePatchIDsIsANoOpAtTheWriter(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/integration",
		Pattern: "^(TestWriteOnce_RowRa_ASecondDeliveryWithTheSamePatchIDsIsANoOp|TestWriteOnce_ASecondDeliveryReadsTheRecordFileNotTheWaveMember)$",
		Names: []string{
			"TestWriteOnce_RowRa_ASecondDeliveryWithTheSamePatchIDsIsANoOp",
			"TestWriteOnce_ASecondDeliveryReadsTheRecordFileNotTheWaveMember",
		},
	})
}

func TestC2653_002_RowRbNewPatchIDsAreRefusedWithATypedErrorNamingTheMemberAndTheRecordStaysUntouched(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/integration",
		Pattern: "^(TestWriteOnce_RowRb_ASecondDeliveryWithNewPatchIDsIsRefusedWithATypedErrorNamingTheMember|TestWriteOnce_PatchIDsAreComparedAsAWholeSetNotByPrefix)$",
		Names: []string{
			"TestWriteOnce_RowRb_ASecondDeliveryWithNewPatchIDsIsRefusedWithATypedErrorNamingTheMember",
			"TestWriteOnce_PatchIDsAreComparedAsAWholeSetNotByPrefix",
		},
	})
}

func TestC2653_003_TheDriverNeverReplacesAMemberOfAnyStatusWithNewPatchesAtEnforceAndAMergedMemberLandsOnce(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/integration",
		Pattern: "^(TestDriverAdmits_RowRb_AtEnforceANewPatchSetNeverReplacesAMemberInAnyStatus|TestDriverAdmits_RowRc_AMergedMemberLandsAtMostOnce)$",
		Names: []string{
			"TestDriverAdmits_RowRb_AtEnforceANewPatchSetNeverReplacesAMemberInAnyStatus",
			"TestDriverAdmits_RowRc_AMergedMemberLandsAtMostOnce",
		},
	})
}

func TestC2653_004_TheGuardTableHoldsForEveryShipPathAndBothRaceRows(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/integration",
		Pattern: "^(TestWriteOnce_GuardTable_EveryShipPathAtEnforce|TestWriteOnce_GuardTable_ARaceUnderExclusiveCreateKeepsTheFirstRecord)$",
		Names: []string{
			"TestWriteOnce_GuardTable_EveryShipPathAtEnforce",
			"TestWriteOnce_GuardTable_ARaceUnderExclusiveCreateKeepsTheFirstRecord",
		},
	})
}

func TestC2653_005_AShipRetryAfterADeliveryReadsAsDeliveredThroughTheProductionShipRun(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/phases/ship",
		Tags:    "integration",
		Pattern: "^(TestDeliverWriteOnce_AShipRetryUnderANewSHAWithTheSamePatchesReadsAsDelivered|TestDeliverWriteOnce_AResumedShipOfTheSameCommitReadsAsDelivered)$",
		Names: []string{
			"TestDeliverWriteOnce_AShipRetryUnderANewSHAWithTheSamePatchesReadsAsDelivered",
			"TestDeliverWriteOnce_AResumedShipOfTheSameCommitReadsAsDelivered",
		},
	})
}

func TestC2653_006_ADeliveryWithNewPatchesIsAPermanentRefusalAtEnforceAndShadowStillReplaces(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/phases/ship",
		Tags:    "integration",
		Pattern: "^(TestDeliverWriteOnce_ADeliveryWithNewPatchesIsAPermanentRefusalThatLeavesMainAndTheRecordAlone|TestDeliverWriteOnce_AShadowRetryStillReplacesTheRecord)$",
		Names: []string{
			"TestDeliverWriteOnce_ADeliveryWithNewPatchesIsAPermanentRefusalThatLeavesMainAndTheRecordAlone",
			"TestDeliverWriteOnce_AShadowRetryStillReplacesTheRecord",
		},
	})
}

func TestC2653_007_ShadowKeepsReplaceAndRow10eStillComposesTheNewPatchIDs(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/integration",
		Pattern: "^(TestWriteOnce_TheReplacePolicyStillReplacesAfterTheDefence|TestDriverAdmits_Row10e_AtShadowAPendingMemberDeliveredAgainIsComposedWithTheNewPatchIDs)$",
		Names: []string{
			"TestWriteOnce_TheReplacePolicyStillReplacesAfterTheDefence",
			"TestDriverAdmits_Row10e_AtShadowAPendingMemberDeliveredAgainIsComposedWithTheNewPatchIDs",
		},
	})
}

func TestC2653_008_EveryFunctionOfTheChangedFilesIsFullyCovered(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	for _, run := range []struct {
		pkg, pattern, tags string
		files              []string
	}{
		{"./internal/integration", ".", "", []string{"integration/deliverable.go", "integration/driver.go"}},
		{"./internal/phases/ship", "^(TestDeliver|TestLanded)", "integration", []string{"ship/deliverable.go"}},
	} {
		profile := filepath.Join(t.TempDir(), "cover.out")
		args := []string{"test", "-count=1", "-run", run.pattern, "-coverprofile=" + profile}
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
	}
}

func TestC2653_009_TheChangedSourceFilesHoldNoComment(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	directive := regexp.MustCompile(`^//(go:|line )`)
	for _, rel := range []string{
		"internal/integration/deliverable.go",
		"internal/integration/driver.go",
		"internal/phases/ship/deliverable.go",
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
