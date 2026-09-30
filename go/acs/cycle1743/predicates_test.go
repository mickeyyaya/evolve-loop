//go:build acs

package cycle1743

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
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	auditcalibrationDir = "internal/auditcalibration"
	dossierDir          = "internal/dossier"
	baseCommit          = "baba8085"
	offendersRel        = "go/internal/sizeratchet/offenders.json"
	mutantWorkers       = 6
)

var packageDirs = []string{auditcalibrationDir, dossierDir}

type target struct {
	key, pkg, file string
	allowance      int
}

var targets = []target{
	{key: auditcalibrationDir + ".loadPair", pkg: auditcalibrationDir, file: "auditcalibration.go", allowance: 56},
	{key: auditcalibrationDir + ".render", pkg: auditcalibrationDir, file: "auditcalibration.go", allowance: 67},
	{key: dossierDir + ".Build", pkg: dossierDir, file: "build.go", allowance: 59},
	{key: dossierDir + ".SweepOrphans", pkg: dossierDir, file: "sweep.go", allowance: 53},
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

func TestC1743_001_FourAuditcalibrationDossierFunctionsFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for _, tg := range targets {
		n, found := sizes[tg.key]
		if !found {
			t.Errorf("RED: %s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key, tg.pkg)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("RED: functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

func TestC1743_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — an allowance is a ceiling; this lane leaves the file byte-unchanged:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key]; !listed || got != tg.allowance {
			t.Errorf("RED: offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key, got, listed, tg.allowance)
		}
	}
}

func TestC1743_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1743_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(diff) != "" {
			t.Errorf("RED: baseline %s test files modified or deleted vs %s (existing tests must stay unmodified):\n%s", dir, baseCommit, diff)
		}
	}
}

func TestC1743_005_AuditcalibrationPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+auditcalibrationDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", auditcalibrationDir, code, out)
	}
}

func TestC1743_006_DossierPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+dossierDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", dossierDir, code, out)
	}
}

func TestC1743_007_BothPackagesVetClean(t *testing.T) {
	for _, dir := range packageDirs {
		if out, code := runGo(t, goModuleDir(t), "vet", "./"+dir); code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d):\n%s", dir, code, out)
		}
	}
}

func TestC1743_008_NoCommentLinesAdded(t *testing.T) {
	var out, errOut strings.Builder
	args := []string{"comments", "-base", baseCommit}
	for _, dir := range packageDirs {
		args = append(args, filepath.Join(goModuleDir(t), dir))
	}
	if code := commentaudit.Main(args, &out, &errOut, worktreeGit{root: acsassert.RepoRoot(t)}); code != 0 {
		t.Errorf("RED: commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent):\n%s%s", code, strings.Join(packageDirs, ", "), out.String(), errOut.String())
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
func TestC1743_009_TargetFunctionDocsMatchBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	current := map[string]string{}
	for _, dir := range packageDirs {
		for key, doc := range funcDocs(t, dir, packageSources(t, dir)) {
			current[key] = doc
		}
	}
	for _, tg := range targets {
		src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+tg.pkg+"/"+tg.file)
		if code != 0 {
			t.Fatalf("git show %s:%s failed: %v\n%s", baseCommit, tg.file, err, stderr)
		}
		want, found := funcDocs(t, tg.pkg, map[string][]byte{tg.file: []byte(src)})[tg.key]
		if !found {
			t.Fatalf("baseline %s has no func %s", tg.file, tg.key)
		}
		got, found := current[tg.key]
		if !found {
			t.Errorf("RED: func %s not found in %s", tg.key, tg.pkg)
			continue
		}
		if got != want {
			t.Errorf("RED: doc comment of %s differs from baseline %s\n--- baseline ---\n%s--- worktree ---\n%s", tg.key, baseCommit, want, got)
		}
	}
}

// acs-predicate: config-check — comment text IS the contract under test
func TestC1743_010_NoBaselineCommentDeleted(t *testing.T) {
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
			t.Errorf("RED: no non-test Go file under %s changed since %s — the extraction has not happened", dir, baseCommit)
			continue
		}
		if deleted := commentaudit.AddedComments(current, baseline); len(deleted) > 0 {
			t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s (restore each where its code now lives; code-comments.md:43):\n  %s", len(deleted), dir, baseCommit, strings.Join(deleted, "\n  "))
		}
	}
}

