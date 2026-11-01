package coversplit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func splitModule(t *testing.T) string {
	t.Helper()
	return writeModule(t, map[string]string{
		"go.mod":                    "module example.com/m\n",
		"main.go":                   "package main\n",
		"s/s.go":                    "package s\n",
		"s/st/fixture.go":           "package st\n",
		"s/st/run_test.go":          "package st_test\n\nimport \"example.com/m/s\"\n",
		"s/helper/helper.go":        "package helper\n\nimport \"example.com/m/s\"\n",
		"s/helper/helper_test.go":   "package helper_test\n\nimport \"example.com/m/s\"\n",
		"s/deep/deeper/d_test.go":   "package deeper\n",
		"s/vendor/v/v_test.go":      "package v\n\nimport \"example.com/m/s\"\n",
		"s/testdata/t/t_test.go":    "package t\n\nimport \"example.com/m/s\"\n",
		"s/notes.md":                "notes\n",
		"other/o_test.go":           "package o\n\nimport \"example.com/m/s\"\n",
		"s/st/testdata/broken.go":   "not go",
		"s/.hidden/h/h_test.go":     "package h\n\nimport \"example.com/m/s\"\n",
		"s/_skipped/k/k_test.go":    "package k\n\nimport \"example.com/m/s\"\n",
		"s/st/second_test.go":       "package st_test\n",
		"s/deep/deeper/d.go":        "package deeper\n",
		"s/helper/more_test.go":     "package helper\n",
		"other/o.go":                "package o\n",
		"s/st/fixture_more.go":      "package st\n\nimport \"fmt\"\n\nvar _ = fmt.Sprint\n",
		"s/deep/deeper/raw_test.go": "package deeper\n\nimport `example.com/m/s`\n",
	})
}

func TestDetect_ATestPackageThatTestsAnAncestorItDoesNotBuildOnIsASplit(t *testing.T) {
	t.Parallel()
	got, err := Detect(splitModule(t))
	want := []Split{{"./s/deep/deeper", "./s"}, {"./s/st", "./s"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Detect = %v, %v, want %v", got, err, want)
	}
}

func TestUnlisted_NamesEveryMissingAndEveryStaleEntry(t *testing.T) {
	t.Parallel()
	root := splitModule(t)
	problems, err := Unlisted(root, []Split{{"./s/st", "./s"}, {"./other", "./s"}})
	if err != nil || len(problems) != 2 || !strings.Contains(problems[0], "./s/deep/deeper tests ./s") || !strings.Contains(problems[1], "./other is not a test package split from ./s") {
		t.Fatalf("Unlisted = %q, %v", problems, err)
	}
	if problems, err := Unlisted(root, []Split{{"./s/st", "./s"}, {"./s/deep/deeper", "./s"}}); err != nil || len(problems) != 0 {
		t.Fatalf("a complete list = %q, %v", problems, err)
	}
}

func TestDetect_FailsLoudlyOnAModuleItCannotRead(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"no go.mod":            {"a/a.go": "package a\n"},
		"a go.mod sans module": {"go.mod": "go 1.23\n"},
		"a broken Go file":     {"go.mod": "module m\n", "a/a.go": "package"},
	} {
		if _, err := Detect(writeModule(t, files)); err == nil {
			t.Errorf("%s: Detect must fail", name)
		}
		if _, err := Unlisted(writeModule(t, files), nil); err == nil {
			t.Errorf("%s: Unlisted must fail", name)
		}
	}
	root := writeModule(t, map[string]string{"go.mod": "module m\n", "locked/a.go": "package a\n"})
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if os.Geteuid() != 0 {
		if _, err := Detect(root); err == nil || !strings.Contains(err.Error(), "walk") {
			t.Errorf("an unreadable directory: err = %v, want the walk named", err)
		}
	}
}

func TestEverySplitTestPackageHasItsCoverEntry(t *testing.T) {
	t.Parallel()
	moduleDir := filepath.Join("..", "..")
	listed, err := Load(moduleDir)
	if err != nil {
		t.Fatal(err)
	}
	problems, err := Unlisted(moduleDir, listed)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
