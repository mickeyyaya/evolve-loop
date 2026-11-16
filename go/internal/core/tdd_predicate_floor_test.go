package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
)

const tddFloorExistsHelper = "package %s\n\nimport \"os\"\n\nfunc %s(p string) bool {\n\t_, err := os.Stat(p)\n\treturn err == nil\n}\n"

const tddFloorOtherHelper = "package %s\n\nimport \"os\"\n\nfunc %s(p string) []byte {\n\tdata, _ := os.ReadFile(p)\n\treturn data\n}\n"

func tddFloorWorktree(t *testing.T, predicateHelper string) string {
	t.Helper()
	wt := t.TempDir()
	writeFileAt(t, filepath.Join(wt, "go", ".test-dup-baseline"), "count=0\ndate=2026-11-09\n")
	writeFileAt(t, filepath.Join(wt, "go", "internal", "a", "a_test.go"), fmt.Sprintf(tddFloorExistsHelper, "a", "exists"))
	writeFileAt(t, filepath.Join(wt, "go", "acs", "cycle9", "predicates_test.go"), fmt.Sprintf(predicateHelper, "cycle9", "onDisk"))
	return wt
}

func tddFloorInput(wt string, stderr *bytes.Buffer) ReviewInput {
	return ReviewInput{Phase: string(PhaseTDD), Cycle: 9, Worktree: wt, Stderr: stderr}
}

func TestTddPredicateFloor_TheOrchestratorSendsAPredicateThatCopiesAHelperBackToTdd(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)

	res := NewOrchestrator(nil, nil, nil).performEffectsAndReview(context.Background(), tddFloorInput(wt, &stderr))

	if res.Approve || !res.Retry {
		t.Fatalf("Review = %+v: TDD is the only phase that can change go/acs/cycle9/, so the host must send the copy back to it", res)
	}
	want := "acs/cycle9/predicates_test.go:onDisk = internal/a/a_test.go:exists"
	if len(res.Failures) != 1 || res.Failures[0] != want {
		t.Errorf("Failures = %q, want [%q]", res.Failures, want)
	}
	for _, part := range []string{tddFloorReasonHead, "go/acs/cycle9/", "pkg/acsassert", want} {
		if !strings.Contains(res.Reason, part) {
			t.Errorf("Reason omits %q, so TDD cannot act on it: %q", part, res.Reason)
		}
	}
}

func TestTddPredicateFloor_AComposedReviewerChainStillRunsTheFloor(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)
	o := NewOrchestrator(nil, nil, nil, WithReviewer(ChainReviewers(noopReviewer{})))

	res := o.performEffectsAndReview(context.Background(), tddFloorInput(wt, &stderr))

	if res.Approve || !res.Retry || len(res.Failures) != 1 {
		t.Fatalf("Review = %+v: the composition root mounts its own chain through WithReviewer, and the TDD floor must stay in front of it", res)
	}
}

func TestTddPredicateFloor_ApprovesAPredicateWithItsOwnHelperBody(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorOtherHelper)

	if res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr)); !res.Approve {
		t.Fatalf("Review = %+v, want approve: the predicate copies no helper body", res)
	}
}

func TestTddPredicateFloor_JudgesOnlyTheTddHandoffOfACycleWithAWorktree(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)
	cases := map[string]ReviewInput{
		"build phase": {Phase: string(PhaseBuild), Cycle: 9, Worktree: wt, Stderr: &stderr},
		"no worktree": {Phase: string(PhaseTDD), Cycle: 9, Stderr: &stderr},
		"no cycle":    {Phase: string(PhaseTDD), Worktree: wt, Stderr: &stderr},
	}
	for name, in := range cases {
		if res := (tddPredicateFloor{}).Review(context.Background(), in); !res.Approve {
			t.Errorf("%s: Review = %+v, want approve", name, res)
		}
	}
}

func TestTddPredicateFloor_ATreeWithoutTheRatchetIsNotJudged(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)
	if err := os.Remove(filepath.Join(wt, "go", ".test-dup-baseline")); err != nil {
		t.Fatal(err)
	}

	res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr))

	if !res.Approve || stderr.Len() != 0 {
		t.Fatalf("Review = %+v, stderr = %q: a project without go/.test-dup-baseline has no ratchet to apply", res, stderr.String())
	}
}

func TestTddPredicateFloor_AnUnreadableRatchetWarnsAndLeavesTheCountToShip(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, wt string){
		"malformed baseline": func(t *testing.T, wt string) {
			writeFileAt(t, filepath.Join(wt, "go", ".test-dup-baseline"), "count=many\n")
		},
		"baseline is a directory": func(t *testing.T, wt string) {
			path := filepath.Join(wt, "go", ".test-dup-baseline")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"test file does not parse": func(t *testing.T, wt string) {
			writeFileAt(t, filepath.Join(wt, "go", "internal", "b", "b_test.go"), "package b\n\nfunc broken( {\n")
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			wt := tddFloorWorktree(t, tddFloorExistsHelper)
			breakIt(t, wt)

			res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr))

			if !res.Approve || !strings.Contains(stderr.String(), "[tdd-floor] WARN") {
				t.Fatalf("Review = %+v, stderr = %q: a check that cannot read its inputs warns loudly and leaves the count to the ship ratchet", res, stderr.String())
			}
		})
	}
}

