//go:build acs

package cycle1741

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
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cyclecostDir   = "internal/cyclecost"
	cyclehealthDir = "internal/cyclehealth"
	baseCommit     = "d29ffcb1"
	offendersRel   = "go/internal/sizeratchet/offenders.json"
	mutantWorkers  = 6
)

var packageDirs = []string{cyclecostDir, cyclehealthDir}

type target struct {
	key, pkg, file string
	allowance      int
}

var targets = []target{
	{key: cyclecostDir + ".SummarizeCycle", pkg: cyclecostDir, file: "cyclecost.go", allowance: 57},
	{key: cyclecostDir + ".parseEventsLog", pkg: cyclecostDir, file: "cyclecost.go", allowance: 54},
	{key: cyclehealthDir + ".Check", pkg: cyclehealthDir, file: "cyclehealth.go", allowance: 51},
	{key: cyclehealthDir + ".ClassifyOutcome", pkg: cyclehealthDir, file: "outcome.go", allowance: 70},
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

func TestC1741_001_FourCyclecostCyclehealthFunctionsFitTheRatchetLimit(t *testing.T) {
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

func TestC1741_002_OffendersJSONLeftUnchanged(t *testing.T) {
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

func TestC1741_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1741_004_BaselineTestFilesUnchanged(t *testing.T) {
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

func TestC1741_005_CyclecostPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+cyclecostDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", cyclecostDir, code, out)
	}
}

func TestC1741_006_CyclehealthPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+cyclehealthDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", cyclehealthDir, code, out)
	}
}

func TestC1741_007_BothPackagesVetClean(t *testing.T) {
	for _, dir := range packageDirs {
		if out, code := runGo(t, goModuleDir(t), "vet", "./"+dir); code != 0 {
			t.Errorf("RED: go vet ./%s failed (exit=%d):\n%s", dir, code, out)
		}
	}
}

func TestC1741_008_NoCommentLinesAdded(t *testing.T) {
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
func TestC1741_009_TargetFunctionDocsMatchBaseline(t *testing.T) {
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
func TestC1741_010_NoBaselineCommentDeleted(t *testing.T) {
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

var cyclecostMutants = []mutant{
	{"SummarizeCycle", "a stat failure loses its cause", `fmt.Errorf("stat workspace: %w", err)`, `fmt.Errorf("stat workspace: %v", err)`},
	{"SummarizeCycle", "a glob failure loses its cause", `fmt.Errorf("glob: %w", err)`, `fmt.Errorf("glob: %v", err)`},
	{"SummarizeCycle", "a sidecar-only workspace reports ErrNoLogs", `len(logs) == 0 && len(sidecars) == 0`, `len(logs) == 0`},
	{"parseEventsLog", "a nested kind:result payload becomes the result", `if ev.Kind == "result" {`, `if true {`},
	{"parseEventsLog", "a type-mismatched result line replaces the last good one", `&ev); err != nil {`, `&ev); err != nil && false {`},
	{"parseEventsLog", "a scanner error still returns an earlier result", `scanner.Err(); err != nil`, `scanner.Err(); err != nil && false`},
}

var cyclehealthMutants = []mutant{
	{"Check", "GeneratedAt ignores the clock", `GeneratedAt: nowFn(),`, `GeneratedAt: time.Time{},`},
	{"Check", "dossier_commitment does not run", `checkDossierCommitment,`, ``},
	{"Check", "a report write failure is swallowed", `return report, fmt.Errorf("cyclehealth: write report: %w", err)`, `return report, nil`},
	{"Check", "a report write failure loses its cause", `fmt.Errorf("cyclehealth: write report: %w", err)`, `fmt.Errorf("cyclehealth: write report: %v", err)`},
	{"Check", "a report write failure drops the report", `return report, fmt.Errorf("cyclehealth: write report: %w", err)`, `return Report{}, fmt.Errorf("cyclehealth: write report: %w", err)`},
	{"ClassifyOutcome", "an unreadable timing record counts as absent", `!os.IsNotExist(err)`, `false`},
	{"ClassifyOutcome", "a ship dispatch of any verdict counts as shipped", `e.Phase == "ship" && e.Verdict == "PASS"`, `e.Phase == "ship"`},
	{"ClassifyOutcome", "salvage or artifact alone counts as salvaged", `r.ByRung["salvage"] > 0 && r.ByResult["artifact_appeared"] > 0`, `r.ByRung["salvage"] > 0 || r.ByResult["artifact_appeared"] > 0`},
	{"ClassifyOutcome", "a malformed rollup counts as salvaged", `json.Unmarshal(raw, &r) == nil &&`, `json.Unmarshal(raw, &r) != nil ||`},
	{"ClassifyOutcome", "the salvage detail swaps its counters", `r.ByRung["salvage"], r.ByResult["artifact_appeared"])`, `r.ByResult["artifact_appeared"], r.ByRung["salvage"])`},
	{"ClassifyOutcome", "a quota prefix anywhere in the reason counts as deferred", `strings.HasPrefix(e.AbortReason, abortReasonDeferredPrefix)`, `strings.Contains(e.AbortReason, abortReasonDeferredPrefix)`},
	{"ClassifyOutcome", "FAIL diagnostics join without a semicolon", `strings.Join(msgs, "; ")`, `strings.Join(msgs, ", ")`},
}

func TestC1741_011_CyclecostTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, cyclecostDir, cyclecostMutants)
}

func TestC1741_012_CyclehealthTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, cyclehealthDir, cyclehealthMutants)
}

