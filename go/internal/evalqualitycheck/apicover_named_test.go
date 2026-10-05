package evalqualitycheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck_BindsResultAndClassifiedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eval.md")
	if err := os.WriteFile(path, []byte("```bash\n:\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var res Result
	res, err := Check(Options{Path: path})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Path != path {
		t.Errorf("Result.Path = %q, want %q", res.Path, path)
	}
	if res.Overall != LevelHalt {
		t.Errorf("Result.Overall = %v, want LevelHalt (the `:` tautology)", res.Overall)
	}
	if len(res.Commands) != 1 {
		t.Fatalf("Result.Commands = %d entries, want 1", len(res.Commands))
	}
	want := ClassifiedLine{Line: ":", Level: LevelHalt, Reason: "always-pass tautology"}
	if res.Commands[0] != want {
		t.Errorf("Commands[0] = %+v, want %+v", res.Commands[0], want)
	}
}

func TestCheckDiversity_BindsDiversityStructs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pos.md"),
		[]byte("```bash\ngrep -q \"Feature ready\" src/f.txt\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "neg.md"),
		[]byte("```bash\n! grep -q \"removed\" src/f.txt\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var res DiversityResult
	res, err := CheckDiversity(DiversityOptions{EvalDir: dir})
	if err != nil {
		t.Fatalf("CheckDiversity: %v", err)
	}
	if res.EvalDir != dir {
		t.Errorf("DiversityResult.EvalDir = %q, want %q", res.EvalDir, dir)
	}
	if res.EvalCount != 2 {
		t.Errorf("EvalCount = %d, want 2", res.EvalCount)
	}
	if res.NegativeCaseCount != 1 {
		t.Errorf("NegativeCaseCount = %d, want 1", res.NegativeCaseCount)
	}
	if res.Level != DiversityPass {
		t.Errorf("Level = %v, want DiversityPass (one negative case present)", res.Level)
	}
	if len(res.Files) != 2 {
		t.Fatalf("Files = %d, want 2 per-file fingerprints", len(res.Files))
	}
	negFiles := 0
	for _, f := range res.Files {
		var _ EvalDiversity = f // bind the per-file struct type
		if f.HasNegative {
			negFiles++
		}
	}
	if negFiles != 1 {
		t.Errorf("EvalDiversity.HasNegative set on %d files, want 1", negFiles)
	}
}

func TestLintUnsatisfiablePredicates_BindsReportAndFinding(t *testing.T) {
	dir := t.TempDir()
	src := "package f\n\nimport (\n\t\"testing\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\nfunc TestX(t *testing.T) {\n\tif acsassert.FileContains(t, \"f.go\", \"x\") {\n\t\tt.Errorf(\"x present\")\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var report UnsatisfiableLintReport
	report, err := LintUnsatisfiablePredicates(dir)
	if err != nil {
		t.Fatalf("LintUnsatisfiablePredicates: %v", err)
	}
	if report.Linted() != 1 || len(report.Findings) != 1 {
		t.Fatalf("report = %+v, want 1 file and 1 finding", report)
	}
	var f UnsatisfiableFinding = report.Findings[0]
	if f.Func != "TestX" || f.Kind != UnsatisfiableKindInvertedIdiom || f.File != "p_test.go" {
		t.Errorf("finding = %+v", f)
	}
	if UnsatisfiableKindAbsenceMessage == UnsatisfiableKindInvertedIdiom {
		t.Errorf("finding kinds must be distinct")
	}
}
