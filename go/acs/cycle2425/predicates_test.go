//go:build acs

package cycle2425

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	pinnedPairs   = "TestNormalizeReasonForFingerprint_PinnedPairs"
	shapesFold    = "TestNormalizeReasonForFingerprint_CycleAndCountShapesFold"
	checkCountPin = "TestNormalizeReasonForFingerprint_CountOfFailingChecksStaysIdentity"
	normalizerPkg = "./internal/core"
	floorHead     = "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: "
)

type failReason struct {
	SchemaVersion int      `json:"schema_version"`
	Phase         string   `json:"phase"`
	Reasons       []string `json:"reasons"`
}

var assembledFingerprint = func(t *testing.T, cycle int, reason string) string {
	t.Helper()
	workspace := t.TempDir()
	raw, err := json.Marshal(failReason{SchemaVersion: 1, Phase: "build", Reasons: []string{reason}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "audit-fail-reason.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := core.AssembleFailureDigest(cycle, workspace, nil)
	if err != nil {
		t.Fatalf("assemble the failure digest of cycle %d: %v", cycle, err)
	}
	return digest.Fingerprint
}

func TestC2425_001_TheFourPinnedPairsMergeOrStayApartInTheTableTest(t *testing.T) {
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     dir,
		Package: normalizerPkg,
		Pattern: "^(" + pinnedPairs + "|" + checkCountPin + ")$",
		Names:   []string{pinnedPairs, checkCountPin},
	})
}

func TestC2425_002_EveryCycleAndCountShapeFolds(t *testing.T) {
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     dir,
		Package: normalizerPkg,
		Pattern: "^" + shapesFold + "$",
		Names:   []string{shapesFold},
	})
}

func TestC2425_003_TheDigestOfTwoCyclesWithOneBuildFloorDefectCarriesOneFingerprint(t *testing.T) {
	reasonOf := func(cycle, findings, ulid string) string {
		return floorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
			"  [ste-lint] the changed documents fail the STE lint: " + findings + " (first: docs/explain/builds/cycle-" + cycle + "-" + ulid + ".md:15 STE-SENTENCE)\n" +
			"  ./acs/cycle" + cycle + " (-tags acs): unit tests FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle" + cycle + "\t1.937s"
	}
	first := assembledFingerprint(t, 2049, reasonOf("2049", "3 finding(s) in 2 file(s)", "01m6cy05n0qsch7y9v4apqg24d"))
	second := assembledFingerprint(t, 2051, reasonOf("2051", "5 finding(s) in 1 file(s)", "01m6cy3x8e7tq2w9d5h4ncr0fa"))
	if first != second {
		t.Errorf("one recurring build-floor defect split into two fingerprints: %q vs %q", first, second)
	}

	lanePathOf := func(cycle string) string {
		return floorHead + "1 deterministic check failure(s):\n  wrote outside .evolve/worktrees/cycle-cd3ae73e-" + cycle + "/go"
	}
	if a, b := assembledFingerprint(t, 2071, lanePathOf("2071")), assembledFingerprint(t, 2072, lanePathOf("2072")); a != b {
		t.Errorf("one lane path defect split into two fingerprints: %q vs %q", a, b)
	}
}

func TestC2425_004_TwoDifferentBuildFloorDefectsKeepTwoFingerprints(t *testing.T) {
	acsFail := assembledFingerprint(t, 2054, floorHead+"1 deterministic check failure(s):\n  ./acs/cycle2054 (-tags acs): unit tests FAIL")
	explanation := assembledFingerprint(t, 2093, floorHead+"1 deterministic check failure(s):\n  Explanation Documentation: Changed Areas does not explain material path go/internal/repocontract/repocontract.go")
	if acsFail == explanation {
		t.Errorf("two different defects share the fingerprint %q: the breaker is blind", acsFail)
	}
	three := assembledFingerprint(t, 2073, floorHead+"3 deterministic check failure(s):\n  ./acs/cycle2073 (-tags acs): unit tests FAIL")
	two := assembledFingerprint(t, 2089, floorHead+"2 deterministic check failure(s):\n  ./acs/cycle2089 (-tags acs): unit tests FAIL")
	if three == two {
		t.Errorf("a floor with 3 failing checks and a floor with 2 share the fingerprint %q", three)
	}
}

func TestC2425_005_TheNormalizerIsFullyCoveredByTheTableTests(t *testing.T) {
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	pattern := "^(" + pinnedPairs + "|" + shapesFold + "|" + checkCountPin + ")$"
	if out, err := exec.Command("go", "-C", dir, "test", "-count=1", "-run", pattern, "-coverprofile="+profile, normalizerPkg).CombinedOutput(); err != nil {
		t.Fatalf("go test with coverage: %v\n%s", err, acsassert.OutputTail(string(out), 40))
	}
	funcs, err := exec.Command("go", "-C", dir, "tool", "cover", "-func="+profile).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, funcs)
	}
	pct, err := acsassert.FuncCoverage(string(funcs), "normalizeReasonForFingerprint")
	if err != nil || pct != 100 {
		t.Errorf("normalizeReasonForFingerprint coverage = %.1f%%, err = %v, want 100%%", pct, err)
	}
}

func TestC2425_006_TheChangedSourceAndTestFilesCarryNoComment(t *testing.T) {
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"failure_digest.go", "failure_digest_pinnedpairs_test.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, "internal", "core", name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if len(file.Comments) != 0 {
			t.Errorf("%s carries %d comment group(s); code has no comments", name, len(file.Comments))
		}
	}
}

// acs-predicate: config-check
func TestC2425_007_TheSpecAndThePackageNoteStateTheChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"docs/architecture/failure-identity-lifecycle.md", "docs/architecture/packages/internal-core.md"} {
		path := filepath.Join(root, rel)
		for _, needle := range []string{"cycle=N", "<ulid>", "finding(s)"} {
			if !acsassert.FileContains(t, path, needle) {
				t.Errorf("%s does not state the folded shape %q", rel, needle)
			}
		}
	}
}
