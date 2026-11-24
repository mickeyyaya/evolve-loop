package emptytests

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const header = "package p\n\nimport \"testing\"\n\n"

func layTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, text := range files {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scanFiles(t *testing.T, files map[string]string) []Offender {
	t.Helper()
	found, err := Scan(layTree(t, files))
	if err != nil {
		t.Fatalf("Scan = error %v", err)
	}
	return found
}

func TestScan_FlagsAnEmptyTestBody(t *testing.T) {
	t.Parallel()
	got := scanFiles(t, map[string]string{"a/a_test.go": header + "func TestX(t *testing.T) {}\n"})
	want := []Offender{{File: "a/a_test.go", Line: 5, Func: "TestX"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}

func TestScan_FlagsABodyOfOnlyAComment(t *testing.T) {
	t.Parallel()
	got := scanFiles(t, map[string]string{"a_test.go": header + "func TestX(t *testing.T) {\n\t// todo\n}\n"})
	if len(got) != 1 || got[0].Func != "TestX" {
		t.Errorf("Scan = %+v, want one offender TestX", got)
	}
}

func TestScan_FlagsEmptyFuzzAndBenchmark(t *testing.T) {
	t.Parallel()
	src := header + "func FuzzZ(f *testing.F) {}\n\nfunc BenchmarkB(b *testing.B) {}\n"
	got := scanFiles(t, map[string]string{"a_test.go": src})
	want := []Offender{{File: "a_test.go", Line: 5, Func: "FuzzZ"}, {File: "a_test.go", Line: 7, Func: "BenchmarkB"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan = %+v, want %+v", got, want)
	}
}

func TestScan_FlagsABareAndAliasedForm(t *testing.T) {
	t.Parallel()
	got := scanFiles(t, map[string]string{
		"bare_test.go":  header + "func Test(t *testing.T) {}\n\nfunc Test_under(t *testing.T) {}\n",
		"alias_test.go": "package p\n\nimport tt \"testing\"\n\nfunc TestA(t *tt.T) {}\n",
	})
	var names []string
	for _, o := range got {
		names = append(names, o.Func)
	}
	want := []string{"TestA", "Test", "Test_under"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("offenders = %v, want %v", names, want)
	}
}

func TestScan_FlagsARawStringTestingImport(t *testing.T) {
	t.Parallel()
	got := scanFiles(t, map[string]string{"a_test.go": "package p\n\nimport `testing`\n\nfunc TestX(t *testing.T) {}\n"})
	if len(got) != 1 {
		t.Errorf("Scan = %+v, want one offender", got)
	}
}

func TestScan_PassesASkipWithAReason(t *testing.T) {
	t.Parallel()
	got := scanFiles(t, map[string]string{"a_test.go": header + "func TestX(t *testing.T) {\n\tt.Skip(\"needs tmux\")\n}\n"})
	if len(got) != 0 {
		t.Errorf("Scan = %+v, want none: a skip with a reason is a statement", got)
	}
}

func TestScan_IgnoresNonTests(t *testing.T) {
	t.Parallel()
	src := "package p\n\nimport (\n\t\"testing\"\n\n\tfoo \"example.com/foo\"\n)\n\ntype S struct{}\n\n" +
		"func helper(t *testing.T) {}\n\n" +
		"func Testing(t *testing.T) {}\n\n" +
		"func (s S) TestM(t *testing.T) {}\n\n" +
		"func TestN(x int) {}\n\n" +
		"func TestNone() {}\n\n" +
		"func TestTwo(t *testing.T, x int) {}\n\n" +
		"func TestPair(a, b *testing.T) {}\n\n" +
		"func TestMain(m *testing.M) {}\n\n" +
		"func TestWrongKind(b *testing.B) {}\n\n" +
		"func TestValue(t testing.T) {}\n\n" +
		"func TestBareT(t *T) {}\n\n" +
		"func TestForeign(t *foo.T) {}\n\n" +
		"func TestDecl(t *testing.T)\n"
	got := scanFiles(t, map[string]string{"a_test.go": src, "other_test.go": "package p\n\nfunc TestNoImport(t *testing.T) {}\n"})
	if len(got) != 0 {
		t.Errorf("Scan = %+v, want none", got)
	}
}

func TestScan_SkipsTestdataVendorDotAndUnderscoreDirs(t *testing.T) {
	t.Parallel()
	empty := header + "func TestX(t *testing.T) {}\n"
	got := scanFiles(t, map[string]string{
		"testdata/a_test.go":   empty,
		"vendor/a_test.go":     empty,
		".x/a_test.go":         empty,
		"_x/a_test.go":         empty,
		"p/testdata/a_test.go": empty,
		"p/notatest.go":        "package p\n\nfunc TestX(t *testing.T) {}\n",
	})
	if len(got) != 0 {
		t.Errorf("Scan = %+v, want none", got)
	}
}

func TestScan_ScansADirectoryWhoseRootNameStartsWithADot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), ".hidden")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a_test.go"), []byte(header+"func TestX(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := Scan(root)
	if err != nil || len(found) != 1 {
		t.Errorf("Scan = %+v, %v, want one offender under a dot-named root", found, err)
	}
}

func TestScan_FailsOnAParseError(t *testing.T) {
	t.Parallel()
	if _, err := Scan(layTree(t, map[string]string{"a_test.go": "package p\n\nfunc TestX(t *testing.T) {\n"})); err == nil {
		t.Error("Scan = nil error, want the parse error")
	}
}

func TestScan_FailsOnAnUnreadableFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "absent"), filepath.Join(root, "a_test.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(root); err == nil {
		t.Error("Scan = nil error, want the read error")
	}
}

func TestScan_FailsOnAMissingRoot(t *testing.T) {
	t.Parallel()
	if _, err := Scan(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("Scan = nil error, want an error for a root that does not exist")
	}
}

func TestCheck_NilWhenCleanAndSortedLinesWhenNot(t *testing.T) {
	t.Parallel()
	if err := Check(nil); err != nil {
		t.Errorf("Check(nil) = %v, want nil", err)
	}
	found := []Offender{{File: "b_test.go", Line: 3, Func: "TestB"}, {File: "a_test.go", Line: 9, Func: "TestA"}, {File: "a_test.go", Line: 4, Func: "TestB"}}
	err := Check(found)
	if err == nil {
		t.Fatal("Check = nil, want an error")
	}
	want := "a_test.go:4: TestB has an empty body\na_test.go:9: TestA has an empty body\nb_test.go:3: TestB has an empty body"
	if err.Error() != want {
		t.Errorf("Check = %q, want %q", err.Error(), want)
	}
	if found[0].File != "b_test.go" {
		t.Error("Check reordered the caller's slice")
	}
}
