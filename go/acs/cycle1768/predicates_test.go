//go:build acs

// Package cycle1768 pins the behavior-preserving shrink of
// phasecoherence.Check, phasecoherence.CheckArtifactNames and
// phasecoherence.CheckProvenance to the 50-line function-size ratchet
// (sizeratchet-shrink-phasecoherence). All three functions still exceed the
// limit at authoring time (80/97/95 lines respectively, per
// go/internal/sizeratchet/offenders.json), so TestC1768_001 is genuinely RED;
// the remaining predicates pin the surrounding invariants the extraction must
// not disturb and are pre-existing GREEN until Builder's diff lands — see
// test-report.md's Coverage Map for the per-criterion disposition.
package cycle1768

import (
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
	// nothing has touched go/internal/phasecoherence yet this cycle.
	baseCommit   = "e404b914d41968fc2ba4124694c46f5dd7826b7b"
	offendersRel = "go/internal/sizeratchet/offenders.json"

	phasecoherenceDir = "internal/phasecoherence"
)

var packageDirs = []string{phasecoherenceDir}

type target struct {
	dir, fn   string
	allowance int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var (
	checkTarget              = target{dir: phasecoherenceDir, fn: "Check", allowance: 80}
	checkArtifactNamesTarget = target{dir: phasecoherenceDir, fn: "CheckArtifactNames", allowance: 97}
	checkProvenanceTarget    = target{dir: phasecoherenceDir, fn: "CheckProvenance", allowance: 95}
	targets                  = []target{checkTarget, checkArtifactNamesTarget, checkProvenanceTarget}
)

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

// TestC1768_001_ThreeTargetFunctionsFitTheRatchetLimit is the primary AC
// signal (AC 3): each of the three offender functions must measure at most
// sizeratchet.MaxLines (50). RED today at 80/97/95 lines.
func TestC1768_001_ThreeTargetFunctionsFitTheRatchetLimit(t *testing.T) {
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

// TestC1768_002_OffendersJSONLeftUnchanged guards the AC 3 clause "offenders.json
// entries for these three keys are left byte-unchanged": an allowance is a
// ceiling, so a shrunk function's entry is slack a future boundary-tighten
// removes, never this lane's job.
func TestC1768_002_OffendersJSONLeftUnchanged(t *testing.T) {
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

// TestC1768_003_ModuleWideRatchetCheckPasses is the module-wide equivalent of
// 001: sizeratchet.Check must report zero violations (the three functions'
// offenders.json entries stay listed but under their allowance once shrunk,
// which Check treats as slack, not a failure).
func TestC1768_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("%v", err)
	}
}

// TestC1768_004_BaselineTestFilesUnchanged guards AC 1's "existing tests in
// the package pass unmodified" — no baseline _test.go file may be edited or
// deleted; a new characterization test (if any) belongs in a new file.
func TestC1768_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		names, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "--no-renames", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(names) != "" {
			t.Errorf("baseline %s test files modified or deleted since %s — the package's existing tests must pass unmodified; put any new characterization test in a new _test.go file:\n%s", dir, baseCommit, names)
		}
	}
}

// TestC1768_005_TargetPackageTestsPass drives the real system: AC 1's
// "behavior unchanged" pin — the package's existing 14 test files (which
// already exercise Check/CheckArtifactNames/CheckProvenance per the scout
// report) must exit 0 both before and after the extraction.
func TestC1768_005_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
	}
}

// TestC1768_007_NoCommentLinesAdded guards AC 2: "no comments added
// (docs/conventions/code-comments.md) — names/signatures carry intent."
// Vacuous (no scoped changes yet) until Builder's diff lands, then live.
func TestC1768_007_NoCommentLinesAdded(t *testing.T) {
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
