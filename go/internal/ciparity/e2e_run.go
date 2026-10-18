package ciparity

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const e2eTag = "e2e"

func E2ETestNames(moduleDir string, roots, tags []string) ([]string, error) {
	var names []string
	for _, root := range roots {
		walk := func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			found, err := e2eTestsIn(path, tags)
			names = append(names, found...)
			return err
		}
		if err := filepath.WalkDir(filepath.Join(moduleDir, root), walk); err != nil {
			return nil, err
		}
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

func E2ERunPattern(names []string) string {
	return "^(" + strings.Join(names, "|") + ")$"
}

func E2EMain(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("e2elist", flag.ContinueOnError)
	flags.SetOutput(stderr)
	tags := flags.String("tags", e2eTag, "build tags of the e2e run, space separated")
	dir := flags.String("dir", ".", "module directory that the roots are relative to")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	names, err := E2ETestNames(*dir, flags.Args(), strings.Fields(*tags))
	if err != nil {
		fmt.Fprintln(stderr, "e2elist:", err)
		return 1
	}
	if len(names) == 0 {
		fmt.Fprintln(stderr, "e2elist: no e2e test under", flags.Args())
		return 1
	}
	fmt.Fprintln(stdout, E2ERunPattern(names))
	return 0
}

func e2eTestsIn(dir string, tags []string) ([]string, error) {
	tagged, err := testFilesUnder(dir, tags)
	if err != nil || len(tagged) == 0 {
		return nil, err
	}
	plain, err := testFilesUnder(dir, slices.DeleteFunc(slices.Clone(tags), func(tag string) bool { return tag == e2eTag }))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, file := range tagged {
		if slices.Contains(plain, file) {
			continue
		}
		found, err := testFuncNames(filepath.Join(dir, file))
		if err != nil {
			return nil, err
		}
		names = append(names, found...)
	}
	return names, nil
}

func testFilesUnder(dir string, tags []string) ([]string, error) {
	ctx := build.Default
	ctx.BuildTags = tags
	pkg, err := ctx.ImportDir(dir, 0)
	var noGo *build.NoGoError
	if errors.As(err, &noGo) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append(slices.Clone(pkg.TestGoFiles), pkg.XTestGoFiles...), nil
}

func testFuncNames(path string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && isTestName(fn.Name.Name) {
			names = append(names, fn.Name.Name)
		}
	}
	return names, nil
}

func isTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}
	first, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(first)
}
