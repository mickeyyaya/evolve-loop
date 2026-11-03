//go:build acs

package emptytests

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	goDir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(goDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return goDir
}

func offenderStrings(offenders []offender) []string {
	out := make([]string, 0, len(offenders))
	for _, o := range offenders {
		out = append(out, o.String())
	}
	return out
}

func TestNoNewTestFunctionHasAnEmptyBody(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	offenders, err := findOffenders(goDir)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := readBaseline("baseline.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations(offenders, baseline) {
		t.Error(violation)
	}
}

func TestFindOffenders_AnEmptyBodyAndACommentOnlyBodyFailWithFileLineAndName(t *testing.T) {
	goDir := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n\nfunc TestCommentOnly(t *testing.T) {\n\t// nothing here\n}\n",
	})
	offenders, err := findOffenders(goDir)
	if err != nil {
		t.Fatal(err)
	}
	want := []offender{{"a/a_test.go", 5, "TestX"}, {"a/a_test.go", 7, "TestCommentOnly"}}
	if !slices.Equal(offenders, want) {
		t.Fatalf("findOffenders = %v, want %v", offenders, want)
	}
	got := offenderStrings(offenders)
	for i, w := range []string{"a/a_test.go:5: TestX", "a/a_test.go:7: TestCommentOnly"} {
		if !strings.HasPrefix(got[i], w) || !strings.Contains(got[i], "empty body") {
			t.Errorf("message %q, want the prefix %q and the words empty body", got[i], w)
		}
	}
}

func TestFindOffenders_ASkipWithAReasonAndARealBodyPass(t *testing.T) {
	goDir := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestSkipped(t *testing.T) {\n\tt.Skip(\"the fixture host is gone\")\n}\n\nfunc TestReal(t *testing.T) {\n\tif 1 != 1 {\n\t\tt.Fatal(\"math\")\n\t}\n}\n",
	})
	offenders, err := findOffenders(goDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) != 0 {
		t.Fatalf("findOffenders = %v, want none", offenders)
	}
}

func TestFindOffenders_FuzzAndBenchmarkCountAndOtherFunctionsDoNot(t *testing.T) {
	goDir := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc FuzzF(f *testing.F) {}\n\nfunc BenchmarkB(b *testing.B) {}\n\nfunc Test(t *testing.T) {}\n\nfunc Testify() {}\n\nfunc helper() {}\n\ntype s struct{}\n\nfunc (s) TestMethod(t *testing.T) {}\n\nconst fixture = `\nfunc TestInString(t *testing.T) {}\n`\n",
		"a/prod.go":   "package a\n\nfunc TestInProduction() {}\n",
	})
	offenders, err := findOffenders(goDir)
	if err != nil {
		t.Fatal(err)
	}
	want := []offender{{"a/a_test.go", 5, "FuzzF"}, {"a/a_test.go", 7, "BenchmarkB"}, {"a/a_test.go", 9, "Test"}}
	if !slices.Equal(offenders, want) {
		t.Fatalf("findOffenders = %v, want %v", offenders, want)
	}
}

func TestFindOffenders_ABodylessDeclarationIsNotAnOffender(t *testing.T) {
	goDir := writeTree(t, map[string]string{"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestAsm(t *testing.T)\n"})
	offenders, err := findOffenders(goDir)
	if err != nil || len(offenders) != 0 {
		t.Fatalf("findOffenders = %v, %v, want no offender and no error", offenders, err)
	}
}

func TestFindOffenders_SkipsTestdataVendorAndHiddenDirectories(t *testing.T) {
	empty := "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"
	goDir := writeTree(t, map[string]string{
		"a/testdata/x_test.go": empty,
		"vendor/v/x_test.go":   empty,
		".hidden/x_test.go":    empty,
		"_scratch/x_test.go":   empty,
		"b/b_test.go":          empty,
	})
	offenders, err := findOffenders(goDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []offender{{"b/b_test.go", 5, "TestX"}}; !slices.Equal(offenders, want) {
		t.Fatalf("findOffenders = %v, want %v", offenders, want)
	}
}

func TestFindOffenders_AScanThatCannotDoItsWorkIsAnError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		goDir func(t *testing.T) string
		want  string
	}{
		{"a missing module", func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent") }, "WalkModule"},
		{"no test file", func(t *testing.T) string { return writeTree(t, map[string]string{"a/a.go": "package a\n"}) }, "no test file"},
		{"a file that does not parse", func(t *testing.T) string {
			return writeTree(t, map[string]string{"a/a_test.go": "package a\n\nfunc TestX(t *testing.T) {\n"})
		}, "a_test.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := findOffenders(tc.goDir(t))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("findOffenders error = %v, want one that names %q", err, tc.want)
			}
		})
	}
}

func TestReadBaseline_ReadsOneKeyPerLineAndSkipsBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.txt")
	if err := os.WriteFile(path, []byte("a/a_test.go TestX\n\n  b/b_test.go TestY  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got["a/a_test.go TestX"] || !got["b/b_test.go TestY"] {
		t.Fatalf("readBaseline = %v, want the two keys", got)
	}
}

func TestReadBaseline_AMissingFileIsAnError(t *testing.T) {
	if _, err := readBaseline(filepath.Join(t.TempDir(), "absent.txt")); err == nil {
		t.Fatal("readBaseline of a missing file = nil error, want an error")
	}
}

func TestViolations_ANewOffenderFailsAndABaselineOffenderPasses(t *testing.T) {
	offenders := []offender{{"a/a_test.go", 5, "TestOld"}, {"b/b_test.go", 9, "TestNew"}}
	got := violations(offenders, map[string]bool{"a/a_test.go TestOld": true})
	if len(got) != 1 || !strings.HasPrefix(got[0], "b/b_test.go:9: TestNew") {
		t.Fatalf("violations = %v, want only the new offender TestNew", got)
	}
}

func TestViolations_ABaselineEntryThatMatchesNoOffenderFails(t *testing.T) {
	got := violations(nil, map[string]bool{"a/a_test.go TestFixed": true})
	if len(got) != 1 || !strings.Contains(got[0], `"a/a_test.go TestFixed"`) || !strings.Contains(got[0], "remove it") {
		t.Fatalf("violations = %v, want one stale-entry violation that names the entry", got)
	}
}
