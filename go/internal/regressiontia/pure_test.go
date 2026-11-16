package regressiontia

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
)

const pureModule = "example.com/pure"

func purityModuleDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "go")
	writeModuleFile(t, dir, "go.mod", "module "+pureModule+"\n\ngo 1.21\n")
	writeModuleFile(t, dir, "internal/leaf/leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	writeModuleFile(t, dir, "internal/leaf/leaf_test.go", "package leaf\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {\n\tif Value() != 1 {\n\t\tt.Fatal(\"value\")\n\t}\n}\n")
	writeModuleFile(t, dir, "internal/user/user.go", "package user\n\nimport \""+pureModule+"/internal/leaf\"\n\nfunc Value() int { return leaf.Value() }\n")
	writeModuleFile(t, dir, "internal/viatest/viatest.go", "package viatest\n\nfunc Value() int { return 1 }\n")
	writeModuleFile(t, dir, "internal/viatest/viatest_test.go", "package viatest\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestValue(t *testing.T) {\n\t_ = os.Getpagesize()\n}\n")
	for pkg, hatch := range map[string]string{
		"reados":      "os",
		"readexec":    "os/exec",
		"readuser":    "os/user",
		"readsyscall": "syscall",
		"readnet":     "net",
		"readnethttp": "net/http",
	} {
		writeModuleFile(t, dir, "internal/"+pkg+"/"+pkg+".go", "package "+pkg+"\n\nimport _ \""+hatch+"\"\n")
	}
	writeModuleFile(t, dir, "internal/viaos/viaos.go", "package viaos\n\nimport _ \""+pureModule+"/internal/reados\"\n")
	writeModuleFile(t, dir, "internal/deep/deep.go", "package deep\n\nimport _ \""+pureModule+"/internal/viaos\"\n")
	return dir
}

func TestPure_ProvesAPackageWhoseClosureReachesNoEscapeHatch(t *testing.T) {
	dir := purityModuleDir(t)
	for _, pkg := range []string{"leaf", "user"} {
		if !Pure(dir, pureModule+"/internal/"+pkg) {
			t.Errorf("Pure(%s) = false: its module-local closure reaches no escape hatch, and the standard library reached through testing must not taint it", pkg)
		}
	}
}

func TestPure_RefusesEveryEscapeHatchDirectAndTransitive(t *testing.T) {
	dir := purityModuleDir(t)
	cases := map[string]string{
		"os":                  "reados",
		"os/exec":             "readexec",
		"os/user":             "readuser",
		"syscall":             "readsyscall",
		"net":                 "readnet",
		"net/http":            "readnethttp",
		"one hop":             "viaos",
		"two hops":            "deep",
		"a hatch in the test": "viatest",
	}
	for name, pkg := range cases {
		if Pure(dir, pureModule+"/internal/"+pkg) {
			t.Errorf("Pure(%s) = true for the package that reaches %s", pkg, name)
		}
	}
}

func failingList(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return -1, errors.New("go list is gone")
}

func TestPure_FailsClosedWhenItCannotProve(t *testing.T) {
	dir := purityModuleDir(t)
	if Pure(dir, pureModule+"/internal/ghost") {
		t.Error("a package that is not in the module is not proven pure")
	}
	if Pure("", pureModule+"/internal/leaf") {
		t.Error("an empty module directory proves nothing")
	}
	if Pure(dir, "") {
		t.Error("an empty package proves nothing")
	}
	if Pure(t.TempDir(), pureModule+"/internal/leaf") {
		t.Error("a directory that is no module proves nothing")
	}
	original := depsRunner
	t.Cleanup(func() { depsRunner = original })
	depsRunner = failingList
	if Pure(dir, pureModule+"/internal/leaf") {
		t.Error("a failed go list proves nothing, even for a package that is pure")
	}
}
