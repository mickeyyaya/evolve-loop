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

// externalEnvAllowlist is the pinned set of permitted NON-EVOLVE_ constant
// os.Getenv / os.LookupEnv keys: standard external environment variables,
// legacy-named internal IPC / subprocess handoffs, commit-gate variables, and a
// test-fake injection point. Operator dials use the reserved EVOLVE_ prefix; the
// anti-rename gate fails on any non-EVOLVE_ key NOT listed here, so a dial
// renamed out of the EVOLVE_ namespace to shrink the registry metric is caught.
//
// This list is a Pillar-1 protected surface (it lives under go/acs/regression/),
// so an autonomous cycle cannot add to it to dodge — an addition requires a
// human-gated --class manual edit and review.
var externalEnvAllowlist = map[string]bool{
	// Standard external environment.
	"CI": true, "HOME": true, "CODEX_HOME": true, "XDG_RUNTIME_DIR": true,
	// GitHub-provided token: read by release-verify-binaries to authenticate the
	// release-asset query (CI / rate limits). Standard external var, not a dial.
	"GITHUB_TOKEN": true,
	// Legacy-named internal IPC / subprocess handoffs (not operator dials).
	"CYCLE": true, "SHIP_CLASS": true, "WORKSPACE_PATH": true, "WORKTREE_PATH": true,
	"PROFILE_PATH": true, "PROMPT_FILE": true, "PROMPT_FILE_OVERRIDE": true, "MODEL_TIER_HINT": true,
	// Commit-gate.
	"CG_ATTEST_DIR": true, "CG_TEST_FORCE_MISSING": true, "CG_TEST_INSTALL": true,
	// Test-fake CLI verdict injection (read by the in-binary fake adapter).
	"FAKE_CLI_AUDIT_VERDICT": true,
}

// collectGetenvKeys best-effort type-checks a package's files together and adds
// every compile-time-constant os.Getenv / os.LookupEnv key (at ANY prefix) it
// folds into keys. os.Getenv is matched syntactically (selector os.Getenv), so
// the walk needs no build graph; the argument is folded by the type-checker, so
// a split-const key — including a rename dodge "FO"+"O" — is still caught.
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

// isOSGetenvSelector reports whether fun is the selector os.Getenv or
// os.LookupEnv, matched syntactically (X is the identifier "os"). A shadowed os
// is implausible in production code and would itself be suspicious.
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

// GetenvKeysFromSrc returns the sorted, de-duplicated constant os.Getenv /
// os.LookupEnv keys in a single source snippet — the unit-test entry point for
// collectGetenvKeys.
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

// GetenvConstKeys returns the sorted union of constant os.Getenv / os.LookupEnv
// keys (any prefix) across production Go under goRoot. The anti-rename invariant
// requires every such key to be EVOLVE_-prefixed or in externalEnvAllowlist.
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
