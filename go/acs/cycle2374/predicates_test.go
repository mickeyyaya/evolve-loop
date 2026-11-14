//go:build acs

package cycle2374

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const curatorPackage = "./internal/inboxcurator"

var changedFunctionFiles = []string{"landing.go", "gitlog.go"}

var coverLine = regexp.MustCompile(`(?m)(?:landing|gitlog)\.go:\d+:\s+(\w+)\s`)

func runCuratorTests(t *testing.T, pattern string, extra ...string) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	args := append([]string{"-C", goDir, "test", "-count=1", "-v", "-run", pattern}, extra...)
	args = append(args, curatorPackage)
	out, _, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil || code != 0 {
		t.Fatalf("go test %s exit=%d: %v\n%s", pattern, code, err, acsassert.OutputTail(out, 25))
	}
	return out
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(out, 15))
		}
	}
}

func TestC2374_001_ACloseInboxTrailerInAMergedBranchIsADeterministicConsume(t *testing.T) {
	const name = "TestMatchLanding_AClosesInboxTrailerInAMergedBranchIsADeterministicConsume"

	requirePassed(t, runCuratorTests(t, "^"+name+"$"), name)
}

func TestC2374_002_ThePR1220ReplayReadsTheMergedBranchBodiesThroughGitAndConsumesTheItem(t *testing.T) {
	names := []string{
		"TestCommit_PR1220ReplayAMergeCommitReadsTheClosesIdOfItsMergedBranchBodies",
		"TestCurate_APR1220ReplayThroughTheLandingEntryConsumesTheItem",
		"TestLanded_AMergeReadsOnlyItsMergedBranchAndNeverTheFirstParentHistory",
		"TestCommit_AnOctopusMergeReadsEveryMergedBranch",
		"TestCommit_AnOrdinaryCommitCarriesTheClosesIdsOfItsOwnBody",
	}

	requirePassed(t, runCuratorTests(t, "^("+strings.Join(names, "|")+")$"), names...)
}

func TestC2374_003_NoneRefsAndUnrelatedIdsConsumeNothing(t *testing.T) {
	names := []string{
		"TestMatchLanding_ATrailerThatClosesNoOpenItemProposesNothing",
		"TestCurate_AMergedBranchThatDeclaresNoneOrOnlyRefsConsumesNothing",
	}

	requirePassed(t, runCuratorTests(t, "^("+strings.Join(names, "|")+")$"), names...)
}

func TestC2374_007_TheTrailerGrammarOfAMergedBranchAndOfACommitIsTheGrammarOfClosesinbox(t *testing.T) {
	names := []string{
		"TestCommit_AMergedBranchBodyReadsClosesInboxByTheTrailerGrammarOfClosesinbox",
		"TestCommit_AnOrdinaryCommitBodyReadsClosesInboxByTheTrailerGrammarOfClosesinbox",
	}

	requirePassed(t, runCuratorTests(t, "^("+strings.Join(names, "|")+")$"), names...)
}

func TestC2374_008_TheCuratorHoldsNoSecondCopyOfTheClosesInboxGrammar(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "inboxcurator", "gitlog.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	imported := false
	for _, spec := range f.Imports {
		imported = imported || strings.HasSuffix(strings.Trim(spec.Path.Value, `"`), "/internal/closesinbox")
	}
	if !imported {
		t.Errorf("gitlog.go does not import internal/closesinbox")
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "closeIDs" {
				t.Errorf("gitlog.go still declares closeIDs, a second copy of closesinbox.IDs")
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if v, ok := spec.(*ast.ValueSpec); ok {
					for _, n := range v.Names {
						if n.Name == "closesLineRE" {
							t.Errorf("gitlog.go still declares closesLineRE, a second copy of the trailer grammar")
						}
					}
				}
			}
		}
	}
}

func TestC2374_004_EveryFunctionOfTheChangedFilesHasFullStatementCoverage(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	if out, _, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-coverprofile="+profile, curatorPackage); err != nil || code != 0 {
		t.Fatalf("go test -coverprofile exit=%d: %v\n%s", code, err, acsassert.OutputTail(out, 25))
	}
	report, _, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d: %v", code, err)
	}
	listed := coverLine.FindAllStringSubmatch(report, -1)
	if len(listed) == 0 {
		t.Fatalf("the cover report lists no function of %v", changedFunctionFiles)
	}
	for _, m := range listed {
		pct, ferr := acsassert.FuncCoverage(report, m[1])
		if ferr != nil || pct < 100 {
			t.Errorf("%s coverage = %.1f%% (%v), want 100%%", m[1], pct, ferr)
		}
	}
	for _, required := range []string{"matchOne", "readLog"} {
		if pct, ferr := acsassert.FuncCoverage(report, required); ferr != nil || pct < 100 {
			t.Errorf("%s coverage = %.1f%% (%v), want 100%%", required, pct, ferr)
		}
	}
}

func TestC2374_005_TheChangedFilesCarryNoCodeComment(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "inboxcurator")
	fset := token.NewFileSet()
	for _, name := range changedFunctionFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range f.Comments {
			t.Errorf("%s holds a comment at %s: %q", name, fset.Position(group.Pos()), group.Text())
		}
	}
	doc, err := parser.ParseFile(fset, filepath.Join(dir, "doc.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Comments) != 1 || doc.Doc == nil || doc.Comments[0] != doc.Doc {
		t.Errorf("doc.go holds %d comment groups, want only the package doc", len(doc.Comments))
	}
}

func TestC2374_006_TheSpecAndThePackageNoteStateTheChangeInSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"docs/architecture/inbox-curator.md", "docs/architecture/packages/internal-inboxcurator.md"} {
		path := filepath.Join(root, rel)
		findings, lerr := stelint.LintFile(path, stelint.Options{Words: words})
		if lerr != nil {
			t.Fatal(lerr)
		}
		for _, f := range findings {
			if !f.IsSkipNotice() {
				t.Errorf("%s:%d %s %s: %s", rel, f.Line, f.Rule, f.Message, f.Excerpt)
			}
		}
		// acs-predicate: config-check
		for _, needle := range []string{"Closes-Inbox", "merged branch", "Change.Closes"} {
			if !acsassert.FileContains(t, path, needle) {
				t.Errorf("%s does not state the change: %q missing", rel, needle)
			}
		}
	}
}
