// flakylint.go — authoring-time flaky-shape lint over ACS predicate SOURCES
// (acs-metapredicate-suite-scope). TDD keeps authoring cycle predicates that
// flake under fleet load (cycles 1173/1175/1178: whole-package meta-predicates
// whose inner tests are environment-sensitive); acssuite's scope-lint demotes
// them at RUN time, but by then the cycle has already burned a suite run. This
// lint flags the same shapes — plus the other dominant flaky classes — while
// the predicate is being AUTHORED.
//
// PRODUCTION CALLERS (a lint with no caller is dead code — the review finding
// that held this back):
//
//   - internal/evalgate.flakyShapeGate — Gate D, mounted in NewReviewer's gate
//     slice and fired by the orchestrator's per-phase DeliverableReviewer at the
//     END of the tdd phase, i.e. the moment go/acs/cycle<N>/predicates_test.go
//     first exists. ADVISORY: it can surface, never block.
//   - internal/cli/guardcmd.flakyLintAdvisory — `evolve eval quality-check
//     -predicates <path>`, the operator/agent-facing hand check of the same rule.
//
// Five deterministic patterns, each annotated with its Luo et al. FSE'14
// flakiness-taxonomy class (async-wait 45% + concurrency 20% dominate):
//
//  1. suite-scope go-test shells (concurrency): `./...`, any `/...` expansion,
//     multi-package invocations, or the known 40s+ suites (internal/core,
//     cmd/evolve). A SINGLE named package (`go test ./internal/<pkg>`) is the
//     sanctioned shape.
//  2. time.Now()-derived deadlines/timeouts (async-wait): wall-clock bounds
//     stretch arbitrarily under host contention.
//  3. hardcoded PIDs below 100000 used for liveness (environment — house
//     extension of the taxonomy): PIDs are never stable across hosts/runs.
//  4. subprocess git without `-C <dir>` (environment): resolves the repo from
//     process cwd, which differs between main tree, worktree, and fleet lanes.
//     Setting cmd.Dir (the house idiom) or a leading `cd` in sh -c also counts
//     as anchored.
//  5. unreaped load-generation (resource-leak): spawning `yes`/`stress`/shell
//     busy loops through a constructor that binds NO context to the child's
//     lifetime (exec.Command, acsassert.SubprocessOutput) leaves children
//     pinning the host after the predicate exits.
//
// Stage: ADVISORY at both callers. In the CLI a finding raises PASS→WARN (exit
// 1) via a MONOTONIC severity join, so it can never lower a Level-0 tautology
// HALT; in the gate it is structurally non-blocking (block is a constant false).
// The gate policy machinery may promote later, mirroring the runtime-reference
// promotion path (WARN-only → fail-gate after soak).
//
// FALSE-POSITIVE DISCIPLINE. A lint that prints a claim which is false about the
// code teaches authors to ignore it. Suite-scope findings are therefore
// argv-position aware (execPatternIndex, flakylint_exec.go): a package pattern is
// flagged only when it reaches a `go test` argv, or reaches NO exec argv at all.
// A pattern handed to some OTHER subprocess (`go build`/`go vet`/`rg`) is a
// compile or a search, not a suite, and stays clean. Three cuts, each measured
// against the live 282-dir corpus (341 → 297 → 179 findings, 117 → 102 → 64 dirs):
//
//   - non-`go test` argvs are exempt;
//   - the known-slow finding is suppressed for a -run/-run= narrowed invocation
//     (a RECURSIVE pattern stays flagged — -run selects which tests run, not
//     which packages are built and loaded);
//   - the scan follows ONE level into a same-package helper with the call's args
//     bound to its params, because the canonical corpus idiom puts the pattern in
//     a const the test function names and the -run in a shared helper. Without
//     the hop, 142 of 297 findings (48%) advised "narrow with -run" at code that
//     already narrowed.
//
// "Reaches no exec argv" still keeps its finding: the pattern may be built by
// concatenation or handed to a helper two levels down, which is unresolvable
// here, and an advisory note is the safe direction. Known residuals: a
// package-pattern literal used as pure DATA in a function with no exec at all
// (strings.Contains(enforceFile, "./internal/core")); helper chains deeper than
// one hop; and -run accepted by NAME, not selectivity, so `-run .` silences a
// known-slow finding while running the whole suite.
//
// Other conservative limitations (same direction as acssuite's scope-lint):
// patterns assembled by string concatenation, fmt.Sprintf, or local variables are
// invisible, as is a time.Now() split across statements (`now := time.Now();
// now.Add(...)` — no local dataflow); a missed flaky shape merely goes unflagged,
// and the runtime scope-lint plus the bounded flake retry remain the backstops.
// Package-level string consts ARE resolved (the cycle-1117 bridgePkg shape).
// The package-pattern scope rule itself lives in internal/gopkgpattern, shared
// with acssuite's run-time scope-lint so the two can never drift apart.
package evalqualitycheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gopkgpattern"
)

