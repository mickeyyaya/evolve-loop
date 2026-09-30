//go:build acs

package cycle1745

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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cyclesimulatorDir = "internal/cyclesimulator"
	testlatencyDir    = "cmd/testlatency"
	baseCommit        = "cc16fb89"
	offendersRel      = "go/internal/sizeratchet/offenders.json"
	mutantWorkers     = 6
	probeOutEnv       = "ACS1745_PROBE_OUT"
	probeTestName     = "TestACS1745Probe"
)

var packageDirs = []string{cyclesimulatorDir, testlatencyDir}

type target struct {
	key, pkg, file string
	allowance      int
}

var targets = []target{
	{key: testlatencyDir + ".Parse", pkg: testlatencyDir, file: "report.go", allowance: 69},
	{key: testlatencyDir + ".Report.Markdown", pkg: testlatencyDir, file: "report.go", allowance: 60},
	{key: cyclesimulatorDir + ".Run", pkg: cyclesimulatorDir, file: "cyclesimulator.go", allowance: 152},
	{key: cyclesimulatorDir + ".appendSimLedger", pkg: cyclesimulatorDir, file: "cyclesimulator.go", allowance: 71},
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

func TestC1745_001_FourCyclesimulatorTestlatencyFunctionsFitTheRatchetLimit(t *testing.T) {
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

func TestC1745_002_OffendersJSONLeftUnchanged(t *testing.T) {
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

func TestC1745_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("RED: %v", err)
	}
}

func TestC1745_004_BaselineTestFilesUnchanged(t *testing.T) {
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

func TestC1745_005_CyclesimulatorPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+cyclesimulatorDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", cyclesimulatorDir, code, out)
	}
}

func TestC1745_006_TestlatencyPackageTestsPass(t *testing.T) {
	if out, code := runGo(t, goModuleDir(t), "test", "-count=1", "./"+testlatencyDir); code != 0 {
		t.Errorf("RED: go test -count=1 ./%s failed (exit=%d):\n%s", testlatencyDir, code, out)
	}
}