const (
	quotaReason       = "all-families-exhausted: phase audit: core: all CLI families quota-exhausted (exit=85)"
	salvagedRollup    = `{"by_rung":{"salvage":1},"by_result":{"artifact_appeared":1}}`
	shippedDetail     = "ship dispatch recorded verdict PASS"
	salvagedDetail    = "correction-ladder salvage produced the artifact (salvage=1, artifact_appeared=1)"
	initFailedDetail  = "cycle initialization failed before phase timing was recorded"
	unexplainedDetail = "no ship PASS, no salvage, no recorded abort_reason — a terminal path escaped the C1 chokepoint"
)

func TestC1741_013_ClassifyOutcomePrecedenceUnchanged(t *testing.T) {
	cases := []struct {
		name, timing, rollup string
		want                 cyclehealth.Outcome
		detail               string
	}{
		{"a ship PASS outranks salvage and a quota abort", `[{"phase":"audit","verdict":"FAIL","abort_reason":"` + quotaReason + `"},{"phase":"ship","verdict":"PASS"}]`, salvagedRollup, cyclehealth.OutcomeShipped, shippedDetail},
		{"a repaired ship PASS after a failed ship attempt ships", `[{"phase":"ship","verdict":"FAIL"},{"phase":"ship","verdict":"PASS"}]`, "", cyclehealth.OutcomeShipped, shippedDetail},
		{"salvage outranks a quota abort", `[{"phase":"audit","verdict":"FAIL","abort_reason":"` + quotaReason + `"}]`, salvagedRollup, cyclehealth.OutcomeSalvaged, salvagedDetail},
		{"salvage outranks an explained abort", `[{"phase":"build","verdict":"FAIL","abort_reason":"build crashed"}]`, salvagedRollup, cyclehealth.OutcomeSalvaged, salvagedDetail},
		{"salvage outranks a missing timing record", "", salvagedRollup, cyclehealth.OutcomeSalvaged, salvagedDetail},
		{"a later quota abort outranks an earlier explained abort", `[{"phase":"build","verdict":"FAIL","abort_reason":"build crashed"},{"phase":"audit","verdict":"FAIL","abort_reason":"` + quotaReason + `"}]`, "", cyclehealth.OutcomeDeferred, quotaReason},
		{"a later abort reason outranks an earlier FAIL verdict", `[{"phase":"triage","verdict":"FAIL"},{"phase":"build","verdict":"FAIL","abort_reason":"build crashed"}]`, "", cyclehealth.OutcomeFailedExplained, "build crashed"},
		{"the first FAIL verdict names the detail", `[{"phase":"build","verdict":"PASS"},{"phase":"audit","verdict":"FAIL"},{"phase":"ship","verdict":"FAIL"}]`, "", cyclehealth.OutcomeFailedExplained, "phase audit recorded verdict FAIL (no abort — cycle completed through its failure path)"},
		{"no timing record is an initialization failure", "", "", cyclehealth.OutcomeFailedExplained, initFailedDetail},
		{"phases that ran with nothing explaining the end are unexplained", `[{"phase":"build","verdict":"PASS"}]`, "", cyclehealth.OutcomeFailedUnexplained, unexplainedDetail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeIfSet(t, filepath.Join(ws, "phase-timing.json"), tc.timing)
			writeIfSet(t, filepath.Join(ws, "interaction-summary.json"), tc.rollup)
			got, detail := cyclehealth.ClassifyOutcome(ws)
			if got != tc.want || detail != tc.detail {
				t.Errorf("RED: ClassifyOutcome = (%s, %q), want (%s, %q)", got, detail, tc.want, tc.detail)
			}
		})
	}
}

func writeIfSet(t *testing.T, path, body string) {
	t.Helper()
	if body == "" {
		return
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
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
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				docs[dir+"."+fn.Name.Name] = fn.Doc.Text()
			}
		}
	}
	return docs
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
