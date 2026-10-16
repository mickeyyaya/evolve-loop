//go:build acs

package cycle1885

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const preCycleHead = "517de13ad14e39e958c383e797849904fa55fdb3"

func goTest(t *testing.T, pkg string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"test", "-count=1", "-v"}, append(args, pkg)...)...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test %s failed: %v\n%s", pkg, err, out.String())
	}
	return out.String()
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC1885_001_CommitTreeModeJudgesACommitWithoutTheDisk(t *testing.T) {
	out := goTest(t, "./internal/landed")
	requirePassed(t, out,
		"TestChanges_CommitTree_ACommitWithNoChangeIsLanded",
		"TestChanges_CommitTree_AChangeMainCarriesIsLandedWithoutReadingTheDisk",
		"TestChanges_CommitTree_AChangeMainLacksIsNotLandedAndNamesItsPath",
		"TestChanges_CommitTree_ACommittedDeletionIsLandedOnlyWhenMainDeletedTheFileToo",
		"TestChanges_CommitTree_ExactAcceptsOnlyByteEqualContent",
		"TestChanges_CommitTree_AnUnknownCommitIsAnErrorNotAVerdict",
		"TestChangesOption_NamedWithCommitAndWithExactAreChangesOptions",
	)
}

func TestC1885_002_PruneKeepsItsDeleteDecisionsAfterTheRewire(t *testing.T) {
	out := goTest(t, "./internal/wtcheckpoint", "-run", "TestPrune")
	requirePassed(t, out,
		"TestPrune_LandedDeletesOnlyCheckpointsWhoseChangesAreAllOnOriginMain",
		"TestPrune_LandedKeepsExactEqualityAndSparesACheckpointMainOnlyMerges",
		"TestPrune_LandedKeepsACheckpointThatChangedNothingAgainstItsBase",
		"TestPrune_LandedFailsOnACheckpointWithoutABaseAndDeletesNothing",
		"TestPrune_LandedWithoutOriginMainFailsAndDeletesNothing",
	)
}

func TestC1885_003_HasLandedCallsTheSharedProofAndKeepsNoWalkOfItsOwn(t *testing.T) {
	// acs-predicate: config-check
	file := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "wtcheckpoint", "prune.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	callsChanges, hasWalk := false, false
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "changedPaths" {
			hasWalk = true
		}
		if fn.Name.Name != "hasLanded" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "landed" && sel.Sel.Name == "Changes" {
					callsChanges = true
				}
			}
			return true
		})
	}
	if !callsChanges {
		t.Errorf("hasLanded does not call landed.Changes")
	}
	if hasWalk {
		t.Errorf("prune.go still defines its own changedPaths walk")
	}
}

func TestC1885_004_TheExactEqualityDecisionIsRecordedWithItsPinningTest(t *testing.T) {
	// acs-predicate: config-check
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "internal-landed.md")
	acsassert.FileContains(t, doc, "TestPrune_LandedKeepsExactEqualityAndSparesACheckpointMainOnlyMerges")
	acsassert.FileContains(t, doc, "WithCommit")
	acsassert.FileContains(t, doc, "exact")
}

func TestC1885_005_NoGoLineThisCycleAddsAComment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		"go/internal/landed/landed.go", "go/internal/landed/landed_test.go",
		"go/internal/landed/apicover_named_test.go",
		"go/internal/wtcheckpoint/prune.go", "go/internal/wtcheckpoint/prune_test.go",
		"go/acs/cycle1885/predicates_test.go",
	}
	cmd := exec.Command("git", append([]string{"-C", root, "diff", "-U0", preCycleHead, "--"}, files...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git diff: %v", err)
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		added := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if strings.HasPrefix(added, "//") && !strings.HasPrefix(added, "//go:build") && !strings.HasPrefix(added, "// acs-predicate:") {
			t.Errorf("added comment: %s", added)
		}
	}
}
