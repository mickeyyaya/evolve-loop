//go:build acs

package cycle1151

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasecmdPkg = "github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	deliverPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

	adrRelPath      = "docs/architecture/adr/0077-docs-floor-for-architecture-changes.md"
	reviewerRelPath = "go/internal/core/build_floor_reviewer.go"

	strictLabeler = "IsArchitectureClass"
	broadLabeler  = "LabelArchitecture"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1151_001_ArchClassBuildWithoutDocsFailsSelfCheck(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_ArchitectureClassDiffWithoutDocs_Exit1|TestPhaseVerify_ArchitectureClassDiffWithoutDocs_JSONCarriesCode")
	if !ok {
		t.Errorf("architecture-class build diff with no docs delta does not fail `phase verify build`:\n%s", out)
	}
}

func TestC1151_002_FailOpenShapesAreUnchanged(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_ArchitectureClassDiffWithDocs_Exit0|TestPhaseVerify_NonArchitectureDiffInWorktree_Exit0|TestPhaseVerify_NoWorktree_ByteIdentical|TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor")
	if !ok {
		t.Errorf("fail-open behaviour regressed — the docs floor is taxing diffs it must ignore:\n%s", out)
	}
}

func TestC1151_003_SeamContractsHold(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"TestChangedWorktreePaths_ExportedForCLIConsumers|TestChangedWorktreePaths_EmptyWorktreeIsEmpty"); !ok {
		t.Errorf("core.ChangedWorktreePaths contract broken:\n%s", out)
	}
	if ok, out := runGoTest(t, deliverPkg,
		"TestVerifyBuildWithChangedPathsStage_AppliesFloorAndPreservesStage"); !ok {
		t.Errorf("VerifyBuildWithChangedPathsStage drops the caller's resolver/stage:\n%s", out)
	}
}

func productionLabeler(t *testing.T) string {
	t.Helper()
	reviewer := filepath.Join(acsassert.RepoRoot(t), reviewerRelPath)
	strictN, err := acsassert.CountInGoFunc(reviewer, "docsFloorWarn", "docsfloor."+strictLabeler+"(")
	if err != nil {
		t.Fatalf("cannot inspect docsFloorWarn in %s: %v", reviewer, err)
	}
	broadN, err := acsassert.CountInGoFunc(reviewer, "docsFloorWarn", "docsfloor."+broadLabeler+"(")
	if err != nil {
		t.Fatalf("cannot inspect docsFloorWarn in %s: %v", reviewer, err)
	}
	switch {
	case strictN > 0 && broadN == 0:
		return strictLabeler
	case broadN > 0 && strictN == 0:
		return broadLabeler
	default:
		t.Fatalf("ambiguous production labeler in docsFloorWarn: %s=%d %s=%d", strictLabeler, strictN, broadLabeler, broadN)
		return ""
	}
}

func adrText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), adrRelPath))
	if err != nil {
		t.Fatalf("cannot read %s: %v", adrRelPath, err)
	}
	return string(b)
}

func section(body, start, end string) string {
	i := strings.Index(body, start)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	if j := strings.Index(rest[len(start):], end); j >= 0 {
		return rest[:len(start)+j]
	}
	return rest
}

func TestC1151_004_ADRNamesTheProductionLabeler(t *testing.T) {
	want := productionLabeler(t)
	body := adrText(t)

	boundary := section(body, "2. **The label is derived from the diff", "3. **SKIP is not PASS")
	if boundary == "" {
		t.Fatalf("ADR-0077 boundary-2 bullet not found — has the ADR been restructured?")
	}
	if !strings.Contains(boundary, want) {
		t.Errorf("ADR-0077 boundary 2 does not name the production labeler %q:\n%s", want, boundary)
	}

	shape := section(body, "### Shape", "Decision order:")
	if shape == "" {
		t.Fatalf("ADR-0077 \"### Shape\" section not found")
	}
	if !strings.Contains(shape, want) {
		t.Errorf("ADR-0077 \"### Shape\" API listing omits the production labeler %q:\n%s", want, shape)
	}
}

func countProductionCallers(t *testing.T, sym string) int {
	t.Helper()
	root := filepath.Join(acsassert.RepoRoot(t), "go")
	needle := "docsfloor." + sym + "("
	n := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		n += strings.Count(string(b), needle)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return n
}

func noteNearMention(body, mention string, notes []string, window int) bool {
	for off := 0; ; {
		i := strings.Index(body[off:], mention)
		if i < 0 {
			return false
		}
		i += off
		lo, hi := i-window, i+len(mention)+window
		if lo < 0 {
			lo = 0
		}
		if hi > len(body) {
			hi = len(body)
		}
		for _, n := range notes {
			if strings.Contains(body[lo:hi], n) {
				return true
			}
		}
		off = i + len(mention)
	}
}

func TestC1151_005_ADRRecordsUnusedLabelerStatus(t *testing.T) {
	body := adrText(t)
	callers := countProductionCallers(t, broadLabeler)
	if callers > 0 {
		t.Skipf("%s now has %d production caller(s) — no non-production note is owed", broadLabeler, callers)
	}
	if !strings.Contains(body, broadLabeler) {
		return
	}

	notes := []string{
		"no production caller",
		"zero production caller",
		"not called in production",
		"unused in production",
		"no live caller",
	}
	if !noteNearMention(strings.ToLower(body), strings.ToLower(broadLabeler), notes, 400) {
		t.Errorf("ADR-0077 still documents %s but records nothing about its measured %d production callers "+
			"within 400 chars of any mention; expected one of %v", broadLabeler, callers, notes)
	}

	boundary := section(body, "2. **The label is derived from the diff", "3. **SKIP is not PASS")
	if strings.Contains(boundary, broadLabeler) && !strings.Contains(boundary, strictLabeler) {
		t.Errorf("ADR-0077 boundary 2 presents %s as the live labeler without naming %s (the symbol production calls):\n%s",
			broadLabeler, strictLabeler, boundary)
	}
}
