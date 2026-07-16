package fleet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// modulePrefix identifies module-internal import paths. Only these can
// merge-skew a concurrent build (research finding #2: rename + call-site);
// stdlib/third-party imports are excluded so two todos that merely share a
// stdlib dependency (e.g. both import "fmt") don't spuriously conflict.
const modulePrefix = "github.com/mickeyyaya/evolve-loop/go/"

// TransitivePackageSet resolves files (module-root-relative paths, e.g.
// "internal/fleet/partition.go") to the set of module-internal import paths
// transitively reachable from their containing packages, including each
// file's own package. It shells out to `go list -deps` (no synthetic graph —
// the real build graph, so renames/refactors are always current) once per
// distinct containing directory. repoRoot is the Go module root (containing
// go.mod). A file that doesn't exist on disk is a caller-contract violation —
// fail loud rather than silently resolving an empty/wrong package.
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

// globalZoneFiles are module-root-relative files whose edit can affect the
// build of ANY package regardless of the import graph: go.mod/go.sum change
// dependency resolution for the whole module, and policy/hook files are
// cross-cutting by convention (research finding #2's prescribed design).
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

// PartitionGraph is [Partition]'s package-graph-aware sibling (rung 1 of the
// merge ladder): it assigns todos to n concurrent buckets such that no two
// buckets' TRANSITIVE PACKAGE sets intersect, not just their literal files.
// Two todos touching disjoint files but connected through the import graph
// (a rename in package foo, a call-site edit in package bar that imports foo)
// are co-scheduled into the same bucket, or deferred — never split, matching
// Partition's cross-bucket-disjointness invariant but at build-graph
// granularity. A todo touching a global-zone file (go.mod, go.sum, ...)
// always conflicts with every bucket already holding work, since it can
// change any package's build. Deterministic: input order preserved, ties
// break to the lowest bucket index.
func PartitionGraph(todos []Todo, n int, repoRoot string) (buckets [][]Todo, deferred []Todo, err error) {
	if n < 1 {
		n = 1
	}
	buckets = make([][]Todo, n)
	owner := map[string]int{} // module-internal package path -> owning bucket
	gzBucket := -1            // bucket holding the global-zone-touching todo(s), -1 = none yet

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

// splitGlobalZone partitions files into global-zone files and ordinary
// package-graph files (see IsGlobalZone) — global-zone files are excluded
// from TransitivePackageSet since they aren't Go source (no package to
// resolve); their conflict semantics are handled separately in PartitionGraph.
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
