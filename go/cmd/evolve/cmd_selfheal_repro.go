package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

func resolveReproTest(root, importPath, test string) string {
	name, _, _ := strings.Cut(test, "/")
	if name == "" {
		return ""
	}
	rel, entries, ok := packageDir(root, importPath)
	if !ok {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel), e.Name()))
		if err == nil && selfheal.DefinesTest(raw, name) {
			return rel + "/" + e.Name() + ":" + name
		}
	}
	return ""
}

func packageDir(root, importPath string) (string, []os.DirEntry, bool) {
	parts := strings.Split(importPath, "/")
	for i := range parts {
		rel := strings.Join(parts[i:], "/")
		if entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			return rel, entries, true
		}
	}
	return "", nil, false
}
