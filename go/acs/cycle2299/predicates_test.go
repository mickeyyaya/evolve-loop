//go:build acs

package cycle2299

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	noTestsMarker       = "no tests to run"
	releasedPattern     = `^Test(Plan|Apply)DevWorktrees_(DetachedTreeOnAMergedHead|DetachedMerged|DetachedTreeOnAPullRequestHead|DetachedPullRequestHead)`
	releasedTestCount   = 8
	savedFirstPattern   = `^Test(Plan|Apply)DevWorktrees_(DetachedTreeWithAUniqueHead|DetachedTreeWhoseHead|DetachedTreeIsUnique|ThePullRequestHeadsAreRead|DetachedUniqueHead|DetachedTreeIsRefused|DetachedTreeIsJudgedAgain)`
	savedFirstTestCount = 9
	wiringPattern       = `^TestDevWorktreeGCOptions_TheWiredPullRequestHeadLookup`
	wiringTestCount     = 3
	cmdCoveragePattern  = `^Test(DevWorktreeGCOptions|GhOpenPRUpdates|ParseOpenPRUpdates|RunGC|RunDevWorktreeGC)`
)

func moduleDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	return dir
}

func requirePassingTests(t *testing.T, pkg, pattern string, want int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir(t), "test", "-count=1", "-v", "-run", pattern, pkg)
	out := stdout + stderr
	if code != 0 || err != nil || strings.Contains(out, noTestsMarker) {
		t.Fatalf("go test -run %s %s exit=%d err=%v\n%s", pattern, pkg, code, err, acsassert.OutputTail(out, 40))
	}
	if got := strings.Count("\n"+out, "\n--- PASS: "); got != want {
		t.Errorf("go test -run %s %s passed %d test(s), want %d", pattern, pkg, got, want)
	}
}

func TestC2299_001_ADetachedTreeFreeOfUniqueWorkIsReleased(t *testing.T) {
	requirePassingTests(t, "./internal/gc", releasedPattern, releasedTestCount)
}

func TestC2299_002_ADetachedTreeWithAUniqueHeadIsSavedBeforeItIsRemoved(t *testing.T) {
	requirePassingTests(t, "./internal/gc", savedFirstPattern, savedFirstTestCount)
}

func TestC2299_003_TheProductionOptionsWireThePullRequestHeadLookup(t *testing.T) {
	requirePassingTests(t, "./cmd/evolve", wiringPattern, wiringTestCount)
}

func changedFunctionsAreFullyCovered(t *testing.T, pkgRel, runFilter string) {
	t.Helper()
	dir := moduleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	args := []string{"test", "-count=1", "-tags", "integration", "-coverprofile=" + profile}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	test := exec.Command("go", append(args, "./"+pkgRel)...)
	test.Dir = dir
	test.WaitDelay = 10 * time.Second
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./%s failed: %v\n%s", pkgRel, err, acsassert.OutputTail(string(out), 30))
	}
	packages, err := covchanged.ParsePackageSet([]byte(pkgRel))
	if err != nil {
		t.Fatalf("package set: %v", err)
	}
	ws := covchanged.Workspace{ModuleDir: dir, Run: sysexec.DefaultRunner}
	verdict, err := ws.Check(context.Background(), covchanged.Options{Base: "origin/main", Profile: profile, Tags: covchanged.DefaultTags, Packages: packages})
	if err != nil {
		t.Fatalf("covchanged: %v", err)
	}
	if verdict.Changed == 0 {
		t.Fatalf("no function of %s changed against origin/main", pkgRel)
	}
	if !verdict.Clean() {
		t.Errorf("changed functions of %s below 100%% coverage:\n%s", pkgRel, strings.Join(verdict.Problems(), "\n"))
	}
}

func TestC2299_004_ChangedGcFunctionsAreFullyCovered(t *testing.T) {
	changedFunctionsAreFullyCovered(t, "internal/gc", "")
}

func TestC2299_005_ChangedGcCommandFunctionsAreFullyCovered(t *testing.T) {
	changedFunctionsAreFullyCovered(t, "cmd/evolve", cmdCoveragePattern)
}

// acs-predicate: config-check
func TestC2299_006_TheBuildExplanationNamesDecisionsD5AndD7(t *testing.T) {
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-2299-*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("docs/explain/builds holds %d cycle-2299 document(s), want exactly 1", len(matches))
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", matches[0]); code == 0 {
		t.Errorf("%s is gitignored, so ship drops it", matches[0])
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"D5", "D7"} {
		if !strings.Contains(string(body), id) {
			t.Errorf("%s does not name decision %s", matches[0], id)
		}
	}
}
