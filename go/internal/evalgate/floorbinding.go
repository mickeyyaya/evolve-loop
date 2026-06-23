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

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/triagecap"
)

// floorbinding.go — Gate C (R9.3, triage capacity): EGPS floor predicates
// must bind ONLY floors triage committed this cycle. The cycle-280 failure
// mode: TDD authored coverage-floor predicates for tasks triage had
// DEFERRED, and the builder starved the committed task clearing gates that
// were never this cycle's work. The check is fully deterministic:
//
//  1. extract the target packages of coverage-floor predicates from this
//     cycle's acs package (go/ast over predicates_test.go — floor predicates
//     name their target as a path literal, e.g. "./internal/core/");
//  2. ask the triage artifact which of those packages appear in
//     floor-bearing ## deferred / ## dropped items;
//  3. any overlap is a CERTAIN violation → block at enforce.
//
// Fail-open on every ambiguity: missing predicates file, unparseable Go,
// missing triage artifact, or no floor predicates at all.
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
		return "", false // no floor predicates → nothing to bind wrongly
	}
	artifact, err := os.ReadFile(filepath.Join(in.Workspace, triagecap.TriageArtifactName()))
	if err != nil {
		return "", false // no triage artifact → fail open
	}
	companionPath := filepath.Join(in.Workspace, triagecap.TriageDecisionName())
	deferred := triagecap.DeferredFloorPackagesDecl(string(artifact), companionPath, targets)
	// Committed-wins subtraction: a package floor-committed this cycle may
	// carry predicates even if more of its work was ALSO deferred for later
	// (cycle 310: the gate blocked the committed package's own predicates).
	// Provenance rule (declarations outrank prose, the Layer-1 contract): a
	// DECLARED deferred_floors entry yields only to a DECLARED committed
	// floor; prose-derived deferral yields to committed evidence of any rank.
	_, deferredDeclared, _ := triagecap.ReadDeferredFloors(companionPath)
	_, committedDeclared, _ := triagecap.ReadDeclaredFloors(companionPath)
	if !deferredDeclared || committedDeclared {
		if committed := triagecap.CommittedFloorPackages(string(artifact), companionPath, targets); len(committed) > 0 {
			committedSet := map[string]bool{}
			for _, pkg := range committed {
				committedSet[pkg] = true
			}
			// [:0:0] zero-caps the reuse so appends allocate fresh — a future
			// callee returning a shared sub-slice cannot be silently corrupted.
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

// cycleNumFromWorkspace parses N from the run-dir basename "cycle-<N>".
// Sub-paths of the workspace (e.g. cycle-300/artifacts) return 0 → fail
// open; the orchestrator always passes the workspace root.
var cycleDirRE = regexp.MustCompile(`^cycle-(\d+)$`)

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

// pkgPathLitRE recognizes a Go package-path string literal a floor predicate
// targets (e.g. "./internal/core/", "./internal/adapters/bridge/"). Segments
// never start with a dot (kills "..." and hidden-dir literals); the go-test
// wildcard suffix "/..." is trimmed by the caller before matching so the
// ellipsis form still attributes to its package.
var pkgPathLitRE = regexp.MustCompile(`^\.?/?(?:internal|cmd|go)(?:/[A-Za-z0-9_][A-Za-z0-9_.-]*)+/?$`)

// floorNameRE selects coverage/floor predicates by function name.
var floorNameRE = regexp.MustCompile(`(?i)coverage|floor`)

// floorPredicateTargets parses the cycle's predicates file and returns the
// distinct basenames of package paths referenced inside coverage/floor
// predicate functions (sorted). Any read/parse failure → nil (fail open).
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
		// Only Test* functions are predicates — a non-Test helper named
		// "floorPercentHelper" must never contribute targets (its literals
		// could false-block a healthy cycle). The walk below visits the whole
		// body including nested closures (t.Run subtests).
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
			s = strings.TrimSuffix(s, "/...") // go-test wildcard targets the same package
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
