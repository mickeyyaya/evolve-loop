package acssuite

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// scopelint.go — demote out-of-scope meta-predicates instead of letting them
// false-fail cycles.
//
// Cycles 1115/1116/1117/1123 each FAILed on one predicate of the shape
// "go test <core+bridge+recovery> stays green". A whole-suite sweep samples
// everything in those packages — an auditor probe, a shared test fixture
// root, a concurrent lane — and reports any contamination as a builder
// regression. Whole-repo staleness is the regression suite's job; a cycle
// predicate re-sweeping UNTOUCHED packages is duplication with a false-red
// surface. Such predicates are demoted to SKIP (a skip counts neither red nor
// green, so a lint false-positive can never fail a cycle) with a loud
// EvidenceNote + verdict warning.
//
// Known conservative limitation: patterns assembled by string concatenation
// or returned from helper functions are invisible to the lint. That is the
// safe direction (a missed broad predicate merely stays un-demoted), and the
// demotion floor below bounds what a deliberately-broad authoring style could
// ever hide.

// ScopeFinding is one out-of-scope package reference inside a cycle predicate.
type ScopeFinding struct {
	Test    string // predicate test function name
	File    string // basename of the source file
	Pattern string // the offending package pattern, as written
}

// LintPredicateScope parses the cycle predicate sources in dir and returns a
// finding per test function that references a Go package pattern outside
// touched. An empty touched set lints nothing — with no scope authority a
// wrong guess would demote a legitimate gate. Const indirection is resolved
// (cycle-1117's bridgePkg shape): package-level string consts/vars count as
// references in every function that names them.
func LintPredicateScope(dir string, touched []string) ([]ScopeFinding, error) {
	if len(touched) == 0 {
		return nil, nil
	}
	inScope := map[string]bool{}
	for _, p := range touched {
		if key := patternKey(p); key != "" {
			inScope[key] = true
		}
	}

	files, names, err := parseGoDir(dir)
	if err != nil {
		return nil, err
	}
	consts := packageStringConsts(files)
	var findings []ScopeFinding
	for i, file := range files {
		findings = append(findings, fileScopeFindings(file, names[i], consts, inScope)...)
	}
	return findings, nil
}

// parseGoDir parses every .go file in dir (house idiom: os.ReadDir +
// parser.ParseFile — parser.ParseDir/ast.Package are deprecated since 1.22).
// Deterministic file order, unlike ParseDir's map iteration.
func parseGoDir(dir string) ([]*ast.File, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if perr != nil {
			return nil, nil, perr
		}
		files = append(files, f)
		names = append(names, e.Name())
	}
	return files, names, nil
}

// fileScopeFindings lints one parsed file's test functions.
func fileScopeFindings(file *ast.File, name string, consts map[string]string, inScope map[string]bool) []ScopeFinding {
	var findings []ScopeFinding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		for _, pat := range functionPackagePatterns(fn, consts) {
			key := patternKey(pat)
			if key == "" || strings.HasPrefix(key, "acs/") || inScope[key] {
				continue
			}
			findings = append(findings, ScopeFinding{Test: fn.Name.Name, File: name, Pattern: pat})
		}
	}
	return findings
}

// packageStringConsts collects package-level string const/var values by name.
func packageStringConsts(files []*ast.File) map[string]string {
	out := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range vs.Names {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if v, err := strconv.Unquote(lit.Value); err == nil {
								out[ident.Name] = v
							}
						}
					}
				}
			}
		}
	}
	return out
}

// functionPackagePatterns returns every package-pattern-shaped string a test
// function references — inline literals plus package-level const/var names.
func functionPackagePatterns(fn *ast.FuncDecl, consts map[string]string) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				if s, err := strconv.Unquote(v.Value); err == nil && isPackagePattern(s) {
					out = append(out, s)
				}
			}
		case *ast.Ident:
			if s, ok := consts[v.Name]; ok && isPackagePattern(s) {
				out = append(out, s)
			}
		}
		return true
	})
	return out
}

// isPackagePattern reports whether s is shaped like a `go test` package
// argument. Delegates to gopkgpattern — the SAME rule the authoring-time
// flaky-shape lint applies (internal/evalqualitycheck), so a shape one lint
// stops recognizing cannot silently stop being demoted by the other.
func isPackagePattern(s string) bool {
	return gopkgpattern.IsPackagePattern(s)
}

// wholeModuleKey is patternKey's marker for "./..." — every package at once,
// in scope only if the touched set itself says "everything" (it never does).
const wholeModuleKey = "(whole-module)"

// patternKey normalizes a pattern to a module-relative directory key so
// "./internal/bridge/...", "./internal/bridge" and the full import path all
// compare equal ("internal/bridge"). Unrecognized shapes yield "". The bare
// whole-module sweep gets this package's never-in-scope marker instead of
// gopkgpattern's "" (which would make it fail open into "unrecognized").
func patternKey(p string) string {
	if strings.TrimSpace(p) == gopkgpattern.WholeModule {
		return wholeModuleKey
	}
	return gopkgpattern.Key(p)
}

