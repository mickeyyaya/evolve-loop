// Package regressiontia selects the regression predicates a change can skip, as shadow evidence.
package regressiontia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const ArtifactName = "acs-tia-shadow.json"

const depsTimeout = 120 * time.Second

var regressionDir = filepath.Join("acs", "regression")

var depsRunner sysexec.RunFunc = sysexec.DefaultRunner

type Decision struct {
	Stage           string   `json:"stage"`
	ChangedPackages []string `json:"changed_packages,omitempty"`
	Selected        []string `json:"selected,omitempty"`
	WouldSkip       []string `json:"would_skip,omitempty"`
	WouldSkipCount  int      `json:"would_skip_count"`
	DurationMS      int64    `json:"duration_ms"`
}

func Select(patterns, scope []string, deps map[string][]string) (selected, wouldSkip []string) {
	if len(patterns) == 0 {
		return nil, nil
	}
	if len(scope) == 0 {
		return append([]string(nil), patterns...), nil
	}
	for _, p := range patterns {
		d := deps[p]
		if len(d) == 0 || overlaps(d, scope) {
			selected = append(selected, p)
			continue
		}
		wouldSkip = append(wouldSkip, p)
	}
	return selected, wouldSkip
}

func ChangedScope(repoRoot string, changed []string) []string {
	if len(changed) == 0 {
		return nil
	}
	return changedpkgs.ImporterClosure(repoRoot, changed)
}

func Compute(stage, repoRoot, moduleDir string, changed []string) Decision {
	switch stage {
	case "shadow", "enforce":
	default:
		return Decision{}
	}
	start := time.Now()
	patterns := corpus(moduleDir)
	scope := ChangedScope(repoRoot, changed)
	var deps map[string][]string
	if len(scope) > 0 && len(patterns) > 0 {
		deps = resolveDeps(moduleDir, patterns)
	}
	selected, wouldSkip := Select(patterns, scope, deps)
	return Decision{
		Stage:           stage,
		ChangedPackages: scope,
		Selected:        selected,
		WouldSkip:       wouldSkip,
		WouldSkipCount:  len(wouldSkip),
		DurationMS:      time.Since(start).Milliseconds(),
	}
}

func Emit(workspace string, d Decision) (string, error) {
	if strings.TrimSpace(workspace) == "" {
		return "", errors.New("regressiontia: empty workspace, refusing to drop the decision")
	}
	raw, _ := json.MarshalIndent(d, "", "  ")
	path := filepath.Join(workspace, ArtifactName)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("regressiontia: write %s: %w", path, err)
	}
	return path, nil
}

func corpus(moduleDir string) []string {
	if moduleDir == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(moduleDir, regressionDir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, "./acs/regression/"+e.Name())
		}
	}
	return out
}

var escapeHatches = map[string]struct{}{
	"os":       {},
	"os/exec":  {},
	"os/user":  {},
	"syscall":  {},
	"net":      {},
	"net/http": {},
}

func resolveDeps(moduleDir string, patterns []string) map[string][]string {
	raw, modPath := listCorpusDeps(moduleDir, patterns)
	if len(raw) == 0 {
		return nil
	}
	out := map[string][]string{}
	for key, imports := range raw {
		var pats []string
		derivable := true
		for _, dep := range imports {
			if _, hatch := escapeHatches[dep]; hatch {
				derivable = false
				break
			}
			pats = append(pats, "./"+strings.TrimPrefix(dep, modPath+"/")+"/...")
		}
		if !derivable || len(pats) == 0 {
			continue
		}
		sort.Strings(pats)
		out[key] = pats
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func listCorpusDeps(moduleDir string, patterns []string) (map[string][]string, string) {
	if moduleDir == "" || len(patterns) == 0 {
		return nil, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), depsTimeout)
	defer cancel()

	modPath, err := sysexec.Output(ctx, depsRunner, moduleDir, "go", "list", "-m")
	if err != nil || modPath == "" {
		return nil, ""
	}
	out, err := sysexec.Output(ctx, depsRunner, moduleDir, "go", "list", "-e", "-tags", "acs", "-test", "-deps",
		"-f", "{{.ImportPath}}|{{join .Imports \" \"}}", "./"+filepath.ToSlash(regressionDir)+"/...")
	if err != nil {
		return nil, ""
	}

	want := map[string]struct{}{}
	for _, p := range patterns {
		want[p] = struct{}{}
	}
	graph := map[string][]string{}
	starts := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		self, rest, ok := strings.Cut(line, "|")
		if !ok {
			continue
		}
		node := stripVariant(self)
		graph[node] = append(graph[node], stripVariants(strings.Fields(rest))...)
		key := "./" + strings.TrimPrefix(testPackageOwner(self), modPath+"/")
		if _, ok := want[key]; ok && !strings.HasSuffix(node, ".test") {
			starts[key] = append(starts[key], node)
		}
	}
	deps := map[string][]string{}
	for key, nodes := range starts {
		deps[key] = moduleLocalReach(graph, modPath, nodes)
	}
	return deps, modPath
}

func moduleLocalReach(graph map[string][]string, modPath string, starts []string) []string {
	seen := map[string]struct{}{}
	queue := append([]string(nil), starts...)
	var reach []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, imp := range graph[node] {
			if _, dup := seen[imp]; dup {
				continue
			}
			if strings.HasPrefix(imp, modPath+"/") {
				seen[imp] = struct{}{}
				reach = append(reach, imp)
				queue = append(queue, imp)
				continue
			}
			if _, hatch := escapeHatches[imp]; hatch {
				seen[imp] = struct{}{}
				reach = append(reach, imp)
			}
		}
	}
	return reach
}

func stripVariant(importPath string) string {
	if i := strings.Index(importPath, " ["); i >= 0 {
		return importPath[:i]
	}
	return importPath
}

func stripVariants(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = stripVariant(p)
	}
	return out
}

func testPackageOwner(importPath string) string {
	importPath = strings.TrimSuffix(stripVariant(importPath), ".test")
	return strings.TrimSuffix(importPath, "_test")
}

func overlaps(deps, scope []string) bool {
	for _, d := range deps {
		for _, s := range scope {
			if intersects(d, s) {
				return true
			}
		}
	}
	return false
}

func intersects(a, b string) bool {
	x, y := normalize(a), normalize(b)
	if x == "" || y == "" {
		return true
	}
	return x == y || strings.HasPrefix(x, y+"/") || strings.HasPrefix(y, x+"/")
}

func normalize(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/...")
	p = strings.TrimSuffix(p, "...")
	return strings.Trim(p, "/")
}
