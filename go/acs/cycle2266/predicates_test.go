//go:build acs

package cycle2266

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	judgmentID = "cycle-5-judgment-plan-review"
	designRel  = "docs/architecture/recovery-retrospective.md"
	planRel    = "docs/plans/recovery-retrospective-2026-10.md"
)

var judgmentTodos = []cyclestate.CarryoverTodo{{ID: judgmentID, Action: "the plan widens the lane scope"}}

func plane(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("README.md", "plane\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func workspaceOf(r *gittest.Repo) string {
	return filepath.Join(r.Dir, ".evolve", "runs", "cycle-5")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func goTestPasses(t *testing.T, pkg, name string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: %s in %s exit=%d: %v\n%s", name, pkg, code, err, acsassert.OutputTail(out, 20))
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Errorf("RED: %s in %s did not run and pass:\n%s", name, pkg, acsassert.OutputTail(out, 10))
	}
}

func TestC2266_001_EnforcePairGoesToThePendingDirAndNowhereElse(t *testing.T) {
	r := plane(t)
	workspace := workspaceOf(r)

	written, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, workspace, 5, judgmentTodos)

	if err != nil || len(written) != 2 {
		t.Fatalf("RED: WriteInto = %v, %v; want one pair", written, err)
	}
	for _, ext := range []string{".md", ".yaml"} {
		if !exists(filepath.Join(recoveryretro.PendingDir(r.Dir), judgmentID+ext)) {
			t.Errorf("RED: no pending %s", ext)
		}
	}
	if exists(filepath.Join(workspace, recoveryretro.DirName)) {
		t.Errorf("RED: enforce also wrote into the workspace")
	}
	if exists(filepath.Join(r.Dir, recoveryretro.DocsHome)) {
		t.Errorf("RED: the closeout wrote into %s; only the boundary publish may", recoveryretro.DocsHome)
	}
}

func TestC2266_002_BoundaryPublishCommitsThePendingPairIntoDocs(t *testing.T) {
	r := plane(t)
	if _, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, workspaceOf(r), 5, judgmentTodos); err != nil {
		t.Fatalf("RED: WriteInto: %v", err)
	}
	wantMD, err := os.ReadFile(filepath.Join(recoveryretro.PendingDir(r.Dir), judgmentID+".md"))
	if err != nil {
		t.Fatalf("RED: pending md unreadable: %v", err)
	}

	published, err := recoveryretro.PublishPending(r.Dir, &strings.Builder{})

	if err != nil || len(published) != 1 || published[0] != judgmentID {
		t.Fatalf("RED: PublishPending = %v, %v", published, err)
	}
	names := r.Git("show", "--name-only", "--format=", "HEAD")
	for _, ext := range []string{".md", ".yaml"} {
		if !strings.Contains(names, recoveryretro.DocsHome+"/"+judgmentID+ext) {
			t.Errorf("RED: HEAD does not commit %s%s:\n%s", judgmentID, ext, names)
		}
	}
	got, err := os.ReadFile(filepath.Join(r.Dir, recoveryretro.DocsHome, judgmentID+".md"))
	if err != nil || string(got) != string(wantMD) {
		t.Errorf("RED: the published md differs from the pending md: %v", err)
	}
	if exists(filepath.Join(recoveryretro.PendingDir(r.Dir), judgmentID+".md")) {
		t.Errorf("RED: the pending copy remains after the publish")
	}
}

func TestC2266_003_BoundaryPublishLeavesAHalfPairPendingWithAWarn(t *testing.T) {
	r := plane(t)
	if _, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, workspaceOf(r), 5, judgmentTodos); err != nil {
		t.Fatalf("RED: WriteInto: %v", err)
	}
	if err := os.Remove(filepath.Join(recoveryretro.PendingDir(r.Dir), judgmentID+".yaml")); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder

	published, err := recoveryretro.PublishPending(r.Dir, &warn)

	if err != nil || len(published) != 0 {
		t.Fatalf("RED: PublishPending = %v, %v; a half pair publishes nothing", published, err)
	}
	if !strings.Contains(warn.String(), "WARN") {
		t.Errorf("RED: no WARN line for the half pair: %q", warn.String())
	}
	if exists(filepath.Join(r.Dir, recoveryretro.DocsHome, judgmentID+".md")) {
		t.Errorf("RED: a half pair reached %s", recoveryretro.DocsHome)
	}
}

