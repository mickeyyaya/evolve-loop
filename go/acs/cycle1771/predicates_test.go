//go:build acs

// Package cycle1771 pins the behavior-preserving shrink of
// tokenusage.ScanConfigRoot, llmcalls.Aggregate, changedpkgs.DirectImporters
// and addedtests.BuildTags to the 50-line function-size ratchet
// (sizeratchet-shrink-usage-scanners). All four functions exceed the limit at
// authoring time (60/53/79/56 lines per
// go/internal/sizeratchet/offenders.json), so TestC1771_001 is genuinely RED;
// the rest are invariant and characterization pins that are pre-existing
// GREEN and must stay GREEN through the extraction — see test-report.md's
// Coverage Map.
package cycle1771

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// baseCommit is this worktree's HEAD at TDD authoring time (2026-09-30):
	// nothing has touched any of the four target packages yet.
	baseCommit   = "6aa43b70d6d0f68302ce12e2ab2fe54180394d79"
	offendersRel = "go/internal/sizeratchet/offenders.json"

	tokenusageDir  = "internal/tokenusage"
	llmcallsDir    = "internal/llmcalls"
	changedpkgsDir = "internal/changedpkgs"
	addedtestsDir  = "internal/addedtests"
)

var packageDirs = []string{tokenusageDir, llmcallsDir, changedpkgsDir, addedtestsDir}

type target struct {
	dir, fn   string
	allowance int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var targets = []target{
	{dir: tokenusageDir, fn: "ScanConfigRoot", allowance: 60},
	{dir: llmcallsDir, fn: "Aggregate", allowance: 53},
	{dir: changedpkgsDir, fn: "DirectImporters", allowance: 79},
	{dir: addedtestsDir, fn: "BuildTags", allowance: 56},
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(offendersRel))
}

func walkModule(t *testing.T) []sizeratchet.FuncSpan {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	return spans
}

// TestC1771_001_TargetFunctionsFitTheRatchetLimit is the primary AC 3
// signal: all four offender functions must measure at most
// sizeratchet.MaxLines under the ratchet's own walker, keeping their names
// and packages.
func TestC1771_001_TargetFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for _, tg := range targets {
		n, found := sizes[tg.key()]
		if !found {
			t.Errorf("%s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key(), tg.dir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key()+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

// TestC1771_002_OffendersJSONLeftUnchanged guards AC 3's "offenders.json is
// left unchanged": the shrunk functions' allowances stay as slack for the
// boundary tighten.
func TestC1771_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("%s edited since %s — this lane never edits the ratchet file:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key()]; !listed || got != tg.allowance {
			t.Errorf("offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key(), got, listed, tg.allowance)
		}
	}
}

// TestC1771_003_ModuleWideRatchetCheckPasses guards AC 3's "the module-wide
// ratchet stays green": an extracted helper over the limit is an unlisted
// violation here.
func TestC1771_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("%v", err)
	}
}

// TestC1771_004_BaselineTestDeclarationsUnchanged guards AC 1's "existing
// tests pass unmodified": every top-level declaration of every baseline
// _test.go file must survive byte-for-byte. Appending a new characterization
// test (to an existing file or a new one) is allowed.
func TestC1771_004_BaselineTestDeclarationsUnchanged(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	for _, dir := range packageDirs {
		files, err := git.TrackedTestFiles(baseCommit, "go/"+dir)
		if err != nil {
			t.Fatalf("list baseline test files: %v", err)
		}
		if len(files) == 0 {
			t.Fatalf("no baseline _test.go files under go/%s at %s", dir, baseCommit)
		}
		for _, rel := range files {
			assertDeclsPreserved(t, git, rel)
		}
	}
}

func assertDeclsPreserved(t *testing.T, git worktreeGit, rel string) {
	t.Helper()
	baseSrc, err := git.Show(baseCommit, rel)
	if err != nil {
		t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
	}
	curSrc, err := os.ReadFile(filepath.Join(git.root, filepath.FromSlash(rel)))
	if err != nil {
		t.Errorf("baseline test file %s is gone: %v", rel, err)
		return
	}
	baseDecls, err := topLevelDecls(rel, baseSrc)
	if err != nil {
		t.Fatalf("parse baseline %s: %v", rel, err)
	}
	curDecls, err := topLevelDecls(rel, curSrc)
	if err != nil {
		t.Errorf("parse current %s: %v", rel, err)
		return
	}
	for decl := range baseDecls {
		if !curDecls[decl] {
			first, _, _ := strings.Cut(decl, "\n")
			t.Errorf("%s: baseline declaration modified or removed: %s", rel, first)
		}
	}
}

// TestC1771_005_TargetPackageTestsPass drives AC 1's "the packages' existing
// tests pass": one named package per invocation.
func TestC1771_005_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
	}
}

// TestC1771_006_NoCommentLinesAdded guards AC 2: no comments added
// (docs/conventions/code-comments.md) — names carry the intent. Vacuous until
// the diff touches one of the four packages, then live.
func TestC1771_006_NoCommentLinesAdded(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	args := []string{"comments", "-base", baseCommit}
	var scoped []string
	for _, dir := range packageDirs {
		for _, rel := range changed {
			if strings.HasPrefix(rel, "go/"+dir+"/") && strings.HasSuffix(rel, ".go") {
				scoped = append(scoped, dir)
				args = append(args, filepath.Join(goModuleDir(t), dir))
				break
			}
		}
	}
	if len(scoped) == 0 {
		return
	}
	var out, errOut strings.Builder
	if code := commentaudit.Main(args, &out, &errOut, git); code != 0 {
		t.Errorf("commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent, per docs/conventions/code-comments.md):\n%s%s", code, strings.Join(scoped, ", "), out.String(), errOut.String())
	}
}
