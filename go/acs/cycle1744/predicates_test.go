//go:build acs

package cycle1744

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
	aggregatorDir = "internal/aggregator"
	apicoverDir   = "internal/apicover"
	baseCommit    = "baba8085"
	offendersRel  = "go/internal/sizeratchet/offenders.json"
	mutantWorkers = 6
)

var packageDirs = []string{aggregatorDir, apicoverDir}

type target struct {
	key, pkg, file string
	allowance      int
}

var targets = []target{
	{key: aggregatorDir + ".Aggregate", pkg: aggregatorDir, file: "aggregator.go", allowance: 73},
	{key: aggregatorDir + ".writeCrossCLIVote", pkg: aggregatorDir, file: "aggregator.go", allowance: 64},
	{key: apicoverDir + ".Run", pkg: apicoverDir, file: "run.go", allowance: 68},
	{key: apicoverDir + ".exportedSymbols", pkg: apicoverDir, file: "enumerate.go", allowance: 67},
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

func TestC1744_001_FourAggregatorApicoverFunctionsFitTheRatchetLimit(t *testing.T) {
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

func TestC1744_002_OffendersJSONLeftUnchanged(t *testing.T) {
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

func TestC1744_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1744_004_BaselineTestFilesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range packageDirs {
		diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--diff-filter=a", baseCommit, "--", "go/"+dir+"/*_test.go", "go/"+dir+"/testdata")
		if code != 0 {
			t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
		}
		if strings.TrimSpace(diff) != "" {
			t.Errorf("RED: baseline %s test files or fixtures modified or deleted vs %s (existing tests must stay unmodified):\n%s", dir, baseCommit, diff)
		}
	}
}

func TestC1744_005_AggregatorPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+aggregatorDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", aggregatorDir, code, out)
	}
}

func TestC1744_006_ApicoverPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+apicoverDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", apicoverDir, code, out)
	}
}

func TestC1744_007_BothPackagesVetAndGofmtClean(t *testing.T) {
	goDir := goModuleDir(t)
	for _, dir := range packageDirs {
		if out, code := runGo(t, goDir, "vet", "./"+dir); code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d):\n%s", dir, code, out)
		}
		out, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", filepath.Join(goDir, dir))
		if code != 0 {
			t.Fatalf("gofmt -l %s failed (exit=%d): %v\n%s", dir, code, err, stderr)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("RED: gofmt -l lists unformatted files under %s (a line count shrunk by hand-joining lines is not an extraction):\n%s", dir, out)
		}
	}
}

func TestC1744_008_NoCommentLinesAdded(t *testing.T) {
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
func TestC1744_009_TargetFunctionDocsMatchBaseline(t *testing.T) {
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
func TestC1744_010_NoBaselineCommentDeleted(t *testing.T) {
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

var aggregatorMutants = []mutant{
	{"Aggregate", "an empty output path passes the usage check", `in.Phase == "" || in.Output == ""`, `in.Phase == ""`},
	{"Aggregate", "the missing-worker error drops its wording", `"error: worker artifact not found: %s"`, `"error: worker artifact absent: %s"`},
	{"Aggregate", "the empty-worker error drops its wording", `"error: worker artifact is empty: %s"`, `"error: worker artifact has no bytes: %s"`},
	{"Aggregate", "an unknown phase passes the mode check", `mode == ModeUnknown`, `mode == ModeUnknown && false`},
	{"Aggregate", "the unknown-phase error hides the phase", `"error: unknown phase '%s'"`, `"error: unknown phase '%.0s'"`},
	{"Aggregate", "a nil Now seam stamps the zero time", `in.Now = time.Now`, `in.Now = func() time.Time { return time.Time{} }`},
	{"Aggregate", "the aggregation time is not converted to UTC", `in.Now().UTC()`, `in.Now()`},
	{"Aggregate", "the aggregation time drops its seconds", `"2006-01-02T15:04:05Z"`, `"2006-01-02T15:04Z"`},
	{"Aggregate", "a missing output directory is not created", `os.MkdirAll(filepath.Dir(in.Output), 0o755)`, `error(nil)`},
	{"Aggregate", "the mkdir error drops its wording", `"error: mkdir %s: %v"`, `"error: create %s: %v"`},
	{"Aggregate", "a failed rename leaves the temp file behind", `os.Remove(tmp)`, `os.Remove(tmp + ".kept")`},
	{"Aggregate", "the merge-read error drops its wording", `"error: read worker: %v"`, `"error: merge: %v"`},
	{"Aggregate", "the rename error drops its wording", `"error: write %s: %v"`, `"error: rename %s: %v"`},
	{"Aggregate", "stderr lines lose the [aggregator] prefix", `"[aggregator] "`, `""`},
	{"Aggregate", "the usage line drops its synopsis", `"usage: aggregator <phase> <output> <worker-artifact>..."`, `"usage: aggregator"`},
	{"Aggregate", "the no-workers error drops its wording", `"error: at least one worker artifact required"`, `"error: no workers"`},
	{"writeCrossCLIVote", "a verdict-less worker is labeled blank", `verdictLabel = "MISSING"`, `verdictLabel = ""`},
	{"writeCrossCLIVote", "per-CLI entries use a colon", `"%s=%s", name, verdictLabel`, `"%s:%s", name, verdictLabel`},
	{"writeCrossCLIVote", "the quorum is a strict majority", `(total + 1) / 2`, `total/2 + 1`},
	{"writeCrossCLIVote", "an exact quorum of PASS falls to WARN", `passCount >= quorum`, `passCount > quorum`},
	{"writeCrossCLIVote", "the veto flag stays 0 under a FAIL", `failVetoFlag = "1"`, `failVetoFlag = "0"`},
	{"writeCrossCLIVote", "the veto is reported inactive under a FAIL", `failVetoActive = "yes"`, `failVetoActive = "no"`},
	{"writeCrossCLIVote", "each voter counts twice", `total++`, `total += 2`},
	{"writeCrossCLIVote", "the below-quorum reason drops the fluent-default note", `; ships per fluent default unless workflow.strict_audit"`, `"`},
	{"writeCrossCLIVote", "the quorum reason calls it a majority", `CLIs returned PASS (quorum=%d)`, `CLIs returned PASS (majority=%d)`},
	{"writeCrossCLIVote", "the consensus summary drops the aggregation time", `at %s. CLIs voting`, `at %.0s. CLIs voting`},
	{"writeCrossCLIVote", "per-CLI sections are level-2 headings", `"### Worker: %s\n\n"`, `"## Worker: %s\n\n"`},
	{"writeCrossCLIVote", "per-CLI verdicts are joined without a space", `strings.Join(perCLI, ", ")`, `strings.Join(perCLI, ",")`},
	{"writeCrossCLIVote", "the decision block drops the reason", `"**Reason**: %s\n\n"`, `"**Reason**: %.0s\n\n"`},
	{"writeCrossCLIVote", "the decision block drops the verdict", `"**Verdict**: %s\n\n"`, `"**Verdict**: %.0s\n\n"`},
	{"writeCrossCLIVote", "the report title changes", `"# Aggregated Cross-CLI Consensus Audit\n\n"`, `"# Aggregated Consensus Audit\n\n"`},
	{"writeCrossCLIVote", "the decision heading changes", `"## Consensus Decision\n\n"`, `"## Decision\n\n"`},
	{"writeCrossCLIVote", "the per-CLI reports heading changes", `"## Per-CLI Audit Reports\n\n"`, `"## Audit Reports\n\n"`},
	{"writeCrossCLIVote", "the protocol line misstates the veto", `Any FAIL forces consensus FAIL`, `Any FAIL forces consensus WARN`},
}

var apicoverMutants = []mutant{
	{"Run", "methods get no cover percentage", `s.Kind != KindFunc && s.Kind != KindMethod`, `s.Kind != KindFunc`},
	{"Run", "the pre-existing-debt header loses its guidance", `PRE-EXISTING DEBT (files untouched by this change — WARN only, pay down separately):`, `PRE-EXISTING DEBT:`},
	{"Run", "uncovered symbols do not count toward enforcement", `len(rep.Uncovered) + len(rep.FalseGreens)`, `len(rep.FalseGreens)`},
	{"Run", "RequireDoc is ignored", `cfg.RequireDoc`, `false`},
	{"exportedSymbols", "a malformed directive's symbol is not bucketed ignored", `ignored = true`, `ignored = false`},
	{"exportedSymbols", "the last malformed directive's error wins", `firstErr == nil`, `true`},
	{"exportedSymbols", "the directive error drops its file:line prefix", `"%s:%d %s: %w"`, `"%.0s%.0d%s: %w"`},
	{"exportedSymbols", "symbol lines are columns", `fset.Position(pos).Line`, `fset.Position(pos).Column`},
	{"exportedSymbols", "HasDoc is inverted", `doc != nil`, `doc == nil`},
	{"exportedSymbols", "methods on unexported types are enumerated", `recv == "" || !ast.IsExported(recv)`, `recv == ""`},
	{"exportedSymbols", "a method's doc comment is dropped", `KindMethod, d.Pos(), d.Doc`, `KindMethod, d.Pos(), nil`},
	{"exportedSymbols", "unexported types are enumerated", `!s.Name.IsExported()`, `false`},
	{"exportedSymbols", "a grouped decl's doc is ignored", `firstDoc(s.Doc, d.Doc)`, `firstDoc(s.Doc)`},
	{"exportedSymbols", "a spec's own doc loses to its group's", `firstDoc(s.Doc, d.Doc)`, `firstDoc(d.Doc, s.Doc)`},
	{"exportedSymbols", "a grouped type takes the decl's position", `KindType, s.Pos()`, `KindType, d.Pos()`},
	{"exportedSymbols", "the package name is the file name", `pkg := f.Name.Name`, `pkg := file`},
}

func TestC1744_011_AggregatorTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, aggregatorDir, aggregatorMutants)
}

func TestC1744_012_ApicoverTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, apicoverDir, apicoverMutants)
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