func TestC1745_007_BothPackagesVetAndGofmtClean(t *testing.T) {
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

func TestC1745_008_NoCommentLinesAdded(t *testing.T) {
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

// acs-predicate: config-check — doc comment text is the contract; parsed with go/parser, not grepped.
func TestC1745_009_TargetFunctionDocsMatchBaseline(t *testing.T) {
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

// acs-predicate: config-check — comment text is the contract; graded by commentaudit, not grepped.
func TestC1745_010_NoBaselineCommentDeleted(t *testing.T) {
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
			t.Errorf("RED: %d baseline comment line(s) deleted from %s since %s (restore each where its code now lives; code-comments.md:3):\n  %s", len(deleted), dir, baseCommit, strings.Join(deleted, "\n  "))
		}
	}
}

type mutant struct{ fn, name, anchor, replacement string }

var cyclesimulatorMutants = []mutant{
	{"Run", "the non-positive-cycle log drops its wording", `"cycle must be positive integer, got: %d"`, `"cycle must be positive, got: %d"`},
	{"Run", "the missing-workspace log drops its wording", `"missing workspace arg"`, `"missing workspace"`},
	{"Run", "the missing-project-root log drops the env name", `"missing project root (EVOLVE_PROJECT_ROOT)"`, `"missing project root"`},
	{"Run", "an empty PluginRoot defaults to the workspace", `in.PluginRoot = in.ProjectRoot`, `in.PluginRoot = in.Workspace`},
	{"Run", "a nil Now seam stamps the zero time", `in.Now = time.Now`, `in.Now = func() time.Time { return time.Time{} }`},
	{"Run", "the default token drops the pid", `in.Cycle, os.Getpid()`, `in.Cycle, 0`},
	{"Run", "the default advance swaps phase and agent", `"advance", phase, agent`, `"advance", agent, phase`},
	{"Run", "the default ship drops --dry-run", `"--dry-run", msg`, `msg`},
	{"Run", "the workspace-mkdir log drops its wording", `"workspace mkdir failed: %v"`, `"workspace mkdir: %v"`},
	{"Run", "the start log drops its wording", `"starting simulated walk for cycle %d"`, `"starting walk for cycle %d"`},
	{"Run", "a refusal after the first loop phase is ignored", `in.AdvanceFn(ph.phase, ph.agent)`, `func() error { err := in.AdvanceFn(ph.phase, ph.agent); if ph.phase != "intent" { return nil }; return err }()`},
	{"Run", "the loop-refusal log drops its wording", `"FAIL: cycle-state advance to %s (%s) refused"`, `"FAIL: advance to %s (%s) refused"`},
	{"Run", "the artifact-write log drops its wording", `"FAIL: writing %s: %v"`, `"FAIL: write %s: %v"`},
	{"Run", "loop ledger entries carry the phase, not the agent", `in.Cycle, ph.agent, artifactPath`, `in.Cycle, ph.phase, artifactPath`},
	{"Run", "the loop ledger-append log drops its wording", `"FAIL: ledger append for %s: %v"`, `"FAIL: ledger %s: %v"`},
	{"Run", "the per-phase success log drops the ledger note", `"  ✓ %s → wrote %s, ledger entry"`, `"  ✓ %s → wrote %s"`},
	{"Run", "the ship-refusal log drops its wording", `"FAIL: cycle-state advance to ship refused"`, `"FAIL: ship refused"`},
	{"Run", "the ship-start log drops its wording", `"  ▶ ship phase: invoking ship.sh --dry-run"`, `"  ▶ ship phase"`},
	{"Run", "the ship message drops its suffix", `"simulator: cycle %d plumbing test"`, `"simulator: cycle %d"`},
	{"Run", "the non-zero-ship log drops its reason", `(acceptable for tree-state-mismatch in simulator context)`, `(tolerated)`},
	{"Run", "the retro-refusal log drops its wording", `"FAIL: cycle-state advance to retrospective refused"`, `"FAIL: retrospective refused"`},
	{"Run", "the retro artifact loses the challenge token", `in.writeRetro(in.Token)`, `in.writeRetro("")`},
	{"Run", "the retro-write log drops its wording", `"FAIL: writing retrospective: %v"`, `"FAIL: write retrospective: %v"`},
	{"Run", "the retro ledger entry carries the wrong role", `"retrospective", retroPath`, `"retro", retroPath`},
	{"Run", "the retro ledger-append log drops its wording", `"FAIL: ledger append for retrospective: %v"`, `"FAIL: ledger retrospective: %v"`},
	{"Run", "the retro success log drops its wording", `"  ✓ retrospective → wrote retrospective-report.md, ledger entry"`, `"  ✓ retrospective"`},
	{"Run", "the chain-verify log drops its wording", `"verifying ledger chain post-simulation..."`, `"verifying ledger chain..."`},
	{"Run", "the chain-verify warning drops its reason", `(may be pre-existing; simulator did not break it)`, `(may be pre-existing)`},
	{"Run", "the report records ship rc 0", `in.Token, in.Cycle, shipRC`, `in.Token, in.Cycle, 0`},
	{"Run", "the report title changes", `# Cycle Simulator Report — Cycle %d`, `# Simulator Report — Cycle %d`},
	{"Run", "the report drops the phase and entry counts", `All 6 phases advanced cleanly. 6 ledger entries appended (chain intact).`, `All phases advanced cleanly.`},
	{"Run", "the report drops the ship line", `Ship phase exercised via ship.sh --dry-run (rc=%d).`, `Ship phase exercised (rc=%d).`},
	{"Run", "the report drops the quality disclaimer", `This is a no-LLM plumbing validation; agent output quality is NOT validated.`, `This is a no-LLM plumbing validation.`},
	{"Run", "the report-write log drops its wording", `"FAIL: writing simulator-report: %v"`, `"FAIL: write simulator-report: %v"`},
	{"Run", "the done log drops its wording", `"DONE: simulated cycle %d complete"`, `"DONE: cycle %d complete"`},
	{"Run", "stderr lines lose the [simulator] prefix", `"[simulator] "`, `""`},
	{"appendSimLedger", "the artifact SHA ignores the artifact bytes", `sha256.Sum256(data)`, `sha256.Sum256(data[:0])`},
	{"appendSimLedger", "an unresolvable HEAD is left blank", `gitHEAD = "unknown"`, `gitHEAD = ""`},
	{"appendSimLedger", "git_head records the short SHA", `"rev-parse", "HEAD"`, `"rev-parse", "--short", "HEAD"`},
	{"appendSimLedger", "the tree-state SHA ignores the diff", `sha256Hex(treeStateDiff)`, `sha256Hex(treeStateDiff[:0])`},
	{"appendSimLedger", "the tree-state diff drops staged changes", `"diff", "HEAD"`, `"diff"`},
	{"appendSimLedger", "the entry time is not converted to UTC", `now().UTC()`, `now()`},
	{"appendSimLedger", "the entry time drops its seconds", `"2006-01-02T15:04:05Z"`, `"2006-01-02T15:04Z"`},
	{"appendSimLedger", "the model field changes", `"simulator",`, `"sim",`},
	{"appendSimLedger", "the duration field changes", `"0",`, `"1",`},
	{"appendSimLedger", "an unresolvable run id is stamped empty", `runID != ""`, `runID != "" || true`},
	{"appendSimLedger", "the tip separator changes", `"%d:%s\n", entrySeq`, `"%d-%s\n", entrySeq`},
	{"appendSimLedger", "the tip hashes the newline-terminated line", `sha256Hex(line)`, `sha256Hex(line + "\n")`},
	{"appendSimLedger", "the tip is never renamed into place", `os.Rename(tmp, tipPath)`, `os.Rename(tmp, tmp+".done")`},
	{"appendSimLedger", "a failed tip write leaves the temp path behind", `_ = os.Remove(tmp)`, `_ = os.Remove(tmp + ".kept")`},
}

var testlatencyMutants = []mutant{
	{"Parse", "the scanner buffer shrinks to 64 KiB", `16*1024*1024`, `64*1024`},
	{"Parse", "a start event does not register its package", `case "run", "start":`, `case "run":`},
	{"Parse", "a run event registers the empty package", `e.Package != ""`, `e.Package != "" || true`},
	{"Parse", "a fail summary is not recorded", `case "pass", "fail", "skip":`, `case "pass", "skip":`},
	{"Parse", "every summary reads as pass", `p.Status = e.Action`, `p.Status = "pass"`},
	{"Parse", "an equal-elapsed later test becomes the slowest", `e.Elapsed > p.SlowestSecs`, `e.Elapsed >= p.SlowestSecs`},
	{"Parse", "the scan error drops its wording", `"scan test2json stream: %w"`, `"scan: %w"`},
	{"Parse", "incomplete packages are not sorted", `sort.Strings(rep.Incomplete)`, `sort.Strings(nil)`},
	{"Markdown", "the aggregate wall sums serial time", `aggWall += p.Wall`, `aggWall += p.SerialSum`},
	{"Markdown", "the serial bound sums wall time", `aggSerial += p.SerialSum`, `aggSerial += p.Wall`},
	{"Markdown", "the test total counts packages", `totalTests += p.NumTests`, `totalTests++`},
	{"Markdown", "the title is a level-2 heading", `"# %s\n\n", o.Title`, `"## %s\n\n", o.Title`},
	{"Markdown", "the package count loses its bold", `"- Packages: **%d**\n"`, `"- Packages: %d\n"`},
	{"Markdown", "the test-count label changes", `"- Top-level tests: **%d**\n"`, `"- Tests: **%d**\n"`},
	{"Markdown", "the aggregate-wall note drops its explanation", `(sum of parallel-aware per-package times)`, `(sum)`},
	{"Markdown", "the serial-bound label drops its formula", `"- Fully-serial upper bound (Σ test elapsed): **%.1fs**\n\n"`, `"- Fully-serial upper bound: **%.1fs**\n\n"`},
	{"Markdown", "the incomplete warning always renders", `len(rep.Incomplete) > 0`, `len(rep.Incomplete) >= 0`},
	{"Markdown", "incomplete packages are joined without a space", `strings.Join(rep.Incomplete, ", ")`, `strings.Join(rep.Incomplete, ",")`},
	{"Markdown", "the incomplete warning drops its consequence", `(truncated stream / panic / timeout) — their wall time is missing and they are NOT counted above`, `(truncated stream / panic / timeout)`},
	{"Markdown", "the slow-packages heading drops its purpose", `"## Slow packages (> %.1fs wall) — optimization targets\n\n"`, `"## Slow packages (> %.1fs wall)\n\n"`},
	{"Markdown", "a package at the wall threshold is flagged", `p.Wall <= o.ThresholdPkg`, `p.Wall < o.ThresholdPkg`},
	{"Markdown", "the slow-package row swaps serial and slowest", `p.SerialSum, p.SlowestTest, p.SlowestSecs`, `p.SlowestSecs, p.SlowestTest, p.SerialSum`},
	{"Markdown", "the slow-package row rounds wall to one decimal", `"| %s | %.2f | %d | %.2f | %s | %.2f |\n"`, `"| %s | %.1f | %d | %.2f | %s | %.2f |\n"`},
	{"Markdown", "the none row renders even with flagged packages", `flagged++`, `flagged += 0`},
	{"Markdown", "the slow-packages header row drops columns", `"| Package | Wall (s) | Tests | Σserial (s) | Slowest test | Slowest (s) |\n"`, `"| Package | Wall (s) |\n"`},
	{"Markdown", "the slow-packages alignment row changes", `"|---|--:|--:|--:|---|--:|\n"`, `"|---|---|---|---|---|---|\n"`},
	{"Markdown", "the slowest-tests heading loses its blank line", `"## Slowest %d tests\n\n", o.Top`, `"## Slowest %d tests\n", o.Top`},
	{"Markdown", "the slowest-tests header drops its unit", `"| Test | Package | Elapsed (s) |\n|---|---|--:|\n"`, `"| Test | Package | Elapsed |\n|---|---|--:|\n"`},
	{"Markdown", "a test at the per-test threshold is counted", `t.Elapsed > o.ThresholdTst`, `t.Elapsed >= o.ThresholdTst`},
	{"Markdown", "sections lose their trailing blank line", `b.WriteString("\n")`, `b.WriteString("")`},
}

func TestC1745_011_CyclesimulatorTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, cyclesimulatorDir, cyclesimulatorMutants)
}