const tddFloorKitSource = `package acsassert

import "testing"

var TestingForms = map[string]string{"ModuleDir": "GoModuleDir"}

func ModuleDir(start string) (string, error) { return start, nil }

func GoModuleDir(tb testing.TB) string { return "" }

func RepoRoot(t *testing.T) string { return "" }

func OutputTail(output string, n int) string { return output }
`

const (
	tddWrapRepoRoot = "package %s\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n\n\t\"x/pkg/acsassert\"\n)\n\nfunc %s(t *testing.T) string {\n\tt.Helper()\n\treturn filepath.Join(acsassert.RepoRoot(t), \"go\")\n}\n"
	tddWrapModule   = "package %s\n\nimport (\n\t\"testing\"\n\n\t\"x/pkg/acsassert\"\n)\n\nfunc %s(t *testing.T) string {\n\tt.Helper()\n\tdir, err := acsassert.ModuleDir(\".\")\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\treturn dir\n}\n"
	tddWrapBoth     = "package %s\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n\n\t\"x/pkg/acsassert\"\n)\n\nfunc %s(t *testing.T) string {\n\tt.Helper()\n\tdir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), \"go\"))\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\treturn dir\n}\n"
)

func tddFloorKitWorktree(t *testing.T, files map[string]string) string {
	t.Helper()
	wt := t.TempDir()
	writeFileAt(t, filepath.Join(wt, "go", ".test-dup-baseline"), "count=0\ndate=2026-11-09\n")
	writeFileAt(t, filepath.Join(wt, "go", "pkg", "acsassert", "predicate_kit.go"), tddFloorKitSource)
	for rel, content := range files {
		writeFileAt(t, filepath.Join(wt, "go", filepath.FromSlash(rel)), content)
	}
	return wt
}

func TestTddPredicateFloor_TheReasonNamesTheReplacementOfEachIncidentBody(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body, wantText string
	}{
		"cycle2320 goDir wraps RepoRoot":         {tddWrapRepoRoot, "inline it, or call acsassert.RepoRoot directly"},
		"cycle2323 moduleDir wraps ModuleDir":    {tddWrapModule, "call acsassert.GoModuleDir"},
		"cycle2328 wraps RepoRoot and ModuleDir": {tddWrapBoth, "call acsassert.GoModuleDir"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			wt := tddFloorKitWorktree(t, map[string]string{
				"acs/cycle8/predicates_test.go": fmt.Sprintf(tc.body, "cycle8", "moduleDir"),
				"acs/cycle9/predicates_test.go": fmt.Sprintf(tc.body, "cycle9", "moduleDir"),
			})

			res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr))

			if res.Approve {
				t.Fatalf("Review = %+v, want reject", res)
			}
			if !strings.Contains(res.Reason, tc.wantText) || strings.Contains(res.Reason, "the existing helper") {
				t.Errorf("Reason = %q, want %q and no \"the existing helper\": a helper of another cycle cannot be imported", res.Reason, tc.wantText)
			}
		})
	}
}

func TestTddPredicateFloor_OnlyAGroupInsideTheCycleDirectoryGetsTheExistingHelper(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		files        map[string]string
		wantExisting bool
	}{
		"every site in the cycle directory": {map[string]string{
			"acs/cycle9/predicates_test.go": fmt.Sprintf(tddFloorExistsHelper, "cycle9", "onDisk"),
			"acs/cycle9/more_test.go":       fmt.Sprintf(tddFloorExistsHelper, "cycle9", "present"),
		}, true},
		"two cycle sites and a package site": {map[string]string{
			"acs/cycle9/predicates_test.go": fmt.Sprintf(tddFloorExistsHelper, "cycle9", "onDisk"),
			"acs/cycle9/more_test.go":       fmt.Sprintf(tddFloorExistsHelper, "cycle9", "present"),
			"internal/a/a_test.go":          fmt.Sprintf(tddFloorExistsHelper, "a", "exists"),
		}, false},
		"a package site and a cycle site": {map[string]string{
			"acs/cycle9/predicates_test.go": fmt.Sprintf(tddFloorExistsHelper, "cycle9", "onDisk"),
			"internal/a/a_test.go":          fmt.Sprintf(tddFloorExistsHelper, "a", "exists"),
		}, false},
		"a later cycle and an earlier cycle": {map[string]string{
			"acs/cycle9/predicates_test.go": fmt.Sprintf(tddFloorExistsHelper, "cycle9", "onDisk"),
			"acs/cycle8/predicates_test.go": fmt.Sprintf(tddFloorExistsHelper, "cycle8", "exists"),
		}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			wt := tddFloorKitWorktree(t, tc.files)

			res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr))

			if res.Approve {
				t.Fatalf("Review = %+v, want reject", res)
			}
			if got := strings.Contains(res.Reason, "call the existing helper"); got != tc.wantExisting {
				t.Errorf("Reason = %q: existing-helper text = %v, want %v", res.Reason, got, tc.wantExisting)
			}
			if !tc.wantExisting && !strings.Contains(res.Reason, "cannot be imported") {
				t.Errorf("Reason = %q, want the cannot-be-imported cause", res.Reason)
			}
		})
	}
}

