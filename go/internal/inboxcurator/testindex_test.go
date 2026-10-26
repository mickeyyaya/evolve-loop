package inboxcurator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexTests_FindsEveryTestFuncOutsideTestdataAndVendor(t *testing.T) {
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "internal", "a", "a_test.go"), "package a\n\nfunc TestAlpha(t *testing.T) {}\nfunc BenchmarkBeta(b *testing.B) {}\nfunc helper() {}\n")
	writeFile(t, filepath.Join(goDir, "internal", "a", "a.go"), "package a\n\nfunc TestNotATestFile() {}\n")
	writeFile(t, filepath.Join(goDir, "internal", "a", "testdata", "x_test.go"), "func TestFixture(t *testing.T) {}\n")
	writeFile(t, filepath.Join(goDir, "vendor", "v", "v_test.go"), "func TestVendored(t *testing.T) {}\n")

	got, err := IndexTests(goDir)

	if err != nil || !got["TestAlpha"] || !got["BenchmarkBeta"] || len(got) != 2 {
		t.Fatalf("IndexTests = %v, %v; want only TestAlpha and BenchmarkBeta", got, err)
	}
}

func TestIndexTests_AProjectWithNoGoTreeHasNoTests(t *testing.T) {
	got, err := IndexTests(filepath.Join(t.TempDir(), "absent"))

	if err != nil || len(got) != 0 {
		t.Fatalf("IndexTests of an absent dir = %v, %v; want an empty index", got, err)
	}
}

func TestIndexTests_AnUnreadableTestFileIsAnError(t *testing.T) {
	goDir := t.TempDir()
	file := filepath.Join(goDir, "a_test.go")
	writeFile(t, file, "func TestA(t *testing.T) {}\n")
	if err := os.Chmod(file, 0o000); err != nil {
		t.Fatal(err)
	}

	if _, err := IndexTests(goDir); err == nil {
		t.Fatal("IndexTests with an unreadable test file = nil error, want an error")
	}
}

func TestIndexTests_AnUnreadableDirectoryIsAnError(t *testing.T) {
	goDir := t.TempDir()
	dir := filepath.Join(goDir, "internal")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if _, err := IndexTests(goDir); err == nil {
		t.Fatal("IndexTests with an unreadable directory = nil error, want an error")
	}
}