func TestC1745_012_TestlatencyTestsKillBehaviorMutants(t *testing.T) {
	assertMutantsKilled(t, testlatencyDir, testlatencyMutants)
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
			outcomes[i] = execGo(goDir, nil, append(args, "./"+dir)...)
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

func TestC1745_013_CyclesimulatorBehaviorMatchesBaseline(t *testing.T) {
	assertBehaviorMatchesBaseline(t, cyclesimulatorDir, "cyclesimulator", "cyclesimulator_probe.go.txt")
}

func TestC1745_014_TestlatencyBehaviorMatchesBaseline(t *testing.T) {
	assertBehaviorMatchesBaseline(t, testlatencyDir, "main", "testlatency_probe.go.txt")
}

func TestC1745_015_ExplanationSizeClaimsMatchTheRatchetScanner(t *testing.T) {
	rel, doc := explanationDoc(t)
	measured := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > measured[s.Key] {
			measured[s.Key] = s.Lines
		}
	}
	claimed := map[string]int{}
	for _, m := range sizeClaim.FindAllStringSubmatch(doc, -1) {
		tg, ok := targetNamed(m[1])
		if !ok {
			continue
		}
		claimed[tg.key]++
		before, errBefore := strconv.Atoi(m[2])
		after, errAfter := strconv.Atoi(m[3])
		if errBefore != nil || errAfter != nil {
			t.Errorf("RED: %s size claim %q is not a pair of line counts", rel, m[0])
			continue
		}
		if before != tg.allowance {
			t.Errorf("RED: %s says %s shrank from %d lines; at %s sizeratchet measured %d", rel, m[1], before, baseCommit, tg.allowance)
		}
		if now := measured[tg.key]; after != now {
			t.Errorf("RED: %s says %s is now %d lines; sizeratchet.Walk measures %d — quote the scanner, do not hand-count", rel, m[1], after, now)
		}
	}
	for _, tg := range targets {
		if claimed[tg.key] == 0 {
			t.Errorf("RED: %s states no `name` (before → after) size claim for %s", rel, tg.key)
		}
	}
}

