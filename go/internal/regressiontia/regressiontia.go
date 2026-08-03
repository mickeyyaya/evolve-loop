// Package regressiontia computes test-impact selection over the EGPS Go
// regression corpus (go/acs/regression/<sub>) and emits it as evidence.
//
// WHY THIS IS ITS OWN PACKAGE, not a change to the gate runner. The suite
// runner go/internal/acssuite is PROTECTED CONTROL PLANE
// (guards.ProtectedSurfaceManifest: {"/go/internal/acssuite/", "the gate
// runner"}) — a cycle may not edit the gate that grades it. That constraint
// yields the better design rather than an obstacle to route around: the SHADOW
// stage changes nothing about what the gate runs (that is what shadow MEANS),
// so it needs no code in the runner at all. It is pure observability computed
// beside the suite by the suite's own production caller, the audit phase's
// generateACSVerdict. Only the future `enforce` cutover, which actually skips
// packages, must live inside acssuite — and that is human-gated
// `evolve ship --class manual` outside a cycle by construction.
//
// SAFETY INVARIANT (from the inbox item this implements): selection must never
// be able to hide a regression class. Every fail-safe therefore resolves toward
// RUNNING a predicate — an underivable changed scope skips nothing, unresolved
// dependency data skips nothing, an unknown stage is off, and a broken evidence
// sink degrades quietly instead of failing the audit. Under-selecting is a
// missed red that reaches main; over-selecting is only slower.
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

// ArtifactName is the cycle-workspace file the shadow decision is written to.
const ArtifactName = "acs-tia-shadow.json"

// depsTimeout bounds the two `go list` invocations that resolve the regression
// corpus's dependency sets. Past the bound we return no dependency data, which
// the fail-safe reads as "unresolvable" ⇒ skip nothing.
const depsTimeout = 120 * time.Second

// regressionDir is the module-relative corpus root, mirroring the runner's own
// enumeration (acssuite.goLanePatterns walks acs/regression/*).
var regressionDir = filepath.Join("acs", "regression")

// Decision is one cycle's test-impact reasoning over the regression corpus. It
// is the operator's only view of what selection WOULD have done before enforce
// is ever considered, so it names the corpus it reasoned about, not just a
// count.
type Decision struct {
	Stage           string   `json:"stage"`
	ChangedPackages []string `json:"changed_packages,omitempty"`
	Selected        []string `json:"selected,omitempty"`
	WouldSkip       []string `json:"would_skip,omitempty"`
	// WouldSkipCount is a projection of WouldSkip, never an independent tally.
	WouldSkipCount int `json:"would_skip_count"`
}

// Select partitions the regression corpus against the changed-package scope. A
// package is a skip candidate ONLY when its dependency set is known and
// provably disjoint from the scope; selected ⊎ wouldSkip is always exactly the
// input, and input order is preserved.
//
// The three fail-safes are the point of this function, not edge cases: an empty
// scope means UNKNOWN impact (never zero impact), a package with no resolved
// dependency data must always run, and an empty corpus decides nothing.
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

// ChangedScope widens a cycle's changed-package set with its REVERSE
// dependencies, so a change confined to internal/router selects
// internal/routingtest — which imports it and holds the keystone parity
// invariant. Forward-only scope marks the routing regression package skippable
// and hides exactly the class that kept main red for 5 commits (cycle-1250).
//
// Best-effort, inheriting changedpkgs.ImporterClosure's contract: an empty
// repoRoot or any `go list` failure degrades to the input set unchanged.
// Closure only ever widens; narrowing below the forward-only baseline would be
// strictly worse than having no selection at all.
func ChangedScope(repoRoot string, changed []string) []string {
	if len(changed) == 0 {
		return nil
	}
	return changedpkgs.ImporterClosure(repoRoot, changed)
}

// Compute produces the decision for one cycle. stage is the resolved
// policy.RegressionTIAConfig().Stage; repoRoot is the repository root (the dir
// containing the go/ module dir, for reverse-dependency closure); moduleDir is
// the Go module dir (for corpus enumeration); changed is the cycle's
// forward-only changed-package set.
//
// A dormant or unrecognized stage computes NOTHING and returns the zero
// Decision — the caller writes no artifact, so the audit path stays
// byte-identical under the live (block-absent) configuration.
func Compute(stage, repoRoot, moduleDir string, changed []string) Decision {
	switch stage {
	case "shadow", "enforce":
	default:
		return Decision{}
	}
	patterns := corpus(moduleDir)
	scope := ChangedScope(repoRoot, changed)
	// Dependency resolution is the expensive half and only ever narrows. With
	// no scope nothing is skippable regardless of it, so skip the work.
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
	}
}

// Emit writes d to <workspace>/ArtifactName and returns the path. An empty
// workspace is a loud error rather than a silent no-write: evidence that was
// never written must not look like evidence of nothing to report.
func Emit(workspace string, d Decision) (string, error) {
	if strings.TrimSpace(workspace) == "" {
		return "", errors.New("regressiontia: empty workspace, refusing to drop the decision")
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", fmt.Errorf("regressiontia: marshal decision: %w", err)
	}
	path := filepath.Join(workspace, ArtifactName)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("regressiontia: write %s: %w", path, err)
	}
	return path, nil
}

