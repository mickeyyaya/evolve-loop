package structure

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, src := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceFiles_ListsNonTestGoFilesOutsideTestdataAndVendor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"internal/a/a.go":          "package a",
		"internal/a/a_test.go":     "package a",
		"internal/a/notes.md":      "x",
		"internal/a/b/b.go":        "package b",
		"internal/a/testdata/x.go": "package x",
		"internal/a/vendor/v/v.go": "package v",
		"internal/other/other.go":  "package other",
		"cmd/tool/main.go":         "package main",
	})

	got, err := SourceFiles(root, "internal/a", "cmd")

	want := []string{filepath.Join("internal", "a", "a.go"), filepath.Join("internal", "a", "b", "b.go"), filepath.Join("cmd", "tool", "main.go")}
	slices.Sort(got)
	slices.Sort(want)
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("SourceFiles = %q, %v, want %q", got, err, want)
	}
}

func TestSourceFiles_AnAbsentDirectoryIsAnError(t *testing.T) {
	t.Parallel()
	_, err := SourceFiles(t.TempDir(), "internal/gone")

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("SourceFiles = %v, want a not-exist error", err)
	}
}

func TestSelectorUses_FindsEveryUseOfTheNamedMembersUnderAnyImportName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"plain.go":   "package x\nimport \"time\"\nfunc f() {\n\ttime.Sleep(1)\n\t_ = time.Now()\n\tg := time.NewTicker\n\t_ = g\n}\n",
		"aliased.go": "package x\nimport clock \"time\"\nfunc f() { <-clock.Tick(1) }\n",
		"other.go":   "package x\nimport \"os\"\ntype time struct{}\nfunc (time) Sleep() {}\nfunc f() { var t time; t.Sleep(); os.Exit(0) }\n",
	})

	plain, plainErr := SelectorUses(root, "plain.go", "time", "Sleep", "Tick", "NewTicker")
	aliased, aliasedErr := SelectorUses(root, "aliased.go", "time", "Sleep", "Tick", "NewTicker")
	other, otherErr := SelectorUses(root, "other.go", "time", "Sleep", "Tick", "NewTicker")

	if want := []string{"plain.go:4: time.Sleep", "plain.go:6: time.NewTicker"}; plainErr != nil || !slices.Equal(plain, want) {
		t.Errorf("plain = %q, %v, want %q", plain, plainErr, want)
	}
	if want := []string{"aliased.go:3: clock.Tick"}; aliasedErr != nil || !slices.Equal(aliased, want) {
		t.Errorf("aliased = %q, %v, want %q", aliased, aliasedErr, want)
	}
	if otherErr != nil || len(other) != 0 {
		t.Errorf("other = %q, %v, want nothing: a file that does not import the package has no use", other, otherErr)
	}
}

func TestSelectorUses_AnUnparsableFileIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"bad.go": "package"})

	if _, err := SelectorUses(root, "bad.go", "time", "Sleep"); err == nil {
		t.Errorf("SelectorUses(bad.go) = nil error, want a parse error")
	}
}
