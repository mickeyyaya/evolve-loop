// Package shipmanifest selects the paths a Ship commit binds from the declared phase-report manifest.
package shipmanifest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

const bareRootFileExts = `go|mod|sum|md|json|ya?ml|txt|toml|lock|sh`

var pathToken = regexp.MustCompile(
	`[A-Za-z0-9_.][A-Za-z0-9_.-]*(?:/[A-Za-z0-9_.-]+)+` +
		`|[A-Za-z0-9_][A-Za-z0-9_-]*\.(?:` + bareRootFileExts + `)\b`)

func isPathContinuationByte(b byte) bool {
	return b == '.' || b == '-' || b == '/' || b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func extractReportPaths(md string) []string {
	seen := map[string]bool{}
	for _, loc := range pathToken.FindAllStringIndex(md, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && isPathContinuationByte(md[start-1]) {
			continue
		}
		m := strings.TrimRight(md[start:end], ".")
		for strings.HasPrefix(m, "./") {
			m = m[2:]
		}
		if strings.HasPrefix(m, "..") || !isRepoRelative(m) {
			continue
		}
		seen[m] = true
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func Declared(workspacePath string, reportFiles []string) []string {
	seen := map[string]bool{}
	fold := func(ws string) {
		for _, f := range reportFiles {
			b, err := os.ReadFile(filepath.Join(ws, f))
			if err != nil {
				continue
			}
			for _, p := range extractReportPaths(string(b)) {
				seen[p] = true
			}
		}
	}
	fold(workspacePath)
	if c, ok, _ := continuation.ReadManifest(workspacePath); ok && c.Cycle > 0 {
		fold(filepath.Join(filepath.Dir(workspacePath), fmt.Sprintf("cycle-%d", c.Cycle)))
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func isRepoRelative(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") {
		return false
	}
	c := path.Clean(p)
	return c != "" && c != "." && !strings.HasPrefix(c, "/") &&
		c != ".." && !strings.HasPrefix(c, "../")
}

func manifestCovers(manifest []string, p string) bool {
	for _, m := range manifest {
		if p == m || strings.HasPrefix(p, strings.TrimSuffix(m, "/")+"/") {
			return true
		}
	}
	return false
}

func OutOfManifest(changed, manifest []string) []string {
	var extras []string
	for _, c := range changed {
		if !manifestCovers(manifest, c) {
			extras = append(extras, c)
		}
	}
	sort.Strings(extras)
	return extras
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func pathspec(manifest, changed []string, isFile func(string) bool) []string {
	staged := map[string]bool{}
	for _, d := range manifest {
		if !isRepoRelative(d) {
			continue
		}
		if isFile(d) {
			staged[d] = true
		}
	}
	for _, c := range changed {
		if manifestCovers(manifest, c) {
			staged[c] = true
		}
	}
	return sortedKeys(staged)
}
