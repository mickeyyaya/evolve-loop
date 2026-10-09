package evalqualitycheck

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func unsatisfiableFindings(t *testing.T, src string) []UnsatisfiableFinding {
	t.Helper()
	report, err := LintUnsatisfiablePredicates(writePredicateDir(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if report.Linted() != 1 || report.Files[0] != "predicates_test.go" {
		t.Fatalf("receipt must name the one file parsed; got Linted=%d Files=%v", report.Linted(), report.Files)
	}
	return report.Findings
}

func findingKinds(findings []UnsatisfiableFinding) []string {
	var kinds []string
	for _, f := range findings {
		kinds = append(kinds, f.Kind)
	}
	return kinds
}

func TestAbsenceIntent(t *testing.T) {
	cases := []struct {
		name string
		msgs []string
		want bool
	}{
		{"absence demand", []string{"f.go still holds the duplicated helper"}, true},
		{"missing state with still", []string{"f.go does not mention X — still undocumented"}, false},
		{"expectation with still", []string{"expected the file to still list pkg"}, false},
		{"requirement with must", []string{"f.go still uses the bare form — it must carry the marker"}, false},
		{"plain missing", []string{"newFlag missing from f.go"}, false},
		{"no messages", nil, false},
	}
	for _, c := range cases {
		if got := absenceIntent(c.msgs); got != c.want {
			t.Errorf("%s: absenceIntent(%q) = %v, want %v", c.name, c.msgs, got, c.want)
		}
	}
}

func TestLintUnsatisfiablePredicates_FlagsOnlyUnconditionalSelfReportingShapes(t *testing.T) {
	const header = "package f\n\nimport (\n\t\"testing\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\ntype nopTB struct{}\n\nfunc (nopTB) Helper()               {}\nfunc (nopTB) Errorf(string, ...any) {}\n\n"
	cases := []struct {
		name string
		body string
		want string
	}{
		{"FileExists through t", "func TestX(t *testing.T) {\n\tif acsassert.FileExists(t, \"a\") {\n\t\tt.Errorf(\"a still exists\")\n\t}\n}\n", UnsatisfiableKindInvertedIdiom},
		{"JSONFieldEquals through t", "func TestX(t *testing.T) {\n\tif acsassert.JSONFieldEquals(t, \"a\", \"k\", 1) {\n\t\tt.Fatal(\"k still set\")\n\t}\n}\n", UnsatisfiableKindInvertedIdiom},
		{"subtest t", "func TestX(t *testing.T) {\n\tt.Run(\"s\", func(st *testing.T) {\n\t\tif acsassert.FileContains(st, \"a\", \"x\") {\n\t\t\tst.Errorf(\"x present\")\n\t\t}\n\t})\n}\n", UnsatisfiableKindInvertedIdiom},
		{"testing.TB helper", "func check(tb testing.TB) {\n\tif acsassert.FileMatchesRegex(tb, \"a\", \"x\") {\n\t\ttb.Errorf(\"x present\")\n\t}\n}\n", UnsatisfiableKindInvertedIdiom},
		{"split absence message", "func TestX(t *testing.T) {\n\tif !acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"a still holds \" +\n\t\t\t\"the duplicated helper\")\n\t}\n}\n", UnsatisfiableKindAbsenceMessage},
		{"swallowing TB", "func TestX(t *testing.T) {\n\tif acsassert.FileExists(nopTB{}, \"a\") {\n\t\tt.Errorf(\"a still exists\")\n\t}\n}\n", ""},
		{"swallowing TB held in a variable", "func TestX(t *testing.T) {\n\tprobe := nopTB{}\n\tif acsassert.FileContains(probe, \"a\", \"x\") {\n\t\tt.Errorf(\"x still present\")\n\t}\n}\n", ""},
		{"guard with nested check", "func TestX(t *testing.T) {\n\tif acsassert.FileExists(t, \"a\") {\n\t\tif t.Name() == \"\" {\n\t\t\tt.Errorf(\"unnamed\")\n\t\t}\n\t}\n}\n", ""},
		{"split message negated later", "func TestX(t *testing.T) {\n\tif !acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"a no longer documents x; \" +\n\t\t\t\"it remains a valid surface\")\n\t}\n}\n", ""},
		{"non-acsassert call", "func TestX(t *testing.T) {\n\tif t.Failed() {\n\t\tt.Errorf(\"x still present\")\n\t}\n}\n", ""},
		{"testing name shadowed by a swallowing probe", "func TestX(t *testing.T) {\n\tif true {\n\t\tt := nopTB{}\n\t\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\t\tt.Errorf(\"x still present\")\n\t\t}\n\t}\n}\n", ""},
		{"closure parameter of a probe type", "func TestX(t *testing.T) {\n\tcheck := func(t nopTB) {\n\t\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\t\tt.Errorf(\"x still present\")\n\t\t}\n\t}\n\tcheck(nopTB{})\n}\n", ""},
		{"var-declared probe shadowing the testing name", "func TestX(t *testing.T) {\n\tif true {\n\t\tvar t nopTB\n\t\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\t\tt.Errorf(\"x still present\")\n\t\t}\n\t}\n}\n", ""},
		{"testing name used after a block that shadowed it", "func TestX(t *testing.T) {\n\tif true {\n\t\tt := nopTB{}\n\t\t_ = t\n\t}\n\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"x still present\")\n\t}\n}\n", UnsatisfiableKindInvertedIdiom},
		{"testing name used after a bare block that shadowed it", "func TestX(t *testing.T) {\n\t{\n\t\tt := nopTB{}\n\t\t_ = t\n\t}\n\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"x still present\")\n\t}\n}\n", UnsatisfiableKindInvertedIdiom},
		{"if-init shadows the testing name before the condition is judged", "func TestX(t *testing.T) {\n\tif t := (nopTB{}); acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"x still present\")\n\t}\n}\n", ""},
		{"closure named result of a probe type shadows the testing name", "func TestX(t *testing.T) {\n\tf := func() (t nopTB) {\n\t\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\t\tt.Errorf(\"x still present\")\n\t\t}\n\t\treturn\n\t}\n\t_ = f\n}\n", ""},
		{"testing name captured by a closure still self-reports", "func TestX(t *testing.T) {\n\tcheck := func() {\n\t\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\t\tt.Errorf(\"x present\")\n\t\t}\n\t}\n\tcheck()\n}\n", UnsatisfiableKindInvertedIdiom},
		{"testing.TB var captured by a closure and swapped to a probe before the call", "func TestX(t *testing.T) {\n\tvar tb testing.TB = t\n\tcheck := func() {\n\t\tif acsassert.FileContains(tb, \"a\", \"x\") {\n\t\t\ttb.Errorf(\"x still present\")\n\t\t}\n\t}\n\ttb = nopTB{}\n\tcheck()\n}\n", ""},
		{"testing.TB var initialised with a probe", "func TestX(t *testing.T) {\n\tvar tb testing.TB = nopTB{}\n\tif acsassert.FileContains(tb, \"a\", \"x\") {\n\t\ttb.Errorf(\"x still present\")\n\t}\n}\n", ""},
		{"concrete *testing.T var captured by a closure still self-reports", "func TestX(t *testing.T) {\n\tvar tt *testing.T = t\n\tcheck := func() {\n\t\tif acsassert.FileContains(tt, \"a\", \"x\") {\n\t\t\ttt.Errorf(\"x present\")\n\t\t}\n\t}\n\tcheck()\n}\n", UnsatisfiableKindInvertedIdiom},
		{"testing.TB var initialised from t still self-reports when read directly", "func TestX(t *testing.T) {\n\tvar tb testing.TB = t\n\tif acsassert.FileContains(tb, \"a\", \"x\") {\n\t\ttb.Errorf(\"x present\")\n\t}\n}\n", UnsatisfiableKindInvertedIdiom},
		{"same-named helper outside acsassert", "func TestX(t *testing.T) {\n\tif repoassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"x still present\")\n\t}\n}\n", ""},
		{"absence primitive", "func TestX(t *testing.T) {\n\tif !acsassert.FileNotContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"a still holds x\")\n\t}\n}\n", ""},
		{"presence with a missing-state message", "func TestX(t *testing.T) {\n\tif !acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"x missing from a\")\n\t}\n}\n", ""},
		{"branch that reports no failure", "func TestX(t *testing.T) {\n\tif acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Logf(\"x present\")\n\t}\n}\n", ""},
	}
	for _, c := range cases {
		findings := unsatisfiableFindings(t, header+c.body)
		kinds := findingKinds(findings)
		switch {
		case c.want == "" && len(kinds) != 0:
			t.Errorf("%s: want no finding, got %+v", c.name, findings)
		case c.want != "" && (len(kinds) != 1 || kinds[0] != c.want):
			t.Errorf("%s: want one %s finding, got %+v", c.name, c.want, findings)
		}
	}
}

func TestLintUnsatisfiablePredicates_RemedyMatchesPrimitive(t *testing.T) {
	src := "package f\n\nimport (\n\t\"testing\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\nfunc TestX(t *testing.T) {\n\tif acsassert.FileExists(t, \"a\") {\n\t\tt.Errorf(\"a still exists\")\n\t}\n}\n"
	findings := unsatisfiableFindings(t, src)
	if len(findings) != 1 {
		t.Fatalf("findings=%+v, want one", findings)
	}
	if r := findings[0].Reason; strings.Contains(r, "FileNotContains") || !strings.Contains(r, "fs.ErrNotExist") {
		t.Errorf("a FileExists finding must point at a file-absence remedy, not FileNotContains: %q", r)
	}
}

func TestLintUnsatisfiablePredicates_RefusesAMissingPathAndAnEmptyDirectory(t *testing.T) {
	if _, err := LintUnsatisfiablePredicates(filepath.Join(t.TempDir(), "absent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing path must be a not-exist error, got %v", err)
	}
	if _, err := LintUnsatisfiablePredicates(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a clean result") {
		t.Errorf("a directory with no .go files must be an error that says nothing was linted, got %v", err)
	}
}

func TestPredicateSourcePaths_ListsTopLevelGoFilesInNameOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b_test.go", "a_test.go", "notes.md", filepath.Join("nested", "c_test.go")} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package f\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := predicateSourcePaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "a_test.go"), filepath.Join(dir, "b_test.go")}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("paths = %v, want %v", paths, want)
	}
}

