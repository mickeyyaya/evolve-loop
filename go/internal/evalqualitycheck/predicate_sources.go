package evalqualitycheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func predicateSourcePaths(path string) ([]string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			paths = append(paths, filepath.Join(path, e.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .go files under %s (nothing linted — this is not a clean result)", path)
	}
	return paths, nil
}
