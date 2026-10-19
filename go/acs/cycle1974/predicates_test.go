//go:build acs

package cycle1974

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const requestPkg = "./internal/handoff/request"

const internalPrefix = "github.com/mickeyyaya/evolve-loop/go/internal/"

const (
	forbiddenPackage = internalPrefix + "core"
	handoffPackage   = internalPrefix + "handoff"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func requestDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(goDir(t), "internal", "handoff", "request")
}

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"-C", goDir(t), "test", "-count=1"}, args...)
	out, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil && code == 0 {
		t.Fatalf("go test did not run: %v", err)
	}
	return out + stderr, code
}

func requirePass(t *testing.T, pattern string, passes ...string) {
	t.Helper()
	out, code := goTest(t, "-v", "-run", pattern, requestPkg)
	if code != 0 {
		t.Errorf("RED: go test -run %s %s exit=%d\n%s", pattern, requestPkg, code, out)
	}
	for _, name := range passes {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass\n%s", name, out)
		}
	}
	if strings.Contains(out, "--- SKIP") {
		t.Errorf("a selected test skipped:\n%s", out)
	}
}

func TestC1974_001_TheTriageTddRequestForCycle1946ListsItsTwoAcceptanceEntriesWithSourceAndVerbatimText(t *testing.T) {
	requirePass(t, "^TestTriageTDD",
		"TestTriageTDD_Cycle1946FixtureListsItsTwoAcceptanceEntries",
		"TestTriageTDD_TextIsTheInboxAcceptanceVerbatim",
		"TestTriageTDDFromItems_Table/padding_around_an_entry_is_trimmed_like_the_task_contract",
		"TestTriageTDDFromItems_Table/a_blank_entry_is_skipped_and_the_numbering_keeps_the_position",
		"TestTriageTDDFromItems_Table/asks_follow_the_item_order_and_ids_never_collide_across_items",
		"TestTriageTDD_AnItemDeferredByTriageGetsNoAsk",
		"TestTriageTDD_LaneScopePinWinsOverTheDecision",
		"TestTriageTDD_RequestBindsALedgerThatValidates")
}

func TestC1974_002_TheTriageTddBuilderRefusesARequestItCannotBuildWhole(t *testing.T) {
	requirePass(t, "^TestTriageTDD_RefusesARequestItCannotBuildWhole$",
		"TestTriageTDD_RefusesARequestItCannotBuildWhole/no_triage_decision_means_no_committed_set",
		"TestTriageTDD_RefusesARequestItCannotBuildWhole/an_empty_top_n_commits_nothing",
		"TestTriageTDD_RefusesARequestItCannotBuildWhole/a_committed_id_with_no_inbox_record_is_named_in_the_error",
		"TestTriageTDD_RefusesARequestItCannotBuildWhole/an_unreadable_inbox_record_is_named_in_the_error")
}

func TestC1974_003_TheTddBuildRequestListsEachPredicateOfTheCycle1950Package(t *testing.T) {
	requirePass(t, "^TestTDDBuild",
		"TestTDDBuild_Cycle1950FixtureListsEachPredicate",
		"TestTDDBuild_ListsOnlyPredicatesAndKeepsTheSourceOrder",
		"TestTDDBuild_DigestDoesNotDependOnTheCheckoutPath",
		"TestTDDBuild_RefusesAPackageItCannotListWhole/a_missing_directory",
		"TestTDDBuild_RefusesAPackageItCannotListWhole/a_package_with_no_predicate",
		"TestTDDBuild_RefusesAPackageItCannotListWhole/a_directory_with_no_test_file",
		"TestTDDBuild_RefusesAPackageItCannotListWhole/a_test_file_that_does_not_parse",
		"TestTDDBuild_RequestBindsALedgerThatValidates")
}

func TestC1974_004_TheScoutTriageRequestAsksOncePerSelectedSlug(t *testing.T) {
	requirePass(t, "^Test(ScoutTriage_Table|PairNames_MatchTheSpecTable)$",
		"TestScoutTriage_Table/no_selection_gives_no_asks",
		"TestScoutTriage_Table/each_selected_slug_is_one_ask_in_selection_order",
		"TestScoutTriage_Table/blank_slugs_are_dropped_and_padding_is_trimmed",
		"TestScoutTriage_Table/a_repeated_slug_is_asked_once",
		"TestPairNames_MatchTheSpecTable")
}

func TestC1974_005_ThePackageDoesNotImportCoreDirectlyOrThroughItsDependencies(t *testing.T) {
	requirePass(t, "^TestPackageDoesNot",
		"TestPackageDoesNotImportCore",
		"TestPackageDoesNotReachCoreThroughItsDependencies")
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "list", "-deps", requestPkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go list -deps %s exit=%d: %v\n%s", requestPkg, code, err, stderr)
	}
	deps := strings.Fields(out)
	if !containsString(deps, handoffPackage) {
		t.Errorf("RED: %s does not build on internal/handoff: %v", requestPkg, deps)
	}
	if containsString(deps, forbiddenPackage) {
		t.Errorf("RED: %s reaches %s", requestPkg, forbiddenPackage)
	}
}

func containsString(in []string, want string) bool {
	for _, s := range in {
		if s == want {
			return true
		}
	}
	return false
}

func listedLine(t *testing.T, path, pkg string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], "#") && fields[0] == pkg {
			return fields
		}
	}
	return nil
}

