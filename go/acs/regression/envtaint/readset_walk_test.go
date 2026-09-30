//go:build acs

package envtaint

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadSet_PackageGroupingAndSkips(t *testing.T) {
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	pkg := filepath.Join(goDir, "internal", "demo")
	mustMkdir(t, pkg)

	mustWrite(t, filepath.Join(pkg, "keys.go"), `package demo

const EnvFoo = "EVOLVE_" + "FOO"
`)
	mustWrite(t, filepath.Join(pkg, "reader.go"), `package demo

import "os"

func A() string { return os.Getenv(EnvFoo) }
func B() string { return os.Getenv("EVOLVE_BAR") }
`)
	mustWrite(t, filepath.Join(pkg, "reader_test.go"), `package demo

import "os"

func tImpostor() string { return os.Getenv("EVOLVE_TESTONLY") }
`)
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