func TestTddCopyRemedy_NamesNoFunctionWhenTheKitIsUnreadable(t *testing.T) {
	t.Parallel()
	good := t.TempDir()
	writeFileAt(t, filepath.Join(good, "pkg", "acsassert", "predicate_kit.go"), tddFloorKitSource)
	broken := t.TempDir()
	writeFileAt(t, filepath.Join(broken, "pkg", "acsassert", "predicate_kit.go"), "package acsassert\n\nfunc broken( {\n")
	writeFileAt(t, filepath.Join(broken, "pkg", "acsassert", "other.go"), "package acsassert\n\nvar TestingForms = map[string]string{\"ModuleDir\": \"GoModuleDir\"}\n")
	writeFileAt(t, filepath.Join(good, "pkg", "acsassert", "kit_test.go"), "package acsassert\n\nfunc broken( {\n")
	group := testdup.Group{Sites: []string{"acs/cycle9/p_test.go:m", "internal/a/a_test.go:m"}, Wraps: []string{"ModuleDir"}}

	cases := map[string]struct {
		dir  string
		want string
	}{
		"missing package": {t.TempDir(), "inline it"},
		"unparsable file": {broken, "inline it"},
		"readable kit":    {good, "call acsassert.GoModuleDir"},
	}
	for name, tc := range cases {
		if got := tddCopyRemedy(group, 9, loadTddKit(tc.dir)); !strings.HasSuffix(got, tc.want) {
			t.Errorf("%s: remedy = %q, want suffix %q", name, got, tc.want)
		}
	}
}

func TestTddCopyRemedy_ACopyOfNoKitCallIsInlined(t *testing.T) {
	t.Parallel()
	kit := tddKit{forms: map[string]string{"ModuleDir": "GoModuleDir"}, testing: map[string]bool{"GoModuleDir": true, "RepoRoot": true}}
	other := testdup.Group{Sites: []string{"acs/cycle9/p_test.go:m", "internal/a/a_test.go:m"}, Wraps: []string{"OutputTail", "RepoRoot"}}
	none := testdup.Group{Sites: []string{"acs/cycle9/p_test.go:m", "internal/a/a_test.go:m"}}
	if got := tddCopyRemedy(other, 9, kit); !strings.HasSuffix(got, "inline it") {
		t.Errorf("remedy for a copy that wraps a kit call with no testing form = %q, want inline it", got)
	}
	if got := tddCopyRemedy(none, 9, kit); !strings.HasSuffix(got, "inline it") {
		t.Errorf("remedy for a copy that wraps no kit call = %q, want inline it", got)
	}
}

func TestLoadTddKit_ReadsOnlyWellFormedFormsAndTestingFunctions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	odd := "package acsassert\n\nimport \"testing\"\n\nfunc NoParams() {}\n\nfunc OkForm(t *testing.T) {}\n\nfunc (r recv) Method(t *testing.T) {}\n\nfunc lower(t *testing.T) {}\n\nvar a, b = 1, 2\n\nvar TestingForms = map[string]string{\"Solo\", \"Num\": 7, 8: \"x\", \"Ok\": \"OkForm\"}\n"
	computed := "package acsassert\n\nvar TestingForms = computeForms()\n"
	writeFileAt(t, filepath.Join(dir, "pkg", "acsassert", "odd.go"), odd)
	writeFileAt(t, filepath.Join(dir, "pkg", "acsassert", "computed.go"), computed)

	kit := loadTddKit(dir)

	if len(kit.forms) != 1 || kit.forms["Ok"] != "OkForm" {
		t.Errorf("forms = %v, want only Ok -> OkForm", kit.forms)
	}
	if !kit.testing["OkForm"] || kit.testing["NoParams"] || kit.testing["Method"] || kit.testing["lower"] {
		t.Errorf("testing = %v, want only OkForm", kit.testing)
	}
}

func TestLoadTddKit_TheRealKitNamesGoModuleDirForAModuleDirCopy(t *testing.T) {
	t.Parallel()
	kit := loadTddKit(filepath.Join("..", ".."))
	group := testdup.Group{Sites: []string{"acs/cycle9/p_test.go:moduleDir", "acs/cycle8/p_test.go:moduleDir"}, Wraps: []string{"ModuleDir"}}

	if got := tddCopyRemedy(group, 9, kit); !strings.HasSuffix(got, "call acsassert.GoModuleDir") {
		t.Fatalf("remedy on the real kit = %q, want it to name acsassert.GoModuleDir", got)
	}
	if !kit.testing["GoModuleDir"] || !kit.testing["RepoRoot"] || kit.testing["FileExists"] {
		t.Fatalf("testing forms of the real kit = %v, want GoModuleDir and RepoRoot and not the package TB form FileExists", kit.testing)
	}
}
