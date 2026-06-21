package buildsite

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A template file with an unterminated action makes html/template's ParseGlob
// fail, and Build must surface that as a "parse templates" error.
func TestBuild_ParseGlobFailure(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "broken.html"), `{{define "x"}}{{ oops`)
	_, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       filepath.Join(dir, "dist"),
		Versions:     []Version{{Slug: "x", Template: "x"}},
	})
	if err == nil {
		t.Fatal("Build with malformed template = nil error, want parse failure")
	}
	if !strings.Contains(err.Error(), "parse templates") {
		t.Errorf("error %q does not mention parse templates", err)
	}
}

// A Gallery name that has no matching {{define}} block makes the gallery render
// step fail, and Build must surface that as a "render gallery" error.
func TestBuild_GalleryMissingFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"), `{{define "mini"}}hi{{end}}`)
	out := filepath.Join(dir, "dist")
	_, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       out,
		Gallery:      "nope",
		Versions:     []Version{{Slug: "mini", Title: "Mini", Template: "mini"}},
	})
	if err == nil {
		t.Fatal("Build with missing gallery define = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "render gallery") {
		t.Errorf("error %q does not mention render gallery", err)
	}
}

// AssetsDir pointing at a path that does not exist makes copyDir's WalkDir
// invoke its callback with a non-nil error for the root, which copyDir returns
// and Build wraps as a "copy assets" error.
func TestBuild_AssetsCopyFailure(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"), `{{define "mini"}}hi{{end}}`)
	out := filepath.Join(dir, "dist")
	_, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		AssetsDir:    filepath.Join(dir, "does-not-exist"),
		OutDir:       out,
		Versions:     []Version{{Slug: "mini", Title: "Mini", Template: "mini"}},
	})
	if err == nil {
		t.Fatal("Build with nonexistent AssetsDir = nil error, want copy failure")
	}
	if !strings.Contains(err.Error(), "copy assets") {
		t.Errorf("error %q does not mention copy assets", err)
	}
}

// When OutDir is rooted under a regular file, MkdirAll inside writeFile cannot
// create the version subdirectory, so writeFile's mkdir branch returns an error
// that Build propagates.
func TestBuild_UnwritableOutputFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"), `{{define "mini"}}hi{{end}}`)

	// A regular file where a directory component must be — MkdirAll fails on it.
	blocker := filepath.Join(dir, "blocker")
	mustWrite(t, blocker, "i am a file, not a dir")

	out := filepath.Join(blocker, "sub")
	_, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       out,
		Versions:     []Version{{Slug: "mini", Title: "Mini", Template: "mini"}},
	})
	if err == nil {
		t.Fatal("Build with file-blocked OutDir = nil error, want mkdir failure")
	}
	if !strings.Contains(err.Error(), "mkdir for") {
		t.Errorf("error %q does not mention mkdir for", err)
	}
}

// copyFile's mkdir-for-destination branch: when the destination's parent path
// runs through a regular file, MkdirAll fails and copyFile returns that error.
func TestCopyFile_DestMkdirFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	mustWrite(t, src, "payload")

	blocker := filepath.Join(dir, "blocker")
	mustWrite(t, blocker, "file")

	// dst's parent dir would have to be created through the regular file.
	dst := filepath.Join(blocker, "nested", "out.txt")
	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile into file-blocked dst = nil error, want mkdir failure")
	}
}

// copyFile's open-source branch: a source path that does not exist makes
// os.Open fail and copyFile returns that error.
func TestCopyFile_OpenSourceFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "missing.txt")
	dst := filepath.Join(dir, "out.txt")
	err := copyFile(src, dst)
	if err == nil {
		t.Fatal("copyFile of missing source = nil error, want open failure")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error %q is not an os.ErrNotExist", err)
	}
}

// copyFile's create-destination branch: when the destination path already
// exists as a directory, os.Create cannot truncate/open it as a file and
// copyFile returns that error.
func TestCopyFile_CreateDestFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	mustWrite(t, src, "payload")

	// dst is an existing directory, so os.Create fails.
	dst := filepath.Join(dir, "dstdir")
	mustMkdir(t, dst)

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile onto an existing directory = nil error, want create failure")
	}
}

