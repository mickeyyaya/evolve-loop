package admission

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const ModulePath = "github.com/mickeyyaya/evolve-loop/go"

const goListFormat = `{{.ImportPath}}|{{join .Deps " "}}`

type ModuleGraph struct {
	known map[string]bool
	deps  map[string][]string
	hubs  map[string]bool
}

func LoadModuleGraph(ctx context.Context, run sysexec.RunFunc, goDir string, hubMinImporters int) (*ModuleGraph, error) {
	out, err := sysexec.Output(ctx, run, goDir, "go", "list", "-f", goListFormat, "./...")
	if err != nil {
		return nil, fmt.Errorf("admission: load the module graph: %w", err)
	}
	return NewModuleGraph(out, hubMinImporters)
}

func NewModuleGraph(listing string, hubMinImporters int) (*ModuleGraph, error) {
	raw := map[string][]string{}
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pkg, deps, ok := strings.Cut(line, "|")
		if !ok {
			return nil, fmt.Errorf("admission: go list line %q has no %q separator", line, "|")
		}
		raw[pkg] = strings.Fields(deps)
	}
	g := &ModuleGraph{known: map[string]bool{}, deps: map[string][]string{}, hubs: map[string]bool{}}
	importers := map[string]int{}
	for pkg, deps := range raw {
		g.known[pkg] = true
		for _, d := range deps {
			if _, known := raw[d]; known {
				g.deps[pkg] = append(g.deps[pkg], d)
				importers[d]++
			}
		}
		slices.Sort(g.deps[pkg])
	}
	for pkg := range raw {
		if importers[pkg] >= hubMinImporters {
			g.hubs[pkg] = true
		}
	}
	return g, nil
}

func (g *ModuleGraph) Deps(pkg string) []string { return g.deps[pkg] }

func (g *ModuleGraph) Hubs() map[string]bool { return g.hubs }

func (g *ModuleGraph) Packages(files []string) []string {
	var out []string
	for _, f := range files {
		if pkg, ok := g.packageOf(f); ok && !slices.Contains(out, pkg) {
			out = append(out, pkg)
		}
	}
	slices.Sort(out)
	return out
}

func (g *ModuleGraph) packageOf(file string) (string, bool) {
	rel, ok := strings.CutPrefix(filepath.ToSlash(filepath.Clean(file)), "go/")
	if !ok {
		return "", false
	}
	if strings.HasSuffix(rel, ".go") {
		return importPath(path.Dir(rel)), true
	}
	for dir := rel; dir != "."; dir = path.Dir(dir) {
		if g.known[importPath(dir)] {
			return importPath(dir), true
		}
	}
	return "", false
}

func importPath(dir string) string {
	if dir == "." {
		return ModulePath
	}
	return ModulePath + "/" + dir
}
