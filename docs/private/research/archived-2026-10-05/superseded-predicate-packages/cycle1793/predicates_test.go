//go:build acs

package cycle1793

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/guardcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fixtureHeader = `package fixture

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type nopTB struct{}

func (nopTB) Helper()               {}
func (nopTB) Errorf(string, ...any) {}

type recordingTB struct{ msgs []string }

func (r *recordingTB) Helper()                 {}
func (r *recordingTB) Errorf(f string, a ...any) { r.msgs = append(r.msgs, f) }
`

const invertedIdiomSource = fixtureHeader + `
func TestInvertedIdiom(t *testing.T) {
	if acsassert.FileContains(t, "f.go", "retiredFlag") {
		t.Errorf("retiredFlag still present")
	}
}
`

const fileExistsInvertedSource = fixtureHeader + `
func TestRetiredFileGone(t *testing.T) {
	if acsassert.FileExists(t, "legacy.md") {
		t.Errorf("legacy.md still exists after the archival")
	}
}
`

const jsonFieldInvertedSource = fixtureHeader + `
func TestRetiredKeyCleared(t *testing.T) {
	if acsassert.JSONFieldEquals(t, "state.json", "legacy.enabled", true) {
		t.Fatalf("legacy.enabled is still true")
	}
}
`

const satisfiableSource = fixtureHeader + `
func TestAbsenceWithNegativePrimitive(t *testing.T) {
	if !acsassert.FileNotContains(t, "f.go", "retiredFlag") {
		t.Errorf("f.go still holds retiredFlag")
	}
}

func TestPositiveWithMissingMessage(t *testing.T) {
	if !acsassert.FileContains(t, "f.go", "newFlag") {
		t.Errorf("newFlag missing from f.go")
	}
}

func TestBareConditionBranchDoesNotReportFailure(t *testing.T) {
	if acsassert.FileContains(t, "f.go", "optional") {
		t.Logf("optional section present")
	}
}

func TestSwallowingValueTBProbesExistence(t *testing.T) {
	if acsassert.FileExists(nopTB{}, "item.json") {
		t.Errorf("item.json still lives in the active root")
	}
}

func TestRecordingTBProbesJSONField(t *testing.T) {
	tb := &recordingTB{}
	if acsassert.JSONFieldEquals(tb, "doc.json", "key", "other") {
		t.Errorf("JSONFieldEquals returned true, want a failure")
	}
}

func TestRecordingTBProbesSubstringAbsence(t *testing.T) {
	probe := &recordingTB{}
	if acsassert.FileContains(probe, "f.go", "retiredFlag") {
		t.Errorf("retiredFlag still present")
	}
}
`

var satisfiableFuncs = []string{
	"TestAbsenceWithNegativePrimitive",
	"TestPositiveWithMissingMessage",
	"TestBareConditionBranchDoesNotReportFailure",
	"TestSwallowingValueTBProbesExistence",
	"TestRecordingTBProbesJSONField",
	"TestRecordingTBProbesSubstringAbsence",
}

func writeFixtureDir(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func lintFixture(t *testing.T, src string) evalqualitycheck.UnsatisfiableLintReport {
	t.Helper()
	report, err := evalqualitycheck.LintUnsatisfiablePredicates(writeFixtureDir(t, src))
	if err != nil {
		t.Fatalf("LintUnsatisfiablePredicates: %v", err)
	}
	if report.Linted() != 1 {
		t.Fatalf("receipt must show the one fixture file parsed; Linted=%d Files=%v", report.Linted(), report.Files)
	}
	return report
}

func flaggedFuncs(report evalqualitycheck.UnsatisfiableLintReport) map[string]bool {
	out := map[string]bool{}
	for _, f := range report.Findings {
		out[f.Func] = true
	}
	return out
}

func TestC1793_001_AbsenceAssertionIsExpressibleDirectly(t *testing.T) {
	dir := t.TempDir()
	withoutRetired := filepath.Join(dir, "without.go")
	withRetired := filepath.Join(dir, "with.go")
	for p, body := range map[string]string{withoutRetired: "package x\n", withRetired: "package x // retiredFlag\n"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name       string
		path       string
		wantOK     bool
		wantLogged bool
	}{
		{"absent substring holds", withoutRetired, true, false},
		{"present substring fails", withRetired, false, true},
		{"unreadable file fails", filepath.Join(dir, "gone.go"), false, true},
	} {
		tb := &recordingTB{}
		got := acsassert.FileNotContains(tb, c.path, "retiredFlag")
		if got != c.wantOK || (len(tb.msgs) > 0) != c.wantLogged {
			t.Errorf("RED: FileNotContains/%s = %v with messages %q; want %v, logged=%v", c.name, got, tb.msgs, c.wantOK, c.wantLogged)
		}
	}
}

type recordingTB struct{ msgs []string }

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) { r.msgs = append(r.msgs, format) }

