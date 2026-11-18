package evalqualitycheck

import (
	"os"
	"path/filepath"
	"testing"
)

const stHeader = "package p\n\nimport (\n\t\"testing\"\n\texe \"os/exec\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n\nvar _ = exe.Command\n\n"

func lintSrc(t *testing.T, src string) (SourceTextLintReport, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return LintSourceTextPredicates(dir)
}

func TestLintSourceTextPredicates_Shapes(t *testing.T) {
	cases := map[string]struct {
		src  string
		want int
	}{
		"aliased exec import runs code": {stHeader + "func TestA(t *testing.T) {\n\tacsassert.FileContains(t, \"a\", \"b\")\n\t_ = exe.Command(\"x\")\n}\n", 0},
		"helper that runs code":         {stHeader + "func run() { _ = exe.Command(\"x\") }\n\nfunc TestA(t *testing.T) {\n\tacsassert.FileContains(t, \"a\", \"b\")\n\trun()\n}\n", 0},
		"helper that only greps":        {stHeader + "func grep(t *testing.T) { acsassert.FileMatchesRegex(t, \"a\", \"b\") }\n\nfunc TestA(t *testing.T) {\n\tgrep(t)\n\tacsassert.FileContains(t, \"a\", \"b\")\n}\n", 1},
		"recursive helper":              {stHeader + "func loop(t *testing.T) { loop(t) }\n\nfunc TestA(t *testing.T) {\n\tacsassert.FileContains(t, \"a\", \"b\")\n\tloop(t)\n}\n", 1},
		"waived config check":           {stHeader + "// acs-predicate: config-check\nfunc TestA(t *testing.T) {\n\tacsassert.FileContains(t, \"a\", \"b\")\n}\n", 0},
		"no source text":                {stHeader + "func TestA(t *testing.T) {\n\tt.Log(\"x\")\n}\n", 0},
		"bad import path":               {"package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { t.Log(1) }\n", 0},
	}
	for name, c := range cases {
		report, err := lintSrc(t, c.src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(report.Findings) != c.want {
			t.Errorf("%s: findings = %+v, want %d", name, report.Findings, c.want)
		}
	}
}

func TestLintSourceTextPredicates_Errors(t *testing.T) {
	if _, err := LintSourceTextPredicates(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing path: want error")
	}
	if _, err := lintSrc(t, "package p\nfunc ("); err == nil {
		t.Error("unparsable source: want error")
	}
}
