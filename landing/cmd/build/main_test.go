package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfig verifies the build configuration: five versions with the expected
// slugs, the gallery name, and the content/template/output paths.
func TestConfig(t *testing.T) {
	cfg := config()

	if len(cfg.Versions) != 5 {
		t.Fatalf("config(): got %d versions, want 5", len(cfg.Versions))
	}

	wantSlugs := []string{"luminous", "noir", "editorial", "blueprint", "aurora-glass"}
	for i, want := range wantSlugs {
		if got := cfg.Versions[i].Slug; got != want {
			t.Errorf("config(): version[%d].Slug = %q, want %q", i, got, want)
		}
	}

	if cfg.Gallery != "gallery" {
		t.Errorf("config(): Gallery = %q, want %q", cfg.Gallery, "gallery")
	}
	if cfg.ContentPath == "" {
		t.Error("config(): ContentPath is empty, want a path set")
	}
	if cfg.TemplateGlob == "" {
		t.Error("config(): TemplateGlob is empty, want a glob set")
	}
	if cfg.OutDir == "" {
		t.Error("config(): OutDir is empty, want a path set")
	}
}

// chdir changes into dir for the duration of the test and restores the original
// working directory on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore cwd %q: %v", orig, err)
		}
	})
}

// TestRunSuccess runs the build against the real landing module root (../..
// from this package), where shared/, templates/, and assets/ exist. It builds
// into the real dist/, which is the intended behavior.
func TestRunSuccess(t *testing.T) {
	// The cmd/build package lives at <root>/cmd/build, so the module root is "../..".
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	chdir(t, root)

	var out bytes.Buffer
	if code := run(&out); code != 0 {
		t.Fatalf("run() returned %d, want 0; output:\n%s", code, out.String())
	}
	if got := out.String(); !strings.Contains(got, "built") {
		t.Errorf("run() output = %q, want it to contain %q", got, "built")
	}
}

// TestRunFailure runs the build from an empty temp directory with no shared/ or
// templates/, so buildsite.Build fails and run() must return 1.
func TestRunFailure(t *testing.T) {
	chdir(t, t.TempDir())

	var out bytes.Buffer
	if code := run(&out); code != 1 {
		t.Fatalf("run() returned %d, want 1 (build should fail without shared/ or templates/)", code)
	}
}

// TestMain_DelegatesExitCode verifies main() forwards run()'s exit code via the
// osExit seam. From an empty temp dir the build fails, so main() must exit 1.
func TestMain_DelegatesExitCode(t *testing.T) {
	chdir(t, t.TempDir())
	prev := osExit
	t.Cleanup(func() { osExit = prev })
	var got int
	osExit = func(code int) { got = code }

	main()

	if got != 1 {
		t.Errorf("main() exit code = %d, want 1", got)
	}
}
