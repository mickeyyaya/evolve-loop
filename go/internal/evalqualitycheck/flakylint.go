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

// Flakiness-taxonomy classes (Luo et al., FSE'14) carried on each finding.
const (
	FlakyClassAsyncWait    = "async-wait"    // 45% of flaky tests (Luo FSE'14)
	FlakyClassConcurrency  = "concurrency"   // 20% (Luo FSE'14)
	FlakyClassResourceLeak = "resource-leak" // Luo FSE'14 resource-leak class
	FlakyClassEnvironment  = "environment"   // house extension: PID/cwd/host assumptions
)

// FlakyFinding is one flaky-shaped pattern found in a predicate source.
type FlakyFinding struct {
	Func   string
	File   string // source file basename
	Class  string // FlakyClass* taxonomy annotation
	Reason string // human-readable detail including the offending token
}

// FlakyLintReport carries the files LintFlakyPredicates parsed with its findings, so zero files never reads as clean.
type FlakyLintReport struct {
	Path     string         // the path as requested (file or dir)
	Files    []string       // basenames of the .go files actually parsed, in order
	Findings []FlakyFinding // one per (function, class, reason), deduped
}

// Linted is the receipt count of parsed .go files; callers print it unconditionally.
func (r FlakyLintReport) Linted() int { return len(r.Files) }

// flakyPIDCeiling: a literal PID below it in a liveness check is a stale artifact of the authoring session.
const flakyPIDCeiling = 100000

// flakySlowSuites take 40s+ under contention even as a single named package.
var flakySlowSuites = map[string]bool{
	"internal/core": true,
	"cmd/evolve":    true,
}

var flakyLoadGenBins = map[string]bool{
	"yes":       true,
	"stress":    true,
	"stress-ng": true,
}

// Unix* methods are absent on purpose: time.Now().UnixNano() is the unique-name idiom, not a deadline.
var flakyDeadlineMethods = map[string]bool{
	"Add": true, "Before": true, "After": true, "Sub": true,
}

var procPIDPathRE = regexp.MustCompile(`^/proc/(\d+)(/|$)`)

// LintFlakyPredicates lints a predicate .go file or cycle package dir; a dir with no .go files is an error, not a clean result.
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
		for _, e := range entries { // ReadDir sorts, so file order is deterministic
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

// flakyPackageFuncs indexes plain functions for the one-level helper hop; methods would need receiver resolution.
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

// flakyStringConsts collects package-level string const/var values so const-indirected patterns resolve at the use site.
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

func lintFuncFlaky(fn *ast.FuncDecl, file string, consts map[string]string, helpers map[string]*ast.FuncDecl) []FlakyFinding {
	anchored := dirAnchoredVars(fn.Body)
	assigned := execAssignNames(fn.Body)
	// The argv-position index keeps a pattern handed to go build, go vet or rg from reading as a test suite.
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

// narrowed suppresses only the known-slow finding: a recursive sweep still builds and
// loads every package beneath it, so -run does not bound its cost.
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

func lintWallClockChain(sel *ast.SelectorExpr, add func(class, reason string)) {
	call, ok := sel.X.(*ast.CallExpr)
	if !ok || !isPkgCall(call, "time", "Now") || !flakyDeadlineMethods[sel.Sel.Name] {
		return
	}
	add(FlakyClassAsyncWait, fmt.Sprintf(
		"wall-clock deadline: time.Now().%s(...) — wall-clock bounds stretch under host contention; poll on state or derive bounds from the test context", sel.Sel.Name))
}

func lintWallClockCall(call *ast.CallExpr, add func(class, reason string)) {
	for _, name := range []string{"Since", "Until"} {
		if isPkgCall(call, "time", name) {
			add(FlakyClassAsyncWait, fmt.Sprintf(
				"wall-clock deadline: time.%s(...) — elapsed-wall-clock checks flake under load; assert on state, not on how long it took", name))
		}
	}
}

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

func addPIDFinding(pid int, site string, add func(class, reason string)) {
	add(FlakyClassEnvironment, fmt.Sprintf(
		"hardcoded PID %d in liveness check (%s) — PIDs are never stable across hosts/runs; discover the PID at runtime (os.Getpid, pidfile, pgrep)", pid, site))
}

func lintProcPIDPath(s string, add func(class, reason string)) {
	m := procPIDPathRE.FindStringSubmatch(s)
	if m == nil {
		return
	}
	if pid, err := strconv.Atoi(m[1]); err == nil && pid < flakyPIDCeiling {
		addPIDFinding(pid, "/proc path", add)
	}
}

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

func exprPkgName(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if x, ok := sel.X.(*ast.Ident); ok {
			return x.Name + "." + sel.Sel.Name
		}
	}
	return "liveness call"
}

// lintExecCall applies the subprocess rules to one exec call; single-pattern
// suite-scope findings come from the argv-position-aware literal scan instead.
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
		// -C may sit inside append([]string{"-C", dir}, args...), so scan the raw arg ASTs.
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

func addLoadGenFinding(what string, add func(class, reason string)) {
	add(FlakyClassResourceLeak, fmt.Sprintf(
		"unreaped load-generation: %s spawned with no context bound to its lifetime — use exec.CommandContext (plus WaitDelay for grandchildren) so the load dies with the predicate; orphaned generators burned 8 cores for 9h across batches 18-21", what))
}

// lintGoTestArgs flags only multi-package invocations; single patterns go through the literal scan.
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

// lintShellScript applies the go-test rules per segment because a script string has
// spaces and so never reaches the literal scan as a package pattern.
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