func TestC2266_004_TheCloseoutAtEnforceReachesThePendingWriteFromRunCycle(t *testing.T) {
	goTestPasses(t, "./internal/core", "TestRunCycle_RetrospectivesStageEnforceWritesThePairIntoThePendingDirNotTheWorkspace")
}

func TestC2266_005_TheBoundaryPublishReachesThePendingRetrospectivesFromTheLoopAndTheDossierVerb(t *testing.T) {
	for _, name := range []string{
		"TestPublishPendingDossiers_PublishesTheRetrospectivesBesideTheDossiers",
		"TestPublishPendingDossiers_PublishesThePendingRetrospectivesWithNoDossierPending",
		"TestPublishDossierPairs_PublishesThePendingRetrospectives",
	} {
		goTestPasses(t, "./cmd/evolve", name)
	}
}

func TestC2266_006_ShadowWritesTheWorkspaceOnlyAndLeavesThePendingDirAbsent(t *testing.T) {
	r := plane(t)
	workspace := workspaceOf(r)

	written, err := recoveryretro.WriteShadow(r.Dir, workspace, 5, judgmentTodos)

	if err != nil || len(written) != 2 {
		t.Fatalf("RED: WriteShadow = %v, %v; want one pair", written, err)
	}
	for _, ext := range []string{".md", ".yaml"} {
		if !exists(filepath.Join(workspace, recoveryretro.DirName, judgmentID+ext)) {
			t.Errorf("RED: shadow left no workspace %s", ext)
		}
	}
	if exists(recoveryretro.PendingDir(r.Dir)) {
		t.Errorf("RED: shadow created the pending dir")
	}
	if exists(filepath.Join(r.Dir, recoveryretro.DocsHome)) {
		t.Errorf("RED: shadow wrote into %s", recoveryretro.DocsHome)
	}
}

func TestC2266_007_TheCloseoutAtShadowKeepsTheWorkspaceOnlyOutputFromRunCycle(t *testing.T) {
	goTestPasses(t, "./internal/core", "TestRunCycle_RetrospectivesStageShadowLeavesThePendingDirAbsent")
	goTestPasses(t, "./internal/core", "TestRunCycle_TheCloseoutWritesTheShadowRetrospectiveIntoTheWorkspace")
}

