package apicover

// run_ctx_test.go — apicover-inprocess-ctx-timeout: the audit's in-process
// enforce gate (ciparity.go apicoverEnforceChangedDefault) bounds its exec
// pre-steps with apicoverTimeout, but the folded-in measurement itself
// (Run → Enumerate/NamesReferencedInTests AST walks) took no context, so a
// wedged measurement (pathological package, hung filesystem) escaped the
// gate's own deadline. These tests pin the ctx plumbing: an already-cancelled
// context stops the measurement at the next file/dir boundary and surfaces
// ctx.Err() as the code-2 measurement error.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeGoPackage creates a minimal parseable package dir with one exported
// symbol and one test file naming it, under a module root (go.mod), so a
// non-cancelled Run has real work to do at every stage ctx must interrupt.
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

// TestRun_CancelledContext_ReturnsMeasurementError: a cancelled ctx must stop
// Run before it measures anything, returning the code-2 measurement-failure
// contract with ctx.Err() in the chain — the signal ciparity uses to fail OPEN
// (infra WARN) instead of FAIL (real finding).
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

// TestRun_LiveContext_Unchanged: the ctx thread must not change a healthy
// measurement — same clean run the pre-ctx Run produced.
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

// TestEnumerate_CancelledContext_Stops: Enumerate checks ctx at each file
// boundary, so a cancelled ctx surfaces instead of parsing the whole dir.
func TestEnumerate_CancelledContext_Stops(t *testing.T) {
	dir := writeGoPackage(t)

	if _, err := Enumerate(cancelledCtx(), dir); err == nil || !contextError(err) {
		t.Errorf("Enumerate cancelled ctx: err = %v, want a context error", err)
	}
}

// TestNamesReferencedInTests_CancelledContext_Stops: same file-boundary check
// for the _test.go AST walk.
func TestNamesReferencedInTests_CancelledContext_Stops(t *testing.T) {
	dir := writeGoPackage(t)

	if _, err := NamesReferencedInTests(cancelledCtx(), dir); err == nil || !contextError(err) {
		t.Errorf("NamesReferencedInTests cancelled ctx: err = %v, want a context error", err)
	}
}

// contextError reports whether err chains to a context cancellation/deadline.
func contextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
