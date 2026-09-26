package evalgate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// floorBindingGate (Gate C) blocks EGPS floor predicates that bind a package
// triage deferred or dropped this cycle. It fails open on every ambiguity.
type floorBindingGate struct{}

func (floorBindingGate) name() string                { return "floor-binding" }
func (floorBindingGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (floorBindingGate) check(in core.ReviewInput) (string, bool) {
	cycle := cycleNumFromWorkspace(in.Workspace)
	if cycle <= 0 || in.Worktree == "" {
		return "", false
	}
	predPath := filepath.Join(in.Worktree, "go", "acs", fmt.Sprintf("cycle%d", cycle), "predicates_test.go")
	targets := floorPredicateTargets(predPath)
	if len(targets) == 0 {
		return "", false
	}
	artifact, err := os.ReadFile(filepath.Join(in.Workspace, triagecap.TriageArtifactName()))
	if err != nil {
		return "", false
	}
	companionPath := filepath.Join(in.Workspace, triagecap.TriageDecisionName())
	deferred := triagecap.DeferredFloorPackagesDecl(string(artifact), companionPath, targets)
	// Committed wins over deferred; a declared deferral yields only to a declared commitment.
	// See ADR-0046.
	_, deferredDeclared, _ := triagecap.ReadDeferredFloors(companionPath)
	_, committedDeclared, _ := triagecap.ReadDeclaredFloors(companionPath)
	if !deferredDeclared || committedDeclared {
		if committed := triagecap.CommittedFloorPackages(string(artifact), companionPath, targets); len(committed) > 0 {
			committedSet := map[string]bool{}
			for _, pkg := range committed {
				committedSet[pkg] = true
			}
			// [:0:0] makes appends allocate, so a callee-shared backing array is never overwritten.
			kept := deferred[:0:0]
			for _, pkg := range deferred {
				if !committedSet[pkg] {
					kept = append(kept, pkg)
				}
			}
			deferred = kept
		}
	}
	if len(deferred) == 0 {
		return "", false
	}
	return fmt.Sprintf(
		"floor predicate(s) bind package(s) triage deferred/dropped this cycle: %s — EGPS floor predicates may bind ONLY committed (## top_n) floors; drop these predicates or re-commit the floors",
		strings.Join(deferred, ", ")), true
}

var cycleDirRE = regexp.MustCompile(`^cycle-(\d+)$`)

// cycleNumFromWorkspace parses N from a workspace basename of exactly
// "cycle-<N>", returning 0 for anything else.
func cycleNumFromWorkspace(workspace string) int {
	m := cycleDirRE.FindStringSubmatch(filepath.Base(workspace))
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// pkgPathLitRE matches a package-path literal such as "./internal/core/". A
// segment never starts with a dot, so "..." and hidden directories never match.
var pkgPathLitRE = regexp.MustCompile(`^\.?/?(?:internal|cmd|go)(?:/[A-Za-z0-9_][A-Za-z0-9_.-]*)+/?$`)

var floorNameRE = regexp.MustCompile(`(?i)coverage|floor`)

// floorPredicateTargets returns the sorted, distinct package basenames that
// coverage or floor Test functions name; nil when the file cannot be read or parsed.
func floorPredicateTargets(predPath string) []string {
	src, err := os.ReadFile(predPath)
	if err != nil {
		return nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), predPath, src, 0)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		// Only Test functions are predicates; a helper's literals could false-block a healthy cycle.
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") || !floorNameRE.MatchString(fn.Name.Name) {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			s = strings.TrimSuffix(s, "/...")
			if !pkgPathLitRE.MatchString(s) {
				return true
			}
			seen[filepath.Base(strings.TrimRight(s, "/"))] = true
			return true
		})
	}
	if len(seen) == 0 {
		return nil
	}
	targets := make([]string, 0, len(seen))
	for p := range seen {
		targets = append(targets, p)
	}
	sort.Strings(targets)
	return targets
}
