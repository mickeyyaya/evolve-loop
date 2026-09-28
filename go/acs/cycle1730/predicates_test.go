//go:build acs

// Package cycle1730 materialises the acceptance criteria of
// sizeratchet-shrink-opscmd: the eight oversized internal/cli/opscmd
// functions must each shrink to sizeratchet.MaxLines or fewer and lose their
// go/internal/sizeratchet/offenders.json entries, with existing opscmd
// behavior, tests, and build/vet health untouched, no comment lines added, and
// the doctor live/boot reporting pinned by a mutant-killing characterization
// test.
package cycle1730

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// opscmdDir is the offenders.json key prefix ("<dir>." per sizeratchet's
// funcKey), relative to the go/ module root sizeratchet.Walk is called with.
const opscmdDir = "internal/cli/opscmd"

// targetKeys are the eight functions the inbox item names, exactly as they
// key go/internal/sizeratchet/offenders.json.
var targetKeys = []string{
	opscmdDir + ".RunChangelogGen",
	opscmdDir + ".RunConsoleLease",
	opscmdDir + ".RunMarketplacePoll",
	opscmdDir + ".RunReleasePipeline",
	opscmdDir + ".RunReleasePreflight",
	opscmdDir + ".RunRollback",
	opscmdDir + ".runDoctorBoot",
	opscmdDir + ".runDoctorLive",
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(goModuleDir(t), "internal", "sizeratchet", "offenders.json")
}

// walkOpscmdSpans returns Walk's spans for the opscmd package, keyed as
// sizeratchet keys them (dir relative to the go/ module root).
func walkOpscmdSpans(t *testing.T) map[string]int {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	sizes := make(map[string]int, len(spans))
	for _, s := range spans {
		if n := sizes[s.Key]; s.Lines > n {
			sizes[s.Key] = s.Lines
		}
	}
	return sizes
}

