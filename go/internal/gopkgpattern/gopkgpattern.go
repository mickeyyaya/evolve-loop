// Package gopkgpattern is the shared rule that recognizes and normalizes go test package patterns in Go source.
// See docs/architecture/packages/internal-gopkgpattern.md.
package gopkgpattern

import (
	"path/filepath"
	"strings"
)

const ModulePrefix = "github.com/mickeyyaya/evolve-loop/go/"

const WholeModule = "./..."

func IsPackagePattern(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return false
	}
	if s == WholeModule {
		return true
	}
	if filepath.Ext(strings.TrimSuffix(s, "/...")) != "" {
		return false
	}
	if strings.HasPrefix(s, "./") && len(s) > 2 {
		return true
	}
	return strings.HasPrefix(s, ModulePrefix)
}

func IsRecursive(s string) bool {
	return s == WholeModule || strings.HasSuffix(s, "/...")
}

func Key(p string) string {
	p = strings.TrimSpace(p)
	if p == WholeModule {
		return ""
	}
	p = strings.TrimSuffix(p, "/...")
	p = strings.TrimSuffix(p, "/")
	switch {
	case strings.HasPrefix(p, "./"):
		return strings.TrimPrefix(p, "./")
	case strings.HasPrefix(p, ModulePrefix):
		return strings.TrimPrefix(p, ModulePrefix)
	}
	return ""
}
