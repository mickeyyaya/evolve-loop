package paralleltest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureSource(pkg, body string) string {
	return "package " + pkg + "\nimport (\n\t\"os\"\n\t\"testing\"\n)\nvar _ = os.Args\nvar Seam = 1\nvar sharedSeam = 1\n" + body
}

func fixtureViolations(body string, cfg Config) []string {
	return Violations(map[string]string{"fixture_test.go": fixtureSource("fix", body)}, cfg)
}

func TestViolations_JudgesTheShapeOfEachTopLevelTest(t *testing.T) {
	t.Parallel()
	listed := map[string]string{"fix.TestF": "reason"}
	cases := []struct {
		name   string
		body   string
		serial map[string]string
		want   string
	}{
		{"no t.Parallel is flagged", "func TestF(t *testing.T) {\n\t_ = 1\n}\n", nil, "fix.TestF (fixture_test.go): does not call t.Parallel and is not listed as serial"},
		{"t.Parallel first passes", "func TestF(t *testing.T) {\n\tt.Parallel()\n\t_ = 1\n}\n", nil, ""},
		{"t.Parallel not first is flagged", "func TestF(t *testing.T) {\n\t_ = 1\n\tt.Parallel()\n}\n", nil, "t.Parallel must be the first statement"},
		{"t.Parallel only in a subtest is flagged", "func TestF(t *testing.T) {\n\tt.Run(\"a\", func(t *testing.T) { t.Parallel() })\n}\n", nil, "t.Parallel must be the first statement"},
		{"renamed parameter is read", "func TestF(tt *testing.T) {\n\ttt.Parallel()\n}\n", nil, ""},
		{"renamed parameter without Parallel is flagged", "func TestF(tt *testing.T) {\n\tt := 1\n\t_ = t\n}\n", nil, "does not call t.Parallel"},
		{"blank parameter cannot be parallel", "func TestF(_ *testing.T) {\n\t_ = 1\n}\n", nil, "does not call t.Parallel"},
		{"t.Setenv needs no entry", "func TestF(t *testing.T) {\n\tt.Setenv(\"A\", \"b\")\n}\n", nil, ""},
		{"t.Chdir needs no entry", "func TestF(t *testing.T) {\n\tt.Chdir(\"/\")\n}\n", nil, ""},
		{"setenv test with an entry is flagged", "func TestF(t *testing.T) {\n\tt.Setenv(\"A\", \"b\")\n}\n", listed, "fix.TestF: calls t.Setenv or t.Chdir so it needs no serial entry"},
		{"listed serial passes", "func TestF(t *testing.T) {\n\t_ = 1\n}\n", listed, ""},
		{"listed but parallel is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n}\n", listed, "fix.TestF: calls t.Parallel but is listed as serial"},
		{"listed but absent is flagged", "func TestF(t *testing.T) {\n\tt.Parallel()\n}\n", map[string]string{"fix.TestGone": "r"}, "fix.TestGone: listed as serial but no such test exists"},
		{"TestMain is not judged", "func TestMain(m *testing.M) {}\n", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fixtureViolations(tc.body, Config{Serial: tc.serial})
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("want no violation, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Fatalf("want one violation containing %q, got %v", tc.want, got)
			}
		})
	}
}