type mutant struct{ fn, name, anchor, replacement string }

var auditcalibrationMutants = []mutant{
	{"loadPair", "a shadow read error other than not-exist is labeled missing", `reason := "malformed-shadow"`, `reason := "missing-shadow"`},
	{"loadPair", "a shadow with an invalid verdict or foreign cycle is paired", `!validShadow(shadow, cycle)`, `!validShadow(shadow, cycle) && false`},
	{"loadPair", "an invalid shadow's exclusion detail changes", `"invalid verdict fields or cycle binding"`, `"shadow rejected"`},
	{"loadPair", "a dossier read error other than not-exist is labeled missing", `reason := "malformed-dossier"`, `reason := "missing-dossier"`},
	{"loadPair", "a missing dossier is labeled malformed", `reason = "missing-dossier"`, `reason = "malformed-dossier"`},
	{"loadPair", "a dossier bound to another cycle is paired", `d.Cycle != cycle`, `d.Cycle != cycle && false`},
	{"loadPair", "a dossier that fails Validate is paired", `d.Validate() != nil`, `d.Validate() != nil && false`},
	{"loadPair", "an invalid dossier's exclusion detail changes", `"invalid dossier or cycle binding"`, `"dossier rejected"`},
	{"loadPair", "a malformed fail-reason is labeled as a malformed shadow", `"malformed-fail-reason"`, `"malformed-shadow"`},
	{"loadPair", "the shipped verdict keeps surrounding whitespace", `strings.TrimSpace(shadow.ShippedVerdict)`, `shadow.ShippedVerdict`},
	{"loadPair", "an absent shipped verdict does not fall back to the dossier's final verdict", `shipped = d.FinalVerdict`, `shipped = dossier.VerdictPass`},
	{"loadPair", "the gate defaults to WARN when nothing overrode", `gate := "PASS"`, `gate := "WARN"`},
	{"loadPair", "the narrative keeps surrounding whitespace", `strings.TrimSpace(shadow.NarrativeVerdict)`, `shadow.NarrativeVerdict`},
	{"loadPair", "the chain verdict keeps surrounding whitespace", `strings.TrimSpace(shadow.ChainVerdict)`, `shadow.ChainVerdict`},
	{"loadPair", "overrides are joined without a space", `strings.Join(shadow.OverrodeBy, ", ")`, `strings.Join(shadow.OverrodeBy, ",")`},
	{"render", "defect classes count each class once", `classCounts[class]++`, `classCounts[class] = 1`},
	{"render", "the matrix lists FAIL narratives first", `[]string{"PASS", "WARN", "FAIL"}`, `[]string{"FAIL", "WARN", "PASS"}`},
	{"render", "the matrix lists FAIL gates first", `[]string{"PASS", "FAIL"}`, `[]string{"FAIL", "PASS"}`},
	{"render", "empty matrix cells are printed", `count > 0`, `count >= 0`},
	{"render", "a defect-class cell is not escaped", `markdownCell(class)`, `class`},
	{"render", "every valid pair is listed as a force override", `p.overriddenBy != ""`, `p.overriddenBy != "" || true`},
	{"render", "a force-override cell is not escaped", `p.shipped, markdownCell(p.overriddenBy))`, `p.shipped, p.overriddenBy)`},
	{"render", "the valid-pairs override cell is not escaped", `markdownCell(p.overriddenBy), markdownCell(`, `p.overriddenBy, markdownCell(`},
	{"render", "the valid-pairs class list is joined without a space", `strings.Join(p.classes, ", ")`, `strings.Join(p.classes, ",")`},
	{"render", "the valid-pairs class cell is not escaped", `markdownCell(strings.Join(p.classes, ", "))`, `strings.Join(p.classes, ", ")`},
	{"render", "the valid-pairs row swaps narrative and chain", `p.cycle, p.narrative, p.chain, p.gate`, `p.cycle, p.chain, p.narrative, p.gate`},
	{"render", "an exclusion detail is not escaped", `markdownCell(ex.detail)`, `ex.detail`},
	{"render", "the report title changes", `"# Auditor Calibration Report"`, `"# Auditor Report"`},
	{"render", "the exclusion rule line changes", `Exclusion rule: missing or malformed pair artifacts`, `Exclusion rule: missing pair artifacts`},
	{"render", "the matrix heading changes", `"\n## Narrative × Deterministic Gate Matrix"`, `"\n## Matrix"`},
	{"render", "the defect-classes heading changes", `"\n## Defect Classes"`, `"\n## Classes"`},
	{"render", "the force-overrides heading changes", `"\n## Force Overrides"`, `"\n## Overrides"`},
	{"render", "the valid-pairs heading changes", `"\n## Valid Pairs"`, `"\n## Pairs"`},
	{"render", "the exclusions heading changes", `"\n## Exclusions"`, `"\n## Excluded"`},
	{"render", "the valid-pairs header drops the overrode-by column", `| Overrode By | Defect Classes |"`, `| Defect Classes |"`},
}

