package ciparity

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const DarwinSetFile = ".darwin-set"

const darwinSetHeader = `# The packages whose code or tests differ on macOS. The macOS CI job tests only
# these packages, and it runs cover-changed and cover-strict over them alone.
# "make -C go darwin-set" writes it from build constraints, runtime.GOOS
# checks and the callers of the macOS sandbox. Do not edit: a test fails when
# the file is stale.
`

var darwinSetTags = []string{"integration"}

const sandboxExec = "sandbox" + "-exec"

type readFunc func(string) ([]byte, error)

func DarwinPackages(moduleDir string) ([]string, error) {
	return darwinPackages(moduleDir, os.ReadFile)
}

func darwinPackages(moduleDir string, read readFunc) ([]string, error) {
	var pkgs []string
	walk := func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		dir := filepath.Join(moduleDir, filepath.FromSlash(rel))
		if rel != "." && skipsDarwinScan(dir, d.Name(), rel) {
			return filepath.SkipDir
		}
		sensitive, err := darwinSensitive(dir, read)
		if sensitive {
			pkgs = append(pkgs, "./"+rel)
		}
		return err
	}
	if err := fs.WalkDir(os.DirFS(moduleDir), ".", walk); err != nil {
		return nil, err
	}
	return pkgs, nil
}

func skipsDarwinScan(path, name, rel string) bool {
	if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || rel == "acs" {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

func darwinSensitive(dir string, read readFunc) (bool, error) {
	darwin, err := packageFiles(dir, "darwin")
	if err != nil || len(darwin) == 0 {
		return false, err
	}
	linux, err := packageFiles(dir, "linux")
	if err != nil {
		return false, err
	}
	if !slices.Equal(darwin, linux) {
		return true, nil
	}
	for _, name := range darwin {
		sensitive, err := fileBranchesOnDarwin(filepath.Join(dir, name), read)
		if err != nil || sensitive {
			return sensitive, err
		}
	}
	return false, nil
}

func packageFiles(dir, goos string) ([]string, error) {
	ctx := build.Default
	ctx.GOOS = goos
	ctx.GOARCH = "arm64"
	ctx.CgoEnabled = true
	ctx.BuildTags = darwinSetTags
	pkg, err := ctx.ImportDir(dir, 0)
	var noGo *build.NoGoError
	if errors.As(err, &noGo) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := slices.Concat(pkg.GoFiles, pkg.CgoFiles, pkg.TestGoFiles, pkg.XTestGoFiles)
	slices.Sort(names)
	return names, nil
}

func fileBranchesOnDarwin(path string, read readFunc) (bool, error) {
	data, err := read(path)
	if err != nil {
		return false, err
	}
	if !strings.HasSuffix(path, "_test.go") && bytes.Contains(data, []byte(sandboxExec)) {
		return true, nil
	}
	if namesDarwin(data) {
		return true, nil
	}
	return branchesOnDarwin(path, data)
}

func namesDarwin(data []byte) bool {
	header, _, _ := strings.Cut("\n"+string(data), "\npackage ")
	for _, line := range strings.Split(header, "\n") {
		if expr, err := constraint.Parse(strings.TrimSpace(line)); err == nil && positiveTag(expr, "darwin", true) {
			return true
		}
	}
	return false
}

func positiveTag(expr constraint.Expr, tag string, positive bool) bool {
	switch e := expr.(type) {
	case *constraint.NotExpr:
		return positiveTag(e.X, tag, !positive)
	case *constraint.AndExpr:
		return positiveTag(e.X, tag, positive) || positiveTag(e.Y, tag, positive)
	case *constraint.OrExpr:
		return positiveTag(e.X, tag, positive) || positiveTag(e.Y, tag, positive)
	default:
		leaf, ok := expr.(*constraint.TagExpr)
		return ok && positive && leaf.Tag == tag
	}
}

func branchesOnDarwin(path string, data []byte) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.SkipObjectResolution)
	if err != nil {
		return false, err
	}
	neutral := map[ast.Expr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if cmp, ok := n.(*ast.BinaryExpr); ok && (cmp.Op == token.EQL || cmp.Op == token.NEQ) {
			markNeutralComparison(neutral, cmp.X, cmp.Y)
			markNeutralComparison(neutral, cmp.Y, cmp.X)
		}
		return true
	})
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if expr, ok := n.(ast.Expr); ok && isRuntimeGOOS(expr) && !neutral[expr] {
			found = true
		}
		return !found
	})
	return found, nil
}

func markNeutralComparison(neutral map[ast.Expr]bool, goos, other ast.Expr) {
	lit, ok := other.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING || !isRuntimeGOOS(goos) {
		return
	}
	if value, err := strconv.Unquote(lit.Value); err == nil && value != "darwin" && value != "linux" {
		neutral[goos] = true
	}
}

func isRuntimeGOOS(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "GOOS" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "runtime"
}

func DarwinSetText(pkgs []string) string {
	return darwinSetHeader + strings.Join(pkgs, "\n") + "\n"
}

func DarwinMain(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("darwinset", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".", "module directory to scan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	pkgs, err := DarwinPackages(*dir)
	if err != nil {
		fmt.Fprintln(stderr, "darwinset:", err)
		return 1
	}
	if len(pkgs) == 0 {
		fmt.Fprintln(stderr, "darwinset: no darwin-sensitive package under", *dir)
		return 1
	}
	fmt.Fprint(stdout, DarwinSetText(pkgs))
	return 0
}

var ErrDarwinSetStale = errors.New("the darwin set file is stale")

func SyncDarwinSet(moduleDir string, write bool) (bool, error) {
	pkgs, err := DarwinPackages(moduleDir)
	if err != nil {
		return false, err
	}
	if len(pkgs) == 0 {
		return false, fmt.Errorf("no darwin-sensitive package under %s", moduleDir)
	}
	path := filepath.Join(moduleDir, DarwinSetFile)
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	want := DarwinSetText(pkgs)
	switch {
	case string(current) == want:
		return false, nil
	case !write:
		return false, fmt.Errorf("%w: go/%s; run `evolve ci darwin-set generate`", ErrDarwinSetStale, DarwinSetFile)
	}
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
