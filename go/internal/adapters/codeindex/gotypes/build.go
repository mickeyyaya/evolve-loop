package gotypes

import (
	"context"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/goload"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type buildEnv struct {
	run    sysexec.RunFunc
	env    []string
	encode encodeFunc
	warn   io.Writer
}

func Build(ctx context.Context, moduleDir, out string) error {
	return build(ctx, buildEnv{run: sysexec.DefaultRunner, env: os.Environ(), encode: encodeGob, warn: os.Stderr}, moduleDir, out)
}

func build(ctx context.Context, be buildEnv, moduleDir, out string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("gotypes: %w", err)
	}
	dir, err := filepath.EvalSymlinks(moduleDir)
	if err != nil {
		return fmt.Errorf("gotypes: the module dir: %w", err)
	}
	loader := goload.Loader{Run: be.run}
	req := goload.Request{Dir: dir, Patterns: []string{"./..."}, Env: be.env}
	modules := map[string]bool{}
	var index *indexer
	var warnings []string
	_, err = loader.Walk(ctx, req, true, func(fset *token.FileSet, p *goload.Package) error {
		if index == nil {
			index = newIndexer(fset, checkoutRoot(dir), modules)
		}
		modules[strings.SplitN(p.Path, " ", 2)[0]] = true
		index.addPackage(p.Types, p.Info, p.Files)
		if len(p.Errors) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s: %d error(s), the first: %v", p.Path, len(p.Errors), p.Errors[0]))
		}
		p.Files, p.Info = nil, nil
		return nil
	})
	if err != nil {
		return fmt.Errorf("gotypes: %w", err)
	}
	if index == nil {
		index = newIndexer(token.NewFileSet(), checkoutRoot(dir), modules)
	}
	for _, w := range warnings {
		fmt.Fprintf(be.warn, "gotypes: WARN %s\n", w)
	}
	return writeIndex(out, index.finish(warnings), be.encode)
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