var dossierMutants = []mutant{
	{"Build", "a whitespace-only workspace is accepted", `strings.TrimSpace(opts.WorkspacePath) == ""`, `opts.WorkspacePath == ""`},
	{"Build", "the invalid-cycle error drops the cycle", `"dossier: Build: cycle must be >= 1, got %d", cycle`, `"dossier: Build: cycle must be >= 1, got %d", 0`},
	{"Build", "the run id is dropped", `opts.RunID`, `opts.RunID[:0]`},
	{"Build", "skipped phases are dropped", `opts.SkippedPhases`, `opts.SkippedPhases[:0]`},
	{"Build", "verdicts not adopted are dropped", `opts.VerdictsNotAdopted`, `opts.VerdictsNotAdopted[:0]`},
	{"Build", "a WARN cycle gets the FAIL defect", `verdict == VerdictFail`, `verdict != VerdictPass`},
	{"Build", "the audit-fail defect id changes", `"audit-fail"`, `"audit-failed"`},
	{"Build", "the audit-fail defect severity drops", `Severity: "HIGH"`, `Severity: "MEDIUM"`},
	{"Build", "the defect summary no longer names the audit artifact", `"cycle did not pass audit; see %s + acs-verdict.json", auditArtifactName()`, `"cycle did not pass audit; see %s + acs-verdict.json", "audit"`},
	{"Build", "the defect fix text changes", `"address the audit findings recorded for this cycle"`, `"fix it"`},
	{"Build", "the carryover id changes", `"address-audit-findings"`, `"audit-findings"`},
	{"Build", "the carryover action does not name the cycle", `"resolve the audit findings that failed cycle %d", cycle`, `"resolve the audit findings that failed cycle %d", 0`},
	{"Build", "the carryover priority drops", `Priority: "high"`, `Priority: "low"`},
	{"SweepOrphans", "an enumerate failure is swallowed", `fmt.Errorf("dossier: sweep: enumerate tree: %w", err)`, `error(nil)`},
	{"SweepOrphans", "the enumerate error loses its context", `"dossier: sweep: enumerate tree: %w"`, `"%w"`},
	{"SweepOrphans", "an unparsable cycle number is not skipped", `convErr != nil`, `convErr != nil && false`},
	{"SweepOrphans", "cycles are swept in descending order", `sort.Ints(cycles)`, `sort.Sort(sort.Reverse(sort.IntSlice(cycles)))`},
	{"SweepOrphans", "the error log line loses its prefix", `"[dossier-sweep] ERROR cycle %d (%s): recommit failed: %v\n"`, `"ERROR cycle %d (%s): recommit failed: %v\n"`},
	{"SweepOrphans", "the error log names the file, not its directory", `path.Dir(pp.json)`, `path.Clean(pp.json)`},
}

func TestC1743_011_AuditcalibrationTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, auditcalibrationDir, auditcalibrationMutants)
}

