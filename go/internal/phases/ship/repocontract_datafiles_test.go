package ship

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const (
	requiredWorkflowTest = "TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies"
	evolveLoopModulePath = "github.com/mickeyyaya/evolve-loop/go"
)

func dataLaneFixture(t *testing.T, change func(repo string)) string {
	t.Helper()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "go", "go.mod"), "module "+evolveLoopModulePath+"\n\ngo 1.24\n")
	mustWrite(t, filepath.Join(repo, ".github", "workflows", "required.yml"), "name: required\n")
	mustWrite(t, filepath.Join(repo, "docs", "a.md"), "a\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "baseline")
	change(repo)
	return repo
}

func recordGoTestJSON(t *testing.T) *[][]string {
	t.Helper()
	prev := goTestJSONFn
	var calls [][]string
	goTestJSONFn = func(_ context.Context, _ string, _ packLog, args []string) packOutcome {
		calls = append(calls, slices.Clone(args))
		return greenPack()
	}
	t.Cleanup(func() { goTestJSONFn = prev })
	return &calls
}

func byNameCall(t *testing.T, calls [][]string) []string {
	t.Helper()
	var found [][]string
	for _, args := range calls {
		if slices.Contains(args, "-run") {
			found = append(found, args)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the pack makes exactly one by-name call; got %d in %q", len(found), calls)
	}
	return found[0]
}

func assertDataRowsRunByName(t *testing.T, args []string) {
	t.Helper()
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, requiredWorkflowTest) || !slices.Contains(args, "./internal/ciparity") {
		t.Fatalf("the by-name call must run %s in ./internal/ciparity;\n%s", requiredWorkflowTest, joined)
	}
	for _, row := range repocontract.DataFileReads() {
		if ciparity.EnvExclusivePkg(row.Package) {
			continue
		}
		if !slices.Contains(args, row.Package) || !strings.Contains(joined, row.Tests[0]) {
			t.Fatalf("the by-name call must carry data row %s %s", row.Package, row.Tests[0])
		}
	}
}

const workflowPath = ".github/workflows/required.yml"

func TestRunRepoContractPack_AWorkflowOnlyChangeRunsTheTestsThatReadIt(t *testing.T) {
	calls := recordGoTestJSON(t)
	repo := dataLaneFixture(t, func(repo string) {
		mustWrite(t, filepath.Join(repo, ".github", "workflows", "required.yml"), "name: required\njobs: {}\n")
	})
	if _, _, err := RunRepoContractPack(context.Background(), "", repo, []string{workflowPath}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || slices.Contains((*calls)[0], "-run") {
		t.Fatalf("the pack runs its whole packages first, then one by-name call; got %q", *calls)
	}
	assertDataRowsRunByName(t, byNameCall(t, *calls))
}

func TestRunRepoContractPack_AGoOnlyChangeLeavesOutTheDataSet(t *testing.T) {
	calls := recordGoTestJSON(t)
	repo := dataLaneFixture(t, func(repo string) {
		mustWrite(t, filepath.Join(repo, "go", "internal", "core", "x.go"), "package core\n")
	})
	if _, _, err := RunRepoContractPack(context.Background(), "", repo, []string{"go/internal/core/x.go"}); err != nil {
		t.Fatal(err)
	}
	args := byNameCall(t, *calls)
	if strings.Contains(strings.Join(args, " "), requiredWorkflowTest) {
		t.Fatalf("a Go-only change leaves out the workflow data row;\n%s", strings.Join(args, " "))
	}
	for _, selection := range repocontract.TreeReadingTests() {
		if !slices.Contains(args, selection.Package) && !ciparity.EnvExclusivePkg(selection.Package) {
			t.Fatalf("the always set carries the tree-reading package %s", selection.Package)
		}
	}
	coarse := byNameCallFor(t, repocontract.UnknownChange())
	if len(args) >= len(coarse) {
		t.Fatalf("the always set is smaller than the coarse set; %d args against %d", len(args), len(coarse))
	}
}

func byNameCallFor(t *testing.T, changed []string) []string {
	t.Helper()
	return repoContractSelectionArgs(repoContractSelectionsFor(changed))
}

func TestRunRepoContractPack_ADocsOnlyLaneGetsTheSameByNameCallAsAWorkflowLane(t *testing.T) {
	calls := recordGoTestJSON(t)
	var byName [][]string
	for _, changed := range [][]string{{workflowPath}, {"docs/a.md"}} {
		*calls = nil
		if _, _, err := RunRepoContractPack(context.Background(), "", dataLaneFixture(t, func(string) {}), changed); err != nil {
			t.Fatal(err)
		}
		byName = append(byName, byNameCall(t, *calls))
	}
	if !slices.Equal(byName[0], byName[1]) {
		t.Fatalf("any non-Go path selects the coarse set (D20);\nworkflow lane %q\ndocs lane     %q", byName[0], byName[1])
	}
	assertDataRowsRunByName(t, byName[1])
}

func TestRunRepoContractGate_AWorkflowOnlyChangeRunsTheTestsThatReadIt(t *testing.T) {
	calls := recordGoTestJSON(t)
	repo := dataLaneFixture(t, func(repo string) {
		mustWrite(t, filepath.Join(repo, ".github", "workflows", "required.yml"), "name: required\njobs: {}\n")
	})
	var out strings.Builder
	if err := runRepoContractGateAt(context.Background(), "enforce", repo, "HEAD", "", &out, nil); err != nil {
		t.Fatalf("a green pack ships: %v\n%s", err, out.String())
	}
	assertDataRowsRunByName(t, byNameCall(t, *calls))
	for _, want := range []string{"importer backstop: no Go change in the tree — skipped", "tree-reading and data-read tests by name: go test", requiredWorkflowTest} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the gate log must say %q;\n%s", want, out.String())
		}
	}
}

