//go:build acs

// Package cycle1765 pins the behavior-preserving shrink of
// posteditvalidate.Run, evalqualitycheck.CheckDiversity and verifyeval.Verify
// to the 50-line function-size ratchet (sizeratchet-shrink-eval-validators).
//
// The shrink itself landed on this branch before this cycle started (a
// salvage snapshot carried the extraction forward from an earlier attempt at
// this same fleet-scoped item, then this branch merged origin/main on top).
// These predicates therefore verify the AC set against baseCommit, the point
// this branch diverged from origin/main — not against an in-cycle "before"
// state — so most assertions below are pre-existing GREEN rather than RED at
// authoring time; see test-report.md's Coverage Map for the per-criterion
// disposition.
package cycle1765

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
	// baseCommit is the merge-base of this branch with origin/main at TDD
	// authoring time (2026-09-30): the boundary between "main's state" and
	// "this lane's diff", not an arbitrary point in this branch's own history.
	baseCommit   = "156ee9aba02a05829d81d7f783f456bfd86c9420"
	offendersRel = "go/internal/sizeratchet/offenders.json"
	evalRel      = ".evolve/evals/sizeratchet-shrink-eval-validators.md"
	helpersRel   = "go/acs/cycle1765/helpers_test.go"
	thisCycle    = 1765

	posteditvalidateDir = "internal/posteditvalidate"
	evalqualitycheckDir = "internal/evalqualitycheck"
	verifyevalDir       = "internal/verifyeval"
)

var packageDirs = []string{posteditvalidateDir, evalqualitycheckDir, verifyevalDir}

var acPredicates = []string{
	"TestC1765_001_ThreeTargetFunctionsFitTheRatchetLimit",
	"TestC1765_002_OffendersJSONLeftUnchanged",
	"TestC1765_003_ModuleWideRatchetCheckPasses",
	"TestC1765_004_BaselineTestFilesUnchanged",
	"TestC1765_005_TargetPackageTestsPass",
	"TestC1765_006_TargetPackagesVetAndGofmtClean",
	"TestC1765_007_NoCommentLinesAdded",
	"TestC1765_008_NoCommentsLostFromShrunkFunctions",
	"TestC1765_009_OnlyTargetPackagesTouched",
}

// exemptPrefixes are pipeline-required artifact paths this lane's diff
// against baseCommit legitimately touches beyond the three target packages:
// this cycle's own predicate package, the permanent eval file the AC
// authority lives in, build/closeout explanations, the prior attempt's
// archived predicate packages (A2 continuation-retirement), and knowledge
// base cycle summaries. A deny-list, not an allow-list of the task's own
// packages: cycle-1761's allow-list style scope fence went red on its own
// pipeline-required explanation/eval writes (flaky-predicate-shape lesson).
var exemptPrefixes = []string{
	"go/acs/cycle1765/",
	"go/acs/cycle1761/",
	"go/acs/cycle1764/",
	".evolve/evals/",
	"docs/explain/builds/",
	"docs/private/research/",
	"knowledge-base/cycles/",
}

type target struct {
	dir, fn   string
	allowance int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var (
	runTarget            = target{dir: posteditvalidateDir, fn: "Run", allowance: 108}
	checkDiversityTarget = target{dir: evalqualitycheckDir, fn: "CheckDiversity", allowance: 59}
	verifyTarget         = target{dir: verifyevalDir, fn: "Verify", allowance: 53}
	targets              = []target{runTarget, checkDiversityTarget, verifyTarget}
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

// TestC1765_001_ThreeTargetFunctionsFitTheRatchetLimit is the primary AC
// signal: each of the three offender functions must measure at most
// sizeratchet.MaxLines (50).
func TestC1765_001_ThreeTargetFunctionsFitTheRatchetLimit(t *testing.T) {
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

// TestC1765_002_OffendersJSONLeftUnchanged guards the AC "offenders.json
// entries for these three keys are left byte-unchanged": an allowance is a
// ceiling, so a shrunk function's entry is slack a future boundary-tighten
// removes, never this lane's job.
func TestC1765_002_OffendersJSONLeftUnchanged(t *testing.T) {
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

// TestC1765_003_ModuleWideRatchetCheckPasses is the module-wide equivalent of
// 001: sizeratchet.Check must report zero violations (the three functions'
// offenders.json entries stay listed but under their allowance, which Check
// treats as slack, not a failure).
func TestC1765_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("%v", err)
	}
}

// TestC1765_004_BaselineTestFilesUnchanged guards "behavior unchanged:
// existing tests in all three packages pass unmodified" — no baseline
// _test.go file may be edited or deleted; a new characterization test (if
// any) belongs in a new file.
func TestC1765_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		names, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-status", "--no-renames", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(names) != "" {
			t.Errorf("baseline %s test files modified or deleted since %s — the packages' existing tests must pass unmodified; put any new characterization test in a new _test.go file:\n%s", dir, baseCommit, names)
		}
	}
}

