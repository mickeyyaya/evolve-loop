package shipmanifest

import (
	"os"
	"path/filepath"
)

func Stageable(porcelain string, manifest []string, isFile func(string) bool) []string {
	return withoutGone(porcelain, pathspec(manifest, ChangedPaths(porcelain), isFile))
}

func withoutGone(porcelain string, paths []string) []string {
	gone := gonePaths(porcelain)
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if !gone[p] {
			kept = append(kept, p)
		}
	}
	return kept
}

func RegularFileIn(root string) func(string) bool {
	return func(rel string) bool {
		fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil && fi.Mode().IsRegular()
	}
}