func TestC1743_012_DossierTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, dossierDir, dossierMutants)
}

func assertMutantsKilled(t *testing.T, dir string, mutants []mutant) {
	t.Helper()
	goDir := goModuleDir(t)
	overlays := make([]string, len(mutants)+1)
	for i, m := range mutants {
		overlays[i+1] = writeMutantOverlay(t, dir, m)
	}
	outcomes := make([]goRun, len(overlays))
	sem := make(chan struct{}, mutantWorkers)
	var wg sync.WaitGroup
	for i, overlay := range overlays {
		wg.Add(1)
		go func(i int, overlay string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			args := []string{"test", "-count=1", "-failfast"}
			if overlay != "" {
				args = append(args, "-overlay", overlay)
			}
			outcomes[i] = execGo(goDir, append(args, "./"+dir)...)
		}(i, overlay)
	}
	wg.Wait()
	if base := outcomes[0]; base.err != nil || base.code != 0 {
		t.Fatalf("RED: %s tests fail on the unmutated code (exit=%d, err=%v):\n%s", dir, base.code, base.err, base.out)
	}
	var survivors []string
	for i, m := range mutants {
		o := outcomes[i+1]
		switch {
		case o.err != nil:
			t.Fatalf("mutant %s: %s: %v", m.fn, m.name, o.err)
		case strings.Contains(o.out, "[build failed]") || strings.Contains(o.out, "[setup failed]"):
			survivors = append(survivors, fmt.Sprintf("%s: %s (mutant does not compile — keep the anchor %q verbatim in an expression that still compiles)", m.fn, m.name, m.anchor))
		case o.code == 0 || !strings.Contains(o.out, "--- FAIL"):
			survivors = append(survivors, fmt.Sprintf("%s: %s (exit=%d)", m.fn, m.name, o.code))
		}
	}
	if len(survivors) > 0 {
		t.Errorf("RED: %d/%d behavior mutants survive the %s tests — add characterization tests that pin them:\n  %s", len(survivors), len(mutants), dir, strings.Join(survivors, "\n  "))
	}
}

type goRun struct {
	out  string
	code int
	err  error
}

func execGo(dir string, args ...string) goRun {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return goRun{out: string(out)}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return goRun{out: string(out), err: fmt.Errorf("go %s: %w", strings.Join(args, " "), err)}
	}
	return goRun{out: string(out), code: exitErr.ExitCode()}
}

func runGo(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	r := execGo(dir, args...)
	if r.err != nil {
		t.Fatal(r.err)
	}
	return r.out, r.code
}

func packageSources(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	abs := filepath.Join(goModuleDir(t), dir)
	entries, err := os.ReadDir(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	sources := make(map[string][]byte)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sources[name] = src
	}
	return sources
}

func funcDocs(t *testing.T, dir string, sources map[string][]byte) map[string]string {
	t.Helper()
	docs := make(map[string]string)
	fset := token.NewFileSet()
	for name, src := range sources {
		file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				docs[dir+"."+receiverPrefix(fn)+fn.Name.Name] = fn.Doc.Text()
			}
		}
	}
	return docs
}

func receiverPrefix(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name + "."
	}
	return fmt.Sprintf("%T.", expr)
}

func writeMutantOverlay(t *testing.T, dir string, m mutant) string {
	t.Helper()
	pkgDir := filepath.Join(goModuleDir(t), dir)
	replace := map[string]string{}
	tmp := t.TempDir()
	for name, src := range packageSources(t, dir) {
		if !strings.Contains(string(src), m.anchor) {
			continue
		}
		mutantPath := filepath.Join(tmp, name)
		if err := os.WriteFile(mutantPath, []byte(strings.ReplaceAll(string(src), m.anchor, m.replacement)), 0o644); err != nil {
			t.Fatalf("write mutant: %v", err)
		}
		replace[filepath.Join(pkgDir, name)] = mutantPath
	}
	if len(replace) == 0 {
		t.Fatalf("mutation anchor %q (%s: %s) no longer occurs in the %s non-test sources — keep it verbatim when extracting", m.anchor, m.fn, m.name, dir)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlayPath
}
