package changedpkgs

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
)

// CoveringTests returns the sorted repo-relative _test.go paths under pkgPatterns; nil for any unusable input, "./..." included.
func CoveringTests(repoRoot string, pkgPatterns []string) []string {
	if strings.TrimSpace(repoRoot) == "" || len(pkgPatterns) == 0 {
		return nil
	}
	moduleDir := filepath.Join(repoRoot, "go")
	seen := map[string]struct{}{}
	for _, pat := range pkgPatterns {
		rel, ok := patternDir(pat)
		if !ok {
			continue
		}
		dir := filepath.Join(moduleDir, filepath.FromSlash(rel))
		// An unreadable subtree contributes no files; the corpus fails open.
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(info.Name(), "_test.go") {
				return nil
			}
			r, rerr := filepath.Rel(repoRoot, p)
			if rerr != nil {
				return nil
			}
			seen[filepath.ToSlash(r)] = struct{}{}
			return nil
		})
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// patternDir maps the recursive and bare forms of a pattern to one module-relative dir; false for "./..." or an escape.
func patternDir(pat string) (string, bool) {
	p := strings.TrimSpace(pat)
	if p == "" || p == gopkgpattern.WholeModule || !strings.HasPrefix(p, "./") {
		return "", false
	}
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
	p = path.Clean(p)
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}