func TestC2266_008_AnExistingPendingDocumentIsNeverOverwritten(t *testing.T) {
	r := plane(t)
	pending := recoveryretro.PendingDir(r.Dir)
	if err := os.MkdirAll(pending, 0o755); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(pending, judgmentID+".md")
	if err := os.WriteFile(edited, []byte("a person wrote this judgment\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := recoveryretro.WriteInto(pending, r.Dir, workspaceOf(r), 5, judgmentTodos); err != nil {
		t.Fatalf("RED: WriteInto: %v", err)
	}

	got, err := os.ReadFile(edited)
	if err != nil || string(got) != "a person wrote this judgment\n" {
		t.Errorf("RED: the existing pending document changed: %q, %v", got, err)
	}
}

func TestC2266_009_AnExistingDocumentInDocsStaysAndThePendingPairIsDropped(t *testing.T) {
	r := plane(t)
	held := filepath.Join(r.Dir, recoveryretro.DocsHome, judgmentID+".md")
	if err := os.MkdirAll(filepath.Dir(held), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(held, []byte("the judgment of a person\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	headBefore := r.Git("rev-parse", "HEAD")
	if _, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, workspaceOf(r), 5, judgmentTodos); err != nil {
		t.Fatalf("RED: WriteInto: %v", err)
	}

	published, err := recoveryretro.PublishPending(r.Dir, &strings.Builder{})

	if err != nil || len(published) != 0 {
		t.Fatalf("RED: PublishPending = %v, %v; a held document is not published again", published, err)
	}
	got, _ := os.ReadFile(held)
	if string(got) != "the judgment of a person\n" {
		t.Errorf("RED: the document in docs changed: %q", got)
	}
	if r.Git("rev-parse", "HEAD") != headBefore {
		t.Errorf("RED: the publish made a commit for a held document")
	}
	if exists(filepath.Join(recoveryretro.PendingDir(r.Dir), judgmentID+".md")) {
		t.Errorf("RED: the pending copy of a held document remains")
	}
}

func TestC2266_010_AWriteFailureIsAnErrorForTheCallerAndAWarnLineForTheCloseout(t *testing.T) {
	r := plane(t)
	if err := os.MkdirAll(filepath.Dir(recoveryretro.PendingDir(r.Dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recoveryretro.PendingDir(r.Dir), []byte("a file where the directory goes"), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := recoveryretro.WriteInto(recoveryretro.PendingDir(r.Dir), r.Dir, workspaceOf(r), 5, judgmentTodos)

	if err == nil || len(written) != 0 {
		t.Fatalf("RED: WriteInto = %v, %v; want an error and no path", written, err)
	}
	goTestPasses(t, "./internal/core", "TestRunCycle_AnEnforceWriteFailureIsAWarnLineAndTheCloseoutStillSucceeds")
}

func TestC2266_011_APublishFailureIsAWarnAndTheBoundaryGoesOn(t *testing.T) {
	goTestPasses(t, "./cmd/evolve", "TestPublishPendingDossiers_ARetrospectivePublishFailureIsAWarnAndTheBoundaryGoesOn")
}

func TestC2266_012_TheDesignDocNoLongerCallsTheEnforceStepsADesign(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	design := filepath.Join(root, designRel)
	for _, stale := range []string{
		"Steps 2 to 5 are a design here, not code",
		"Until steps 2 to 5 are built",
	} {
		if !acsassert.FileNotContains(t, design, stale) {
			t.Errorf("RED: %s still says %q", designRel, stale)
		}
	}
}

func TestC2266_013_TheDesignGuardTableNamesTheEnforceTestsAsBuiltAndEachNameIsARealTest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	design := filepath.Join(root, designRel)
	raw, err := os.ReadFile(design)
	if err != nil {
		t.Fatalf("RED: %v", err)
	}
	required := []string{
		"TestRunCycle_RetrospectivesStageEnforceWritesThePairIntoThePendingDirNotTheWorkspace",
		"TestRunCycle_AnEnforceWriteFailureIsAWarnLineAndTheCloseoutStillSucceeds",
		"TestPublishPending_CommitsThePairAndClearsThePendingCopy",
		"TestPublishPending_ADocumentInDocsIsKeptAndThePendingPairIsDropped",
	}
	for _, name := range required {
		built := false
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, name) && strings.Contains(line, "| built |") {
				built = true
			}
		}
		if !built {
			t.Errorf("RED: no guard row of %s names %s with the state built", designRel, name)
		}
	}
	goDir := filepath.Join(root, "go")
	files, err := acsassert.WalkModule(goDir, func(rel string) bool { return strings.HasSuffix(rel, "_test.go") })
	if err != nil {
		t.Fatalf("RED: walk: %v", err)
	}
	var corpus strings.Builder
	for _, rel := range files {
		b, rerr := os.ReadFile(filepath.Join(goDir, rel))
		if rerr == nil {
			corpus.Write(b)
		}
	}
	for _, m := range regexp.MustCompile("`(Test[A-Za-z0-9_]+)`").FindAllStringSubmatch(string(raw), -1) {
		if !strings.Contains(corpus.String(), "func "+m[1]+"(") {
			t.Errorf("RED: %s names %s, which is no test", designRel, m[1])
		}
	}
}

func TestC2266_014_ThePlanMarksC4BuiltAndTheCloseoutNoLongerWorkspaceOnly(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	plan := filepath.Join(root, planRel)
	for _, stale := range []string{
		"Thus the closeout still writes into the workspace only.",
		"The pending dir and the publish are open",
	} {
		if !acsassert.FileNotContains(t, plan, stale) {
			t.Errorf("RED: %s still says %q", planRel, stale)
		}
	}
}
