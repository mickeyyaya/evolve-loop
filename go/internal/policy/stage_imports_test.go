package policy

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPolicyPackageImportsNeitherLedgerNorScorers(t *testing.T) {
	t.Parallel()
	banned := []string{"/adapters/ledger", "/stageevidence", "/gateregistry", "/gatecontract"}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no go files: %v", err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range banned {
				if strings.HasSuffix(path, bad) {
					t.Errorf("%s imports %s; the policy package must not depend on the ledger or the scorers", name, path)
				}
			}
		}
	}
}