// Luo FSE'14 flakiness-taxonomy classes carried on each finding ("environment"
// is the house extension for host-state assumptions the taxonomy folds into
// its long tail).
const (
	FlakyClassAsyncWait    = "async-wait"    // 45% of flaky tests (Luo FSE'14)
	FlakyClassConcurrency  = "concurrency"   // 20% (Luo FSE'14)
	FlakyClassResourceLeak = "resource-leak" // Luo FSE'14 resource-leak class
	FlakyClassEnvironment  = "environment"   // house extension: PID/cwd/host assumptions
)

// FlakyFinding is one flaky-shaped pattern found in a predicate source.
type FlakyFinding struct {
	Func   string // enclosing function name
	File   string // source file basename
	Class  string // FlakyClass* taxonomy annotation
	Reason string // human-readable detail including the offending token
}

// FlakyLintReport is what LintFlakyPredicates actually looked at, alongside what
// it found. Files exists so no caller can report a clean tree it never read:
// zero findings over zero files is indistinguishable from zero findings over a
// linted tree unless the file count is carried out with the findings (the
// silent-clean class that already bit this seam once, via a dropped flag).
type FlakyLintReport struct {
	Path     string         // the path as requested (file or dir)
	Files    []string       // basenames of the .go files actually parsed, in order
	Findings []FlakyFinding // one per (function, class, reason), deduped
}

// Linted is the receipt count: how many .go files this report covers. Callers
// print it UNCONDITIONALLY — "linted 0 file(s)" is the honest rendering of a
// path that taught the caller nothing.
func (r FlakyLintReport) Linted() int { return len(r.Files) }

// flakyPIDCeiling: a literal PID below this used for liveness is a stale
// artifact of the authoring session, never a stable target.
const flakyPIDCeiling = 100000

// flakySlowSuites are the known 40s+ test suites (module-relative keys): even
// as a SINGLE named package they exceed the ACS lane budget under contention.
var flakySlowSuites = map[string]bool{
	"internal/core": true,
	"cmd/evolve":    true,
}

// flakyLoadGenBins are subprocess binaries whose only purpose is load.
var flakyLoadGenBins = map[string]bool{
	"yes":       true,
	"stress":    true,
	"stress-ng": true,
}

// flakyDeadlineMethods are time.Time methods that turn a time.Now() into a
// deadline/timeout comparison. Unix/UnixNano/UnixMilli/UnixMicro are
// DELIBERATELY absent (review MEDIUM): fmt.Sprintf("run-%d",
// time.Now().UnixNano()) is the ubiquitous unique-name idiom, not a deadline —
// flagging it prints a claim that is plainly false about the code, which is
// how authors learn to ignore a linter.
var flakyDeadlineMethods = map[string]bool{
	"Add": true, "Before": true, "After": true, "Sub": true,
}

// procPIDPathRE matches /proc/<pid> liveness paths.
var procPIDPathRE = regexp.MustCompile(`^/proc/(\d+)(/|$)`)

