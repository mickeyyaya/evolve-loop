//go:build acs

package envtaint

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
)

func TestReadSet_PackageGroupingAndSkips(t *testing.T) {
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	pkg := filepath.Join(goDir, "internal", "demo")
	inboxtest.MkdirAll(t, pkg)

	inboxtest.WriteFile(t, filepath.Join(pkg, "keys.go"), `package demo

const EnvFoo = "EVOLVE_" + "FOO"
`)
	inboxtest.WriteFile(t, filepath.Join(pkg, "reader.go"), `package demo

import "os"

func A() string { return os.Getenv(EnvFoo) }
func B() string { return os.Getenv("EVOLVE_BAR") }
`)
	inboxtest.WriteFile(t, filepath.Join(pkg, "reader_test.go"), `package demo

import "os"

func tImpostor() string { return os.Getenv("EVOLVE_TESTONLY") }
`)
	ipc := filepath.Join(goDir, "internal", "ipcenv")
	inboxtest.MkdirAll(t, ipc)
	inboxtest.WriteFile(t, filepath.Join(ipc, "ipcenv.go"), `package ipcenv

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