func TestC1745_016_ExplanationDoesNotCreditTheProbeWithPinningEqualWallOrder(t *testing.T) {
	if !probeIsBlindToEqualWallOrder(t) {
		t.Fatalf("RED: the testlatency probe dump changes with the equal-wall tie-break — it observes map-random Packages order, so 014 would flake")
	}
	rel, doc := explanationDoc(t)
	limitations := markdownSection(doc, "## Limitations")
	if strings.TrimSpace(limitations) == "" {
		t.Fatalf("RED: %s has no ## Limitations section", rel)
	}
	if !quirkDisclosure.MatchString(limitations) {
		t.Errorf("RED: %s Limitations no longer discloses that Packages order among equal walls is map-random", rel)
	}
	for _, sentence := range sentenceEnd.Split(limitations, -1) {
		if probeAttribution.MatchString(sentence) && !negation.MatchString(sentence) {
			t.Errorf("RED: %s Limitations credits the probe with pinning the equal-wall order, but the probe dump is identical under ascending and descending tie-breaks (it sorts by Pkg): %q", rel, strings.TrimSpace(sentence))
		}
	}
	if !behaviorReason.MatchString(limitations) {
		t.Errorf("RED: %s Limitations gives no reason for leaving the equal-wall order map-random — fixing the sort changes behavior, which is out of scope for an extract-method lane", rel)
	}
}

