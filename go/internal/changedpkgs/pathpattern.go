package changedpkgs

import (
	"path"
	"slices"
	"strings"
)

func PatternOf(file string) (string, bool) {
	clean := strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(file), "\\", "/")), "./")
	rel, ok := strings.CutPrefix(clean, "go/")
	if !ok || strings.ContainsAny(rel, " \t\n") {
		return "", false
	}
	if rel == "go.mod" || rel == "go.sum" || strings.HasPrefix(rel, "vendor/") {
		return moduleRootPattern, true
	}
	return dirPattern(beforeTestdata(path.Dir(rel))), true
}

func beforeTestdata(dir string) string {
	parts := strings.Split(dir, "/")
	if i := slices.Index(parts, "testdata"); i >= 0 {
		parts = parts[:i]
	}
	return strings.Join(parts, "/")
}

func dirPattern(dir string) string {
	if dir == "." || dir == "" {
		return moduleRootPattern
	}
	return "./" + dir + "/..."
}
