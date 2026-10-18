package envscope

import (
	"path/filepath"
	"strings"
)

const registryTableSuffix = "internal/flagregistry/registry_table.go"

var skipDirs = map[string]bool{
	"vendor": true, "testdata": true, ".git": true, "node_modules": true,
	".evolve": true, "ipcenv": true, "acs": true,
}

func SkipDir(name string) bool {
	return skipDirs[name]
}

func File(path string) bool {
	slashed := filepath.ToSlash(path)
	return strings.HasSuffix(slashed, ".go") &&
		!strings.HasSuffix(slashed, "_test.go") &&
		!strings.HasSuffix(slashed, registryTableSuffix)
}

func Reads(path string) bool {
	dir, _ := filepath.Split(filepath.ToSlash(path))
	for _, part := range strings.Split(dir, "/") {
		if SkipDir(part) {
			return false
		}
	}
	return File(path)
}