func TestC1745_017_NoRawGitFixtureOutsideGittest(t *testing.T) {
	goDir := goModuleDir(t)
	files, note, err := rawgitratchet.BoundTestFiles(goDir)
	if err != nil {
		t.Fatalf("rawgitratchet.BoundTestFiles: %v", err)
	}
	if note != "" {
		t.Log(note)
	}
	sites, err := rawgitratchet.Sites(goDir, files)
	if err != nil {
		t.Fatalf("rawgitratchet.Sites: %v", err)
	}
	baseline, err := rawgitratchet.LoadBaseline(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rawgitBaselineRel)))
	if err != nil {
		t.Fatalf("rawgitratchet.LoadBaseline: %v", err)
	}
	if err := rawgitratchet.Check(sites, baseline); err != nil {
		t.Errorf("RED: the ship-time repo-contract scanner pack refuses this lane (REPO_CONTRACT_GATE):\n%v", err)
	}
}

func TestC1745_018_RawGitBaselineLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", rawgitBaselineRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s edited since %s — the raw-git list may only shrink and this lane grants itself no slot; build the fixture with gittest.Fixture(t):\n%s", rawgitBaselineRel, baseCommit, diff)
	}
}

func TestC1745_019_CyclesimulatorTestReposDisableMaintenanceBeforeCommitting(t *testing.T) {
	calls := recordGitCalls(t, cyclesimulatorDir)
	quiet := map[string]bool{}
	commits := 0
	var loud []string
	for _, c := range calls {
		switch c.sub {
		case "config":
			if setsMaintenanceOff(c.rest) {
				quiet[c.dir] = true
			}
		case "commit":
			commits++
			if !quiet[c.dir] && !c.maintenanceOff {
				loud = append(loud, c.dir)
			}
		}
	}
	if commits == 0 {
		t.Errorf("RED: the %s tests commit in no git repository — the git_head / tree_state_sha characterization needs a gittest.Fixture(t) repo with a real HEAD", cyclesimulatorDir)
	}
	if len(loud) > 0 {
		t.Errorf("RED: %d commit(s) in %s test repos ran before maintenance.auto=false was set — a hand-rolled fixture lets git's detached maintenance child race t.TempDir() removal; build the repo with gittest.Fixture(t):\n  %s", len(loud), cyclesimulatorDir, strings.Join(loud, "\n  "))
	}
}