// LintFlakyPredicates parses the predicate source at path — a .go file or a
// cycle package dir (go/acs/cycle<N>) — and returns a report naming every file
// it parsed plus one finding per flaky shape per function. Unreadable or
// unparseable input is an ERROR, as is a directory holding no .go files: a
// caller must never be able to mistake "nothing to read" for "tree is clean".
func LintFlakyPredicates(path string) (FlakyLintReport, error) {
	report := FlakyLintReport{Path: path}
	fi, err := os.Stat(path)
	if err != nil {
		return report, fmt.Errorf("flakylint: %w", err)
	}
	paths := []string{path}
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return report, fmt.Errorf("flakylint: %w", err)
		}
		paths = nil
		for _, e := range entries { // ReadDir sorts: deterministic file order
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				paths = append(paths, filepath.Join(path, e.Name()))
			}
		}
		if len(paths) == 0 {
			return report, fmt.Errorf("flakylint: no .go files under %s (nothing linted — this is not a clean result)", path)
		}
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return report, fmt.Errorf("flakylint: %w", perr)
		}
		files = append(files, f)
		report.Files = append(report.Files, filepath.Base(p))
	}
	consts := flakyStringConsts(files)
	helpers := flakyPackageFuncs(files)
	for i, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			report.Findings = append(report.Findings, lintFuncFlaky(fn, report.Files[i], consts, helpers)...)
		}
	}
	return report, nil
}

// flakyPackageFuncs indexes every plain (non-method) function in the cycle
// package by name, so the argv scan can follow ONE level into a same-package
// helper. Methods are excluded: they need receiver resolution, which is a
// different (and unneeded) analysis.
func flakyPackageFuncs(files []*ast.File) map[string]*ast.FuncDecl {
	out := map[string]*ast.FuncDecl{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && fn.Recv == nil {
				out[fn.Name.Name] = fn
			}
		}
	}
	return out
}

// flakyStringConsts collects package-level string const/var values by name so
// const-indirected patterns (cycle-1117's bridgePkg shape) resolve at use site.
func flakyStringConsts(files []*ast.File) map[string]string {
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

// lintFuncFlaky runs every pattern check over one function body. Findings are
// deduped per (class, reason) so a pattern referenced twice reports once.
func lintFuncFlaky(fn *ast.FuncDecl, file string, consts map[string]string, helpers map[string]*ast.FuncDecl) []FlakyFinding {
	anchored := dirAnchoredVars(fn.Body)
	assigned := execAssignNames(fn.Body)
	// M4: one pass over every exec constructor reachable from this body records
	// which argv position each package pattern reached. The literal scan below
	// consults it so a pattern handed to `go build`/`go vet`/`rg` is not reported
	// as a test suite (cycle-969 buildEvolve / cycle-941 module_builds shapes).
	index := indexExecPatterns(fn, consts, helpers)
	var out []FlakyFinding
	seen := map[string]bool{}
	add := func(class, reason string) {
		key := class + "|" + reason
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, FlakyFinding{Func: fn.Name.Name, File: file, Class: class, Reason: reason})
	}
	scopeCheck := func(s string) {
		if index.suiteScopeApplies(s) {
			lintPkgPatternString(s, index.narrowedWithRun(s), add)
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				if s, err := strconv.Unquote(v.Value); err == nil {
					scopeCheck(s)
					lintProcPIDPath(s, add)
				}
			}
		case *ast.Ident:
			if s, ok := consts[v.Name]; ok {
				scopeCheck(s)
			}
		case *ast.SelectorExpr:
			lintWallClockChain(v, add)
		case *ast.CallExpr:
			lintWallClockCall(v, add)
			lintPIDLivenessCall(v, add)
			lintExecCall(v, consts, anchored, assigned, add)
		}
		return true
	})
	return out
}

// --- pattern 1: suite-scope go-test shells ----------------------------------

// lintPkgPatternString flags a package-pattern-shaped string that is recursive
// or names a known-slow suite. Space-free by construction (IsPackagePattern),
// so prose strings never fire. narrowed suppresses only the known-slow finding:
// a recursive sweep still builds and loads every package beneath it, so -run
// does not bound its cost.
func lintPkgPatternString(s string, narrowed bool, add func(class, reason string)) {
	if !gopkgpattern.IsPackagePattern(s) {
		return
	}
	switch {
	case gopkgpattern.IsRecursive(s):
		add(FlakyClassConcurrency, fmt.Sprintf(
			"suite-scope: recursive package pattern %q — whole-subtree sweeps are contention-sensitive under fleet load (cycles 1173/1175/1178); shell a single named package instead", s))
	case flakySlowSuites[gopkgpattern.Key(s)] && !narrowed:
		add(FlakyClassConcurrency, fmt.Sprintf(
			"suite-scope: known-slow suite %q (40s+ under contention) — scope the predicate to the touched package or narrow the invocation with -run, whole-repo staleness is the regression suite's job", s))
	}
}