const cycle1488RepairedCall = `acsassert.FileNotContains(t, bindings, "worktreeTree == headTree")`

const cycle1488Func = "TestC1488_003_AuditBindingPutWiredToSharedPredicate"

func TestC1793_002_LiveCycle1488PredicateBytesFlaggedRedFirst(t *testing.T) {
	livePath := filepath.Join(acsassert.RepoRoot(t), "go", "acs", "cycle1488", "predicates_test.go")
	raw, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("live cycle-1488 predicate bytes unreadable: %v", err)
	}
	live := string(raw)
	if strings.Count(live, cycle1488RepairedCall) != 1 {
		t.Fatalf("live %s no longer holds the repaired call %s exactly once — fixture drift", livePath, cycle1488RepairedCall)
	}
	unrepaired := map[string]string{
		"negated positive primitive with absence message": strings.Replace(live, cycle1488RepairedCall, `acsassert.FileContains(t, bindings, "worktreeTree == headTree")`, 1),
		"bare inverted positive primitive":                strings.Replace(live, "!"+cycle1488RepairedCall, `acsassert.FileContains(t, bindings, "worktreeTree == headTree")`, 1),
	}
	for name, src := range unrepaired {
		if src == live {
			t.Fatalf("%s: unrepair substitution changed nothing", name)
		}
		if !flaggedFuncs(lintFixture(t, src))[cycle1488Func] {
			t.Errorf("RED: the unrepaired cycle-1488 bytes (%s) are not flagged for %s", name, cycle1488Func)
		}
	}
	if flaggedFuncs(lintFixture(t, live))[cycle1488Func] {
		t.Errorf("RED: the repaired live cycle-1488 bytes (FileNotContains) are flagged — a false positive on the live corpus")
	}
}

func TestC1793_003_EverySelfReportingPositivePrimitiveInvertedIsFlagged(t *testing.T) {
	for _, c := range []struct {
		primitive string
		src       string
		fn        string
	}{
		{"FileContains", invertedIdiomSource, "TestInvertedIdiom"},
		{"FileExists", fileExistsInvertedSource, "TestRetiredFileGone"},
		{"JSONFieldEquals", jsonFieldInvertedSource, "TestRetiredKeyCleared"},
	} {
		report := lintFixture(t, c.src)
		if !flaggedFuncs(report)[c.fn] {
			t.Errorf("RED: acsassert.%s reporting through t and used as a bare failure condition is red on every tree, yet %s is not flagged; findings=%+v", c.primitive, c.fn, report.Findings)
		}
		for _, f := range report.Findings {
			if f.File != "predicates_test.go" || strings.TrimSpace(f.Reason) == "" || strings.TrimSpace(f.Kind) == "" {
				t.Errorf("RED: finding must name its file, kind and reason: %+v", f)
			}
		}
	}
}

func TestC1793_004_SatisfiableShapesAreNeverFlagged(t *testing.T) {
	report := lintFixture(t, satisfiableSource)
	flagged := flaggedFuncs(report)
	for _, fn := range satisfiableFuncs {
		if flagged[fn] {
			t.Errorf("RED: %s is satisfiable (absence primitive, presence message, non-failing branch, or a TB that swallows the primitive's own report) but was flagged; findings=%+v", fn, report.Findings)
		}
	}
}

func TestC1793_005_LintRejectsMissingPathAndEmptyDirLoudly(t *testing.T) {
	if _, err := evalqualitycheck.LintUnsatisfiablePredicates(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Errorf("RED: a nonexistent path must be an error, not a clean report")
	}
	if _, err := evalqualitycheck.LintUnsatisfiablePredicates(t.TempDir()); err == nil {
		t.Errorf("RED: a directory with no .go files must be an error, not a clean report")
	}
}

const passingEval = "# Eval\n\n```bash\ngo test -count=1 -run TestSomething ./internal/foo\n```\n"

const tautologyEval = "```bash\n:\n```\n"

