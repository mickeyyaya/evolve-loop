// Package ciparity holds the pure helpers of the cycle audit's CI-parity gate:
// the scope of the whole-repo checks, the offenders they name and the gate set.
package ciparity

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

const RequiredWorkflow = "required.yml"

func IntersectEnforced(changed []string, enforceBytes []byte) []string {
	if len(changed) == 0 {
		return nil
	}
	changedSet := make(map[string]bool, len(changed))
	for _, c := range changed {
		if c = normalizePattern(c); c != "" {
			changedSet[c] = true
		}
	}

	seen := make(map[string]bool)
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(enforceBytes))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if changedSet[line] && !seen[line] {
			seen[line] = true
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func normalizePattern(p string) string {
	return strings.TrimSuffix(strings.TrimSpace(p), "/...")
}

func PackageDirHasProductionGoFiles(moduleDir, pkg string) bool {
	if strings.Contains(pkg, "...") {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(moduleDir, filepath.FromSlash(strings.TrimPrefix(pkg, "./"))))
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			return true
		}
	}
	return false
}

func NewUngraduatedPackages(changed []string, enforceBytes []byte) []string {
	if len(changed) == 0 {
		return nil
	}
	enforced := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(enforceBytes))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		enforced[line] = true
	}

	seen := make(map[string]bool)
	var out []string
	for _, c := range changed {
		c = normalizePattern(c)
		if c == "" || !strings.HasPrefix(c, "./internal/") {
			continue
		}
		if enforced[c] || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

const CoverageTags = "integration acs"

func CoverageTestArgs(coverProfile string, pkgs []string) []string {
	args := []string{"test", "-tags", CoverageTags, "-coverprofile=" + coverProfile}
	return lanebudget.Args(append(args, pkgs...))
}