// corpus enumerates the regression sub-packages under moduleDir, matching the
// runner's own non-recursive "./acs/regression/<sub>" pattern shape. os.ReadDir
// returns entries already sorted by name. A missing corpus is not an error:
// there is simply nothing to reason about.
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

// escapeHatches are the stdlib packages whose presence in a predicate's
// dependency closure means the predicate observes state the import graph cannot
// see: it reads repository files or spawns a subprocess.
//
// This is the correctness crux of import-graph selection over THIS corpus, and
// it is not a hypothetical. go/acs/regression/apicover is the gate that fails a
// cycle for adding an unenrolled internal package — and it does that by reading
// go/.apicover-enforce and shelling out to `go list`, importing none of the
// code it grades. Its static dependency set is disjoint from almost every
// change, so a naive importer-graph selector marks it skippable on exactly the
// diff it exists to catch. A predicate that reaches outside the graph therefore
// has an UNDERIVABLE impact set, which the fail-safe resolves toward running it.
var escapeHatches = map[string]struct{}{
	"os":       {},
	"os/exec":  {},
	"os/user":  {},
	"syscall":  {},
	"net":      {},
	"net/http": {},
}

// resolveDeps maps each corpus pattern to the module-internal package patterns
// it transitively depends on, DROPPING any package whose closure reaches
// outside the import graph (see escapeHatches). An omitted key is read by
// Select as unresolvable ⇒ always run.
//
// Best-effort: any failure yields nil, which likewise skips nothing.
func resolveDeps(moduleDir string, patterns []string) map[string][]string {
	raw, modPath := listCorpusDeps(moduleDir, patterns)
	if len(raw) == 0 {
		return nil
	}
	out := map[string][]string{}
	for key, imports := range raw {
		var pats []string
		seen := map[string]struct{}{}
		derivable := true
		for _, dep := range imports {
			if _, hatch := escapeHatches[dep]; hatch {
				derivable = false
				break
			}
			rel := strings.TrimPrefix(dep, modPath+"/")
			if rel == dep {
				continue // stdlib or an external module: not a change surface
			}
			p := "./" + rel + "/..."
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			pats = append(pats, p)
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

// listCorpusDeps returns each corpus pattern's RAW transitive import closure
// (stdlib included — the escape-hatch classification needs it) plus the module
// path. Predicate files carry `//go:build acs`, so the tag is required or every
// dependency set comes back empty.
func listCorpusDeps(moduleDir string, patterns []string) (map[string][]string, string) {
	if moduleDir == "" || len(patterns) == 0 {
		return nil, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), depsTimeout)
	defer cancel()

	modPath, err := sysexec.Output(ctx, sysexec.DefaultRunner, moduleDir, "go", "list", "-m")
	if err != nil || modPath == "" {
		return nil, ""
	}
	// -test is load-bearing, not a refinement: a regression predicate package
	// holds ONLY _test.go files, so its plain .Deps is empty and a listing
	// without -test reports every corpus package as depending on nothing —
	// which the fail-safe reads as unresolvable and skips nothing, forever.
	// -e keeps one unloadable package from failing the whole listing; .Deps on
	// the synthesized .test package is already transitive.
	out, err := sysexec.Output(ctx, sysexec.DefaultRunner, moduleDir, "go", "list", "-e", "-tags", "acs", "-test",
		"-f", "{{.ImportPath}}|{{join .Deps \" \"}}", "./"+filepath.ToSlash(regressionDir)+"/...")
	if err != nil {
		return nil, ""
	}

	want := map[string]struct{}{}
	for _, p := range patterns {
		want[p] = struct{}{}
	}
	deps := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		self, rest, ok := strings.Cut(line, "|")
		if !ok {
			continue
		}
		key := "./" + strings.TrimPrefix(testPackageOwner(self), modPath+"/")
		if _, ok := want[key]; !ok {
			continue
		}
		deps[key] = append(deps[key], strings.Fields(rest)...)
	}
	return deps, modPath
}

// testPackageOwner maps the synthetic package names `go list -test` emits back
// to the real package they belong to: "X.test", "X [X.test]" and
// "X_test [X.test]" all own X.
func testPackageOwner(importPath string) string {
	if i := strings.Index(importPath, " ["); i >= 0 {
		importPath = importPath[:i]
	}
	importPath = strings.TrimSuffix(importPath, ".test")
	return strings.TrimSuffix(importPath, "_test")
}

// overlaps reports whether any dependency of a corpus package lies inside the
// changed scope (or vice versa — a scope entry may name a parent dir).
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

// intersects compares two go test patterns as directory prefixes. The
// module-root pattern ("./...") normalizes to the empty prefix and therefore
// intersects everything.
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