// TestC1765_005_TargetPackageTestsPass drives the real system: the existing
// test suites in all three packages (which already exercise every branch of
// Run/CheckDiversity/Verify per the scout report) must exit 0.
func TestC1765_005_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
	}
}

func TestC1765_006_TargetPackagesVetAndGofmtClean(t *testing.T) {
	goDir := goModuleDir(t)
	for _, dir := range packageDirs {
		if r := execGo(goDir, "vet", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("go vet ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
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
				t.Errorf("%s/%s does not parse for gofmt: %v", dir, e.Name(), err)
				continue
			}
			if !bytes.Equal(formatted, src) {
				t.Errorf("%s/%s is not gofmt-formatted", dir, e.Name())
			}
		}
	}
}

// TestC1765_007_NoCommentLinesAdded guards "no comments added
// (docs/conventions/code-comments.md) — names/signatures carry intent."
func TestC1765_007_NoCommentLinesAdded(t *testing.T) {
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

// acs-predicate: config-check — comment text IS the contract under test
// (comments have no runtime behavior); graded by commentaudit, not grepped.
func TestC1765_008_NoCommentsLostFromShrunkFunctions(t *testing.T) {
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
			t.Errorf("no non-test Go file under %s changed since %s — the extraction has not happened", dir, baseCommit)
			continue
		}
		if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
			t.Errorf("%d baseline comment line(s) deleted from %s since %s — move each with its code, never strip it to save a line:\n  %s", len(deleted), dir, baseCommit, strings.Join(deleted, "\n  "))
		}
	}
}

// TestC1765_009_OnlyTargetPackagesTouched guards the scout's file scope: this
// hygiene lane touches only the three named packages (source or their own
// tests), plus pipeline-required artifacts the harness itself writes every
// cycle. A deny-list of what must NOT be touched, not an allow-list of the
// task's own packages (see exemptPrefixes doc comment).
func TestC1765_009_OnlyTargetPackagesTouched(t *testing.T) {
	changed, err := (worktreeGit{root: acsassert.RepoRoot(t)}).ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var outside []string
	for _, rel := range changed {
		if isExempt(rel) {
			continue
		}
		inScope := false
		for _, dir := range packageDirs {
			if strings.HasPrefix(rel, "go/"+dir+"/") {
				inScope = true
				break
			}
		}
		if !inScope {
			outside = append(outside, rel)
		}
	}
	if len(outside) > 0 {
		t.Errorf("the lane touched files outside the three target packages since %s:\n  %s", baseCommit, strings.Join(outside, "\n  "))
	}
}

