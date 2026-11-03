package inboxcurator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

var testFuncRE = regexp.MustCompile(`(?m)^func ((?:Test|Benchmark|Fuzz)[A-Za-z0-9_]*)\(`)

var skippedTestDirs = map[string]bool{"vendor": true, "testdata": true, ".git": true, "node_modules": true}

func IndexTests(goDir string) (map[string]bool, error) {
	if _, err := os.Stat(goDir); errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	names, err := indexTestsIn(os.DirFS(goDir), ".")
	if err != nil {
		return nil, fmt.Errorf("inboxcurator: index the tests under %s: %w", goDir, err)
	}
	return names, nil
}

func indexTestsIn(fsys fs.FS, dir string) (map[string]bool, error) {
	names := map[string]bool{}
	isTestFile := func(name string) bool { return strings.HasSuffix(name, "_test.go") }
	err := walkGoFiles(fsys, dir, isTestFile, func(_ string, body []byte) error {
		for _, m := range testFuncRE.FindAllStringSubmatch(string(body), -1) {
			names[m[1]] = true
		}
		return nil
	})
	return names, err
}

func walkGoFiles(fsys fs.FS, dir string, keep func(string) bool, visit func(string, []byte) error) error {
	return fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && skippedTestDirs[d.Name()]:
			return fs.SkipDir
		case d.IsDir() || !keep(path):
			return nil
		}
		body, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return visit(path, body)
	})
}
