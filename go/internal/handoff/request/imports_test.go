package request_test

import (
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const corePackage = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func TestPackageDoesNotImportCore(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no source files found: %v", err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: bad import literal %s", file, spec.Path.Value)
			}
			if path == corePackage || strings.HasPrefix(path, corePackage+"/") {
				t.Errorf("%s imports %s", file, path)
			}
		}
	}
}

func TestPackageDoesNotReachCoreThroughItsDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == corePackage {
			t.Errorf("the package reaches %s through its dependencies", corePackage)
		}
	}
}
