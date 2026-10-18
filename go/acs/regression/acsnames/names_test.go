//go:build acs

package acsnames

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

func predicateFile(body string) string {
	return "package cycle\n\nimport \"testing\"\n\nvar _ = testing.Short\n\n" + body
}

func TestParseAlternatives_ReadsGoTestRunPatterns(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        []alternative
	}{
		{"a bare name", "TestA", []alternative{{"TestA", false, false}}},
		{"an exact name", "^TestA$", []alternative{{"TestA", true, true}}},
		{"an exact group", "^(TestA|TestB)$", []alternative{{"TestA", true, true}, {"TestB", true, true}}},
		{"a prefix", "^TestA_", []alternative{{"TestA_", true, false}}},
		{"a suffix", "TestA$", []alternative{{"TestA", false, true}}},
		{"a bare group is not anchored", "(TestA|TestB)", []alternative{{"TestA", false, false}, {"TestB", false, false}}},
		{"a pipe list is not anchored", "TestA|TestB", []alternative{{"TestA", false, false}, {"TestB", false, false}}},
		{"an anchor binds the edge alternative without a group", "^TestA|TestB$", []alternative{{"TestA", true, false}, {"TestB", false, true}}},
		{"each alternative carries its own anchors", "^TestA|^TestB$", []alternative{{"TestA", true, false}, {"TestB", true, true}}},
		{"a fragment of a concatenated pattern", "TestA|", []alternative{{"TestA", false, false}}},
		{"a leading pipe", "|TestA", []alternative{{"TestA", false, false}}},
		{"the subtest filter is cut", "^TestParent$/^sub$", []alternative{{"TestParent", true, true}}},
		{"a fuzz target", "FuzzParse", []alternative{{"FuzzParse", false, false}}},
		{"a group after a prefix", "TestTriage_(A|B|C)", []alternative{{"TestTriage_A", false, false}, {"TestTriage_B", false, false}, {"TestTriage_C", false, false}}},
		{"a group after an anchored prefix", "^TestTriage_(A|B)", []alternative{{"TestTriage_A", true, false}, {"TestTriage_B", true, false}}},
		{"an anchored group after a prefix is exact", "^TestTriage_(A|B)$", []alternative{{"TestTriage_A", true, true}, {"TestTriage_B", true, true}}},
		{"a group between a prefix and a suffix", "Test(A|B)Tail", []alternative{{"TestATail", false, false}, {"TestBTail", false, false}}},
		{"a group with an unreadable member", "TestTriage_(A|b c)", nil},
		{"a group with an empty member", "TestTriage_(A|)", nil},
		{"an unclosed group after a prefix", "TestTriage_(A|B", nil},
		{"padding is trimmed", "  TestA\n", []alternative{{"TestA", false, false}}},
		{"one unreadable alternative drops the literal", "TestA|not a name", nil},
		{"a pipe alone", "|", nil},
		{"empty", "", nil},
		{"the bare prefix word", "Test", nil},
		{"prose", "TestA did not pass", nil},
		{"a path", "acs/cycle1313", nil},
		{"a metacharacter", "TestA.*", nil},
		{"a benchmark", "BenchmarkA", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseAlternatives(tc.value); !slices.Equal(got, tc.want) {
				t.Fatalf("parseAlternatives(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestDeclaredTests_ReadsEveryTestFunctionOfTheModule(t *testing.T) {
	goDir := writeTree(t, map[string]string{
		"a/a_test.go":          "package a\n\nfunc TestPlain(t *testing.T) {}\n\nfunc FuzzTarget(f *testing.F) {}\n\nfunc helper() {}\n\nfunc (s suite) TestMethod(t *testing.T) {}\n",
		"b/c/c_test.go":        "package c\n\nfunc TestDeep(t *testing.T) {}\n",
		"a/testdata/x_test.go": "package x\n\nfunc TestInTestdata(t *testing.T) {}\n",
		"vendor/v/v_test.go":   "package v\n\nfunc TestVendored(t *testing.T) {}\n",
		".hidden/h_test.go":    "package h\n\nfunc TestHidden(t *testing.T) {}\n",
		"a/production.go":      "package a\n\nfunc TestInProduction() {}\n",
	})
	got, err := declaredTests(goDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"TestPlain": true, "FuzzTarget": true, "TestDeep": true}
	if len(got) != len(want) {
		t.Fatalf("declaredTests = %v, want %v", got, want)
	}
	for name := range want {
		if !got[name] {
			t.Fatalf("declaredTests = %v, want %v", got, want)
		}
	}
}

func TestDeclaredTests_OnlyARealDeclarationCounts(t *testing.T) {
	goDir := writeTree(t, map[string]string{
		"a/a_test.go": "package a\n\nconst fixture = `\nfunc TestInRawString(t *testing.T) {}\n`\n\n/*\nfunc TestInComment(t *testing.T) {}\n*/\n\nfunc TestReal(t *testing.T) {}\n",
	})
	got, err := declaredTests(goDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["TestReal"] {
		t.Fatalf("declaredTests = %v, want only TestReal: a raw string and a comment declare nothing", got)
	}
}

func TestDeclaredTests_ASyntaxErrorInATestFileIsAnError(t *testing.T) {
	if _, err := declaredTests(writeTree(t, map[string]string{"a/a_test.go": "package a\n\nfunc (\n"})); err == nil {
		t.Fatal("a test file that does not parse must be an error, not a silent gap in the index")
	}
}

func TestDeclaredTests_AMissingRootIsAnError(t *testing.T) {
	if _, err := declaredTests(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a root that does not exist must be an error, not an empty module")
	}
}

func TestDeclaredTests_AnUnreadableTestFileIsAnError(t *testing.T) {
	goDir := writeTree(t, map[string]string{"a/a_test.go": "package a\n"})
	path := filepath.Join(goDir, "a", "a_test.go")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o644)
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("the process reads files regardless of mode")
	}
	if _, err := declaredTests(goDir); err == nil {
		t.Fatal("an unreadable test file must be an error, not a silent gap in the index")
	}
}

func violationsFor(t *testing.T, files map[string]string, absent map[string]string) []string {
	t.Helper()
	got, err := findViolations(writeTree(t, files), absent)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

const declaresRenamed = "package decl\n\nfunc TestRenamed(t *testing.T) {}\n\nfunc TestStoreOpen(t *testing.T) {}\n\nfunc TestAck_One(t *testing.T) {}\n\nfunc TestEarlyTestLate(t *testing.T) {}\n"

func TestViolations_AnExactNameNoTestDeclaresIsNamedWithItsLine(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go": declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile(
			"func runGoTest(args ...string) {}\n\nfunc TestC9_001(t *testing.T) {\n\trunGoTest(\"-run\", \"^TestGone$\")\n}\n"),
	}, nil)
	if len(got) != 1 {
		t.Fatalf("violations = %q, want exactly one", got)
	}
	for _, want := range []string{"go/acs/cycle9/predicates_test.go:10", `"TestGone"`, "no test function matches ^TestGone$", "repoint the predicate"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("violation %q lacks %q", got[0], want)
		}
	}
}

