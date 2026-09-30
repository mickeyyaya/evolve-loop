//go:build acs

package cycle1736

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

const (
	pkgDir     = "internal/skillcheck"
	baseCommit = "b22dea3b"
)

var targetFiles = map[string]string{
	"ManifestProblems":  "manifest.go",
	"Run":               "skillcheck.go",
	"collectSkillFacts": "skillcheck.go",
	"commandDiffs":      "commands.go",
}

func targetKeys() []string {
	keys := make([]string, 0, len(targetFiles))
	for name := range targetFiles {
		keys = append(keys, pkgDir+"."+name)
	}
	sort.Strings(keys)
	return keys
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(goModuleDir(t), "internal", "sizeratchet", "offenders.json")
}

func moduleSizes(t *testing.T) map[string]int {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	sizes := make(map[string]int, len(spans))
	for _, s := range spans {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	return sizes
}

func TestC1736_001_FourSkillcheckFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := moduleSizes(t)
	var over []string
	for _, key := range targetKeys() {
		n, found := sizes[key]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", key, pkgDir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, key+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1736_002_OffendersJSONHasNoSkillcheckEntriesLeft(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	var remaining []string
	for _, key := range targetKeys() {
		if allowance, listed := offenders[key]; listed {
			remaining = append(remaining, key+" (allowance "+strconv.Itoa(allowance)+")")
		}
	}
	if len(remaining) > 0 {
		t.Errorf("RED: offenders.json still lists skillcheck entries that must be deleted once fixed:\n  %s", strings.Join(remaining, "\n  "))
	}
}

func TestC1736_003_ModuleWideRatchetCheckPasses(t *testing.T) {
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

func TestC1736_004_SkillcheckTestFilesUnchangedFromBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--diff-filter=a", baseCommit, "--", "go/"+pkgDir+"/*_test.go")
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: baseline skillcheck test files modified or deleted vs %s (existing tests must stay unmodified):\n%s", baseCommit, diff)
	}
}

func TestC1736_005_SkillcheckPackageTestsPass(t *testing.T) {
	out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+pkgDir)
	if code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", pkgDir, code, out)
	}
}

func TestC1736_006_SkillcheckPackageVetsClean(t *testing.T) {
	out, code := runGo(t, goModuleDir(t), "vet", "./"+pkgDir)
	if code != 0 {
		t.Errorf("RED: go vet ./%s failed (exit=%d):\n%s", pkgDir, code, out)
	}
}

func TestC1736_007_NoCommentLinesAddedToSkillcheck(t *testing.T) {
	var out, errOut strings.Builder
	args := []string{"comments", "-base", baseCommit, filepath.Join(goModuleDir(t), pkgDir)}
	if code := commentaudit.Main(args, &out, &errOut, worktreeGit{root: acsassert.RepoRoot(t)}); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (criterion 3 forbids any):\n%s%s", code, pkgDir, out.String(), errOut.String())
	}
}

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

// acs-predicate: config-check — the doc comment text IS the contract under
func TestC1736_008_TargetFunctionDocsMatchBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	current := funcDocs(t, currentPkgSources(t))
	for name, file := range targetFiles {
		src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+pkgDir+"/"+file)
		if code != 0 {
			t.Fatalf("git show %s:%s failed: %v\n%s", baseCommit, file, err, stderr)
		}
		want, found := funcDocs(t, map[string][]byte{file: []byte(src)})[name]
		if !found {
			t.Fatalf("baseline %s has no func %s", file, name)
		}
		got, found := current[name]
		if !found {
			t.Errorf("RED: func %s not found in %s", name, pkgDir)
			continue
		}
		if got != want {
			t.Errorf("RED: doc comment of %s differs from baseline %s\n--- baseline ---\n%s--- worktree ---\n%s", name, baseCommit, want, got)
		}
	}
}

var skillcheckMutants = []struct{ fn, name, anchor, replacement string }{
	{"ManifestProblems", "unreadable-manifest problem loses its source path", `"MANIFEST: cannot read .claude-plugin/plugin.json: %v"`, `"MANIFEST: cannot read plugin.json: %v"`},
	{"ManifestProblems", "invalid-JSON problem loses its source path", `"MANIFEST: .claude-plugin/plugin.json is not valid JSON: %v"`, `"MANIFEST: plugin.json is not valid JSON: %v"`},
	{"ManifestProblems", "undeclared dir without SKILL.md is reported as an unlisted skill", `os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md"))`, `os.Stat(filepath.Join(skillsDir, e.Name()))`},
	{"ManifestProblems", "unreadable skills dir is swallowed instead of an infra error", `fmt.Errorf("read skills dir: %w", derr)`, `error(nil)`},
	{"ManifestProblems", "missing-agent problem quotes the trimmed path, not the manifest entry", `but the file is missing", entry))`, `but the file is missing", rel))`},
	{"ManifestProblems", "problems are returned unsorted", `sort.Strings(problems)`, `sort.Strings(problems[:0])`},
	{"Run", "catalog warnings lose their WARN: prefix", `fmt.Fprintln(stderr, "WARN:", w)`, `fmt.Fprintln(stderr, w)`},
	{"Run", "frontmatter name mismatches are neither printed nor counted as drift", `range nameErrs {`, `range nameErrs[:0] {`},
	{"Run", "stale phase-facts drift line loses its remedy", "\"DRIFT: %s phase-facts region is stale (run `evolve skills generate`)\\n\"", "\"DRIFT: %s phase-facts region is stale\\n\""},
	{"Run", "orphan reap line is reworded", `"[skills] reaped orphan %s\n"`, `"[skills] removed %s\n"`},
	{"Run", "orphaned command is reported as stale", "\"DRIFT: %s is an orphaned generated command (run `evolve skills generate`)\\n\"", "\"DRIFT: %s is stale or missing (run `evolve skills generate`)\\n\""},
	{"Run", "write mode exits 2 on a name mismatch", `!write && drift`, `drift`},
	{"Run", "check OK line is shortened", `"[skills] check OK — all phase-facts regions in sync, all commands mirrored, Codex manifests projected, all names match dirs"`, `"[skills] check OK"`},
	{"collectSkillFacts", "a user phase with no outputs keeps the fictional <phase>-report.md", `len(spec.Outputs.Files) == 0`, `false`},
	{"collectSkillFacts", "an .evolve/-targeted contract is labelled cycle workspace", `c.WriteTarget == phasecontract.TargetEvolveDir`, `false`},
	{"commandDiffs", "a skill with unparseable frontmatter still gets a stub", `if parseErr != nil {`, `if parseErr != nil && len(raw) == 0 {`},
	{"commandDiffs", "a skill without a frontmatter name stubs to commands/.md", `name = e.Name()`, `name = ""`},
	{"commandDiffs", "a marker-bearing non-.md file is reaped as an orphan", `|| !strings.HasSuffix(c.Name(), ".md")`, ``},
	{"commandDiffs", "diffs are returned unsorted", `return diffs[i].rel < diffs[j].rel`, `return false`},
	{"commandDiffs", "orphan rel drops the commands/ dir", `filepath.Join("commands", c.Name())`, `c.Name()`},
}

func TestC1736_009_SkillcheckTestsKillBehaviorMutants(t *testing.T) {
	goDir := goModuleDir(t)
	if out, code := runGo(t, goDir, "test", "-count=1", "./"+pkgDir); code != 0 {
		t.Fatalf("RED: skillcheck tests fail on the unmutated code (exit=%d):\n%s", code, out)
	}
	var survivors []string
	for _, m := range skillcheckMutants {
		overlay := writeMutantOverlay(t, filepath.Join(goDir, pkgDir), m.anchor, m.replacement)
		out, code := runGo(t, goDir, "test", "-count=1", "-overlay", overlay, "./"+pkgDir)
		if code == 0 || !strings.Contains(out, "--- FAIL") {
			survivors = append(survivors, fmt.Sprintf("%s: %s (exit=%d)", m.fn, m.name, code))
		}
	}
	if len(survivors) > 0 {
		t.Errorf("RED: %d/%d behavior mutants survive the skillcheck tests — add characterization tests that pin them:\n  %s", len(survivors), len(skillcheckMutants), strings.Join(survivors, "\n  "))
	}
}

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

func currentPkgSources(t *testing.T) map[string][]byte {
	t.Helper()
	dir := filepath.Join(goModuleDir(t), pkgDir)
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

func writeMutantOverlay(t *testing.T, absPkgDir, anchor, replacement string) string {
	t.Helper()
	var target string
	var mutated []byte
	hits := 0
	for name, src := range currentPkgSources(t) {
		if n := strings.Count(string(src), anchor); n > 0 {
			hits += n
			target = filepath.Join(absPkgDir, name)
			mutated = []byte(strings.Replace(string(src), anchor, replacement, 1))
		}
	}
	if hits != 1 {
		t.Fatalf("mutation anchor %q occurs %d times in %s non-test sources, want exactly 1 (keep it verbatim when extracting)", anchor, hits, pkgDir)
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

// acs-predicate: config-check — comment text IS the contract under test
func TestC1736_010_NoBaselineCommentDeletedFromSkillcheck(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	var baseline, current []byte
	scoped := 0
	for _, rel := range changed {
		if filepath.Dir(rel) != "go/"+pkgDir || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
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
		t.Fatalf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", pkgDir, baseCommit)
	}
	if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
		t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s (restore each where its code now lives; code-comments.md:43):\n  %s", len(deleted), pkgDir, baseCommit, strings.Join(deleted, "\n  "))
	}
}
