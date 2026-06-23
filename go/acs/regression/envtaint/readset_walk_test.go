//go:build acs

package envtaint

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestReadSet_PackageGroupingAndSkips proves the walk (1) resolves a constant
// DEFINED in one file and READ in another of the same package — which per-file
// checking would miss — (2) skips _test.go files, and (3) skips the ipcenv SSOT
// directory.
func TestReadSet_PackageGroupingAndSkips(t *testing.T) {
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	pkg := filepath.Join(goDir, "internal", "demo")
	mustMkdir(t, pkg)

	// keys.go DEFINES the constant; reader.go (same package) READS it.
	mustWrite(t, filepath.Join(pkg, "keys.go"), `package demo

const EnvFoo = "EVOLVE_" + "FOO"
`)
	mustWrite(t, filepath.Join(pkg, "reader.go"), `package demo

import "os"

func A() string { return os.Getenv(EnvFoo) }
func B() string { return os.Getenv("EVOLVE_BAR") }
`)
	// A _test.go reader must NOT count toward the production read-set.
	mustWrite(t, filepath.Join(pkg, "reader_test.go"), `package demo

import "os"

func tImpostor() string { return os.Getenv("EVOLVE_TESTONLY") }
`)
	// ipcenv is the IPC SSOT — skipped entirely.
	ipc := filepath.Join(goDir, "internal", "ipcenv")
	mustMkdir(t, ipc)
	mustWrite(t, filepath.Join(ipc, "ipcenv.go"), `package ipcenv

const FleetKey = "EVOLVE_FLEET"
`)

	got, skipped, err := ReadSet(goDir)
	if err != nil {
		t.Fatalf("ReadSet: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("unexpected skipped files: %v", skipped)
	}
	want := []string{"EVOLVE_BAR", "EVOLVE_FOO"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSet = %v, want %v (cross-file const EnvFoo must resolve; "+
			"_test.go and ipcenv must be skipped)", got, want)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
