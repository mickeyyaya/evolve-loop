//go:build integration

package acsassert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type formattingT struct{ errs []string }

func (f *formattingT) Errorf(format string, args ...any) {
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}

func (f *formattingT) Helper() {}

func writeSubPackageModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":               "module example.test/contract\n\ngo 1.23\n",
		"sub/contract_test.go": "package sub\nimport \"testing\"\nfunc TestExpected(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGoTests_ARelativePackageMatchesTheImportPathOfItsEvents(t *testing.T) {
	dir := writeSubPackageModule(t)
	for _, pkg := range []string{"./sub", "example.test/contract/sub"} {
		ft := &fakeT{}
		if !GoTests(ft, GoTestSpec{Dir: dir, Package: pkg, Pattern: "^TestExpected$", Names: []string{"TestExpected"}}) {
			t.Fatalf("Package %q: GoTests = false with %v; go test -json reports the import path example.test/contract/sub, so a passing test must count", pkg, ft.errs)
		}
	}
}

func TestGoTests_APackageThatDoesNotResolveFailsNamingIt(t *testing.T) {
	dir := writeSubPackageModule(t)
	ft := &formattingT{}
	if GoTests(ft, GoTestSpec{Dir: dir, Package: "./absent", Pattern: "^TestExpected$", Names: []string{"TestExpected"}}) {
		t.Fatal("GoTests = true for a package that does not exist")
	}
	if got := strings.Join(ft.errs, "\n"); !strings.Contains(got, "one import path") || !strings.Contains(got, "./absent") {
		t.Fatalf("the go list resolution must reject ./absent before any test runs, naming it; got %v", ft.errs)
	}
}

func TestGoTests_APatternOfSeveralPackagesFailsBecauseOnlyOneImportPathIsChecked(t *testing.T) {
	dir := writeSubPackageModule(t)
	if err := os.MkdirAll(filepath.Join(dir, "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other", "other_test.go"), []byte("package other\nimport \"testing\"\nfunc TestExpected(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ft := &formattingT{}
	if GoTests(ft, GoTestSpec{Dir: dir, Package: "./...", Pattern: "^TestExpected$", Names: []string{"TestExpected"}}) {
		t.Fatal("GoTests = true for ./..., which names two packages; the pass of one package cannot stand for the other")
	}
	if len(ft.errs) == 0 || !strings.Contains(strings.Join(ft.errs, "\n"), "one import path") {
		t.Fatalf("want the go list resolution failure; got %v", ft.errs)
	}
}

type formattingDeadlineT struct {
	formattingT
	deadline time.Time
}

func (f *formattingDeadlineT) Deadline() (time.Time, bool) { return f.deadline, true }

func TestGoTests_AnExpiredDeadlineStopsTheImportPathResolution(t *testing.T) {
	dir := writeSubPackageModule(t)
	ft := &formattingDeadlineT{deadline: time.Now().Add(-time.Second)}
	if GoTests(ft, GoTestSpec{Dir: dir, Package: "./sub", Pattern: "^TestExpected$", Names: []string{"TestExpected"}}) {
		t.Fatal("GoTests = true past an expired caller deadline")
	}
	if got := strings.Join(ft.errs, "\n"); !strings.Contains(got, "one import path") {
		t.Fatalf("the expired deadline must stop the go list step first; got %v", ft.errs)
	}
}