const cycle1488RepairedCall = `acsassert.FileNotContains(t, bindings, "worktreeTree == headTree")`

const cycle1488UnrepairedCall = `acsassert.FileContains(t, bindings, "worktreeTree == headTree")`

const cycle1488Func = "TestC1488_003_AuditBindingPutWiredToSharedPredicate"

func TestLintUnsatisfiablePredicates_FlagsTheLiveCycle1488PredicateOnceUnrepaired(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "acs", "cycle1488", "predicates_test.go"))
	if err != nil {
		t.Fatalf("the live cycle-1488 predicate is unreadable: %v", err)
	}
	live := string(raw)
	if strings.Count(live, "!"+cycle1488RepairedCall) != 1 {
		t.Fatalf("the live cycle-1488 predicate no longer holds !%s exactly once; re-derive this fixture", cycle1488RepairedCall)
	}
	unrepaired := map[string]string{
		UnsatisfiableKindAbsenceMessage: strings.Replace(live, cycle1488RepairedCall, cycle1488UnrepairedCall, 1),
		UnsatisfiableKindInvertedIdiom:  strings.Replace(live, "!"+cycle1488RepairedCall, cycle1488UnrepairedCall, 1),
	}
	for kind, src := range unrepaired {
		var got []string
		for _, f := range unsatisfiableFindings(t, src) {
			if f.Func == cycle1488Func {
				got = append(got, f.Kind)
			}
		}
		if len(got) != 1 || got[0] != kind {
			t.Errorf("the unrepaired cycle-1488 bytes must give %s one %s finding, got %v", cycle1488Func, kind, got)
		}
	}
	if findings := unsatisfiableFindings(t, live); len(findings) != 0 {
		t.Errorf("the repaired live cycle-1488 predicate must be clean, got %+v", findings)
	}
}