func TestViolations_AParallelTestMustNotChangeProcessState(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ stmt, want string }{
		"package variable":                 {"sharedSeam = 2", "sharedSeam = 2"},
		"package variable increment":       {"sharedSeam++", "sharedSeam++"},
		"package variable through the pkg": {"fix.sharedSeam = 2", "fix.sharedSeam = 2"},
		"write inside a closure":           {"func() { sharedSeam = 3 }()", "sharedSeam = 3"},
		"write after a local define":       {"x := 1\n\t_ = x\n\tsharedSeam = 4", "sharedSeam = 4"},
		"multi-assignment":                 {"Seam, sharedSeam = 2, 3", "Seam, sharedSeam = 2, 3"},
		"stderr":                           {"os.Stderr = nil", "os.Stderr = nil"},
		"stdout":                           {"os.Stdout = nil", "os.Stdout = nil"},
		"stdin":                            {"os.Stdin = nil", "os.Stdin = nil"},
		"args":                             {"os.Args = nil", "os.Args = nil"},
		"os.Chdir":                         {"_ = os.Chdir(\"/\")", "os.Chdir"},
		"os.Setenv":                        {"_ = os.Setenv(\"A\", \"b\")", "os.Setenv"},
		"os.Unsetenv":                      {"_ = os.Unsetenv(\"A\")", "os.Unsetenv"},
		"os.Clearenv":                      {"os.Clearenv()", "os.Clearenv"},
		"t.Setenv":                         {"t.Setenv(\"A\", \"b\")", "t.Setenv"},
		"t.Chdir":                          {"t.Chdir(\"/\")", "t.Chdir"},
		"os.NewFile on descriptor 0":       {"_ = os.NewFile(0, os.DevNull)", "os.NewFile(0, os.DevNull)"},
		"os.NewFile through a conversion":  {"_ = os.NewFile(uintptr(2), \"x\")", "os.NewFile(uintptr(2)"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := fixtureViolations("func TestF(t *testing.T) {\n\tt.Parallel()\n\t"+tc.stmt+"\n}\n", Config{})
			if len(got) != 1 || !strings.Contains(got[0], "fix.TestF") || !strings.Contains(got[0], tc.want) {
				t.Fatalf("want one violation naming the test and %q; got %v", tc.want, got)
			}
		})
	}
}

func TestViolations_LocalAndComputedStateIsNotProcessState(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"shadowing local":             "sharedSeam := 1\n\tsharedSeam = 2\n\t_ = sharedSeam",
		"computed descriptor":         "fd := uintptr(9)\n\t_ = os.NewFile(fd, \"x\")",
		"descriptor above two":        "_ = os.NewFile(3, \"x\")",
		"read of a package variable":  "_ = sharedSeam + Seam",
		"local closure parameter set": "func(sharedSeam int) { sharedSeam = 2 }(1)",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := fixtureViolations("func TestF(t *testing.T) {\n\tt.Parallel()\n\t"+stmt+"\n}\n", Config{}); len(got) != 0 {
				t.Fatalf("want no violation, got %v", got)
			}
		})
	}
}

func TestViolations_ConfiguredGlobalSettersAreFlagged(t *testing.T) {
	t.Parallel()
	body := "func TestF(t *testing.T) {\n\tt.Parallel()\n\tbridge.SetModelCatalogDirFn(nil)\n}\n"
	source := "package fix\nimport (\n\t\"testing\"\n\t\"example.com/bridge\"\n)\n" + body
	cfg := Config{GlobalSetters: map[string]bool{"SetModelCatalogDirFn": true}}
	if got := Violations(map[string]string{"fixture_test.go": source}, cfg); len(got) != 1 || !strings.Contains(got[0], "SetModelCatalogDirFn") {
		t.Fatalf("a configured setter must be flagged once; got %v", got)
	}
	if got := Violations(map[string]string{"fixture_test.go": source}, Config{}); len(got) != 0 {
		t.Fatalf("an unconfigured setter must pass; got %v", got)
	}
}

func TestViolations_HelpersAreFollowedTransitively(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ helpers, want string }{
		"write two calls deep": {"func helperOuter() { helperInner() }\nfunc helperInner() { sharedSeam = 5 }\n", "via helper helperInner: sharedSeam = 5"},
		"setenv in a helper":   {"func helperOuter() { _ = os.Setenv(\"A\", \"b\") }\n", "via helper helperOuter: os.Setenv"},
		"helper t.Setenv":      {"func helperOuter(tb *testing.T) { tb.Setenv(\"A\", \"b\") }\n", "via helper helperOuter: tb.Setenv"},
		"recursive helper":     {"func helperOuter() { helperOuter(); os.Stderr = nil }\n", "via helper helperOuter: os.Stderr = nil"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			call := "helperOuter()"
			if strings.Contains(tc.helpers, "tb *testing.T") {
				call = "helperOuter(t)"
			}
			got := fixtureViolations(tc.helpers+"func TestF(t *testing.T) {\n\tt.Parallel()\n\t"+call+"\n}\n", Config{})
			if len(got) != 1 || !strings.Contains(got[0], "fix.TestF") || !strings.Contains(got[0], tc.want) {
				t.Fatalf("want one violation containing %q; got %v", tc.want, got)
			}
		})
	}
}

