package changedpkgs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImporterClosureChecked_NoRepoRootIsNotDerivable(t *testing.T) {
	t.Parallel()
	in := []string{"./internal/b/..."}
	closure, ok := ImporterClosureChecked("", in)
	if ok {
		t.Fatalf("ok = true with no repo root, want not derivable")
	}
	if !reflect.DeepEqual(closure.Patterns, in) {
		t.Errorf("Patterns = %v, want the inputs %v unwidened", closure.Patterns, in)
	}
}

func TestImporterClosureChecked_MissingGoModIsNotDerivable(t *testing.T) {
	t.Parallel()
	in := []string{"./internal/b/..."}
	closure, ok := ImporterClosureChecked(t.TempDir(), in)
	if ok {
		t.Fatalf("ok = true without a go.mod, want not derivable")
	}
	if !reflect.DeepEqual(closure.Patterns, in) {
		t.Errorf("Patterns = %v, want the inputs %v unwidened", closure.Patterns, in)
	}
}

func TestImporterClosureChecked_UnlistableModuleIsNotDerivable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	moduleDir := filepath.Join(root, "go")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	broken := "module example.com/broken\n\nrequire (\n"
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(broken), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if _, ok := ImporterClosureChecked(root, []string{"./internal/b/..."}); ok {
		t.Fatalf("ok = true when go list cannot read the module, want not derivable")
	}
}

func TestImporterClosureChecked_NoTargetDirsNeedsNoWidening(t *testing.T) {
	t.Parallel()
	in := []string{".cache/..."}
	closure, ok := ImporterClosureChecked(t.TempDir(), in)
	if !ok {
		t.Fatalf("ok = false for a pattern with no target directory, want derivable")
	}
	if !reflect.DeepEqual(closure.Patterns, in) {
		t.Errorf("Patterns = %v, want the inputs %v unwidened", closure.Patterns, in)
	}
}

func TestParseListedPkg_ReadsOneGoListLineAndRefusesAMalformedOne(t *testing.T) {
	t.Parallel()
	pkg, ok := parseListedPkg("m/a|m/b m/c|m/d m/e|2 1 0")
	if !ok || pkg.path != "m/a" || len(pkg.transitiveDeps) != 2 || len(pkg.testImports) != 2 || !pkg.testable {
		t.Fatalf("well-formed line = %+v ok=%v", pkg, ok)
	}
	if pkg, _ := parseListedPkg("m/tagonly|||0 0 0"); pkg.testable {
		t.Errorf("a package with no Go files in the default build context is not testable: %+v", pkg)
	}
	for _, malformed := range []string{"", "m/a|only|three", "|m/b||1 0 0"} {
		if pkg, ok := parseListedPkg(malformed); ok {
			t.Errorf("malformed line %q must be refused, got %+v", malformed, pkg)
		}
	}
}
