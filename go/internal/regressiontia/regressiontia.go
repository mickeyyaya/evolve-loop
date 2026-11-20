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
	PureCount       int      `json:"pure_count"`
	IOCount         int      `json:"io_count"`
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
	var pure, io []string
	if len(scope) > 0 && len(patterns) > 0 {
		reach, modPath := listCorpusDeps(moduleDir, patterns)
		deps = skippableDeps(reach, modPath)
		pure, io = Split(patterns, reach)
	}
	selected, wouldSkip := Select(patterns, scope, deps)
	return Decision{
		Stage:           stage,
		ChangedPackages: scope,
		Selected:        selected,
		WouldSkip:       wouldSkip,
		WouldSkipCount:  len(wouldSkip),
		PureCount:       len(pure),
		IOCount:         len(io),
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

const depGraphFormat = "{{.ImportPath}}|{{if .Error}}ERR{{else}}{{join .Imports \" \"}}{{end}}"

var escapeHatches = map[string]struct{}{
	"os":       {},
	"os/exec":  {},
	"os/user":  {},
	"syscall":  {},
	"net":      {},
	"net/http": {},
}

func Pure(moduleDir, importPath string) bool {
	if moduleDir == "" || importPath == "" {
		return false
	}
	return PurePackages(moduleDir, []string{importPath})[importPath]
}

func PurePackages(moduleDir string, importPaths []string) map[string]bool {
	out := make(map[string]bool, len(importPaths))
	if moduleDir == "" || len(importPaths) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), depsTimeout)
	defer cancel()

	modPath, err := sysexec.Output(ctx, depsRunner, moduleDir, "go", "list", "-m")
	if err != nil || modPath == "" {
		return out
	}
	args := append([]string{"list", "-e", "-test", "-deps", "-f", depGraphFormat}, importPaths...)
	raw, err := sysexec.Output(ctx, depsRunner, moduleDir, "go", args...)
	if err != nil {
		return out
	}
	want := map[string]struct{}{}
	for _, p := range importPaths {
		if p != "" && (p == modPath || strings.HasPrefix(p, modPath+"/")) {
			want[p] = struct{}{}
		}
	}
	if len(want) == 0 {
		return out
	}
	graph, starts, errored := parseDepGraph(raw, want)
	for pkg := range want {
		nodes := starts[pkg]
		if len(nodes) == 0 || errored[pkg] {
			continue
		}
		reach := ModuleLocalReach(graph, modPath, nodes)
		out[pkg] = !reachesAny(reach, errored)
	}
	return out
}

func parseDepGraph(raw string, want map[string]struct{}) (map[string][]string, map[string][]string, map[string]bool) {
	graph := map[string][]string{}
	starts := map[string][]string{}
	errored := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		self, rest, ok := strings.Cut(line, "|")
		if !ok {
			continue
		}
		node := stripVariant(self)
		owner := testPackageOwner(self)
		if rest == "ERR" {
			errored[node] = true
			errored[owner] = true
			continue
		}
		graph[node] = append(graph[node], stripVariants(strings.Fields(rest))...)
		if _, ok := want[owner]; ok && !strings.HasSuffix(node, ".test") {
			starts[owner] = append(starts[owner], node)
		}
	}
	return graph, starts, errored
}

func reachesAny(reach []string, errored map[string]bool) bool {
	for _, dep := range reach {
		if _, hatch := escapeHatches[dep]; hatch {
			return true
		}
	}
	return reachesErrored(reach, errored)
}

func reachesErrored(reach []string, errored map[string]bool) bool {
	for _, dep := range reach {
		if errored[dep] {
			return true
		}
	}
	return false
}

func skippable(reach []string) bool {
	return len(reach) > 0 && !reachesAny(reach, nil)
}

func Split(patterns []string, reach map[string][]string) (pure, io []string) {
	for _, p := range patterns {
		if r, known := reach[p]; known && skippable(r) {
			pure = append(pure, p)
			continue
		}
		io = append(io, p)
	}
	return pure, io
}

func skippableDeps(reach map[string][]string, modPath string) map[string][]string {
	out := map[string][]string{}
	for key, imports := range reach {
		if !skippable(imports) {
			continue
		}
		pats := make([]string, len(imports))
		for i, dep := range imports {
			pats[i] = "./" + strings.TrimPrefix(dep, modPath+"/") + "/..."
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
		"-f", depGraphFormat, "./"+filepath.ToSlash(regressionDir)+"/...")
	if err != nil {
		return nil, ""
	}

	want := map[string]struct{}{}
	owners := map[string]string{}
	for _, p := range patterns {
		owner := modPath + "/" + strings.TrimPrefix(p, "./")
		want[owner] = struct{}{}
		owners[owner] = p
	}
	graph, starts, errored := parseDepGraph(out, want)
	deps := map[string][]string{}
	for owner, nodes := range starts {
		if errored[owner] {
			continue
		}
		reach := ModuleLocalReach(graph, modPath, nodes)
		if reachesErrored(reach, errored) {
			continue
		}
		deps[owners[owner]] = reach
	}
	return deps, modPath
}

func ModuleLocalReach(graph map[string][]string, modPath string, starts []string) []string {
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
