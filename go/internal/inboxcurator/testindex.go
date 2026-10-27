package inboxcurator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var testFuncRE = regexp.MustCompile(`(?m)^func ((?:Test|Benchmark|Fuzz)[A-Za-z0-9_]*)\(`)

var skippedTestDirs = map[string]bool{"vendor": true, "testdata": true, ".git": true, "node_modules": true}

func IndexTests(goDir string) (map[string]bool, error) {
	names := map[string]bool{}
	if _, err := os.Stat(goDir); errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	err := filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && skippedTestDirs[d.Name()]:
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, "_test.go"):
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range testFuncRE.FindAllStringSubmatch(string(body), -1) {
			names[m[1]] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inboxcurator: index the tests under %s: %w", goDir, err)
	}
	return names, nil
}
