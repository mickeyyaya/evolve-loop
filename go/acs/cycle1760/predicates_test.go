//go:build acs

package cycle1760

import (
	"bytes"
	"go/format"
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
	baseCommit   = "b401e73d542cbdfd639bf3f44940ed8e9fae3160"
	offendersRel = "go/internal/sizeratchet/offenders.json"

	releasepreflightDir   = "internal/releasepreflight"
	releaseconsistencyDir = "internal/releaseconsistency"
	releasetargetsDir     = "internal/releasetargets"
	versionbumpDir        = "internal/versionbump"
)

var packageDirs = []string{releasepreflightDir, releaseconsistencyDir, releasetargetsDir, versionbumpDir}

type target struct {
	dir, fn   string
	allowance int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var (
	resolveTarget            = target{dir: releasepreflightDir, fn: "resolve", allowance: 54}
	checkRecentAuditTarget   = target{dir: releasepreflightDir, fn: "checkRecentAudit", allowance: 114}
	releaseconsistencyTarget = target{dir: releaseconsistencyDir, fn: "Run", allowance: 97}
	parseConfigTarget        = target{dir: releasetargetsDir, fn: "ParseConfig", allowance: 65}
	versionbumpTarget        = target{dir: versionbumpDir, fn: "Run", allowance: 52}
	targets                  = []target{resolveTarget, checkRecentAuditTarget, releaseconsistencyTarget, parseConfigTarget, versionbumpTarget}
)

var protectedPrefixes = []string{
	"go/cmd/evolve",
	"go/internal/core",
	"go/internal/bridge",
	"go/internal/acssuite",
	"go/internal/subagent",
	"go/internal/phases/audit",
	"go/internal/phases/ship",
	"go/internal/config",
	"go/internal/adapters/bridge",
	"go/internal/guards",
	"go/internal/binaryguard",
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

func TestC1760_001_FiveTargetFunctionsFitTheRatchetLimit(t *testing.T) {
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
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key(), tg.dir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key()+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1760_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — an allowance is a ceiling and this lane never edits the file; a shrunk function's entry is slack the boundary tighten removes:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key()]; !listed || got != tg.allowance {
			t.Errorf("RED: offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key(), got, listed, tg.allowance)
		}
	}
}

func TestC1760_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1760_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		names, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "--no-renames", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(names) != "" {
			t.Errorf("RED: baseline %s test files modified or deleted since %s — the packages' existing tests must pass unmodified; put any new characterization test in a new _test.go file:\n%s", dir, baseCommit, names)
		}
	}
}

func TestC1760_005_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("RED: go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
	}
}

func TestC1760_006_TargetPackagesVetAndGofmtClean(t *testing.T) {
	goDir := goModuleDir(t)
	for _, dir := range packageDirs {
		if r := execGo(goDir, "vet", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
		entries, err := os.ReadDir(filepath.Join(goDir, dir))
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			path := filepath.Join(goDir, dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			formatted, err := format.Source(src)
			if err != nil {
				t.Errorf("RED: %s/%s does not parse for gofmt: %v", dir, e.Name(), err)
				continue
			}
			if !bytes.Equal(formatted, src) {
				t.Errorf("RED: %s/%s is not gofmt-formatted", dir, e.Name())
			}
		}
	}
}

func TestC1760_007_NoCommentLinesAdded(t *testing.T) {
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
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent, per docs/conventions/code-comments.md):\n%s%s", code, strings.Join(scoped, ", "), out.String(), errOut.String())
	}
}

// acs-predicate: config-check — comment text IS the contract under test
func TestC1760_008_NoCommentsLostFromShrunkFunctions(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	for _, dir := range packageDirs {
		var baseline, current []byte
		scoped := 0
		for _, rel := range changed {
			if filepath.Dir(rel) != "go/"+dir || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				continue
			}
			scoped++
			before, err := git.Show(baseCommit, rel)
			if err != nil {
				t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
			}
			after, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatalf("read %s: %v", rel, err)
			}
			baseline = append(append(baseline, before...), '\n')
			current = append(append(current, after...), '\n')
		}
		if scoped == 0 {
			t.Errorf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", dir, baseCommit)
			continue
		}
		if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
			t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s — move each with its code, never strip it to save a line:\n  %s", len(deleted), dir, baseCommit, strings.Join(deleted, "\n  "))
		}
	}
}

func TestC1760_009_NoProtectedSurfaceTouched(t *testing.T) {
	changed, err := worktreeGit{root: acsassert.RepoRoot(t)}.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var touched []string
	for _, rel := range changed {
		for _, prefix := range protectedPrefixes {
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				touched = append(touched, rel)
			}
		}
	}
	if len(touched) > 0 {
		t.Errorf("RED: the lane touched protected surfaces since %s:\n  %s", baseCommit, strings.Join(touched, "\n  "))
	}
}
