package apicover

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeGoPackage(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/mod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "pkg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package pkg\n\n// Exported is a fixture symbol.\nfunc Exported() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tsrc := "package pkg\n\nimport \"testing\"\n\nfunc TestExported(t *testing.T) { Exported() }\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg_test.go"), []byte(tsrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func cancelledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestRun_CancelledContext_ReturnsMeasurementError(t *testing.T) {
	dir := writeGoPackage(t)
	var out bytes.Buffer

	code, err := Run(cancelledCtx(), Config{Dirs: []string{dir}}, &out)

	if code != 2 {
		t.Errorf("cancelled ctx: code = %d, want 2 (measurement failure)", code)
	}
	if err == nil || !contextError(err) {
		t.Errorf("cancelled ctx: err = %v, want a context error (errors.Is Canceled/DeadlineExceeded)", err)
	}
}

func TestRun_LiveContext_Unchanged(t *testing.T) {
	dir := writeGoPackage(t)
	var out bytes.Buffer

	code, err := Run(context.Background(), Config{Dirs: []string{dir}}, &out)

	if err != nil {
		t.Fatalf("live ctx: err = %v, want nil", err)
	}
	if code != 0 {
		t.Errorf("live ctx: code = %d, want 0 (warning-only default)", code)
	}
}

func TestEnumerate_CancelledContext_Stops(t *testing.T) {
	dir := writeGoPackage(t)

	if _, err := Enumerate(cancelledCtx(), dir); err == nil || !contextError(err) {
		t.Errorf("Enumerate cancelled ctx: err = %v, want a context error", err)
	}
}

func TestNamesReferencedInTests_CancelledContext_Stops(t *testing.T) {
	dir := writeGoPackage(t)

	if _, err := NamesReferencedInTests(cancelledCtx(), dir); err == nil || !contextError(err) {
		t.Errorf("NamesReferencedInTests cancelled ctx: err = %v, want a context error", err)
	}
}

func contextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