func TestViolations_AGroupAfterAPrefixIsExpandedIntoItsNames(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var argv = []string{\"-run\", \"^Test(Renamed|Gone)$\"}\n"),
	}, nil)
	if len(got) != 1 || !strings.Contains(got[0], `"TestGone"`) {
		t.Fatalf("violations = %q, want one for TestGone: a group after a prefix still names tests", got)
	}
}

func TestViolations_ANameATestDeclaresPasses(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("func TestC9_001(t *testing.T) {\n\t_ = []string{\"-run\", \"^(TestRenamed|TestStoreOpen)$\"}\n}\n"),
	}, nil)
	if len(got) != 0 {
		t.Fatalf("violations = %q, want none", got)
	}
}

func TestViolations_OnlyAPackageThatRunsTestsByNameIsRead(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var cited = \"TestGone\"\n"),
		"acs/cycle8/predicates_test.go": predicateFile("var argv = []string{\"-run\", \"^TestRenamed$\"}\n"),
	}, nil)
	if len(got) != 0 {
		t.Fatalf("a package with no -run and no GoTests only cites a name in text, violations = %q", got)
	}
}

func TestViolations_APredicateTreeThatNamesNoTestIsAnError(t *testing.T) {
	_, err := findViolations(writeTree(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var cited = \"TestGone\"\n"),
	}), nil)
	if err == nil {
		t.Fatal("a scan that reads no name must be an error: it would pass for the wrong reason")
	}
}

