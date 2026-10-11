//go:build integration

package codequality

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

const unparseableGo = "package sample\n\nfunc Broken( {\n"

func writeGoFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFormatGoFiles_FixesTheParseableFilesAndLeavesTheUnparseableOneForTheGate(t *testing.T) {
	dir := t.TempDir()
	dirty := writeGoFile(t, dir, "a_dirty.go", dirtyGo)
	clean := writeGoFile(t, dir, "b_clean.go", cleanGo)
	broken := writeGoFile(t, dir, "c_broken.go", unparseableGo)
	simplifiable := writeGoFile(t, dir, "d_simplify.go", "package sample\n\nvar s = []struct{ a int }{struct{ a int }{1}}\n")

	fixed, err := FormatGoFiles(dir)

	if err != nil {
		t.Fatalf("FormatGoFiles: %v", err)
	}
	if want := []string{dirty, simplifiable}; !slices.Equal(fixed, want) {
		t.Fatalf("fixed = %q, want %q (never the parse-error entry, never the clean file)", fixed, want)
	}
	if got := readFile(t, dirty); got != cleanGo {
		t.Fatalf("dirty file = %q, want it rewritten to %q", got, cleanGo)
	}
	if got := readFile(t, simplifiable); got != "package sample\n\nvar s = []struct{ a int }{{1}}\n" {
		t.Fatalf("simplifiable file = %q, want the -s rewrite", got)
	}
	if got := readFile(t, broken); got != unparseableGo {
		t.Fatalf("unparseable file changed to %q", got)
	}
	if got := readFile(t, clean); got != cleanGo {
		t.Fatalf("clean file changed to %q", got)
	}
}

func TestFormatGoFiles_OnlyUnparseableFilesIsANoOp(t *testing.T) {
	dir := t.TempDir()
	broken := writeGoFile(t, dir, "broken.go", unparseableGo)

	fixed, err := FormatGoFiles(dir)

	if err != nil || fixed != nil {
		t.Fatalf("FormatGoFiles = %q, %v; want nil, nil", fixed, err)
	}
	if got := readFile(t, broken); got != unparseableGo {
		t.Fatalf("unparseable file changed to %q", got)
	}
}

func TestFormatGoFiles_MissingGofmtIsAnError(t *testing.T) {
	dir := t.TempDir()
	dirty := writeGoFile(t, dir, "dirty.go", dirtyGo)
	t.Setenv("PATH", t.TempDir())

	fixed, err := FormatGoFiles(dir)

	if !errors.Is(err, exec.ErrNotFound) || fixed != nil {
		t.Fatalf("FormatGoFiles = %q, %v; want nil and exec.ErrNotFound", fixed, err)
	}
	if got := readFile(t, dirty); got != dirtyGo {
		t.Fatalf("file changed without gofmt: %q", got)
	}
}

func TestFormatGoFiles_GofmtVanishingBeforeTheRewriteIsAnError(t *testing.T) {
	dir := t.TempDir()
	dirty := writeGoFile(t, dir, "dirty.go", dirtyGo)
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = \"-l\" ]; then echo '" + dirty + "'; /bin/rm -f \"${0%.fakecli}\"; fi\n"
	fakeclitest.Install(t, filepath.Join(bin, "gofmt"), script)
	t.Setenv("PATH", bin)

	fixed, err := FormatGoFiles(dir)

	if fixed != nil || err == nil || !strings.HasPrefix(err.Error(), "gofmt -w -s "+dir+": ") {
		t.Fatalf("FormatGoFiles = %q, %v; want nil and the gofmt -w -s error", fixed, err)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a launch failure, not an exit status", err)
	}
}

func TestFormatGoFiles_ACleanListingNeverInvokesTheRewrite(t *testing.T) {
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	script := "#!/bin/sh\necho \"$1\" >> '" + calls + "'\nif [ \"$1\" = \"-l\" ]; then echo 'x.go:1:1: expected declaration' >&2; exit 2; fi\n"
	fakeclitest.Install(t, filepath.Join(bin, "gofmt"), script)
	t.Setenv("PATH", bin)

	fixed, err := FormatGoFiles(t.TempDir())

	if err != nil || fixed != nil {
		t.Fatalf("FormatGoFiles = %q, %v; want nil, nil", fixed, err)
	}
	if got := readFile(t, calls); got != "-l\n" {
		t.Fatalf("gofmt invocations = %q, want only the -l listing", got)
	}
}