func TestViolations_MethodsOnPackageValuesAreFollowed(t *testing.T) {
	t.Parallel()
	methods := "type box struct{ n int }\nvar shared box\nfunc (box) swap() { sharedSeam = 7 }\nfunc (b *box) bump() { b.n++ }\n"
	cases := map[string]struct{ stmt, want string }{
		"mutating method on a package value": {"shared.swap()", "via helper swap: sharedSeam = 7"},
		"same method on a local value":       {"var local box\n\tlocal.swap()", ""},
		"receiver-only method":               {"shared.bump()", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := fixtureViolations(methods+"func TestF(t *testing.T) {\n\tt.Parallel()\n\t"+tc.stmt+"\n}\n", Config{})
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("want no violation, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Fatalf("want one violation containing %q; got %v", tc.want, got)
			}
		})
	}
}

func TestViolations_SerialListIsKeyedByPackageAndName(t *testing.T) {
	t.Parallel()
	sources := map[string]string{
		"a_test.go": "package fix\nimport \"testing\"\nfunc TestDup(t *testing.T) {\n\t_ = 1\n}\n",
		"b_test.go": "package fix_test\nimport \"testing\"\nfunc TestDup(t *testing.T) {\n\t_ = 1\n}\n",
	}
	got := Violations(sources, Config{Serial: map[string]string{"fix.TestDup": "r"}})
	if len(got) != 1 || !strings.Contains(got[0], "fix_test.TestDup") {
		t.Fatalf("listing fix.TestDup must not exempt fix_test.TestDup; got %v", got)
	}
}

func TestViolations_NonTestFilesSupplyVariablesButAreNotJudged(t *testing.T) {
	t.Parallel()
	sources := map[string]string{
		"prod.go":      "package fix\nvar productionSeam = 1\nfunc TestLooksLikeATest() {}\n",
		"prod_test.go": "package fix\nimport \"testing\"\nfunc TestF(t *testing.T) {\n\tt.Parallel()\n\tproductionSeam = 2\n}\n",
	}
	got := Violations(sources, Config{})
	if len(got) != 1 || !strings.Contains(got[0], "productionSeam = 2") {
		t.Fatalf("want one violation for the production variable write; got %v", got)
	}
}

func TestViolations_AnUnparsableFileIsReported(t *testing.T) {
	t.Parallel()
	got := Violations(map[string]string{"broken_test.go": "package fix\nfunc {"}, Config{})
	if len(got) != 1 || !strings.HasPrefix(got[0], "broken_test.go: ") {
		t.Fatalf("a parse error must be reported against its file; got %v", got)
	}
}

func TestSources_ReadsEveryGoFileByBaseName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, body := range map[string]string{"a.go": "package a\n", "a_test.go": "package a\n", "notes.txt": "x"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Sources(dir)
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(got) != 2 || got["a.go"] != "package a\n" || got["a_test.go"] != "package a\n" {
		t.Fatalf("want the two Go files keyed by base name; got %v", got)
	}
}

func TestSources_FailsLoudly(t *testing.T) {
	t.Parallel()
	if _, err := Sources(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no Go files") {
		t.Fatalf("an empty directory must fail; got %v", err)
	}
	if _, err := Sources("[invalid"); err == nil {
		t.Fatal("a malformed glob must fail")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dir.go"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Sources(dir); err == nil || !strings.Contains(err.Error(), "read ") {
		t.Fatalf("an unreadable match must fail; got %v", err)
	}
}
