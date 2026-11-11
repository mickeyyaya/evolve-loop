package goload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Package struct {
	Path     string
	Dir      string
	Standard bool
	Module   bool
	Types    *types.Package
	Info     *types.Info
	Files    []*ast.File
	Errors   []error
}

type Program struct {
	Fset     *token.FileSet
	Packages map[string]*Package
	Order    []string
}

type Request struct {
	Dir      string
	Patterns []string
	Env      []string
}

type Loader struct {
	Run sysexec.RunFunc
}

type listed struct {
	ImportPath string
	Dir        string
	Standard   bool
	GoFiles    []string
	ImportMap  map[string]string
	Module     *struct{ Main bool }
}

func (p listed) inModule() bool {
	return !p.Standard && p.Module != nil && p.Module.Main
}

type checker struct {
	fset    *token.FileSet
	listed  map[string]listed
	program Program
	visit   func(*token.FileSet, *Package) error
	err     error
}

func (l Loader) Load(ctx context.Context, req Request) (Program, error) {
	return l.load(ctx, req, false)
}

func (l Loader) LoadWithTests(ctx context.Context, req Request) (Program, error) {
	return l.load(ctx, req, true)
}

func (l Loader) Walk(ctx context.Context, req Request, withTests bool, visit func(*token.FileSet, *Package) error) (*token.FileSet, error) {
	c, err := l.run(ctx, req, withTests, visit)
	if err != nil {
		return nil, err
	}
	return c.fset, nil
}

func (l Loader) load(ctx context.Context, req Request, withTests bool) (Program, error) {
	c, err := l.run(ctx, req, withTests, func(*token.FileSet, *Package) error { return nil })
	if err != nil {
		return Program{}, err
	}
	return c.program, nil
}

func (l Loader) run(ctx context.Context, req Request, withTests bool, visit func(*token.FileSet, *Package) error) (*checker, error) {
	all, err := l.list(ctx, req, withTests)
	if err != nil {
		return nil, err
	}
	c := &checker{fset: token.NewFileSet(), listed: map[string]listed{}, visit: visit}
	c.program = Program{Fset: c.fset, Packages: map[string]*Package{}}
	for _, p := range all {
		c.listed[p.ImportPath] = p
	}
	for _, p := range all {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("goload: %w", err)
		}
		c.check(p)
		if c.err != nil {
			return nil, c.err
		}
	}
	return c, nil
}

func (l Loader) list(ctx context.Context, req Request, withTests bool) ([]listed, error) {
	if l.Run == nil {
		return nil, errors.New("goload: the loader has no runner")
	}
	var stdout, stderr strings.Builder
	args := []string{"list", "-buildvcs=false", "-json", "-deps"}
	if withTests {
		args = append(args, "-test")
	}
	args = append(args, req.Patterns...)
	env := slices.Concat(req.Env, []string{"CGO_ENABLED=0"})
	code, err := l.Run(ctx, "go", req.Dir, args, env, nil, &stdout, &stderr)
	if err != nil {
		return nil, fmt.Errorf("goload: run go list in %s: %w", req.Dir, err)
	}
	if code != 0 {
		return nil, fmt.Errorf("goload: go list in %s: exit %d: %s", req.Dir, code, strings.TrimSpace(stderr.String()))
	}
	var out []listed
	dec := json.NewDecoder(strings.NewReader(stdout.String()))
	for dec.More() {
		var p listed
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("goload: decode the go list answer: %w", err)
		}
		if !strings.HasSuffix(p.ImportPath, ".test") {
			out = append(out, p)
		}
	}
	return out, nil
}

func (c *checker) importPath(path string) (*types.Package, error) {
	p, ok := c.listed[path]
	if !ok {
		return nil, fmt.Errorf("goload: go list did not list the import %s", path)
	}
	pkg := c.check(p)
	if c.err != nil {
		return nil, c.err
	}
	return pkg.Types, nil
}

func (c *checker) check(p listed) *Package {
	if done, ok := c.program.Packages[p.ImportPath]; ok {
		return done
	}
	if c.err != nil {
		return &Package{}
	}
	files, errs := c.parse(p)
	info := &types.Info{}
	if p.inModule() {
		info = &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	}
	conf := types.Config{
		Importer:         c.importerFor(p),
		IgnoreFuncBodies: p.Standard,
		FakeImportC:      true,
		Error:            func(err error) { errs = append(errs, err) },
	}
	pkg, _ := conf.Check(strings.SplitN(p.ImportPath, " ", 2)[0], c.fset, files, info)
	out := &Package{Path: p.ImportPath, Dir: p.Dir, Standard: p.Standard, Module: p.inModule(), Types: pkg, Info: info}
	if out.Module {
		out.Files, out.Errors = files, errs
	}
	c.program.Packages[p.ImportPath] = out
	c.program.Order = append(c.program.Order, p.ImportPath)
	if out.Module && c.err == nil {
		if err := c.visit(c.fset, out); err != nil {
			c.err = fmt.Errorf("goload: visit %s: %w", p.ImportPath, err)
		}
	}
	return out
}

func (c *checker) importerFor(p listed) types.Importer {
	return importerFunc(func(path string) (*types.Package, error) {
		if path == "unsafe" {
			return types.Unsafe, nil
		}
		if mapped, ok := p.ImportMap[path]; ok {
			path = mapped
		}
		return c.importPath(path)
	})
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

func (c *checker) parse(p listed) ([]*ast.File, []error) {
	files := make([]*ast.File, 0, len(p.GoFiles))
	var errs []error
	for _, name := range p.GoFiles {
		f, err := parser.ParseFile(c.fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		files = append(files, f)
	}
	return files, errs
}