func exportedFuncs(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, perr := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if perr != nil {
			t.Fatalf("RED: %s does not parse: %v", file, perr)
		}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.IsExported() {
				names = append(names, fn.Name.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

var markdownLink = regexp.MustCompile(`\]\(([^)#\s]+\.md)(?:#[^)]*)?\)`)

func TestC1974_006_ThePackageNoteListsEveryBuilderAndStillLinksTheSpec(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, "docs", "architecture", "packages", "internal-handoff.md")
	raw, err := os.ReadFile(note)
	if err != nil {
		t.Fatalf("RED: package note missing: %v", err)
	}
	body := string(raw)

	builders := exportedFuncs(t, requestDir(t))
	if len(builders) == 0 {
		t.Fatalf("RED: %s declares no exported builder", requestPkg)
	}
	for _, required := range []string{"ScoutTriage", "TriageTDD", "TDDBuild"} {
		if !containsString(builders, required) {
			t.Errorf("RED: %s does not declare the builder %s (found %v)", requestPkg, required, builders)
		}
	}
	for _, name := range builders {
		if !strings.Contains(body, "`"+name+"`") {
			t.Errorf("RED: the package note does not list the builder `%s`", name)
		}
	}
	if !strings.Contains(body, "internal/handoff/request") {
		t.Errorf("RED: the package note does not name the package internal/handoff/request")
	}

	spec := filepath.Join(root, "docs", "architecture", "phase-handoff-contract.md")
	linked := false
	for _, m := range markdownLink.FindAllStringSubmatch(body, -1) {
		target := filepath.Clean(filepath.Join(filepath.Dir(note), m[1]))
		if target != spec {
			continue
		}
		linked = true
		if _, err := os.Stat(target); err != nil {
			t.Errorf("RED: link %q does not resolve: %v", m[1], err)
		}
	}
	if !linked {
		t.Errorf("RED: %s lost its relative link to docs/architecture/phase-handoff-contract.md", note)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", note); code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", note)
	}
}

func TestC1974_007_TheRequestPackageIsAtOneHundredAndEnrolledInBothGates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	strict := listedLine(t, filepath.Join(root, "go", ".cover-strict"), requestPkg)
	if len(strict) != 2 {
		t.Fatalf("RED: go/.cover-strict has no %q line with a minimum: %v", requestPkg, strict)
	}
	minimum, err := strconv.ParseFloat(strict[1], 64)
	if err != nil || minimum != 100 {
		t.Errorf("RED: go/.cover-strict minimum for %s = %q, want 100", requestPkg, strict[1])
	}
	if enforce := listedLine(t, filepath.Join(root, "go", ".apicover-enforce"), requestPkg); len(enforce) != 1 {
		t.Errorf("RED: go/.apicover-enforce does not list %s on a line of its own: %v", requestPkg, enforce)
	}

	profile := filepath.Join(t.TempDir(), "cover.out")
	out, code := goTest(t, "-tags", "integration", "-coverprofile="+profile, requestPkg)
	if code != 0 {
		t.Fatalf("RED: go test %s exit=%d\n%s", requestPkg, code, out)
	}
	if !strings.Contains(out, "coverage: 100.0% of statements") {
		t.Errorf("RED: %s is not at 100.0%% of statements\n%s", requestPkg, out)
	}

	funcs, stderr, fcode, ferr := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if ferr != nil || fcode != 0 {
		t.Fatalf("go tool cover exit=%d: %v\n%s", fcode, ferr, stderr)
	}
	funcFile := filepath.Join(t.TempDir(), "cover.func.txt")
	if err := os.WriteFile(funcFile, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	apiOut, apiErr, apiCode, aerr := acsassert.SubprocessOutput("go", "-C", goDir(t), "run", "./cmd/apicover", "-enforce", "-cover", funcFile, requestDir(t))
	if aerr != nil && apiCode == 0 {
		t.Fatalf("apicover did not run: %v", aerr)
	}
	if apiCode != 0 {
		t.Errorf("RED: apicover -enforce exit=%d (an exported symbol is unnamed or uncovered)\n%s%s", apiCode, apiOut, apiErr)
	}
}

func TestC1974_008_TheRequestPackageCarriesNoComments(t *testing.T) {
	dir := requestDir(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("RED: no Go files in %s (err=%v)", dir, err)
	}
	var production int
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			production++
		}
		parsed, perr := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
		if perr != nil {
			t.Errorf("RED: %s does not parse: %v", file, perr)
			continue
		}
		for _, group := range parsed.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") {
					t.Errorf("RED: %s carries a comment: %q", filepath.Base(file), c.Text)
				}
			}
		}
	}
	if production == 0 {
		t.Errorf("RED: %s has no production Go file", dir)
	}
}

func TestC1974_009_TheFixturesHideNoGoFileFromTheBuildFloor(t *testing.T) {
	// acs-predicate: config-check
	requirePass(t, "^TestTDDBuild_Cycle1950FixtureListsEachPredicate$",
		"TestTDDBuild_Cycle1950FixtureListsEachPredicate")
	var goFiles []string
	err := filepath.WalkDir(filepath.Join(requestDir(t), "testdata"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RED: testdata of %s is unreadable: %v", requestPkg, err)
	}
	if len(goFiles) != 0 {
		t.Errorf("RED: the build floor runs tagged Go files under testdata (cycle 1965 failed on this): %v", goFiles)
	}
}
