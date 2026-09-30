//go:build acs

package envtaint

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
)

var externalEnvAllowlist = map[string]bool{
	"CI": true, "HOME": true, "CODEX_HOME": true, "XDG_RUNTIME_DIR": true,
	"TMUX_TMPDIR":  true,
	"GITHUB_TOKEN": true,
	"CYCLE":        true, "SHIP_CLASS": true, "WORKSPACE_PATH": true, "WORKTREE_PATH": true,
	"PROFILE_PATH": true, "PROMPT_FILE": true, "PROMPT_FILE_OVERRIDE": true, "MODEL_TIER_HINT": true,
	"CG_ATTEST_DIR": true, "CG_TEST_FORCE_MISSING": true, "CG_TEST_INSTALL": true,
	"FAKE_CLI_AUDIT_VERDICT": true,
}

func collectGetenvKeys(fset *token.FileSet, pkgName string, files []*ast.File, keys map[string]bool) {
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	conf := &types.Config{Importer: stubImporter{}, Error: func(error) {}}
	_, _ = conf.Check(pkgName, fset, files, info)

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isOSGetenvSelector(call.Fun) {
				return true
			}
			if tv, ok := info.Types[call.Args[0]]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
				keys[constant.StringVal(tv.Value)] = true
			}
			return true
		})
	}
}

func isOSGetenvSelector(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != "os" {
		return false
	}
	return sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv"
}

func GetenvKeysFromSrc(src string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	keys := map[string]bool{}
	collectGetenvKeys(fset, file.Name.Name, []*ast.File{file}, keys)
	return sortedKeys(keys), nil
}

func GetenvConstKeys(goRoot string) (keys []string, skipped []string, err error) {
	set := map[string]bool{}
	skipped, err = forEachProductionPackage(goRoot, func(fset *token.FileSet, pkgName string, files []*ast.File) {
		collectGetenvKeys(fset, pkgName, files, set)
	})
	if err != nil {
		return nil, skipped, err
	}
	return sortedKeys(set), skipped, nil
}
