//go:build acs

package cycle2408

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func digestOf(t *testing.T, cycle int, reason string) core.FailureDigest {
	t.Helper()
	workspace := t.TempDir()
	body := `{"schema_version":1,"phase":"build","reasons":[` + quote(reason) + `]}`
	if err := os.WriteFile(filepath.Join(workspace, "audit-fail-reason.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := core.AssembleFailureDigest(cycle, workspace, nil)
	if err != nil {
		t.Fatalf("assemble the failure digest of cycle %d: %v", cycle, err)
	}
	return digest
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(s) + `"`
}

func TestC2408_001_TheFourPinnedPairsMergeOrStayApartInTheTableTest(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: normalizerPkg,
		Pattern: "^(" + pinnedPairs + "|" + checkCountPin + ")$",
		Names:   []string{pinnedPairs, checkCountPin},
	})
}

func TestC2408_002_EveryNewCycleAndCountShapeFolds(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: normalizerPkg,
		Pattern: "^" + shapesFold + "$",
		Names:   []string{shapesFold},
	})
}

func TestC2408_003_TheDigestOfTwoCyclesWithOneBuildFloorDefectCarriesOneFingerprint(t *testing.T) {
	reasonOf := func(cycle string, findings string, ulid string) string {
		return floorHead + "2 deterministic check failure(s) — fix these exactly before handoff:\n" +
			"  [ste-lint] the changed documents fail the STE lint: " + findings + " (first: docs/explain/builds/cycle-" + cycle + "-" + ulid + ".md:15 STE-SENTENCE)\n" +
			"  ./acs/cycle" + cycle + " (-tags acs): unit tests FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle" + cycle + "\t1.937s"
	}
	first := digestOf(t, 2049, reasonOf("2049", "3 finding(s) in 2 file(s)", "01m6cy05n0qsch7y9v4apqg24d"))
	second := digestOf(t, 2051, reasonOf("2051", "5 finding(s) in 1 file(s)", "01m6cy3x8e7tq2w9d5h4ncr0fa"))
	if first.Fingerprint != second.Fingerprint {
		t.Errorf("one recurring build-floor defect split into two fingerprints: %q vs %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestC2408_004_TwoDifferentBuildFloorDefectsKeepTwoFingerprints(t *testing.T) {
	acsFail := digestOf(t, 2054, floorHead+"1 deterministic check failure(s):\n  ./acs/cycle2054 (-tags acs): unit tests FAIL")
	explanation := digestOf(t, 2093, floorHead+"1 deterministic check failure(s):\n  Explanation Documentation: Changed Areas does not explain material path go/internal/repocontract/repocontract.go")
	if acsFail.Fingerprint == explanation.Fingerprint {
		t.Errorf("two different defects share the fingerprint %q: the breaker is blind", acsFail.Fingerprint)
	}
	three := digestOf(t, 2073, floorHead+"3 deterministic check failure(s):\n  ./acs/cycle2073 (-tags acs): unit tests FAIL")
	two := digestOf(t, 2089, floorHead+"2 deterministic check failure(s):\n  ./acs/cycle2089 (-tags acs): unit tests FAIL")
	if three.Fingerprint == two.Fingerprint {
		t.Errorf("a floor with 3 failing checks and a floor with 2 share the fingerprint %q", three.Fingerprint)
	}
}

func TestC2408_005_TheNormalizerIsFullyCoveredByTheTableTests(t *testing.T) {
	dir := moduleRoot(t)
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

func TestC2408_006_TheChangedSourceAndTestFilesCarryNoComment(t *testing.T) {
	coreDir := filepath.Join(moduleRoot(t), "internal", "core")
	for _, name := range []string{"failure_digest.go", "failure_digest_pinnedpairs_test.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(coreDir, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if len(file.Comments) != 0 {
			t.Errorf("%s carries %d comment group(s); code has no comments", name, len(file.Comments))
		}
	}
}

// acs-predicate: config-check
func TestC2408_007_TheSpecAndThePackageNoteStateTheChange(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"docs/architecture/failure-identity-lifecycle.md", "docs/architecture/packages/internal-core.md"} {
		path := filepath.Join(root, rel)
		for _, needle := range []string{"cycle=", "file(s)"} {
			if !acsassert.FileContains(t, path, needle) {
				t.Errorf("%s does not state the folded shape %q", rel, needle)
			}
		}
	}
}