// --- pattern 2: wall-clock deadlines ----------------------------------------

// lintWallClockChain flags time.Now().Add/Before/After/... chains — a wall
// clock read turned into a deadline or timestamp comparison.
func lintWallClockChain(sel *ast.SelectorExpr, add func(class, reason string)) {
	call, ok := sel.X.(*ast.CallExpr)
	if !ok || !isPkgCall(call, "time", "Now") || !flakyDeadlineMethods[sel.Sel.Name] {
		return
	}
	add(FlakyClassAsyncWait, fmt.Sprintf(
		"wall-clock deadline: time.Now().%s(...) — wall-clock bounds stretch under host contention; poll on state or derive bounds from the test context", sel.Sel.Name))
}

// lintWallClockCall flags time.Since / time.Until elapsed-time checks.
func lintWallClockCall(call *ast.CallExpr, add func(class, reason string)) {
	for _, name := range []string{"Since", "Until"} {
		if isPkgCall(call, "time", name) {
			add(FlakyClassAsyncWait, fmt.Sprintf(
				"wall-clock deadline: time.%s(...) — elapsed-wall-clock checks flake under load; assert on state, not on how long it took", name))
		}
	}
}

// --- pattern 3: hardcoded PID liveness --------------------------------------

// lintPIDLivenessCall flags os.FindProcess / syscall.Kill / unix.Kill with a
// literal PID below the ceiling. Only liveness call sites fire — a plain small
// int const elsewhere is untouched.
func lintPIDLivenessCall(call *ast.CallExpr, add func(class, reason string)) {
	liveness := isPkgCall(call, "os", "FindProcess") ||
		isPkgCall(call, "syscall", "Kill") || isPkgCall(call, "unix", "Kill")
	if !liveness || len(call.Args) == 0 {
		return
	}
	if pid, ok := intLitBelow(call.Args[0], flakyPIDCeiling); ok {
		addPIDFinding(pid, exprPkgName(call), add)
	}
}

// addPIDFinding is the shared reporter for every hardcoded-PID shape.
func addPIDFinding(pid int, site string, add func(class, reason string)) {
	add(FlakyClassEnvironment, fmt.Sprintf(
		"hardcoded PID %d in liveness check (%s) — PIDs are never stable across hosts/runs; discover the PID at runtime (os.Getpid, pidfile, pgrep)", pid, site))
}

// lintProcPIDPath flags /proc/<pid> path literals below the ceiling.
func lintProcPIDPath(s string, add func(class, reason string)) {
	m := procPIDPathRE.FindStringSubmatch(s)
	if m == nil {
		return
	}
	if pid, err := strconv.Atoi(m[1]); err == nil && pid < flakyPIDCeiling {
		addPIDFinding(pid, "/proc path", add)
	}
}

// intLitBelow returns (value, true) when e is a positive int literal < ceiling.
func intLitBelow(e ast.Expr, ceiling int) (int, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, false
	}
	n, err := strconv.Atoi(lit.Value)
	if err != nil || n <= 0 || n >= ceiling {
		return 0, false
	}
	return n, true
}

// exprPkgName renders a call's pkg.Fn selector for finding text ("syscall.Kill").
func exprPkgName(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if x, ok := sel.X.(*ast.Ident); ok {
			return x.Name + "." + sel.Sel.Name
		}
	}
	return "liveness call"
}

// --- patterns 1/3/4/5 at exec.Command call sites -----------------------------

