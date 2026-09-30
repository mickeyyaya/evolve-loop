//go:build acs

package cycle1287

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTestRun(t *testing.T, root, pkg string, names ...string) {
	t.Helper()
	anchored := make([]string, 0, len(names))
	for _, n := range names {
		anchored = append(anchored, "^"+n+"$")
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", strings.Join(anchored, "|"), pkg)
	combined := stdout + stderr
	for _, n := range names {
		if !strings.Contains(combined, "--- PASS: "+n) {
			t.Errorf("%s: %s did not run-and-pass — it is missing from this tree or failing, so the behaviour it pins is unprotected", pkg, n)
		}
	}
	if err != nil || code != 0 {
		t.Errorf("go test %s exited %d (err=%v)\n%s", pkg, code, err, combined)
	}
}

func TestC1287_001_RetroStaleWorktreeCriticalSurvivesTheLanding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/retro",
		"TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd",
		"TestRetroWorktree_FleetNeverEmitsANonExistentPath",
		"TestRetroWorktree_FleetProvisionedWorktreePassesThroughVerbatim",
		"TestRetroWorktree_NonFleetStalePathPassesThroughVerbatim")
}

func TestC1287_002_MintSpecSelectMetadataSurvivesTheLanding(t *testing.T) {
	root := acsassert.RepoRoot(t)

	var spec router.MintSpec
	if err := json.Unmarshal([]byte(`{"prompt":"p","tier":"deep","description":"D","when_to_use":"W"}`), &spec); err != nil {
		t.Fatalf("unmarshal MintSpec: %v", err)
	}
	v := reflect.ValueOf(spec)
	for field, want := range map[string]string{"Description": "D", "WhenToUse": "W"} {
		f := v.FieldByName(field)
		if !f.IsValid() {
			t.Errorf("router.MintSpec has no field %s — ADR-0038 SELECT metadata was dropped by the landing (base drift, not a real change from this lane)", field)
			continue
		}
		if got := f.String(); got != want {
			t.Errorf("MintSpec.%s = %q after decoding the advisor wire form, want %q", field, got, want)
		}
	}

	bare, err := json.Marshal(router.MintSpec{Prompt: "p"})
	if err != nil {
		t.Fatalf("marshal bare MintSpec: %v", err)
	}
	for _, key := range []string{`"description"`, `"when_to_use"`} {
		if strings.Contains(string(bare), key) {
			t.Errorf("empty MintSpec marshalled %s (%s) — the metadata keys must be omitempty", key, bare)
		}
	}

	goTestRun(t, root, "./internal/router",
		"TestMintSpec_CarriesSelectMetadata",
		"TestMintSpec_MetadataOmitEmpty")
}

func TestC1287_003_DefectLedgerReconcileIsLiveAtTheClassifySeam(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/audit",
		"TestClassify_RejectingAuditEmitsDefectLedger",
		"TestClassify_ContinuationCannotPassWithUnaccountedDefect",
		"TestClassify_ContinuationWithNoDispositionArtifactCannotPass",
		"TestClassify_UnresolvableEvidenceDoesNotCloseADefect",
		"TestClassify_ContinuationLedgerRetainsEveryEntry",
		"TestClassify_PassingAuditWritesNoLedger")
}

func TestC1287_004_RetroRemediationReachesTheInboxTransactionally(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/faillearn",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective")
}

func TestC1287_005_TreeBuildsAfterTheDriftResolution(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "build", "./...")
	if err != nil || code != 0 {
		t.Errorf("go build ./... exited %d (err=%v)\n%s%s", code, err, stdout, stderr)
	}
}

func TestC1287_006_GovernedDocsPassTheClosureCitationGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/audit", "TestC1287_DocsPassClosureCitationGate")
}

func TestC1287_007_ClosureGateStillRejectsUncitedClaims(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/phases/audit", "TestC1287_ClosureGateRejectsUncitedClaim")
}