func runQualityCheck(t *testing.T, evalBody, predicatesDir string) (string, int) {
	t.Helper()
	evalPath := filepath.Join(t.TempDir(), "eval.md")
	if err := os.WriteFile(evalPath, []byte(evalBody), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := guardcmd.RunEval([]string{"quality-check", "-predicates", predicatesDir, evalPath}, nil, &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func TestC1793_006_EvalQualityPreflightCLIWarnsAndKeepsHalt(t *testing.T) {
	cleanOut, cleanCode := runQualityCheck(t, passingEval, writeFixtureDir(t, satisfiableSource))
	if cleanCode != 0 {
		t.Errorf("RED: control (satisfiable predicates) must PASS rc=0 through `evolve eval quality-check`, got rc=%d:\n%s", cleanCode, cleanOut)
	}
	if !strings.Contains(cleanOut, "unsatisfiable-lint: linted 1 file(s)") {
		t.Errorf("RED: pre-flight must print the unconditional linted-N-files receipt on a clean run:\n%s", cleanOut)
	}
	for _, src := range []string{invertedIdiomSource, fileExistsInvertedSource} {
		out, code := runQualityCheck(t, passingEval, writeFixtureDir(t, src))
		if code != 1 {
			t.Errorf("RED: an unsatisfiable predicate must raise PASS→WARN (rc=1), got rc=%d:\n%s", code, out)
		}
		if !strings.Contains(out, "unsatisfiable[") {
			t.Errorf("RED: pre-flight output must carry an unsatisfiable[...] finding line:\n%s", out)
		}
	}
	haltOut, haltCode := runQualityCheck(t, tautologyEval, writeFixtureDir(t, invertedIdiomSource))
	if haltCode != 2 {
		t.Errorf("RED: a Level-0 tautology must stay HALT (rc=2) with an advisory finding present, got rc=%d:\n%s", haltCode, haltOut)
	}
	if !strings.Contains(haltOut, "unsatisfiable[") {
		t.Errorf("RED: the unsatisfiable lint must still report alongside the HALT:\n%s", haltOut)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	captured := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		captured <- string(b)
	}()
	orig := os.Stderr
	os.Stderr = w
	func() {
		defer func() { os.Stderr = orig }()
		fn()
	}()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return <-captured
}

const hostReviewCycle = 987654

func reviewTDDDeliverable(t *testing.T, predicateSource string) string {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	predicateDir := filepath.Join(worktree, "go", "acs", "cycle987654")
	workspace := filepath.Join(root, "runs", "cycle-987654")
	for _, d := range []string{predicateDir, workspace} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(predicateDir, "predicates_test.go"), []byte(predicateSource), 0o644); err != nil {
		t.Fatal(err)
	}
	var res core.ReviewResult
	logged := captureStderr(t, func() {
		res = evalgate.NewReviewer(config.StageAdvisory).Review(context.Background(), core.ReviewInput{
			Cycle:       hostReviewCycle,
			Phase:       string(core.PhaseTDD),
			Workspace:   workspace,
			Worktree:    worktree,
			ProjectRoot: worktree,
		})
	})
	return logged + "\n" + res.Reason
}

func linesNaming(out, fn string) []string {
	var hits []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, fn) && strings.Contains(strings.ToLower(line), "unsatisfiable") {
			hits = append(hits, line)
		}
	}
	return hits
}

func TestC1793_007_HostTDDReviewFlagsUnsatisfiablePredicateBeforeDispatch(t *testing.T) {
	for _, c := range []struct {
		src string
		fn  string
	}{
		{invertedIdiomSource, "TestInvertedIdiom"},
		{fileExistsInvertedSource, "TestRetiredFileGone"},
	} {
		out := reviewTDDDeliverable(t, c.src)
		if len(linesNaming(out, c.fn)) == 0 {
			t.Errorf("RED: the production TDD-phase review (evalgate.NewReviewer, wired at cmd/evolve/cmd_cycle.go) never names %s as unsatisfiable — the lint runs only when an agent hand-runs quality-check, so the cycle-1488 burn still reaches build:\n%s", c.fn, out)
		}
	}
	clean := reviewTDDDeliverable(t, satisfiableSource)
	if !strings.Contains(strings.ToLower(clean), "unsatisfiable") {
		t.Errorf("RED: a clean TDD-phase review must still log an unsatisfiable-lint receipt, so a clean cycle reads differently from a lint that never ran:\n%s", clean)
	}
	for _, fn := range satisfiableFuncs {
		if hits := linesNaming(clean, fn); len(hits) > 0 {
			t.Errorf("RED: the TDD-phase review flags satisfiable %s: %q", fn, hits)
		}
	}
}

