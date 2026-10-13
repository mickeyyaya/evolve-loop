package evalqualitycheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFingerprintEval_UnopenableAndOverlongFilesAreErrorsNotSilentSkips(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		ed, has, err := fingerprintEval(filepath.Join(t.TempDir(), "absent.md"))
		if err == nil || !strings.Contains(err.Error(), "evalqualitycheck: open ") || has || ed.Path != "" {
			t.Fatalf("ed=%+v has=%v err=%v", ed, has, err)
		}
	})
	t.Run("a single line beyond the scanner limit", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "huge.md")
		if err := os.WriteFile(path, []byte("```bash\n"+strings.Repeat("x", 128*1024)+"\n```\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, has, err := fingerprintEval(path)
		if err == nil || !strings.Contains(err.Error(), "evalqualitycheck: scan ") || has {
			t.Fatalf("has=%v err=%v", has, err)
		}
	})
}

func TestCheckDiversity_ADanglingEvalLinkStopsTheScoreWithTheCause(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(fence(`! grep -q x f`)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "absent-target.md"), filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}

	res, err := CheckDiversity(DiversityOptions{EvalDir: dir})

	if err == nil || !strings.Contains(err.Error(), "b.md") || res.EvalCount != 0 {
		t.Fatalf("res=%+v err=%v: a suite that cannot be read must not be scored as a smaller suite", res, err)
	}
}

func TestPredicateSourcePaths_Shapes(t *testing.T) {
	t.Run("directory that cannot be listed", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "preds")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(sub, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })
		if _, err := os.ReadDir(sub); err == nil {
			t.Skip("directory permissions are not enforced for this user")
		}
		paths, err := predicateSourcePaths(sub)
		if err == nil || paths != nil {
			t.Fatalf("paths=%v err=%v", paths, err)
		}
	})
	t.Run("a single file is its own source", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "p.go")
		if err := os.WriteFile(f, []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if paths, err := predicateSourcePaths(f); err != nil || len(paths) != 1 || paths[0] != f {
			t.Fatalf("paths=%v err=%v", paths, err)
		}
	})
	t.Run("a directory without Go files is not a clean result", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := predicateSourcePaths(dir); err == nil || !strings.Contains(err.Error(), "not a clean result") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestLintUnsatisfiablePredicates_UnparseableSourceIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad_test.go"), []byte("package p\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := LintUnsatisfiablePredicates(dir)

	if err == nil || len(report.Findings) != 0 {
		t.Fatalf("report=%+v err=%v: unparseable predicates are not a clean lint", report, err)
	}
}

func TestFailureReporterCall_OnlyAFailureReportingCallStatementQualifies(t *testing.T) {
	src := `package p
func f(t T, ch chan int) {
	t.Errorf("bad")
	t.Log("note")
	<-ch
	x := 1
	_ = x
	helper()
	t.Fatal("stop")
}`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	body := file.Decls[0].(*ast.FuncDecl).Body.List
	want := []bool{true, false, false, false, false, false, true}
	if len(body) != len(want) {
		t.Fatalf("fixture has %d statements", len(body))
	}
	for i, stmt := range body {
		if got := failureReporterCall(stmt) != nil; got != want[i] {
			t.Errorf("statement %d: failureReporterCall != nil is %v, want %v", i, got, want[i])
		}
	}
}