func isExempt(rel string) bool {
	for _, p := range exemptPrefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

func TestC1765_010_EvalEvidenceRunsTheLivePredicates(t *testing.T) {
	if os.Getenv(evidenceChildEnv) != "" {
		t.Skip("nested inside an eval evidence run; the parent run grades the evidence")
	}
	root := acsassert.RepoRoot(t)
	caps, err := loadScoreCaps(filepath.Join(root, evalRel))
	if err != nil {
		t.Fatalf("load %s: %v", evalRel, err)
	}
	if len(caps) < len(acPredicates) {
		t.Errorf("%s declares %d score_cap entries, want one per acceptance criterion (%d)", evalRel, len(caps), len(acPredicates))
	}
	var evidence []string
	for i, c := range caps {
		if strings.TrimSpace(c.Criterion) == "" {
			t.Errorf("score_cap[%d] has an empty criterion", i)
		}
		if c.MaxIfMissing < 1 || c.MaxIfMissing > 10 {
			t.Errorf("score_cap[%d] max_if_missing = %d, want 1..10", i, c.MaxIfMissing)
		}
		if strings.TrimSpace(c.Evidence) == "" {
			t.Errorf("score_cap[%d] declares no evidence command", i)
			continue
		}
		evidence = append(evidence, c.Evidence)
		r := runEvidence(root, c.Evidence)
		if r.err != nil || r.code != 0 {
			t.Errorf("score_cap[%d] evidence %q exit=%d err=%v — an eval whose evidence cannot run caps nothing:\n%s", i, c.Evidence, r.code, r.err, r.out)
			continue
		}
		if strings.Contains(r.out, "no tests to run") {
			t.Errorf("score_cap[%d] evidence %q exits 0 but selects no test, so it proves nothing:\n%s", i, c.Evidence, r.out)
		}
	}
	joined := strings.Join(evidence, "\n")
	for _, name := range acPredicates {
		if !strings.Contains(joined, name) {
			t.Errorf("no score_cap evidence in %s runs %s", evalRel, name)
		}
	}
}

// acs-predicate: config-check — the eval's text is the permanent acceptance
// record; its cited base is graded against the base the predicates diff from.
func TestC1765_011_EvalCitesTheLiveBaseAndPackage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, evalRel))
	if err != nil {
		t.Fatalf("read %s: %v", evalRel, err)
	}
	for _, stale := range []string{"e8452207", "TestC1761_", "acs/cycle1761"} {
		if n := strings.Count(string(raw), stale); n > 0 {
			t.Errorf("%s still cites %q %d time(s) — a superseded base or an archived predicate package no command can reach", evalRel, stale, n)
		}
	}
	caps, err := loadScoreCaps(filepath.Join(root, evalRel))
	if err != nil {
		t.Fatalf("load %s: %v", evalRel, err)
	}
	pinned := false
	for _, c := range caps {
		if !strings.Contains(c.Criterion, "offenders.json") {
			continue
		}
		pinned = true
		if !strings.Contains(c.Criterion, baseCommit[:8]) {
			t.Errorf("offenders.json criterion %q does not cite %s, the base TestC1765_002 diffs against", c.Criterion, baseCommit[:8])
		}
	}
	if !pinned {
		t.Errorf("%s has no score_cap criterion pinning offenders.json", evalRel)
	}
}

func TestC1765_012_ExplanationDocVerificationCommandsResolve(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path, doc, err := readExplanationDoc(root, thisCycle)
	if err != nil {
		t.Fatal(err)
	}
	section := markdownSection(doc, "## Verification")
	if strings.TrimSpace(section) == "" {
		t.Fatalf("%s has no ## Verification section", path)
	}
	resolved := 0
	for _, span := range backtickSpans(section) {
		tags, patterns, isGo := goCommandPackages(span)
		if !isGo {
			continue
		}
		resolved++
		args := append(append([]string{"list"}, tags...), patterns...)
		r := execGo(goModuleDir(t), args...)
		if r.err != nil || r.code != 0 || strings.Contains(r.out, "matched no packages") {
			t.Errorf("%s Verification cites `%s`, but its packages do not resolve from the go/ module root (go %s → exit=%d err=%v):\n%s", filepath.Base(path), span, strings.Join(args, " "), r.code, r.err, r.out)
		}
	}
	if resolved == 0 {
		t.Errorf("%s Verification cites no go test / go vet command to reproduce", filepath.Base(path))
	}
}

// acs-predicate: config-check — the explanation document is prose; each claim
// is graded against the code it describes, not merely grepped.
func TestC1765_013_ExplanationDocClaimsMatchTheCode(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path, doc, err := readExplanationDoc(root, thisCycle)
	if err != nil {
		t.Fatal(err)
	}
	imports, err := fileImports(filepath.Join(root, filepath.FromSlash(helpersRel)))
	if err != nil {
		t.Fatalf("parse %s: %v", helpersRel, err)
	}
	claims := []struct{ word, importPath string }{
		{"gofmt", "go/format"},
		{"comment", "/commentaudit"},
	}
	described := false
	for _, line := range strings.Split(doc, "\n") {
		if !strings.Contains(line, helpersRel) {
			continue
		}
		described = true
		lower := strings.ToLower(line)
		for _, c := range claims {
			if strings.Contains(lower, c.word) && !importsPath(imports, c.importPath) {
				t.Errorf("%s credits %s with %q work, but that file never imports %s:\n  %s", filepath.Base(path), helpersRel, c.word, c.importPath, line)
			}
		}
	}
	if !described {
		t.Errorf("%s does not describe %s, a file in the Build diff", filepath.Base(path), helpersRel)
	}
	if strings.Contains(doc, "98/98") {
		t.Errorf("%s claims a 98/98 module-wide package count that no run measured (go test -count=1 ./... from go/ yields 247 ok, 0 FAIL, 5 no test files)", filepath.Base(path))
	}
}