func goFilesByDir(t *testing.T, root string) map[string]int {
	t.Helper()
	counts := map[string]int{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			counts[filepath.Dir(path)]++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return counts
}

var unsatisfiableProofKinds = map[string]bool{
	UnsatisfiableKindInvertedIdiom: true,
	UnsatisfiableKindGoRunExitCode: true,
}

func corpusProofFindings(t *testing.T, acsDir string) []string {
	t.Helper()
	var proofs []string
	files, linted := 0, 0
	onDisk := goFilesByDir(t, acsDir)
	for dir, n := range onDisk {
		files += n
		report, err := LintUnsatisfiablePredicates(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		if report.Linted() != n {
			t.Errorf("%s: linted %d of %d .go files", dir, report.Linted(), n)
		}
		linted += report.Linted()
		for _, f := range report.Findings {
			line := fmt.Sprintf("%s/%s:%s [%s] %s", dir, f.File, f.Func, f.Kind, f.Reason)
			if unsatisfiableProofKinds[f.Kind] {
				proofs = append(proofs, line)
				continue
			}
			t.Logf("advisory %s finding, not a proof: %s", f.Kind, line)
		}
	}
	t.Logf("swept %d predicate packages: %d of %d .go files linted", len(onDisk), linted, files)
	return proofs
}

func TestCorpusProofFindings_ReturnsProofKindsAndOnlyLogsAnAbsenceMessage(t *testing.T) {
	corpus := t.TempDir()
	header := "package f\n\nimport (\n\t\"testing\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\n"
	for pkg, body := range map[string]string{
		"heuristic": "func TestHeuristic(t *testing.T) {\n\tif !acsassert.FileContains(t, \"a\", \"x\") {\n\t\tt.Errorf(\"a still holds the duplicated helper\")\n\t}\n}\n",
		"proof":     "func TestProof(t *testing.T) {\n\tif acsassert.FileExists(t, \"a\") {\n\t\tt.Errorf(\"a still exists\")\n\t}\n}\n",
	} {
		if err := os.MkdirAll(filepath.Join(corpus, pkg), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(corpus, pkg, "predicates_test.go"), []byte(header+body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	proofs := corpusProofFindings(t, corpus)
	if len(proofs) != 1 || !strings.Contains(proofs[0], "TestProof [inverted-idiom]") {
		t.Errorf("only the inverted-idiom finding is a proof; the absence-message one is logged, not returned: %q", proofs)
	}
}

func TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding(t *testing.T) {
	acsDir := filepath.Join("..", "..", "acs")
	if onDisk := goFilesByDir(t, acsDir); onDisk[filepath.Join(acsDir, "regression", "docgo")] == 0 {
		t.Fatalf("the walk must reach the nested regression packages; found %d package dirs", len(onDisk))
	}
	for _, proof := range corpusProofFindings(t, acsDir) {
		t.Errorf("%s: a proof kind (inverted-idiom or go-run-exit-code) is red on every tree; rewrite the predicate with the remedy named", proof)
	}
}

const goRunHeader = "package f\n\nimport (\n\t\"context\"\n\t\"errors\"\n\t\"os/exec\"\n\t\"testing\"\n)\n\n"

const cycle1788ExitCodeShape = goRunHeader + `func TestC1788_008_PreflightNeverLowersATautologyHalt(t *testing.T) {
	evalPath := filepath.Join(t.TempDir(), "eval.md")
	if err := os.WriteFile(evalPath, []byte("tautology eval"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/evolve", "eval", "quality-check", evalPath, "-predicates", writeFixtureDir(t, invertedIdiomSource))
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Errorf("RED: Level-0 tautology must stay HALT (rc=2) with an advisory finding present; err=%v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "unsatisfiable[") {
		t.Errorf("RED: unsatisfiable lint must still report alongside the HALT:\n%s", out)
	}
}
`

func TestLintUnsatisfiablePredicates_FlagsCycle1788ExitCodeThroughGoRun(t *testing.T) {
	findings := unsatisfiableFindings(t, cycle1788ExitCodeShape)
	if len(findings) != 1 || findings[0].Kind != UnsatisfiableKindGoRunExitCode || findings[0].Func != "TestC1788_008_PreflightNeverLowersATautologyHalt" {
		t.Fatalf("want one %s finding on TestC1788_008, got %+v", UnsatisfiableKindGoRunExitCode, findings)
	}
	if r := findings[0].Reason; !strings.Contains(r, "exit code 2 never reaches ExitCode()") || !strings.Contains(r, "go build") {
		t.Errorf("the reason must name the impossible code and the built-binary remedy: %q", r)
	}
}

func TestLintUnsatisfiablePredicates_GoRunExitCodeRuleSparesReachableCodesAndBuiltBinaries(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"CommandContext compared to 3", "func TestX(t *testing.T) {\n\terr := exec.CommandContext(context.Background(), \"go\", \"run\", \".\").Run()\n\tvar ee *exec.ExitError\n\tif errors.As(err, &ee) && ee.ExitCode() == 3 {\n\t\treturn\n\t}\n\tt.Fatal(err)\n}\n", true},
		{"literal on the left", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tif 2 != cmd.ProcessState.ExitCode() {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", true},
		{"compared to 1", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 1 {\n\t\tt.Fatal(\"want 1\")\n\t}\n}\n", false},
		{"compared to 0", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 0 {\n\t\tt.Fatal(\"want 0\")\n\t}\n}\n", false},
		{"built binary compared to 2", "func TestX(t *testing.T) {\n\tcmd := exec.Command(t.TempDir()+\"/evolve\", \"eval\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"go build then the built binary", "func TestX(t *testing.T) {\n\tbin := t.TempDir() + \"/evolve\"\n\t_ = exec.Command(\"go\", \"build\", \"-o\", bin, \".\").Run()\n\tcmd := exec.Command(bin, \"eval\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"go run result checked beside a built binary", "func TestX(t *testing.T) {\n\tgoRun := exec.Command(\"go\", \"run\", \".\")\n\t_ = goRun.Run()\n\t_ = exec.Command(t.TempDir()+\"/evolve\").Run()\n\tif goRun.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", true},
		{"ExitCode of a value unrelated to go run", "func TestX(t *testing.T) {\n\t_ = exec.Command(\"go\", \"run\", \".\").Run()\n\tr := result{}\n\tif r.ExitCode() != 7 {\n\t\tt.Fatal(\"want 7\")\n\t}\n}\n", false},
		{"go run error rebound before the check", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \".\").Run()\n\terr = os.Remove(\"x\")\n\tvar ee *exec.ExitError\n\tif errors.As(err, &ee) && ee.ExitCode() == 2 {\n\t\treturn\n\t}\n\tt.Fatal(err)\n}\n", false},
		{"range variable shadows the go run error", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \"prog.go\")\n\terr := cmd.Run()\n\tfor _, err := range []error{io.EOF} {\n\t\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\t_ = err\n}\n", false},
		{"closure parameter shadows the go run error", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tcheck := func(err error) {\n\t\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\tcheck(err)\n}\n", false},
		{"type-switch variable shadows inside its clause only", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tswitch err := other().(type) {\n\tcase error:\n\t\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t}\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"select case variable shadows inside its own clause only", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tselect {\n\tcase err := <-errs:\n\t\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\tdefault:\n\t\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t}\n}\n", true},
		{"go flags before the package can exit 2", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \"-race\", \"./prog\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"go flags before the package never exit 3", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \"-race\", \"./prog\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n", true},
		{"a package named by a variable may be a go flag", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", pkg)\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"go vet result without go flags is outside the go run rule", "func TestX(t *testing.T) {\n\tvet := exec.Command(\"go\", \"vet\", \"./prog\")\n\t_ = vet.Run()\n\tif vet.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"go vet result is outside the go run rule", "func TestX(t *testing.T) {\n\tvet := exec.Command(\"go\", \"vet\", \"-nosuchflag\", \".\")\n\t_ = vet.Run()\n\tif vet.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want usage exit 2\")\n\t}\n}\n", false},
		{"closure named result shadows the go run error", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tcheck := func() (err error) {\n\t\tvar ee *exec.ExitError\n\t\tif errors.As(err, &ee) && ee.ExitCode() == 3 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t\treturn\n\t}\n\t_, _ = check, err\n}\n", false},
		{"range with = rebinds the go run command", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \"prog.go\")\n\tfor _, cmd = range bins {\n\t\t_ = cmd.Run()\n\t\tif cmd.ProcessState.ExitCode() != 3 {\n\t\t\tt.Fatal(\"want 3\")\n\t\t}\n\t}\n}\n", false},
		{"errors.As in a guard binds the outer target", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tvar ee *exec.ExitError\n\tif !errors.As(err, &ee) {\n\t\tt.Fatal(err)\n\t}\n\tif ee.ExitCode() != 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"closure parameter does not leak past the closure", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tcheck := func(err error) {}\n\tcheck(nil)\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"range variable does not leak past the loop", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tfor _, err := range errs {\n\t\t_ = err\n\t}\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"case-clause declaration does not leak into the next clause", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tswitch {\n\tcase true:\n\t\terr := other()\n\t\t_ = err\n\tdefault:\n\t\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t}\n}\n", true},
		{"assignment in a nested scope rebinds the outer name", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tif cond {\n\t\terr = other()\n\t}\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", false},
		{"for-init declaration does not leak past the loop", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tfor err := other(); err != nil; err = other() {\n\t}\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"if-init declaration does not leak past the if", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tif err := other(); err != nil {\n\t}\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"switch-init declaration does not leak past the switch", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tswitch err := other(); err {\n\t}\n\tif ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 3 {\n\t\tt.Fatal(ee)\n\t}\n}\n", true},
		{"assignment to a package-level name binds it", "func TestX(t *testing.T) {\n\tcmd = exec.Command(\"go\", \"run\", \"prog.go\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n", true},
		{"if-init that re-declares its own source is judged once", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \"prog.go\").Run()\n\tif err, ok := err.(*exec.ExitError); ok && err.ExitCode() == 3 {\n\t\tt.Fatal(err)\n\t}\n}\n", true},
		{"closure write of a go run result leaves the outer name unknown", "func TestX(t *testing.T) {\n\terr := exec.Command(\"./built\").Run()\n\tlater := func() { err = exec.Command(\"go\", \"run\", \"./p\").Run() }\n\tvar ee *exec.ExitError\n\tif errors.As(err, &ee) && ee.ExitCode() == 3 {\n\t\tt.Fatal(\"x\")\n\t}\n\tlater()\n}\n", false},
		{"deferred closure errors.As target leaves the outer name unknown", "func TestX(t *testing.T) {\n\tgoErr := exec.Command(\"go\", \"run\", \"./p\").Run()\n\tvar ee *exec.ExitError\n\tbuiltErr := exec.Command(\"./built\").Run()\n\terrors.As(builtErr, &ee)\n\tdefer func() { errors.As(goErr, &ee) }()\n\tif ee.ExitCode() == 3 {\n\t\tt.Fatal(\"x\")\n\t}\n}\n", false},
		{"closure reading an outer go run result sees an unknown value", "func TestX(t *testing.T) {\n\tgoErr := exec.Command(\"go\", \"run\", \"./p\").Run()\n\tvar ee *exec.ExitError\n\terrors.As(goErr, &ee)\n\tcheck := func() {\n\t\tif ee.ExitCode() == 3 {\n\t\t\tt.Fatal(\"x\")\n\t\t}\n\t}\n\terrors.As(exec.Command(\"./built\").Run(), &ee)\n\tcheck()\n}\n", false},
		{"closure assigning its own local name still tracks it", "func TestX(t *testing.T) {\n\tcheck := func() {\n\t\tvar ee *exec.ExitError\n\t\terr := exec.Command(\"go\", \"run\", \"./p\").Run()\n\t\tif errors.As(err, &ee) && ee.ExitCode() == 3 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t}\n\tcheck()\n}\n", true},
		{"go run result beside a house-helper subprocess", "func TestX(t *testing.T) {\n\t_, _, _, _ = acsassert.SubprocessOutput(\"git\", \"status\")\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", true},
		{"go run beside a built binary", "func TestX(t *testing.T) {\n\t_ = exec.Command(\"go\", \"run\", \"./gen\").Run()\n\tcmd := exec.Command(t.TempDir()+\"/evolve\", \"eval\")\n\t_ = cmd.Run()\n\tif cmd.ProcessState.ExitCode() != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
	}
	for _, c := range cases {
		findings := unsatisfiableFindings(t, goRunHeader+c.body)
		flagged := len(findings) == 1 && findings[0].Kind == UnsatisfiableKindGoRunExitCode
		if flagged != c.want || (!c.want && len(findings) != 0) {
			t.Errorf("%s: want flagged=%v, got %+v", c.name, c.want, findings)
		}
	}
}

func TestLintUnsatisfiablePredicates_GoRunExitCodeRuleFollowsExitCodeVariables(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"ExitError code copied into a variable compared to 3", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \".\").Run()\n\tvar ee *exec.ExitError\n\tif !errors.As(err, &ee) {\n\t\tt.Fatal(err)\n\t}\n\tcode := ee.ExitCode()\n\tif code != 3 {\n\t\tt.Fatalf(\"want 3, got %d\", code)\n\t}\n}\n", true},
		{"ProcessState code copied into a variable compared to 2", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tcode := cmd.ProcessState.ExitCode()\n\tif code != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", true},
		{"var declaration of the ExitError code", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \".\").Run()\n\tif ee, ok := err.(*exec.ExitError); ok {\n\t\tvar code = ee.ExitCode()\n\t\tif code == 3 {\n\t\t\treturn\n\t\t}\n\t}\n\tt.Fatal(err)\n}\n", true},
		{"copied code with the literal on the left", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tcode := cmd.ProcessState.ExitCode()\n\tif 3 != code {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n", true},
		{"SubprocessOutput go run code compared to 3", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \"./cmd/evolve\", \"eval\")\n\tif code != 3 {\n\t\tt.Fatalf(\"want 3, got %d\", code)\n\t}\n}\n", true},
		{"SubprocessOutput go run code compared to 2 without go flags", "func TestX(t *testing.T) {\n\tstdout, stderr, code, err := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif code != 2 {\n\t\tt.Fatal(stdout, stderr, err)\n\t}\n}\n", true},
		{"SubprocessOutput go run with go flags compared to 3", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \"-race\", \".\")\n\tif code == 3 {\n\t\treturn\n\t}\n\tt.Fatal(\"want 3\")\n}\n", true},
		{"SubprocessOutput go run code assigned with =", "func TestX(t *testing.T) {\n\tvar code int\n\t_, _, code, _ = acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif code != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n", true},
		{"copied code compared to 1", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \".\")\n\t_ = cmd.Run()\n\tcode := cmd.ProcessState.ExitCode()\n\tif code != 1 {\n\t\tt.Fatal(\"want 1\")\n\t}\n}\n", false},
		{"copied code compared to 0", "func TestX(t *testing.T) {\n\terr := exec.Command(\"go\", \"run\", \".\").Run()\n\tvar ee *exec.ExitError\n\tif errors.As(err, &ee) {\n\t\tcode := ee.ExitCode()\n\t\tif code == 0 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t}\n}\n", false},
		{"SubprocessOutput go run code compared to 1", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif code != 1 {\n\t\tt.Fatal(\"want 1\")\n\t}\n}\n", false},
		{"SubprocessOutput go run code compared to 0", "func TestX(t *testing.T) {\n\t_, _, code, err := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif code != 0 || err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n", false},
		{"SubprocessOutput go run with go flags compared to 2", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \"-race\", \".\")\n\tif code != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"copied code with go flags compared to 2", "func TestX(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"run\", \"-race\", \"./prog\")\n\t_ = cmd.Run()\n\tcode := cmd.ProcessState.ExitCode()\n\tif code != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"SubprocessOutput built binary code compared to 2", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(t.TempDir()+\"/evolve\", \"eval\")\n\tif code != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"built binary code copied into a variable compared to 2", "func TestX(t *testing.T) {\n\tcmd := exec.Command(t.TempDir()+\"/evolve\", \"eval\")\n\t_ = cmd.Run()\n\tcode := cmd.ProcessState.ExitCode()\n\tif code != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"built binary ExitError code copied into a variable compared to 3", "func TestX(t *testing.T) {\n\terr := exec.Command(\"./built\").Run()\n\tvar ee *exec.ExitError\n\tif errors.As(err, &ee) {\n\t\tcode := ee.ExitCode()\n\t\tif code != 3 {\n\t\t\tt.Fatal(ee)\n\t\t}\n\t}\n}\n", false},
		{"SubprocessOutput go vet code is outside the go run rule", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"vet\", \"./prog\")\n\tif code != 2 {\n\t\tt.Fatal(\"want 2\")\n\t}\n}\n", false},
		{"go run code rebound before the check", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tcode = other()\n\tif code != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n", false},
		{"inner declaration shadows the go run code", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif cond {\n\t\tcode := other()\n\t\tif code != 3 {\n\t\t\tt.Fatal(\"want 3\")\n\t\t}\n\t}\n\t_ = code\n}\n", false},
		{"closure reading the outer go run code sees an unknown value", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tcheck := func() {\n\t\tif code != 3 {\n\t\t\tt.Fatal(\"want 3\")\n\t\t}\n\t}\n\tcheck()\n}\n", false},
		{"go run code variable never compared to a literal", "func TestX(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif code != want {\n\t\tt.Fatal(\"mismatch\")\n\t}\n}\n", false},
		{"go run stdout is not an exit code", "func TestX(t *testing.T) {\n\tcode, _, _, _ := acsassert.SubprocessOutput(\"go\", \"run\", \".\")\n\tif code != \"3\" {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n", false},
	}
	for _, c := range cases {
		findings := unsatisfiableFindings(t, goRunHeader+c.body)
		flagged := len(findings) == 1 && findings[0].Kind == UnsatisfiableKindGoRunExitCode
		if flagged != c.want || (!c.want && len(findings) != 0) {
			t.Errorf("%s: want flagged=%v, got %+v", c.name, c.want, findings)
		}
	}
}

func TestLintUnsatisfiablePredicates_GoRunExitCodeVariableReasonNamesTheCodeAndRemedy(t *testing.T) {
	body := "func TestC9_001_CopiedCode(t *testing.T) {\n\t_, _, code, _ := acsassert.SubprocessOutput(\"go\", \"run\", \"./cmd/evolve\")\n\tif code != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n"
	findings := unsatisfiableFindings(t, goRunHeader+body)
	if len(findings) != 1 || findings[0].Func != "TestC9_001_CopiedCode" {
		t.Fatalf("want one finding on TestC9_001_CopiedCode, got %+v", findings)
	}
	if r := findings[0].Reason; !strings.Contains(r, "exit code 3") || !strings.Contains(r, "go build") {
		t.Errorf("the reason must name the impossible code and the built-binary remedy: %q", r)
	}
}
