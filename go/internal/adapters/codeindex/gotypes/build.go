package gotypes

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type listedPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	GoFiles    []string
	ImportMap  map[string]string
	Module     *struct{ Main bool }
}

func (p listedPackage) inModule() bool {
	return !p.Standard && p.Module != nil && p.Module.Main
}

type loader struct {
	fset     *token.FileSet
	listed   map[string]listedPackage
	done     map[string]*types.Package
	index    *indexer
	warnings []string
}

type buildEnv struct {
	run    sysexec.RunFunc
	env    []string
	encode encodeFunc
	warn   io.Writer
}

func Build(moduleDir, out string) error {
	return build(context.Background(), buildEnv{run: sysexec.DefaultRunner, env: os.Environ(), encode: encodeGob, warn: os.Stderr}, moduleDir, out)
}

func build(ctx context.Context, be buildEnv, moduleDir, out string) error {
	dir, err := filepath.EvalSymlinks(moduleDir)
	if err != nil {
		return fmt.Errorf("gotypes: the module dir: %w", err)
	}
	listed, err := listPackages(ctx, be.run, be.env, dir)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	l := &loader{fset: fset, listed: map[string]listedPackage{}, done: map[string]*types.Package{}, index: newIndexer(fset, checkoutRoot(dir), modulePaths(listed))}
	for _, p := range listed {
		l.listed[p.ImportPath] = p
	}
	for _, p := range listed {
		l.checkListed(p)
	}
	for _, w := range l.warnings {
		fmt.Fprintf(be.warn, "gotypes: WARN %s\n", w)
	}
	return writeIndex(out, l.index.finish(l.warnings), be.encode)
}

func modulePaths(listed []listedPackage) map[string]bool {
	paths := map[string]bool{}
	for _, p := range listed {
		if p.inModule() {
			paths[strings.SplitN(p.ImportPath, " ", 2)[0]] = true
		}
	}
	return paths
}

func listPackages(ctx context.Context, run sysexec.RunFunc, base []string, dir string) ([]listedPackage, error) {
	var stdout, stderr strings.Builder
	env := slices.Concat(base, []string{"CGO_ENABLED=0"})
	code, err := run(ctx, "go", dir, []string{"list", "-buildvcs=false", "-json", "-deps", "-test", "./..."}, env, nil, &stdout, &stderr)
	if err != nil {
		return nil, fmt.Errorf("gotypes: run go list in %s: %w", dir, err)
	}
	if code != 0 {
		return nil, fmt.Errorf("gotypes: go list in %s: exit %d: %s", dir, code, strings.TrimSpace(stderr.String()))
	}
	var out []listedPackage
	dec := json.NewDecoder(strings.NewReader(stdout.String()))
	for dec.More() {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("gotypes: decode the go list answer: %w", err)
		}
		if !strings.HasSuffix(p.ImportPath, ".test") {
			out = append(out, p)
		}
	}
	return out, nil
}

func (l *loader) check(path string) (*types.Package, error) {
	p, ok := l.listed[path]
	if !ok {
		return nil, fmt.Errorf("gotypes: go list did not list the import %s", path)
	}
	return l.checkListed(p), nil
}

func (l *loader) checkListed(p listedPackage) *types.Package {
	if pkg, ok := l.done[p.ImportPath]; ok {
		return pkg
	}
	files, errs := l.parse(p)
	info := &types.Info{}
	if p.inModule() {
		info = &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	}
	conf := types.Config{
		Importer:         l.importerFor(p),
		IgnoreFuncBodies: p.Standard,
		FakeImportC:      true,
		Error:            func(err error) { errs = append(errs, err) },
	}
	pkg, _ := conf.Check(strings.SplitN(p.ImportPath, " ", 2)[0], l.fset, files, info)
	l.done[p.ImportPath] = pkg
	if p.inModule() {
		l.index.addPackage(pkg, info, files)
		l.warnOf(p, errs)
	}
	return pkg
}

func (l *loader) warnOf(p listedPackage, errs []error) {
	if len(errs) > 0 {
		l.warnings = append(l.warnings, fmt.Sprintf("%s: %d error(s), the first: %v", p.ImportPath, len(errs), errs[0]))
	}
}

func (l *loader) importerFor(p listedPackage) types.Importer {
	return importerFunc(func(path string) (*types.Package, error) {
		if path == "unsafe" {
			return types.Unsafe, nil
		}
		if mapped, ok := p.ImportMap[path]; ok {
			path = mapped
		}
		return l.check(path)
	})
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

func (l *loader) parse(p listedPackage) ([]*ast.File, []error) {
	files := make([]*ast.File, 0, len(p.GoFiles))
	var errs []error
	for _, name := range p.GoFiles {
		f, err := parser.ParseFile(l.fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		files = append(files, f)
	}
	return files, errs
}

func checkoutRoot(dir string) string {
	for current := dir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		if filepath.Dir(current) == current {
			return dir
		}
	}
}
