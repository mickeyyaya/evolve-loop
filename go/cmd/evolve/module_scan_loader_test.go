package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type moduleSourceFile struct {
	Rel      string
	Dir      string
	Path     string
	Src      []byte
	parseAST func() (*ast.File, error)
}

func (f *moduleSourceFile) AST(t *testing.T) *ast.File {
	t.Helper()
	parsed, err := f.parseAST()
	if err != nil {
		t.Fatalf("parse %s: %v", f.Rel, err)
	}
	return parsed
}

func walkSourceFiles(root string) ([]*moduleSourceFile, error) {
	var files []*moduleSourceFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || name == "testdata" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slashRel := filepath.ToSlash(rel)
		slashDir := filepath.ToSlash(filepath.Dir(rel))
		files = append(files, &moduleSourceFile{
			Rel:  slashRel,
			Dir:  slashDir,
			Path: path,
			Src:  src,
			parseAST: sync.OnceValues(func() (*ast.File, error) {
				return parser.ParseFile(token.NewFileSet(), path, src, 0)
			}),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

type cachedDirWalk struct {
	once  sync.Once
	files []*moduleSourceFile
	err   error
}

var dirWalks sync.Map

func cachedWalkSourceFiles(root string) ([]*moduleSourceFile, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	val, ok := dirWalks.Load(abs)
	if !ok {
		val, _ = dirWalks.LoadOrStore(abs, &cachedDirWalk{})
	}
	entry := val.(*cachedDirWalk)
	entry.once.Do(func() {
		entry.files, entry.err = walkSourceFiles(abs)
	})
	return entry.files, entry.err
}

var loadModuleSourceFiles = sync.OnceValues(func() ([]*moduleSourceFile, error) {
	return cachedWalkSourceFiles(filepath.Join("..", ".."))
})

func sharedModuleFiles(t *testing.T) []*moduleSourceFile {
	t.Helper()
	files, err := loadModuleSourceFiles()
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sourceFilesInDir(t *testing.T, dir string) []*moduleSourceFile {
	t.Helper()
	files, err := cachedWalkSourceFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	return files
}
