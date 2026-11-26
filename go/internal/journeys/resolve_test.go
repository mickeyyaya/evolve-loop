package journeys

import (
	"errors"
	"reflect"
	"testing"
)

func TestResolve_FindsTheFileThatDeclaresTheTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go/internal/x/one_test.go", "package x\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n")
	writeFile(t, root, "go/internal/x/two_test.go", "package x\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) {}\n")
	file, err := Resolve(root, "internal/x.TestTwo")
	if err != nil || file != "go/internal/x/two_test.go" {
		t.Errorf("Resolve = %q, %v, want go/internal/x/two_test.go", file, err)
	}
}

func TestResolve_RefusesWhatOnlyLooksLikeTheTest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go/internal/x/text_test.go", "package x\n\n// func TestInComment(t *testing.T) {\nvar s = \"func TestInString(t *testing.T) {\"\n")
	writeFile(t, root, "go/internal/x/method_test.go", "package x\n\ntype S struct{}\n\nfunc (S) TestOnMethod() {}\n")
	writeFile(t, root, "go/internal/x/gated_test.go", "//go:build integration\n\npackage x\n\nimport \"testing\"\n\nfunc TestGated(t *testing.T) {}\n")
	writeFile(t, root, "go/internal/x/mac_darwin_test.go", "package x\n\nimport \"testing\"\n\nfunc TestMacOnly(t *testing.T) {}\n")
	writeFile(t, root, "go/internal/x/broken_test.go", "package x\n\nfunc TestBroken(\n")
	writeFile(t, root, "go/internal/x/prod.go", "package x\n\nfunc TestInProduction() {}\n")
	writeFile(t, root, "go/internal/x/lin_linux_test.go", "package x\n\nimport \"testing\"\n\nfunc TestLinuxOnly(t *testing.T) {}\n")
	if _, err := Resolve(root, "internal/x.TestLinuxOnly"); err != nil {
		t.Errorf("a linux-only test resolves on every host: %v", err)
	}
	tests := map[string]error{
		"internal/x.TestInComment":     ErrNotFound,
		"internal/x.TestInString":      ErrNotFound,
		"internal/x.TestOnMethod":      ErrNotFound,
		"internal/x.TestInProduction":  ErrNotFound,
		"internal/x.TestGated":         ErrTagGated,
		"internal/x.TestMacOnly":       ErrTagGated,
		"internal/x.TestAbsent":        ErrNotFound,
		"internal/absent.TestAnything": ErrNotFound,
	}
	for test, want := range tests {
		if _, err := Resolve(root, test); !errors.Is(err, want) {
			t.Errorf("Resolve(%q) err = %v, want %v", test, err, want)
		}
	}
}

func TestResolve_RefusesATestNameWithNoPackage(t *testing.T) {
	if _, err := Resolve(t.TempDir(), "TestA"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUnresolved_NamesEachActiveRowWhoseTestIsGone(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go/internal/x/one_test.go", "package x\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n")
	rows := []Row{
		{ID: "ok", Test: "internal/x.TestOne"},
		{ID: "gone", Test: "internal/x.TestGone"},
		{ID: "retired", Test: "internal/x.TestRetiredAway", Retired: "removed"},
	}
	got := Unresolved(root, rows)
	want := []string{"gone: " + ErrNotFound.Error() + ": internal/x.TestGone"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unresolved = %v, want %v", got, want)
	}
}