func TestViolations_EveryRunSpellingMarksAPackage(t *testing.T) {
	for name, body := range map[string]string{
		"run flag":      "var argv = []string{\"-run\", \"^TestGone$\"}\n",
		"run equals":    "var argv = []string{\"-run=^TestGone$\", \"TestGone\"}\n",
		"test run flag": "var argv = []string{\"-test.run\", \"^TestGone$\"}\n",
		"test run eq":   "var argv = []string{\"-test.run=^TestGone$\", \"TestGone\"}\n",
		"GoTests call":  "var run = acsassert.GoTests\n\nvar names = []string{\"TestGone\"}\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := violationsFor(t, map[string]string{
				"internal/decl/decl_test.go":    declaresRenamed,
				"acs/cycle9/predicates_test.go": predicateFile(body),
			}, nil)
			if len(got) == 0 {
				t.Fatal("the package runs tests by name, so its stale name must be a violation")
			}
		})
	}
}

func TestViolations_APrefixAndASubstringReadTheWayGoTestReadsThem(t *testing.T) {
	for _, tc := range []struct {
		name, literal, wantViolationFor string
	}{
		{"a prefix that some test starts with", "^TestAck_", ""},
		{"a prefix that no test starts with", "^TestNack_", "^TestNack_"},
		{"a bare name that some test contains", "TestStore", ""},
		{"a bare name that no test contains", "TestShelf", "TestShelf"},
		{"a bare name as a prefix of a longer test", "TestRename", ""},
		{"a bare name inside a longer test", "TestLate", ""},
		{"a prefix does not match inside a longer test", "^TestLate", "^TestLate"},
		{"a suffix that some test ends with", "TestStoreOpen$", ""},
		{"a suffix that no test ends with", "TestStoreClose$", "TestStoreClose$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := violationsFor(t, map[string]string{
				"internal/decl/decl_test.go":    declaresRenamed,
				"acs/cycle9/predicates_test.go": predicateFile("var argv = []string{\"-run\", " + `"` + tc.literal + `"` + "}\n"),
			}, nil)
			if tc.wantViolationFor == "" {
				if len(got) != 0 {
					t.Fatalf("violations = %q, want none", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], "no test function matches "+tc.wantViolationFor+":") {
				t.Fatalf("violations = %q, want one that shows the pattern %s", got, tc.wantViolationFor)
			}
		})
	}
}

func TestViolations_ANameInAStringsCallIsNotAReference(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go": declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile(
			"var argv = []string{\"-run\", \"^TestRenamed$\"}\n\nfunc TestC9_001(t *testing.T) {\n\tif strings.Contains(\"out\", \"TestGone\") || strings.Count(\"out\", \"TestAlsoGone\") > 0 {\n\t}\n\t_ = strings.Join([]string{\"TestJoined\"}, \"|\")\n}\n"),
	}, nil)
	if len(got) != 1 || !strings.Contains(got[0], `"TestJoined"`) {
		t.Fatalf("violations = %q, want only the strings.Join element: an absence check on output is not a reference, a joined pattern is", got)
	}
}

func TestViolations_ANameThePackageDefinesInAFixtureIsNotAReference(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":               declaresRenamed,
		"acs/cycle9/predicates_test.go":            predicateFile("var argv = []string{\"-run\", \"^TestProbe$\", \"^TestOverlayProbe$\", \"^TestRenamed$\"}\n\nconst probeSource = \"package p\\n\\nfunc TestProbe(t *testing.T) {}\\n\"\n"),
		"acs/cycle9/testdata/overlay_probe.go.txt": "package p\n\nfunc TestOverlayProbe(t *testing.T) {}\n",
	}, nil)
	if len(got) != 0 {
		t.Fatalf("violations = %q, want none: the predicate writes both probes itself", got)
	}
}

func TestViolations_AnAbsentNameWithAReasonIsAllowedOnce(t *testing.T) {
	files := map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var argv = []string{\"-run\", \"^TestSentinel$\"}\n"),
	}
	if got := violationsFor(t, files, map[string]string{"cycle9/TestSentinel": "proves that go test warns on a miss"}); len(got) != 0 {
		t.Fatalf("a listed absent name must pass, violations = %q", got)
	}
	if got := violationsFor(t, files, map[string]string{"cycle8/TestSentinel": "another package"}); len(got) != 2 {
		t.Fatalf("an entry for another package must not cover this one and must itself be unused, violations = %q", got)
	}
}