func TestC1793_008_CorpusSweepCoversEveryPredicatePackage(t *testing.T) {
	acsDir := filepath.Join(acsassert.RepoRoot(t), "go", "acs")
	goFilesByDir := map[string]int{}
	err := filepath.WalkDir(acsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			goFilesByDir[filepath.Dir(path)]++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", acsDir, err)
	}
	totalGoFiles, linted, findings := 0, 0, 0
	sawRegression, sawRedteam := false, false
	flaggedByPkg := map[string]map[string]bool{}
	for dir, n := range goFilesByDir {
		totalGoFiles += n
		rel, _ := filepath.Rel(acsDir, dir)
		sawRegression = sawRegression || strings.HasPrefix(rel, "regression"+string(filepath.Separator))
		sawRedteam = sawRedteam || rel == "redteam"
		report, err := evalqualitycheck.LintUnsatisfiablePredicates(dir)
		if err != nil {
			t.Errorf("RED: corpus sweep failed on %s: %v", rel, err)
			continue
		}
		linted += report.Linted()
		flaggedByPkg[rel] = flaggedFuncs(report)
		for _, f := range report.Findings {
			findings++
			t.Logf("corpus finding: %s/%s:%s [%s] — %s", rel, f.File, f.Func, f.Kind, f.Reason)
			if f.Func == "" || f.File == "" || f.Kind == "" || f.Reason == "" {
				t.Errorf("RED: incomplete finding in %s: %+v", rel, f)
			}
		}
	}
	t.Logf("corpus sweep: %d packages, %d .go files on disk, %d linted, %d findings", len(goFilesByDir), totalGoFiles, linted, findings)
	if linted != totalGoFiles || !sawRegression || !sawRedteam {
		t.Errorf("RED: the sweep must lint every .go file under go/acs including regression/ and redteam/ (no silent caps); onDisk=%d linted=%d regression=%v redteam=%v", totalGoFiles, linted, sawRegression, sawRedteam)
	}
	for _, live := range []struct{ pkg, fn string }{
		{"cycle986", "TestC986_001_stale_inbox_item_retired"},
		{"cycle1702", "TestC1702_009_ErrorAndJSONReadersHintOnlyForNotExist"},
		{"cycle1702", "TestC1702_005_HintOnlyForNotExist"},
		{"cycle1488", cycle1488Func},
	} {
		if flaggedByPkg[live.pkg][live.fn] {
			t.Errorf("RED: live corpus %s:%s is satisfiable (swallowing TB or the absence primitive) but the sweep flags it", live.pkg, live.fn)
		}
	}
	if own := flaggedByPkg["cycle1793"]; len(own) != 0 {
		t.Errorf("RED: this cycle's own predicates must be satisfiable: %v", own)
	}
}

func TestC1793_009_TouchedPackagesVetAndRaceGreen(t *testing.T) {
	goRoot := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, pkg := range []string{"./internal/evalqualitycheck", "./internal/evalgate", "./internal/cli/guardcmd"} {
		for _, args := range [][]string{
			{"vet", pkg},
			{"test", "-race", "-count=1", pkg},
		} {
			cmd := exec.Command("go", args...)
			cmd.Dir = goRoot
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("RED: go %s failed: %v\n%s", strings.Join(args, " "), err, out)
				continue
			}
			if args[0] == "test" && strings.Contains(string(out), "no test files") {
				t.Errorf("RED: go %s ran no tests — vacuous:\n%s", strings.Join(args, " "), out)
			}
		}
	}
}

func TestC1793_010_EveryEvalqualitycheckExportIsNamedByATest(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "evalqualitycheck")
	ctx := context.Background()
	syms, err := apicover.Enumerate(ctx, dir)
	if err != nil {
		t.Fatalf("apicover.Enumerate: %v", err)
	}
	named, err := apicover.NamesReferencedInTests(ctx, dir)
	if err != nil {
		t.Fatalf("apicover.NamesReferencedInTests: %v", err)
	}
	report := apicover.Classify(syms, named, map[string]float64{})
	if len(syms) == 0 {
		t.Fatalf("RED: apicover enumerated no exported symbols in %s — vacuous", dir)
	}
	for _, s := range report.Uncovered {
		t.Errorf("RED: exported %s %s is named by no test — the repo-wide ADR-0069 apicover gate rejects the package", s.Kind, s.Name)
	}
}
