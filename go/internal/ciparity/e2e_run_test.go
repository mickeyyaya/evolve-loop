package ciparity

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeE2EFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func e2eTags() []string { return []string{"e2e", "evolve_test_phases"} }

func TestE2ETestNames_FollowsTheBuildTagAcrossRoots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeE2EFixture(t, root, map[string]string{
		"cmd/a/a.go":               "package a\n",
		"cmd/a/plain_test.go":      "package a\nimport \"testing\"\nfunc TestPlainOnly(t *testing.T) {}\nfunc TestShared(t *testing.T) {}\n",
		"cmd/a/e2e_test.go":        "//go:build e2e\n\npackage a\nimport \"testing\"\nfunc TestTaggedB(t *testing.T) {}\nfunc TestTaggedA(t *testing.T) {}\nfunc Testify(t *testing.T) {}\nfunc helper() {}\nfunc (x T) TestMethod() {}\ntype T struct{}\n",
		"cmd/a/phases_test.go":     "//go:build e2e && evolve_test_phases\n\npackage a_test\nimport \"testing\"\nfunc TestCompound(t *testing.T) {}\n",
		"cmd/a/notphases_test.go":  "//go:build e2e && !evolve_test_phases\n\npackage a\nimport \"testing\"\nfunc TestExcludedByTag(t *testing.T) {}\n",
		"cmd/a/integ_test.go":      "//go:build integration\n\npackage a\nimport \"testing\"\nfunc TestIntegrationOnly(t *testing.T) {}\n",
		"cmd/a/testdata/x_test.go": "//go:build e2e\n\npackage x\nimport \"testing\"\nfunc TestInTestdata(t *testing.T) {}\n",
		"test/e2e/z_test.go":       "//go:build e2e\n\npackage e2e\nimport \"testing\"\nfunc TestZ(t *testing.T) {}\nfunc TestTaggedA(t *testing.T) {}\n",
		"test/e2e/phase_test.go":   "//go:build evolve_test_phases\n\npackage e2e\nimport \"testing\"\nfunc TestPhaseOnly(t *testing.T) {}\n",
	})

	got, err := E2ETestNames(root, []string{"cmd", "test/e2e"}, e2eTags())

	if err != nil {
		t.Fatal(err)
	}
	want := []string{"TestCompound", "TestTaggedA", "TestTaggedB", "TestZ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("E2ETestNames = %v, want %v", got, want)
	}
}

func TestE2ETestNames_PicksUpANewE2EFileAndIgnoresANewPlainFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeE2EFixture(t, root, map[string]string{
		"cmd/a/a.go":          "package a\n",
		"cmd/a/first_test.go": "//go:build e2e\n\npackage a\nimport \"testing\"\nfunc TestFirst(t *testing.T) {}\n",
	})
	before, err := E2ETestNames(root, []string{"cmd"}, e2eTags())
	if err != nil {
		t.Fatal(err)
	}
	writeE2EFixture(t, root, map[string]string{
		"cmd/a/second_test.go": "//go:build e2e\n\npackage a\nimport \"testing\"\nfunc TestSecond(t *testing.T) {}\n",
		"cmd/a/third_test.go":  "package a\nimport \"testing\"\nfunc TestFirstExtra(t *testing.T) {}\nfunc TestThird(t *testing.T) {}\n",
	})

	after, err := E2ETestNames(root, []string{"cmd"}, e2eTags())

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, []string{"TestFirst"}) || !reflect.DeepEqual(after, []string{"TestFirst", "TestSecond"}) {
		t.Fatalf("names before = %v, after = %v; want the new e2e file picked up and the plain file left out", before, after)
	}
	pattern := E2ERunPattern(after)
	if pattern != "^(TestFirst|TestSecond)$" {
		t.Fatalf("pattern = %q", pattern)
	}
}

func TestE2ETestNames_RefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string]string{
		"missing root": nil,
		"tagged package clash": {
			"cmd/a/a.go":        "package a\n",
			"cmd/a/b.go":        "package b\n",
			"cmd/a/_test.go":    "",
			"cmd/a/e2e_test.go": "//go:build e2e\n\npackage a\n",
		},
		"plain package clash": {
			"cmd/a/a.go":        "package a\n",
			"cmd/a/b.go":        "//go:build !e2e\n\npackage b\n",
			"cmd/a/e2e_test.go": "//go:build e2e\n\npackage a\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n",
		},
		"unparsable e2e file": {
			"cmd/a/a.go":        "package a\n",
			"cmd/a/e2e_test.go": "//go:build e2e\n\npackage a\nfunc TestA( {\n",
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeE2EFixture(t, root, files)
			if got, err := E2ETestNames(root, []string{"cmd"}, e2eTags()); err == nil {
				t.Fatalf("E2ETestNames = %v, nil; want an error", got)
			}
		})
	}
}

func TestE2EMain_PrintsTheAnchoredPatternOrFailsLoudly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeE2EFixture(t, root, map[string]string{
		"cmd/a/a.go":        "package a\n",
		"cmd/a/e2e_test.go": "//go:build e2e\n\npackage a\nimport \"testing\"\nfunc TestB(t *testing.T) {}\nfunc TestA(t *testing.T) {}\n",
		"empty/a.go":        "package a\n",
	})
	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"pattern", []string{"-dir", root, "-tags", "e2e evolve_test_phases", "cmd"}, 0, "^(TestA|TestB)$\n", ""},
		{"default tag", []string{"-dir", root, "cmd"}, 0, "^(TestA|TestB)$\n", ""},
		{"bad flag", []string{"-nope"}, 2, "", "flag provided but not defined"},
		{"missing root", []string{"-dir", root, "absent"}, 1, "", "e2elist:"},
		{"no e2e test", []string{"-dir", root, "empty"}, 1, "", "no e2e test under [empty]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := E2EMain(tc.args, &stdout, &stderr)
			if code != tc.code || stdout.String() != tc.stdout || !strings.Contains(stderr.String(), tc.stderr) {
				t.Fatalf("E2EMain(%v) = %d, stdout %q, stderr %q; want %d, %q, %q", tc.args, code, stdout.String(), stderr.String(), tc.code, tc.stdout, tc.stderr)
			}
		})
	}
}

func TestIsTestName_FollowsGoTestDiscovery(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"Test": true, "TestA": true, "Test_x": true, "Test1": true, "Testify": false, "TestÄ": true, "Tes": false, "helper": false, "BenchmarkX": false} {
		if got := isTestName(name); got != want {
			t.Errorf("isTestName(%q) = %v, want %v", name, got, want)
		}
	}
}
