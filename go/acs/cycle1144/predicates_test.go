//go:build acs

package cycle1144

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1144_001_ArchitectureClassDiffWithoutDocsIsAViolation(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestArchitectureDocsViolations_ArchitectureClassWithoutDocs|TestArchitectureDocs_CodeConstantIsStable")
	if !ok {
		t.Errorf("architecture-class-without-docs classification is not implemented (or regressed):\n%s", out)
	}
}

func TestC1144_002_DocsDeltaSatisfiesTheFloor(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestArchitectureDocsViolations_DocsDeltaSatisfiesFloor")
	if !ok {
		t.Errorf("a documented architecture-class diff must pass the floor:\n%s", out)
	}
}

func TestC1144_003_NonArchitectureDiffsFailOpen(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg,
		"TestArchitectureDocsViolations_NonArchitectureDiffsFailOpen|TestVerify_BuildWithoutDiff_ByteIdentical")
	if !ok {
		t.Errorf("non-architecture diffs must be unaffected by the docs floor:\n%s", out)
	}
}

func TestC1144_004_FloorIsWiredIntoTheVerifySeam(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestVerify_ArchitectureClassRequiresDocsDelta")
	if !ok {
		t.Errorf("the docs floor is not wired into deliverable.Verify's seam:\n%s", out)
	}
}

func TestC1144_005_DeliverablePackageStaysGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", deliverablePkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", deliverablePkg, code, err, out)
	}
	if code != 0 {
		t.Errorf("internal/deliverable must stay green with the docs floor added:\n%s", out)
	}
}

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Errorf("cannot read %s: %v", rel, err)
		return ""
	}
	return string(b)
}

func TestC1144_006_FleetLandingDocumentedInControlFlags(t *testing.T) {
	const rel = "docs/architecture/control-flags.md"
	doc := readDoc(t, rel)
	for _, needle := range []string{"fleet.landing", "per-lane", "prefix-queue"} {
		if !strings.Contains(doc, needle) {
			t.Errorf("%s does not document %q — fleet.landing has real resolved-config semantics (policy.go:1043-1208) and zero doc coverage", rel, needle)
		}
	}
}

func TestC1144_007_FleetLandingDocumentedInRuntimeReference(t *testing.T) {
	const rel = "docs/operations/runtime-reference.md"
	doc := readDoc(t, rel)
	for _, needle := range []string{"fleet.landing", "prefix-queue"} {
		if !strings.Contains(doc, needle) {
			t.Errorf("%s does not carry the operator-visible %q note", rel, needle)
		}
	}
}

func TestC1144_008_FleetLandingADRExists(t *testing.T) {
	adrDir := filepath.Join(acsassert.RepoRoot(t), "docs/architecture/adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", adrDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(adrDir, e.Name()))
		if err != nil {
			continue
		}
		body := string(b)
		if strings.Contains(body, "fleet.landing") && strings.Contains(body, "prefix-queue") {
			return
		}
	}
	t.Errorf("no ADR under docs/architecture/adr/ documents fleet.landing / prefix-queue")
}

func TestC1144_009_DocsFloorFlagsItsOwnBackfillInstance(t *testing.T) {
	ok, out := runGoTest(t, deliverablePkg, "TestArchitectureDocs_BackfillInstance_FleetLanding")
	if !ok {
		t.Errorf("the docs floor does not flag its own fleet.landing backfill instance:\n%s", out)
	}
}