type gitCall struct {
	dir, sub       string
	rest           []string
	maintenanceOff bool
}

func recordGitCalls(t *testing.T, dir string) []gitCall {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not on PATH: %v", err)
	}
	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "calls.log")
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(gitShim), 0o755); err != nil {
		t.Fatalf("write git shim: %v", err)
	}
	env := []string{
		"PATH=" + shimDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		gitLogEnv + "=" + logPath,
		realGitEnv + "=" + realGit,
	}
	r := execGo(goModuleDir(t), env, "test", "-count=1", "./"+dir)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.code != 0 {
		t.Fatalf("RED: go test -count=1 ./%s failed under the git recorder (exit=%d):\n%s", dir, r.code, r.out)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read git call log: %v", err)
	}
	var calls []gitCall
	for _, line := range strings.Split(string(raw), "\n") {
		if fields := strings.Split(line, gitArgSep); len(fields) > 1 {
			calls = append(calls, parseGitCall(fields[0], fields[1:]))
		}
	}
	if len(calls) == 0 {
		t.Fatalf("the git recorder logged no git call from ./%s — the PATH shim was bypassed, so fixture repos cannot be observed", dir)
	}
	return calls
}

func parseGitCall(pwd string, args []string) gitCall {
	c := gitCall{dir: pwd}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-C" && i+1 < len(args):
			i++
			c.dir = resolveDir(c.dir, args[i])
		case a == "-c" && i+1 < len(args):
			i++
			c.maintenanceOff = c.maintenanceOff || args[i] == "maintenance.auto=false"
		case strings.HasPrefix(a, "-"):
			continue
		default:
			c.sub, c.rest = a, args[i+1:]
			return c
		}
	}
	return c
}

func resolveDir(base, dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(base, dir)
}

func setsMaintenanceOff(args []string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "maintenance.auto" && args[i+1] == "false" {
			return true
		}
	}
	return false
}

const (
	rawgitBaselineRel = "go/internal/rawgitratchet/baseline.json"
	gitLogEnv         = "ACS1745_GIT_LOG"
	realGitEnv        = "ACS1745_REAL_GIT"
	gitArgSep         = "\x1f"
	gitShim           = "#!/bin/sh\nsep=$(printf '\\037')\nline=\"$PWD\"\nfor a in \"$@\"; do line=\"$line$sep$a\"; done\nprintf '%s\\n' \"$line\" >> \"$" + gitLogEnv + "\"\nexec \"$" + realGitEnv + "\" \"$@\"\n"
)

const (
	explanationGlob   = "docs/explain/builds/cycle-1745-*.md"
	equalWallSort     = "return rep.Packages[i].Wall > rep.Packages[j].Wall"
	equalWallTieBreak = "if rep.Packages[i].Wall != rep.Packages[j].Wall { return rep.Packages[i].Wall > rep.Packages[j].Wall }; return rep.Packages[i].Pkg %s rep.Packages[j].Pkg"
)

