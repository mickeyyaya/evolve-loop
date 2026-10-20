//go:build acs

package cycle2021

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	specPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	cmdPkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runNamedTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", pkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("RED: %s tests %v fail (exit %d)\n%s", pkg, names, code, tail(out))
		return
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not run and pass %s", pkg, name)
		}
	}
}

func tail(s string) string {
	if len(s) > 1500 {
		return s[len(s)-1500:]
	}
	return s
}

func TestC2021_001_StageIsAClosedSetThatValidationEnforces(t *testing.T) {
	runNamedTests(t, specPkg,
		"TestStage_DecodesFromPhaseJSON",
		"TestValidateUserSpec_StageIsAClosedSet",
		"TestApplyUserRouting_AnUnknownStageIsNotRouted",
		"TestLoad_ARegistrySpecWithAnUnknownStageIsAHardError",
		"TestDescriptorSchema_StageEnumIsTheClosedSet",
	)
}

func TestC2021_002_APreBuildPhaseAfterBuildIsRefusedWithACodedSignal(t *testing.T) {
	runNamedTests(t, specPkg,
		"TestApplyUserRouting_RefusesAPreBuildPhaseAnchoredAfterBuild",
		"TestApplyUserRouting_RefusesAPreBuildPhaseWithNoAnchor",
		"TestApplyUserRouting_RefusesAPreBuildPhaseChainedBehindAnAfterBuildAnchor",
		"TestApplyUserRouting_PreBuildPhasesBeforeBuildAreRouted",
		"TestApplyUserRouting_OnlyAPreBuildStageIsFencedFromFollowingBuild",
		"TestApplyUserRouting_APreBuildPhaseIsLegalWhenTheOrderHasNoBuild",
		"TestApplyUserRouting_AnchorToLaterSpecInBatchIsHonored",
	)
	runNamedTests(t, cmdPkg,
		"TestWireOrchestrator_APreBuildPhaseAnchoredAfterBuildReachesTheSignalCenter",
		"TestWireOrchestrator_APostBuildPhaseAnchoredAfterBuildRaisesNoRoutingDefect",
		"TestWireOrchestrator_AnUnresolvableUserAnchorReachesTheSignalCenter",
	)
}

func TestC2021_003_TheTrackedPhasesDeclareTheStageAndThePinReadsIt(t *testing.T) {
	runNamedTests(t, specPkg,
		"TestRepoPhaseCatalog_ThePreBuildEvaluatePhasesDeclareTheirStage",
		"TestRepoPhaseCatalog_EveryTrackedStageIsInTheClosedSet",
	)
	runNamedTests(t, cmdPkg,
		"TestRealTreeRoutingOrder_EveryPreBuildPhaseSitsBeforeBuild",
		"TestRealTreeRoutingOrder_ThePinReadsTheStageFieldNotPhaseNames",
		"TestRealTreeRoutingOrder_AnchoredPhasesFollowTheirAnchors",
	)
}

// acs-predicate: config-check — inherently a documentation presence assertion.
func TestC2021_004_ThePackageNoteAndThePhaseConfigSpecDocumentTheField(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs := []string{
		filepath.Join(root, "docs", "architecture", "packages", "internal-phasespec.md"),
		filepath.Join(root, "docs", "architecture", "user-defined-phases.md"),
	}
	for _, doc := range docs {
		for _, needle := range []string{"`stage`", "pre-build", "post-build"} {
			if !acsassert.FileContains(t, doc, needle) {
				t.Errorf("RED: %s does not document %s", doc, needle)
			}
		}
	}
	if !acsassert.FileContains(t, docs[1], "| `stage` |") {
		t.Errorf("RED: the phase.json field table of %s has no `stage` row", docs[1])
	}
}
