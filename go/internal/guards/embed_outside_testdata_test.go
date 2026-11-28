package guards

import (
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/structure"
)

func testdataEmbeds(dir string) ([]string, error) {
	ctx := build.Default
	ctx.UseAllFiles = true
	pkg, err := ctx.ImportDir(dir, 0)
	var multiple *build.MultiplePackageError
	var noGo *build.NoGoError
	if err != nil && !errors.As(err, &multiple) && !errors.As(err, &noGo) {
		return nil, err
	}
	var hits []string
	for _, pattern := range pkg.EmbedPatterns {
		if slices.Contains(strings.Split(strings.TrimPrefix(pattern, "all:"), "/"), "testdata") {
			hits = append(hits, pattern)
		}
	}
	return hits, nil
}

func TestTestdataEmbeds_NamesEachPatternUnderATestdataDirectoryInEveryFormGoAccepts(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"p.go":       "package p\n\nimport _ \"embed\"\n\n//go:embed all:testdata/a\nvar a string\n\n//go:embed\ttestdata/b.txt\nvar b string\n\n//go:embed \"testdata/c d.txt\" `testdata/e.txt` data/f.txt mytestdata/g.txt assets/testdata/k.txt\nvar c string\n",
		"tagged.go":  "//go:build integration\n\npackage p\n\nimport _ \"embed\"\n\n//go:embed testdata/h.txt\nvar h string\n",
		"ignored.go": "//go:build ignore\n\npackage main\n\nimport _ \"embed\"\n\n//go:embed testdata/j.txt\nvar j string\n",
		"p_test.go":  "package p\n\nimport _ \"embed\"\n\n//go:embed testdata/i.txt\nvar i string\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := testdataEmbeds(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"all:testdata/a", "assets/testdata/k.txt", "testdata/b.txt", "testdata/c d.txt", "testdata/e.txt", "testdata/h.txt", "testdata/j.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("testdataEmbeds = %q, want %q", got, want)
	}
}

func TestTestdataEmbeds_ADirectoryWithOnlyTestFilesHasNoHits(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\n\nimport _ \"embed\"\n\n//go:embed testdata/i.txt\nvar i string\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := testdataEmbeds(dir)
	if err != nil || len(got) != 0 {
		t.Fatalf("testdataEmbeds = (%q, %v), want no hits and no error", got, err)
	}
}

func TestTestdataEmbeds_AnUnparsableFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte("this is not go"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := testdataEmbeds(dir); err == nil {
		t.Fatal("testdataEmbeds read an unparsable file without an error")
	}
}

func TestNoProductionFileEmbedsFromATestdataDirectory(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files, err := structure.SourceFiles(module, ".")
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, rel := range files {
		dirs = append(dirs, filepath.Dir(rel))
	}
	slices.Sort(dirs)
	var offenders []string
	for _, rel := range slices.Compact(dirs) {
		hits, err := testdataEmbeds(filepath.Join(module, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, pattern := range hits {
			offenders = append(offenders, filepath.ToSlash(rel)+": "+pattern)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("production code embeds from a testdata directory; tools that copy a tree skip testdata, so the build breaks there. Move each file out of testdata:\n%s", strings.Join(offenders, "\n"))
	}
}