var (
	sizeClaim        = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_.]*)`\\s*\\((\\d+)\\s*(?:→|->|to)\\s*(\\d+)")
	sentenceEnd      = regexp.MustCompile(`[.!?](?:\s+|$)`)
	probeAttribution = regexp.MustCompile(`(?i)\b(probe|predicate|differential)\b.*\bpin(s|ned|ning)?\b|\bpin(s|ned|ning)?\b.*\b(probe|predicate|differential)\b`)
	negation         = regexp.MustCompile(`(?i)\b(not|neither|nor|never|no)\b|n't`)
	quirkDisclosure  = regexp.MustCompile(`(?i)equal[- ]walls?|map[- ]random`)
	behaviorReason   = regexp.MustCompile(`(?i)behavio|scope`)
)

func explanationDoc(t *testing.T) (rel, body string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(explanationGlob)))
	if err != nil {
		t.Fatalf("glob %s: %v", explanationGlob, err)
	}
	if len(matches) != 1 {
		t.Fatalf("RED: want exactly one build explanation matching %s, found %d: %v", explanationGlob, len(matches), matches)
	}
	rel, err = filepath.Rel(root, matches[0])
	if err != nil {
		t.Fatalf("relativize %s: %v", matches[0], err)
	}
	rel = filepath.ToSlash(rel)
	if _, stderr, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s is not tracked, so it would not ship: %s", rel, stderr)
	}
	src, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return rel, string(src)
}

func targetNamed(name string) (target, bool) {
	for _, tg := range targets {
		if strings.HasSuffix(tg.key, "."+name) {
			return tg, true
		}
	}
	return target{}, false
}

func markdownSection(doc, heading string) string {
	var body []string
	inSection := false
	for _, line := range strings.Split(doc, "\n") {
		switch {
		case strings.TrimSpace(line) == heading:
			inSection = true
		case inSection && strings.HasPrefix(line, "## "):
			return strings.Join(body, "\n")
		case inSection:
			body = append(body, line)
		}
	}
	return strings.Join(body, "\n")
}