func TestRunRepoContractGate_AGoOnlyChangeLogsAndRunsTheAlwaysSet(t *testing.T) {
	calls := recordGoTestJSON(t)
	repo := dataLaneFixture(t, func(repo string) {
		mustWrite(t, filepath.Join(repo, "go", "internal", "core", "x.go"), "package core\n")
	})
	var out strings.Builder
	_ = runRepoContractGateAt(context.Background(), "enforce", repo, "HEAD", "", &out, nil)
	if strings.Contains(strings.Join(byNameCall(t, *calls), " "), requiredWorkflowTest) || strings.Contains(out.String(), requiredWorkflowTest) {
		t.Fatalf("a Go-only change leaves out the workflow data row, in the call and in the log;\n%s", out.String())
	}
}

func TestRunRepoContractGate_ADiscoveryFailureRefusesBeforeThePackRuns(t *testing.T) {
	calls := recordGoTestJSON(t)
	root := t.TempDir()
	var out strings.Builder
	err := runRepoContractGateAt(context.Background(), "enforce", root, "HEAD", "", &out, nil)
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractInfra {
		t.Fatalf("an undiscoverable change is the INFRA refusal, got %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("ship has no fallback: the pack must not run when the change cannot be read; got %q", *calls)
	}
}

func TestRepoContractSelections_LeaveOutEnvExclusiveRows(t *testing.T) {
	t.Parallel()
	row := func(pkg string, tests ...string) repocontract.TestSelection {
		return repocontract.TestSelection{Package: pkg, Tests: tests}
	}
	got := tierSelections(
		[]repocontract.TestSelection{row("./internal/bridge", "TestTree", "TestData"), row("./internal/ciparity", "TestCI"), row("./internal/bridge/sub", "TestSubOnly")},
		[]repocontract.TestSelection{row("./internal/bridge", "TestTree")},
	)
	want := []repocontract.TestSelection{row("./internal/bridge", "TestTree"), row("./internal/ciparity", "TestCI"), row("./internal/bridge/sub", "TestSubOnly")}
	if len(got) != len(want) {
		t.Fatalf("tierSelections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Package != want[i].Package || !slices.Equal(got[i].Tests, want[i].Tests) {
			t.Errorf("row %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRepoContractSelections_DropAnEnvExclusivePackageWithOnlyDataRows(t *testing.T) {
	t.Parallel()
	got := tierSelections([]repocontract.TestSelection{{Package: "./internal/bridge", Tests: []string{"TestData"}}}, nil)
	if len(got) != 0 {
		t.Errorf("tierSelections = %v, want no row for a data-only env-exclusive package", got)
	}
}

func TestRepoContractSelections_CarryNoDataOnlyTestOfAnEnvExclusivePackage(t *testing.T) {
	t.Parallel()
	tree := map[string][]string{}
	for _, selection := range repocontract.TreeReadingTests() {
		tree[selection.Package] = selection.Tests
	}
	for _, selection := range repoContractSelectionsFor(repocontract.UnknownChange()) {
		if !ciparity.EnvExclusivePkg(selection.Package) {
			continue
		}
		for _, test := range selection.Tests {
			if !slices.Contains(tree[selection.Package], test) {
				t.Errorf("%s.%s is a data-read test of an env-exclusive package; its tmux boots time out under a live wave", selection.Package, test)
			}
		}
	}
	if !slices.ContainsFunc(repocontract.DataFileReads(), func(row repocontract.DataFileRead) bool { return ciparity.EnvExclusivePkg(row.Package) }) {
		t.Fatal("the registry holds no data row of an env-exclusive package, so the filter proves nothing")
	}

}
