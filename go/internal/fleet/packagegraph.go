package fleet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// modulePrefix limits conflicts to module-internal imports, so a shared stdlib import never conflicts.
const modulePrefix = "github.com/mickeyyaya/evolve-loop/go/"

// TransitivePackageSet returns the module-internal packages reachable from the packages holding files (module-root-relative).
func TransitivePackageSet(files []string, repoRoot string) (map[string]bool, error) {
	dirs := map[string]bool{}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(repoRoot, f)); err != nil {
			return nil, fmt.Errorf("packagegraph: %s: %w", f, err)
		}
		dirs[filepath.Dir(f)] = true
	}
	set := map[string]bool{}
	for dir := range dirs {
		rel := "./" + filepath.ToSlash(dir)
		out, err := sysexec.Output(context.Background(), sysexec.DefaultRunner, repoRoot, "go", "list", "-deps", rel)
		if err != nil {
			return nil, fmt.Errorf("packagegraph: go list -deps %s: %w", rel, err)
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, modulePrefix) {
				set[line] = true
			}
		}
	}
	return set, nil
}

// globalZoneFiles can affect every package's build regardless of the import graph.
var globalZoneFiles = []string{
	"go.mod",
	"go.sum",
	".evolve/policy.json",
}

// IsGlobalZone reports whether file (any leading "./") is a global-zone file.
func IsGlobalZone(file string) bool {
	clean := filepath.Clean(file)
	for _, gz := range globalZoneFiles {
		if clean == filepath.Clean(gz) {
			return true
		}
	}
	return false
}

// GlobalZoneFiles returns a copy of the fixed global-zone file list.
func GlobalZoneFiles() []string {
	out := make([]string, len(globalZoneFiles))
	copy(out, globalZoneFiles)
	return out
}

// PartitionGraph is [Partition] over transitive package sets, treating global-zone files as conflicting with every bucket.
func PartitionGraph(todos []Todo, n int, repoRoot string) (buckets [][]Todo, deferred []Todo, err error) {
	if n < 1 {
		n = 1
	}
	buckets = make([][]Todo, n)
	owner := map[string]int{} // module-internal package path -> owning bucket
	gzBucket := -1            // bucket holding the global-zone todo(s); -1 = none yet

	for _, td := range todos {
		gzFiles, pkgFiles := splitGlobalZone(td.Files)
		isGZ := len(gzFiles) > 0

		pkgs, perr := TransitivePackageSet(pkgFiles, repoRoot)
		if perr != nil {
			return nil, nil, fmt.Errorf("PartitionGraph: todo %s: %w", td.ID, perr)
		}

		owning := map[int]bool{}
		for pkg := range pkgs {
			if b, ok := owner[pkg]; ok {
				owning[b] = true
			}
		}
		if isGZ {
			for i, b := range buckets {
				if len(b) > 0 {
					owning[i] = true
				}
			}
		} else if gzBucket >= 0 {
			owning[gzBucket] = true
		}

		var chosen int
		switch len(owning) {
		case 0:
			chosen = leastLoaded(buckets)
		case 1:
			chosen = only(owning)
		default:
			deferred = append(deferred, td)
			continue
		}
		buckets[chosen] = append(buckets[chosen], td)
		for pkg := range pkgs {
			owner[pkg] = chosen
		}
		if isGZ {
			gzBucket = chosen
		}
	}
	return buckets, deferred, nil
}

// splitGlobalZone separates global-zone files, which have no package to resolve, from package-graph files.
func splitGlobalZone(files []string) (gzFiles, pkgFiles []string) {
	for _, f := range files {
		if IsGlobalZone(f) {
			gzFiles = append(gzFiles, f)
		} else {
			pkgFiles = append(pkgFiles, f)
		}
	}
	return gzFiles, pkgFiles
}