func probeIsBlindToEqualWallOrder(t *testing.T) bool {
	t.Helper()
	probeSrc, err := os.ReadFile(filepath.Join("testdata", "testlatency_probe.go.txt"))
	if err != nil {
		t.Fatalf("read probe fixture: %v", err)
	}
	tmp := t.TempDir()
	probe := filepath.Join(tmp, "probe_test.go")
	if err := os.WriteFile(probe, append([]byte("package main\n\n"), probeSrc...), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	current, _ := probeOverlays(t, testlatencyDir, probe, tmp)
	var dumps []string
	for _, order := range []string{"<", ">"} {
		name := "tiebreak-asc"
		if order == ">" {
			name = "tiebreak-desc"
		}
		replace := map[string]string{}
		for path, backing := range current {
			replace[path] = backing
		}
		mutated := 0
		for file, src := range packageSources(t, testlatencyDir) {
			if !strings.Contains(string(src), equalWallSort) {
				continue
			}
			backing := filepath.Join(tmp, name+"-"+file)
			if err := os.WriteFile(backing, []byte(strings.ReplaceAll(string(src), equalWallSort, fmt.Sprintf(equalWallTieBreak, order))), 0o644); err != nil {
				t.Fatalf("write %s: %v", backing, err)
			}
			replace[filepath.Join(goModuleDir(t), testlatencyDir, file)] = backing
			mutated++
		}
		if mutated == 0 {
			t.Fatalf("the Packages wall sort %q no longer occurs in %s — re-derive the tie-break probe", equalWallSort, testlatencyDir)
		}
		dumps = append(dumps, runProbe(t, testlatencyDir, writeOverlay(t, tmp, name, replace), filepath.Join(tmp, name+".out")))
	}
	return dumps[0] == dumps[1]
}

func assertBehaviorMatchesBaseline(t *testing.T, dir, pkgName, probeFixture string) {
	t.Helper()
	probeSrc, err := os.ReadFile(filepath.Join("testdata", probeFixture))
	if err != nil {
		t.Fatalf("read probe fixture: %v", err)
	}
	tmp := t.TempDir()
	probe := filepath.Join(tmp, "probe_test.go")
	if err := os.WriteFile(probe, append([]byte("package "+pkgName+"\n\n"), probeSrc...), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	current, baseline := probeOverlays(t, dir, probe, tmp)
	want := runProbe(t, dir, writeOverlay(t, tmp, "baseline", baseline), filepath.Join(tmp, "baseline.out"))
	got := runProbe(t, dir, writeOverlay(t, tmp, "current", current), filepath.Join(tmp, "current.out"))
	if got != want {
		t.Errorf("RED: %s behavior differs from baseline %s on the probe scenarios (the extraction must preserve every byte):\n%s", dir, baseCommit, firstDifference(want, got))
	}
}

func probeOverlays(t *testing.T, dir, probe, tmp string) (current, baseline map[string]string) {
	t.Helper()
	pkgDir := filepath.Join(goModuleDir(t), dir)
	probeTarget := filepath.Join(pkgDir, "zz_acs1745_probe_test.go")
	current = map[string]string{probeTarget: probe}
	baseline = map[string]string{probeTarget: probe}
	baseSources := baselineSources(t, dir)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read %s: %v", pkgDir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		path := filepath.Join(pkgDir, name)
		if strings.HasSuffix(name, "_test.go") {
			current[path] = ""
			baseline[path] = ""
			continue
		}
		if _, atBase := baseSources[name]; !atBase {
			baseline[path] = ""
		}
	}
	for name, src := range baseSources {
		backing := filepath.Join(tmp, "base-"+name)
		if err := os.WriteFile(backing, src, 0o644); err != nil {
			t.Fatalf("write baseline %s: %v", name, err)
		}
		baseline[filepath.Join(pkgDir, name)] = backing
	}
	return current, baseline
}

func baselineSources(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	root := acsassert.RepoRoot(t)
	listing, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-tree", "--name-only", baseCommit+":go/"+dir)
	if code != 0 {
		t.Fatalf("git ls-tree %s:go/%s failed: %v\n%s", baseCommit, dir, err, stderr)
	}
	sources := map[string][]byte{}
	for _, name := range strings.Fields(listing) {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "show", baseCommit+":go/"+dir+"/"+name)
		if code != 0 {
			t.Fatalf("git show %s:go/%s/%s failed: %v\n%s", baseCommit, dir, name, err, stderr)
		}
		sources[name] = []byte(src)
	}
	if len(sources) == 0 {
		t.Fatalf("baseline %s has no non-test Go source under go/%s", baseCommit, dir)
	}
	return sources
}

func writeOverlay(t *testing.T, tmp, name string, replace map[string]string) string {
	t.Helper()
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	path := filepath.Join(tmp, name+"-overlay.json")
	if err := os.WriteFile(path, overlay, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return path
}

func runProbe(t *testing.T, dir, overlay, out string) string {
	t.Helper()
	r := execGo(goModuleDir(t), []string{probeOutEnv + "=" + out}, "test", "-count=1", "-run", "^"+probeTestName+"$", "-overlay", overlay, "./"+dir)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.code != 0 {
		t.Fatalf("RED: probe run for %s failed (exit=%d):\n%s", dir, r.code, r.out)
	}
	dump, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("probe for %s wrote no dump (the probe did not run): %v\n%s", dir, err, r.out)
	}
	if len(dump) == 0 {
		t.Fatalf("probe for %s wrote an empty dump", dir)
	}
	return string(dump)
}

func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("first difference at dump line %d:\n  baseline: %q\n  worktree: %q", i+1, w, g)
		}
	}
	return "dumps differ only in length"
}

type goRun struct {
	out  string
	code int
	err  error
}

func execGo(dir string, env []string, args ...string) goRun {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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
	r := execGo(dir, nil, args...)
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
	return writeOverlay(t, tmp, "mutant", replace)
}