// scopeLintChangedPackages derives the touched set for the lint from GIT, and
// git only — never the builder-written handoff. Two reasons, both from
// adversarial review: the handoff has been extinct since ~cycle-215 (reading
// it made the lint dead code on every live cycle), and it is agent-authored,
// so trusting it would let a builder shrink `touched` to demote sibling
// predicates (gate-weakening). Seam var so tests can inject.
var scopeLintChangedPackages = func(worktreeRoot string) []string {
	pkgs, ok := changedpkgs.FromGitChecked(worktreeRoot, "HEAD")
	if !ok {
		return nil // git underivable → no scope authority → lint stands down
	}
	return pkgs
}

// demoteOutOfScope applies the scope lint to the CURRENT cycle's results: a
// predicate whose source references package patterns outside the cycle's
// git-derived touched set is demoted to SKIP regardless of outcome —
// authorship-based, so TDD sees it on the first run, not only under
// contamination. Loud (EvidenceNote + verdict warning); a lint error disables
// the lint WITH a warning (fail-open is the direction that cannot weaken the
// gate, but it must not be silent).
//
// DEMOTION FLOOR (gate-weakening guard): if demotion would leave the cycle
// with ZERO live (non-skip) own predicates, ALL demotions are cancelled and a
// warning says so — a cycle can never ship with its whole predicate set
// demoted, so a stray broad literal in every predicate cannot vanish a real
// red. Corollary: a cycle whose ONLY predicate is broad keeps its red — the
// false-red survives there, but in the safe (never gate-weakening) direction.
func demoteOutOfScope(results []Result, opts Options) []string {
	touched := scopeLintChangedPackages(opts.Root)
	if len(touched) == 0 {
		return nil
	}
	findings, err := LintPredicateScope(currentCycleGoPkgDir(moduleRoot(opts), opts.Cycle), touched)
	if err != nil {
		return []string{fmt.Sprintf("scope-lint: disabled (predicate sources unparseable: %v)", err)}
	}
	if len(findings) == 0 {
		return nil
	}
	flagged := map[string][]string{}
	for _, f := range findings {
		flagged[f.Test] = append(flagged[f.Test], f.Pattern)
	}

	cyclePrefix := fmt.Sprintf("cycle%d/", opts.Cycle)
	var demote []int
	liveOwn := 0
	for i, r := range results {
		name, ok := strings.CutPrefix(r.ACID, cyclePrefix)
		if !ok {
			continue
		}
		if r.ResultStr != "skip" {
			liveOwn++
		}
		if _, hit := flagged[testRootName(name)]; hit && r.ResultStr != "skip" {
			demote = append(demote, i)
		}
	}
	if len(demote) > 0 && len(demote) == liveOwn {
		return []string{fmt.Sprintf("scope-lint: %d out-of-scope predicate(s) found but demotion CANCELLED — it would leave zero live own predicates (gate-weakening floor); scope the predicates to the touched packages %v", len(demote), touched)}
	}

	var warnings []string
	for _, i := range demote {
		name := strings.TrimPrefix(results[i].ACID, cyclePrefix)
		pats := flagged[testRootName(name)]
		results[i].ResultStr = "skip"
		results[i].ExitCode = SkipExitCode
		// APPEND to any existing note — parseGoTestJSON may have recorded the
		// no-FAIL-line diagnosis (compile failure/timeout), and the demotion
		// must not erase fix-1's evidence with fix-4's.
		if results[i].EvidenceNote != "" {
			results[i].EvidenceNote += " | "
		}
		results[i].EvidenceNote += fmt.Sprintf(
			"out-of-scope meta-predicate demoted to SKIP: shells go test over %v beyond this cycle's touched packages %v — "+
				"whole-repo staleness is the regression suite's job; scope the predicate to the touched packages (whole-suite false-red class, e.g. cycles 1115/1117 auditor probes, 1116 shared-fixture contention)",
			pats, touched)
		warnings = append(warnings, fmt.Sprintf("scope-lint: %s demoted to SKIP (references %v; touched %v)", results[i].ACID, pats, touched))
	}
	return warnings
}

// testRootName maps a go-test result name to its declaring function: a
// subtest "TestFoo/case_3" demotes with its parent (the lint sees functions,
// go test emits one Result per subtest — cycles with table-driven predicates
// would otherwise keep red subtests after the parent demoted).
func testRootName(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

// moduleRoot resolves the Go module dir (shared default with runGoTest).
func moduleRoot(opts Options) string {
	if opts.GoModuleDir != "" {
		return opts.GoModuleDir
	}
	return filepath.Join(opts.Root, "go")
}