// lintExecCall applies the subprocess rules to one recognized exec-constructor
// call (execConstructors): multi-package go test, git without -C (unless the
// command var gets .Dir assigned in the same function), literal-PID kill,
// un-contexted load-gen, and sh -c script bodies. Single-pattern suite-scope
// findings come from the argv-position-aware literal scan (indexExecPatterns).
func lintExecCall(call *ast.CallExpr, consts map[string]string, anchored map[string]bool, assigned map[*ast.CallExpr]string, add func(class, reason string)) {
	argv, hasCtx, ok := execArgv(call, consts)
	if !ok || len(argv) == 0 {
		return
	}
	switch argv[0] {
	case "go":
		if len(argv) > 1 && argv[1] == "test" {
			lintGoTestArgs(argv[2:], add)
		}
	case "git":
		// -C may be nested (append([]string{"-C", dir}, args...)... — the
		// cycle-962/968 house idiom), so scan the raw arg ASTs, not just the
		// top-level argv.
		if !argsContainStringLit(call.Args, "-C", consts) {
			if name := assigned[call]; name == "" || !anchored[name] {
				add(FlakyClassEnvironment,
					"git without -C: subprocess git resolves the repo from process cwd (main tree vs worktree vs fleet lane); pass -C <dir> or set cmd.Dir")
			}
		}
	case "kill":
		for _, a := range argv[1:] {
			if pid, err := strconv.Atoi(a); err == nil && pid > 0 && pid < flakyPIDCeiling {
				addPIDFinding(pid, "`kill`", add)
			}
		}
	case "bash", "sh", "zsh":
		for i := 1; i < len(argv)-1; i++ {
			if argv[i] == "-c" {
				lintShellScript(argv[i+1], hasCtx, add)
			}
		}
	default:
		if flakyLoadGenBins[argv[0]] && !hasCtx {
			addLoadGenFinding(argv[0], add)
		}
	}
}

// addLoadGenFinding is the shared reporter for unreaped load-generation.
func addLoadGenFinding(what string, add func(class, reason string)) {
	add(FlakyClassResourceLeak, fmt.Sprintf(
		"unreaped load-generation: %s spawned with no context bound to its lifetime — use exec.CommandContext (plus WaitDelay for grandchildren) so the load dies with the predicate; orphaned generators burned 8 cores for 9h across batches 18-21", what))
}

// lintGoTestArgs flags a multi-package `go test` invocation (single recursive
// and known-slow patterns are flagged by the argv-position-aware literal scan).
func lintGoTestArgs(args []string, add func(class, reason string)) {
	n := 0
	for _, a := range args {
		if gopkgpattern.IsPackagePattern(a) {
			n++
		}
	}
	if n >= 2 {
		add(FlakyClassConcurrency, fmt.Sprintf(
			"suite-scope: multi-package go test (%d package patterns in one invocation) — each extra package multiplies contention exposure; shell a single named package per predicate", n))
	}
}

// lintShellScript applies the same rules inside a sh -c script body: busy
// loops and load-gen bins (resource-leak, unless the exec carries a context),
// cwd-dependent git (a leading `cd` segment anchors it), suite-scope go test,
// and literal-PID kill. A pattern inside a script STRING never reaches the
// literal scan (the script has spaces, so it is not a package pattern), which
// is why the go-test rules are applied per segment here.
func lintShellScript(script string, hasCtx bool, add func(class, reason string)) {
	if !hasCtx && (strings.Contains(script, "while true") || strings.Contains(script, "while :")) {
		add(FlakyClassResourceLeak,
			"unreaped load-generation: shell busy loop (`while true`/`while :`) spawned with no context bound to its lifetime — use exec.CommandContext so the load dies with the predicate")
	}
	sawCd := false
	for _, seg := range strings.FieldsFunc(script, func(r rune) bool {
		return r == ';' || r == '&' || r == '|' || r == '\n'
	}) {
		f := strings.Fields(seg)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "cd":
			sawCd = true
		case "git":
			if !containsArg(f[1:], "-C") && !sawCd {
				add(FlakyClassEnvironment,
					"git without -C in shell command: subprocess git resolves the repo from process cwd; pass -C <dir> (or cd first)")
			}
		case "go":
			if len(f) > 1 && f[1] == "test" {
				narrowed := argvHasRunFilter(f)
				for _, a := range f[2:] {
					lintPkgPatternString(a, narrowed, add)
				}
				lintGoTestArgs(f[2:], add)
			}
		case "kill":
			for _, a := range f[1:] {
				if pid, err := strconv.Atoi(a); err == nil && pid > 0 && pid < flakyPIDCeiling {
					addPIDFinding(pid, "`kill`", add)
				}
			}
		default:
			if flakyLoadGenBins[f[0]] && !hasCtx {
				addLoadGenFinding(f[0], add)
			}
		}
	}
}