// copyFile's io.Copy branch: a source that opens but cannot be read as a byte
// stream (a directory) makes io.Copy fail after both files are open.
func TestCopyFile_CopyFailure(t *testing.T) {
	dir := t.TempDir()
	// src is a directory: os.Open succeeds, but io.Copy reading from it fails.
	src := filepath.Join(dir, "srcdir")
	mustMkdir(t, src)
	dst := filepath.Join(dir, "out.txt")

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile reading from a directory = nil error, want copy failure")
	}
}

// writeFile's os.WriteFile branch: when the target path already exists as a
// directory, MkdirAll on its parent succeeds but WriteFile cannot write the
// file and writeFile returns a "write" error.
func TestWriteFile_WriteFailure(t *testing.T) {
	dir := t.TempDir()
	// The path we ask writeFile to write is itself an existing directory.
	target := filepath.Join(dir, "occupied")
	mustMkdir(t, target)

	err := writeFile(target, []byte("data"))
	if err == nil {
		t.Fatal("writeFile onto an existing directory = nil error, want write failure")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Errorf("error %q does not mention write", err)
	}
}

// The gallery page goes through writeFile too: making OutDir/index.html an
// existing directory forces that second writeFile to fail, exercising Build's
// gallery-write error return.
func TestBuild_GalleryWriteFailure(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "page.html"),
		`{{define "mini"}}hi{{end}}{{define "gallery"}}g{{end}}`)

	out := filepath.Join(dir, "dist")
	// Pre-create the gallery's destination as a directory so WriteFile fails,
	// while the per-version page (out/mini/index.html) still writes fine.
	mustMkdir(t, filepath.Join(out, "index.html"))

	_, err := Build(Config{
		ContentPath:  "../../shared/content.json",
		TemplateGlob: filepath.Join(dir, "*.html"),
		OutDir:       out,
		Gallery:      "gallery",
		Versions:     []Version{{Slug: "mini", Title: "Mini", Template: "mini"}},
	})
	if err == nil {
		t.Fatal("Build with directory-blocked gallery output = nil error, want write failure")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Errorf("error %q does not mention write", err)
	}
}

// copyDir must skip non-regular entries (here a symlink) without copying them,
// exercising the !d.Type().IsRegular() return path.
func TestCopyDir_SkipsNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	mustMkdir(t, src)
	// A real regular file that must be copied.
	mustWrite(t, filepath.Join(src, "real.txt"), "keep")
	// A symlink (non-regular) that must be skipped.
	if err := os.Symlink(filepath.Join(src, "real.txt"), filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	dst := filepath.Join(dir, "dst")
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "real.txt")); err != nil {
		t.Errorf("regular file not copied: %v", err)
	}
	// The non-regular entry was skipped, so it must not exist in dst.
	if _, err := os.Lstat(filepath.Join(dst, "link.txt")); !os.IsNotExist(err) {
		t.Errorf("non-regular entry should have been skipped, stat err = %v", err)
	}
}

// copyDir's rel-error branch is unreachable through the real filesystem (both
// arguments are absolute and share a root), so it is exercised by overriding
// the relFn seam to fail. Behaviour is otherwise identical at runtime.
func TestCopyDir_RelError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	mustMkdir(t, src)
	mustWrite(t, filepath.Join(src, "a.txt"), "x")

	orig := relFn
	t.Cleanup(func() { relFn = orig })
	relFn = func(string, string) (string, error) {
		return "", errors.New("boom from relFn")
	}

	err := copyDir(src, filepath.Join(dir, "dst"))
	if err == nil {
		t.Fatal("copyDir with failing relFn = nil error, want rel failure")
	}
	if !strings.Contains(err.Error(), "boom from relFn") {
		t.Errorf("error %q does not surface the relFn failure", err)
	}
}