// TestC1730_001_EightOpscmdFunctionsFitTheRatchetLimit — each of the eight
// named functions must be sizeratchet.MaxLines lines or fewer (behavioral:
// re-parses the real source via sizeratchet.Walk, the ratchet's own scanner —
// no source-grep). RED today: all eight exceed the limit.
func TestC1730_001_EightOpscmdFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := walkOpscmdSpans(t)
	var over []string
	for _, key := range targetKeys {
		n, found := sizes[key]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — function renamed, removed, or moved out of %s", key, opscmdDir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, key+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		sort.Strings(over)
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

// TestC1730_002_OffendersJSONHasNoOpscmdEntriesLeft — none of the eight keys
// may remain in offenders.json once their functions are within the limit
// (the ratchet only ever shrinks an allowance, so a fixed function's entry
// must be deleted, not lowered). RED today: all eight keys are present.
func TestC1730_002_OffendersJSONHasNoOpscmdEntriesLeft(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	var remaining []string
	for _, key := range targetKeys {
		if allowance, listed := offenders[key]; listed {
			remaining = append(remaining, key+" (allowance "+strconv.Itoa(allowance)+")")
		}
	}
	if len(remaining) > 0 {
		sort.Strings(remaining)
		t.Errorf("RED: offenders.json still lists opscmd entries that must be deleted once fixed:\n  %s", strings.Join(remaining, "\n  "))
	}
}

// TestC1730_003_ModuleWideRatchetCheckPasses — the repo-wide ratchet gate
// (sizeratchet.Check over every function in the module against the loaded
// offenders map) must report zero problems, covering both directions of the
// cheapest gaming fake: deleting the entries without shrinking the code trips
// this because Check flags an unlisted function over the limit. Since
// 2026-09-28 an allowance is a ceiling, so shrinking without deleting is slack
// here and only the _002 key-absence check catches it. Currently green (the allowances match the current
// oversized code) — a guardrail predicate that must stay green throughout the
// build phase, not a RED-today check.
func TestC1730_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(spans, offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

// baseCommit is the cycle's baseline: the tree before any extraction.
const baseCommit = "ad816b70"

// TestC1730_004_OpscmdTestFilesUnchangedFromBaseline — acceptance criterion
// 2, first clause: "the package's existing tests pass unmodified". Every
// *_test.go under go/internal/cli/opscmd that exists at the baseline must be
// byte-identical in the worktree (no modification, deletion, or rename),
// which rules out weakening an existing assertion to paper over a behavior
// change. Added test files are excluded (--diff-filter=a) because the same
// criterion's second clause REQUIRES a new characterization test for any
// function whose behavior no existing test pins (audit round 1, M2).
func TestC1730_004_OpscmdTestFilesUnchangedFromBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := "go/internal/cli/opscmd"
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--diff-filter=a", baseCommit, "--", rel+"/*_test.go")
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: baseline opscmd test files modified/deleted vs %s (existing tests must stay unmodified):\n%s", baseCommit, diff)
	}
}

// TestC1730_005_OpscmdPackageTestsPass — the opscmd package's own test suite
// must pass with count=1 (behavior preserved end to end). Currently green
// (pre-existing GREEN baseline per scout-report.md) — a guardrail predicate
// the build phase must keep green through every extraction step.
func TestC1730_005_OpscmdPackageTestsPass(t *testing.T) {
	goDir := goModuleDir(t)
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "./internal/cli/opscmd/...")
	if code != 0 {
		t.Errorf("RED: go test -count=1 ./internal/cli/opscmd/... failed (exit=%d): %v\n%s%s", code, err, out, stderr)
	}
}

// TestC1730_006_ModuleBuildsAndVetsClean — the negative/edge criterion: the
// whole module must still build and vet clean, catching a break introduced
// anywhere by the extraction (e.g. an unused param left behind). Currently
// green — a guardrail predicate the build phase must keep green.
func TestC1730_006_ModuleBuildsAndVetsClean(t *testing.T) {
	goDir := goModuleDir(t)
	if out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "build", "./..."); code != 0 {
		t.Errorf("RED: go build ./... failed (exit=%d): %v\n%s%s", code, err, out, stderr)
	}
	if out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "vet", "./..."); code != 0 {
		t.Errorf("RED: go vet ./... failed (exit=%d): %v\n%s%s", code, err, out, stderr)
	}
}

// TestC1730_007_NoCommentLinesAddedToOpscmd — acceptance criterion 3: "No
// comments are added (docs/conventions/code-comments.md); names carry the
// intent". Runs the repo's own grader for that convention in-process —
// commentaudit.Main, i.e. `commentaudit comments -base <baseline>
// <opscmd dir>` — which lists every non-directive comment line the diff adds
// (a line moved within its file is not "added") and exits 1 when any exist.
// RED in audit round 1: 54 added doc-comment lines on the new unexported
// helpers.
func TestC1730_007_NoCommentLinesAddedToOpscmd(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var out, errOut strings.Builder
	args := []string{"comments", "-base", baseCommit, filepath.Join(goModuleDir(t), opscmdDir)}
	if code := commentaudit.Main(args, &out, &errOut, worktreeGit{root: root}); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (criterion 3 forbids any):\n%s%s", code, opscmdDir, out.String(), errOut.String())
	}
}

// worktreeGit is cmd/commentaudit's git view, pinned to one work tree with
// `git -C` instead of the process cwd.
type worktreeGit struct{ root string }

func (g worktreeGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g worktreeGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g worktreeGit) Show(base, path string) ([]byte, error) {
	if _, err := g.run("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return g.run("show", base+":"+path)
}

func (g worktreeGit) Root() (string, error) { return g.root, nil }

// targetFiles maps each target function to the file it lives in at the
// baseline commit.
var targetFiles = map[string]string{
	"RunChangelogGen":     "changelog.go",
	"RunConsoleLease":     "console_lease.go",
	"RunMarketplacePoll":  "marketplace_poll.go",
	"RunReleasePipeline":  "release_pipeline.go",
	"RunReleasePreflight": "release_preflight.go",
	"RunRollback":         "rollback.go",
	"runDoctorBoot":       "doctor_boot.go",
	"runDoctorLive":       "doctor_live.go",
}

// TestC1730_008_TargetFunctionDocsMatchBaseline — acceptance criterion 3 and
// audit round 1 finding L1: the extraction must not rewrite, trim, or scatter
// the doc comment of any of the eight target functions (round 1 moved the
// exit-code tables of RunRollback/RunReleasePipeline/RunMarketplacePoll into
// unexported helpers godoc never shows, leaving dangling fragments). Each
// target's go/ast doc text in the worktree must equal its baseline doc text.
//
// acs-predicate: config-check — the doc comment text IS the contract under
// test (comments have no runtime behavior); parsed with go/parser, not grepped.
func TestC1730_008_TargetFunctionDocsMatchBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	current := funcDocs(t, currentOpscmdSources(t))
	for name, file := range targetFiles {
		src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+opscmdDir+"/"+file)
		if code != 0 {
			t.Fatalf("git show %s:%s failed: %v\n%s", baseCommit, file, err, stderr)
		}
		want, found := funcDocs(t, map[string][]byte{file: []byte(src)})[name]
		if !found {
			t.Fatalf("baseline %s has no func %s", file, name)
		}
		got, found := current[name]
		if !found {
			t.Errorf("RED: func %s not found in %s", name, opscmdDir)
			continue
		}
		if got != want {
			t.Errorf("RED: doc comment of %s differs from baseline %s\n--- baseline ---\n%s--- worktree ---\n%s", name, baseCommit, want, got)
		}
	}
}

// doctorMutants are behavior changes to the doctor live/boot result reporting
// that the pre-existing opscmd tests do not detect (audit round 1, M2). Each
// keeps the source compiling and vet-clean, so a characterization test that
// pins the reported messages, JSON field names, and pane-tail length must fail
// on every one.
var doctorMutants = []struct{ name, anchor, replacement string }{
	{"live WALLED message", `"[doctor] LIVE WALLED: %s rc=%d pattern=%s\n"`, `"[doctor] LIVE FAILED: %s rc=%d pattern=%s\n"`},
	{"live healthy JSON field", "`json:\"healthy\"`", "`json:\"ok\"`"},
	{"live WALLED pane tail length", "ScrollbackTail(scrollback, 6)", "ScrollbackTail(scrollback, 12)"},
	{"boot OK message", `"[doctor] BOOT OK: %s REPL booted (sandbox=%v)\n"`, `"[doctor] BOOT OK: %s booted (sandbox=%v)\n"`},
	{"boot sandbox JSON field", "`json:\"sandbox\"`", "`json:\"sandboxed\"`"},
}

// characterizationRun selects the opscmd characterization tests for
// runDoctorLive/runDoctorBoot result reporting.
const characterizationRun = "^TestDoctorCharacterization"

// TestC1730_009_DoctorCharacterizationTestsKillReportMutants — acceptance
// criterion 2, second clause: "a function without a test that pins its
// behavior gets a characterization test first, red on a mutant". The only
// pre-existing test reaching runDoctorLive/runDoctorBoot's OK/WALLED/FAILED
// reporting is an rc-in-{1,10} smoke (audit round 1, M2). The opscmd tests
// matching ^TestDoctorCharacterization must run and pass on the real code,
// and must FAIL (a test failure, not a build failure) when `go test -overlay`
// swaps in each doctorMutants source mutation.
func TestC1730_009_DoctorCharacterizationTestsKillReportMutants(t *testing.T) {
	goDir := goModuleDir(t)
	out, code := runGo(t, goDir, "test", "-count=1", "-v", "-run", characterizationRun, "./"+opscmdDir)
	if code != 0 || !strings.Contains(out, "--- PASS: TestDoctorCharacterization") {
		t.Fatalf("RED: no passing %s tests in %s on the unmutated code (exit=%d):\n%s", characterizationRun, opscmdDir, code, out)
	}
	for _, m := range doctorMutants {
		overlay := writeMutantOverlay(t, filepath.Join(goDir, opscmdDir), m.anchor, m.replacement)
		out, code := runGo(t, goDir, "test", "-count=1", "-v", "-overlay", overlay, "-run", characterizationRun, "./"+opscmdDir)
		if code == 0 || !strings.Contains(out, "--- FAIL: TestDoctorCharacterization") {
			t.Errorf("RED: mutant %q survived the characterization tests (exit=%d):\n%s", m.name, code, out)
		}
	}
}

// runGo runs the go tool in dir and returns its combined output and exit code.
func runGo(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	}
	return string(out), exitErr.ExitCode()
}

// currentOpscmdSources reads every non-test .go file of the opscmd package in
// the worktree, so a target moved to another file is still found.
func currentOpscmdSources(t *testing.T) map[string][]byte {
	t.Helper()
	dir := filepath.Join(goModuleDir(t), opscmdDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	sources := make(map[string][]byte)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sources[name] = src
	}
	return sources
}

// funcDocs parses sources and maps each top-level func name to its doc text.
func funcDocs(t *testing.T, sources map[string][]byte) map[string]string {
	t.Helper()
	docs := make(map[string]string)
	fset := token.NewFileSet()
	for name, src := range sources {
		file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				docs[fn.Name.Name] = fn.Doc.Text()
			}
		}
	}
	return docs
}

// writeMutantOverlay finds the one non-test source file in pkgDir holding
// anchor (exactly once package-wide), writes a mutated copy to a temp dir, and
// returns the path of a `go build -overlay` file that swaps it in.
func writeMutantOverlay(t *testing.T, pkgDir, anchor, replacement string) string {
	t.Helper()
	var target string
	var mutated []byte
	hits := 0
	for name, src := range currentOpscmdSources(t) {
		if n := strings.Count(string(src), anchor); n > 0 {
			hits += n
			target = filepath.Join(pkgDir, name)
			mutated = []byte(strings.Replace(string(src), anchor, replacement, 1))
		}
	}
	if hits != 1 {
		t.Fatalf("mutation anchor %q occurs %d times in %s non-test sources, want exactly 1 (reported behavior changed?)", anchor, hits, opscmdDir)
	}
	tmp := t.TempDir()
	mutantPath := filepath.Join(tmp, filepath.Base(target))
	if err := os.WriteFile(mutantPath, mutated, 0o644); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {target: mutantPath}})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlayPath
}

// TestC1730_010_NoBaselineCommentDeletedFromOpscmd — acceptance criterion 3
// read with docs/conventions/code-comments.md:43 (audit round 2, M1): a pure
// extraction adds no comment AND keeps every baseline comment, so a *why*
// such as console_lease.go's interspersed-parse rationale is never stripped
// with its knowledge left nowhere. Runs the repo's own comment-line grader,
// commentaudit.AddedComments, with the sides swapped: a non-directive comment
// line the baseline versions of the changed opscmd sources hold and their
// current versions lack is deleted. Moves within or across the changed files
// are not deletions. RED in audit round 2: the 5-line interspersed-parse
// comment of RunConsoleLease.
//
// acs-predicate: config-check — comment text IS the contract under test
// (comments have no runtime behavior); graded by commentaudit, not grepped.
func TestC1730_010_NoBaselineCommentDeletedFromOpscmd(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var baseline, current []byte
	scoped := 0
	for _, rel := range changed {
		if filepath.Dir(rel) != "go/"+opscmdDir || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		scoped++
		before, err := git.Show(baseCommit, rel)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
		}
		after, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("read %s: %v", rel, err)
		}
		baseline = append(append(baseline, before...), '\n')
		current = append(append(current, after...), '\n')
	}
	if scoped == 0 {
		t.Fatalf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", opscmdDir, baseCommit)
	}
	if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
		t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s (restore each where its code now lives; code-comments.md:43):\n  %s", len(deleted), opscmdDir, baseCommit, strings.Join(deleted, "\n  "))
	}
}