func TestViolations_AnAbsentEntryThatNoLongerAppliesIsAViolation(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var argv = []string{\"-run\", \"^TestRenamed$\"}\n"),
	}, map[string]string{
		"cycle9/TestRenamed": "a test declares it now",
		"cycle9/TestGone":    "no predicate names it now",
	})
	if len(got) != 2 {
		t.Fatalf("violations = %q, want one for each stale entry", got)
	}
	for _, v := range got {
		if !strings.Contains(v, "absent list entry") {
			t.Errorf("violation %q must name the absent list", v)
		}
	}
}

func TestViolations_ReportsFollowPackageThenSourceOrderAndTheOwnPackageIsSkipped(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":                 declaresRenamed,
		"acs/cycle2/predicates_test.go":              predicateFile("var argv = []string{\"-run\", \"^TestB$\"}\n"),
		"acs/cycle1/predicates_test.go":              predicateFile("var argv = []string{\"-run\", \"^TestA$\", \"^TestC$\"}\n"),
		"acs/regression/acsnames/predicates_test.go": predicateFile("var argv = []string{\"-run\", \"^TestSelf$\"}\n"),
	}, nil)
	if len(got) != 3 || !strings.Contains(got[0], `"TestA"`) || !strings.Contains(got[1], `"TestC"`) || !strings.Contains(got[2], "cycle2/") {
		t.Fatalf("violations = %q, want cycle1 in source order, then cycle2, and nothing from the own package", got)
	}
}

func TestViolations_AHelperFileOfAPackageIsRead(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var kept = 1\n"),
		"acs/cycle9/helper.go":          "package cycle\n\nvar argv = []string{\"-run\", \"^TestGone$\"}\n",
	}, nil)
	if len(got) != 1 || !strings.Contains(got[0], "go/acs/cycle9/helper.go:3") {
		t.Fatalf("violations = %q, want the stale name in helper.go: a helper file that is not a test file still builds the run argument", got)
	}
}

func TestViolations_ATestdataTreeIsNotAPredicatePackage(t *testing.T) {
	got := violationsFor(t, map[string]string{
		"internal/decl/decl_test.go":              declaresRenamed,
		"acs/cycle9/predicates_test.go":           predicateFile("var argv = []string{\"-run\", \"^TestRenamed$\"}\n"),
		"acs/cycle9/testdata/streamprobe/main.go": "package main\n\nvar argv = []string{\"-run\", \"^TestElsewhere$\"}\n",
	}, nil)
	if len(got) != 0 {
		t.Fatalf("violations = %q, want none: testdata holds programs a predicate runs, not predicates", got)
	}
}

func TestViolations_ASyntaxErrorInAPredicateIsAnError(t *testing.T) {
	_, err := findViolations(writeTree(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var kept = 1\n"),
		"acs/cycle9/helper.go":          "package cycle\n\nfunc (\n",
	}), nil)
	if err == nil {
		t.Fatal("a predicate helper that does not parse must be an error, not a package that names nothing")
	}
}

func TestViolations_AMissingModuleIsAnError(t *testing.T) {
	if _, err := findViolations(filepath.Join(t.TempDir(), "absent"), nil); err == nil {
		t.Fatal("a module root that does not exist must be an error")
	}
}

func TestViolations_AModuleWithoutAnAcsTreeIsAnError(t *testing.T) {
	if _, err := findViolations(writeTree(t, map[string]string{"internal/decl/decl_test.go": declaresRenamed}), nil); err == nil {
		t.Fatal("a module with no acs tree must be an error: a gate that reads no predicate passes for the wrong reason")
	}
}

func TestViolations_AnUnreadableFixtureFileIsAnError(t *testing.T) {
	goDir := writeTree(t, map[string]string{
		"internal/decl/decl_test.go":    declaresRenamed,
		"acs/cycle9/predicates_test.go": predicateFile("var argv = []string{\"-run\", \"^TestRenamed$\"}\n"),
		"acs/cycle9/testdata/probe.txt": "func TestProbe(t *testing.T) {}\n",
	})
	path := filepath.Join(goDir, "acs", "cycle9", "testdata", "probe.txt")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o644)
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("the process reads files regardless of mode")
	}
	if _, err := findViolations(goDir, nil); err == nil {
		t.Fatal("an unreadable fixture must be an error, not a silent gap")
	}
}
