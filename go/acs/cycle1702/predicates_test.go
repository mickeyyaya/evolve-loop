//go:build acs

// Package cycle1702 materialises the acceptance criteria for
// acs-cycle50-predicates-point-at-a-moved-file: ACS predicates that read a
// source file must resolve it at its current location, and an acsassert read of
// a path that does not exist must say the file may have moved instead of
// surfacing a bare "no such file" error.
package cycle1702

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir is the worktree's Go module root, so the shelled tests compile the
// cycle's tree rather than main's copy.
func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// runACSTests runs the named predicates of one ACS package and requires a real
// PASS line for each, so a deleted, renamed or skipped predicate cannot pass.
func runACSTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	args := []string{"test", "-C", goDir(t), "-tags", "acs", "-count=1", "-v"}
	if len(names) > 0 {
		args = append(args, "-run", "^("+strings.Join(names, "|")+")$")
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", append(args, pkg)...)
	if code != 0 {
		t.Fatalf("go test %s exit=%d err=%v\nstdout:\n%s\nstderr:\n%s", pkg, code, err, stdout, stderr)
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("%s: no PASS line for %s (deleted, renamed or skipped?)\nstdout:\n%s", pkg, name, stdout)
		}
	}
}

// recordingTB captures acsassert failure messages without failing the caller.
type recordingTB struct{ msgs []string }

func (r *recordingTB) Errorf(format string, args ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}
func (r *recordingTB) Helper() {}

// hintText is every recorded message with the probed path removed, so a path
// component can never supply the "moved" wording on its own.
func (r *recordingTB) hintText(path string) string {
	return strings.ToLower(strings.ReplaceAll(strings.Join(r.msgs, "\n"), path, "<path>"))
}

// sourceReaders are the acsassert helpers that resolve a source path.
var sourceReaders = []struct {
	name string
	call func(tb acsassert.TB, path string) bool
}{
	{"FileExists", func(tb acsassert.TB, p string) bool { return acsassert.FileExists(tb, p) }},
	{"FileContains", func(tb acsassert.TB, p string) bool { return acsassert.FileContains(tb, p, "needle") }},
	{"FileNotContains", func(tb acsassert.TB, p string) bool { return acsassert.FileNotContains(tb, p, "needle") }},
	{"FileMatchesRegex", func(tb acsassert.TB, p string) bool { return acsassert.FileMatchesRegex(tb, p, `needle`) }},
}

// joinTarget is one filepath.Join(<repo root>, "lit", ...) call in a predicate file.
type joinTarget struct {
	file, fn string
	line     int
	rel      string
}

// rootIdents names the identifiers a function body binds to the repo root:
// any variable assigned from a call to a function named RepoRoot/repoRoot.
func rootIdents(body *ast.BlockStmt) map[string]bool {
	ids := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok && strings.EqualFold(name, "RepoRoot") {
			ids[id.Name] = true
		}
		return true
	})
	return ids
}

// repoJoins returns every filepath.Join whose first argument is a repo-root
// identifier and whose remaining arguments are all string literals.
func repoJoins(t *testing.T, path string) []joinTarget {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []joinTarget
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		roots := rootIdents(fd.Body)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Join" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "filepath" {
				return true
			}
			if id, ok := call.Args[0].(*ast.Ident); !ok || !roots[id.Name] {
				return true
			}
			parts := make([]string, 0, len(call.Args)-1)
			for _, a := range call.Args[1:] {
				lit, ok := a.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				parts = append(parts, s)
			}
			out = append(out, joinTarget{file: path, fn: fd.Name.Name, line: fset.Position(call.Pos()).Line, rel: filepath.ToSlash(filepath.Join(parts...))})
			return true
		})
	}
	return out
}

// staleGoJoins sweeps every cycle and regression predicate file under root and
// returns the Go-source join targets that do not exist, plus how many predicate
// files and Go-source targets it examined.
func staleGoJoins(t *testing.T, root string) (missing []string, files, goTargets int) {
	t.Helper()
	var paths []string
	for _, g := range []string{"go/acs/cycle*/predicates_test.go", "go/acs/regression/*/predicates_test.go"} {
		m, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			t.Fatalf("glob %s: %v", g, err)
		}
		paths = append(paths, m...)
	}
	for _, p := range paths {
		for _, j := range repoJoins(t, p) {
			if !strings.HasSuffix(j.rel, ".go") || strings.ContainsAny(j.rel, "*?[") {
				continue
			}
			goTargets++
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(j.rel))); err != nil {
				relFile, _ := filepath.Rel(root, j.file)
				missing = append(missing, fmt.Sprintf("%s:%d (%s) -> %s", filepath.ToSlash(relFile), j.line, j.fn, j.rel))
			}
		}
	}
	sort.Strings(missing)
	return missing, len(paths), goTargets
}

// joinsIn returns the Go-source join targets of one predicate function.
func joinsIn(t *testing.T, file, fn string) []string {
	t.Helper()
	var rels []string
	for _, j := range repoJoins(t, file) {
		if j.fn == fn && strings.HasSuffix(j.rel, ".go") {
			rels = append(rels, j.rel)
		}
	}
	sort.Strings(rels)
	return rels
}

// TestC1702_001_Cycle50SuiteGreen: `go test -count=1 -tags acs ./acs/cycle50`
// exits 0 with both release-preflight predicates actually passing.
func TestC1702_001_Cycle50SuiteGreen(t *testing.T) {
	runACSTests(t, "./acs/cycle50")
	runACSTests(t, "./acs/cycle50",
		"TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight",
		"TestC50B_005_StrictPassFlag_RegisteredInPreflight")
}

// repointed lists every predicate that read a file the cmd/evolve
// decomposition renamed, with the path it must now resolve.
var repointed = []struct {
	pkg, fn string
	want    []string
}{
	{"cycle50", "TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight", []string{"go/internal/cli/opscmd/release_preflight.go"}},
	{"cycle50", "TestC50B_005_StrictPassFlag_RegisteredInPreflight", []string{"go/internal/cli/opscmd/release_preflight.go"}},
	{"cycle47", "TestC47A_003_ReleasePreflight_AbsentFromProdSource", []string{"go/internal/cli/opscmd/release_pipeline.go"}},
	{"cycle47", "TestC47A_005_ReleasePipelineTest_NoEnvKey", []string{"go/internal/cli/opscmd/release_pipeline_test.go"}},
	{"cycle48", "TestC48B_002_GuardsLog_AbsentFromProdSource", []string{"go/internal/cli/guardcmd/guard.go"}},
	{"cycle48", "TestC48B_003_AppendGuardsLog_HasLogPathParam", []string{"go/internal/cli/guardcmd/guard.go"}},
	{"cycle48", "TestC48B_004_GuardTest_NoSetenvGuardsLog", []string{"go/internal/cli/guardcmd/guard_test.go"}},
	{"cycle11", "TestC11_004_ObserverEnvReadsGoneFromPhaseObserverCmd", []string{"go/internal/cli/phasecmd/phase_observer.go"}},
	{"cycle11", "TestC11_005_InactivityEnvReadsGoneFromPhaseWatchdogCmd", []string{"go/internal/cli/phasecmd/phase_watchdog.go"}},
}

// TestC1702_002_RepointedPredicatesResolveTheRenamedFile: each predicate that
// read a renamed cmd/evolve file now names the file's current location, not
// some other existing file.
func TestC1702_002_RepointedPredicatesResolveTheRenamedFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, r := range repointed {
		file := filepath.Join(root, "go", "acs", r.pkg, "predicates_test.go")
		got := joinsIn(t, file, r.fn)
		if strings.Join(got, ",") != strings.Join(r.want, ",") {
			t.Errorf("%s.%s resolves %v, want %v", r.pkg, r.fn, got, r.want)
		}
	}
	file := filepath.Join(root, "go", "acs", "cycle31", "predicates_test.go")
	got := joinsIn(t, file, "TestC31_004_NoEnvBypassReadsInProductionGo")
	for _, want := range []string{"go/internal/cli/guardcmd/commit_prefix_gate.go", "go/internal/cli/guardcmd/postedit_validate.go"} {
		if !contains(got, want) {
			t.Errorf("cycle31.TestC31_004_NoEnvBypassReadsInProductionGo does not resolve %s (got %v)", want, got)
		}
	}
	for _, g := range got {
		if strings.HasPrefix(g, "go/cmd/evolve/") {
			t.Errorf("cycle31.TestC31_004_NoEnvBypassReadsInProductionGo still resolves pre-move path %s", g)
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// TestC1702_003_RepointedSiblingPredicatesPass: the same stale-path class in
// cycles 47, 48, 11 and 31 now passes when run through the real ACS runner.
func TestC1702_003_RepointedSiblingPredicatesPass(t *testing.T) {
	byPkg := map[string][]string{}
	for _, r := range repointed {
		if r.pkg != "cycle50" {
			byPkg[r.pkg] = append(byPkg[r.pkg], r.fn)
		}
	}
	byPkg["cycle31"] = append(byPkg["cycle31"], "TestC31_004_NoEnvBypassReadsInProductionGo")
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	for _, p := range pkgs {
		t.Run(p, func(t *testing.T) { runACSTests(t, "./acs/"+p, byPkg[p]...) })
	}
}

// TestC1702_004_NotExistReadHintsAtRelocation: every acsassert helper that
// resolves a source path fails a missing path with a message that names the
// moved-file possibility and still names the path it tried.
func TestC1702_004_NotExistReadHintsAtRelocation(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "cmd", "evolve", "cmd_gone.go")
	for _, h := range sourceReaders {
		tb := &recordingTB{}
		if h.call(tb, gone) {
			t.Errorf("%s(%s) returned true for a nonexistent path", h.name, gone)
		}
		if len(tb.msgs) == 0 {
			t.Errorf("%s(missing path) logged no failure", h.name)
			continue
		}
		if !strings.Contains(strings.Join(tb.msgs, "\n"), gone) {
			t.Errorf("%s(missing path) message does not name the path: %q", h.name, tb.msgs)
		}
		if !strings.Contains(tb.hintText(gone), "moved") {
			t.Errorf("%s(missing path) message does not name the moved-file possibility: %q", h.name, tb.msgs)
		}
	}
}

// TestC1702_005_HintOnlyForNotExist: the moved-file hint fires only when the
// path does not exist — a content mismatch or a non-ENOENT read error keeps its
// plain message, and a satisfied assertion logs nothing.
func TestC1702_005_HintOnlyForNotExist(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.go")
	withNeedle := filepath.Join(dir, "with_needle.go")
	for p, body := range map[string]string{plain: "package x\n", withNeedle: "package x // needle\n"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	failing := []struct {
		name string
		path string
		call func(acsassert.TB) bool
	}{
		{"FileContains/absent-substring", plain, func(tb acsassert.TB) bool { return acsassert.FileContains(tb, plain, "needle") }},
		{"FileNotContains/present-substring", withNeedle, func(tb acsassert.TB) bool { return acsassert.FileNotContains(tb, withNeedle, "needle") }},
		{"FileMatchesRegex/no-match", plain, func(tb acsassert.TB) bool { return acsassert.FileMatchesRegex(tb, plain, `needle`) }},
		{"FileContains/directory", dir, func(tb acsassert.TB) bool { return acsassert.FileContains(tb, dir, "needle") }},
	}
	for _, c := range failing {
		tb := &recordingTB{}
		if c.call(tb) {
			t.Errorf("%s: returned true, want a failure", c.name)
		}
		if len(tb.msgs) == 0 {
			t.Errorf("%s: logged no failure", c.name)
		}
		if strings.Contains(tb.hintText(c.path), "moved") {
			t.Errorf("%s: moved-file hint on an existing path: %q", c.name, tb.msgs)
		}
	}
	for _, h := range sourceReaders {
		tb := &recordingTB{}
		path := withNeedle
		if h.name == "FileNotContains" {
			path = plain
		}
		if !h.call(tb, path) || len(tb.msgs) != 0 {
			t.Errorf("%s on a satisfied path: want true with no messages, got messages %q", h.name, tb.msgs)
		}
	}
}

// TestC1702_006_NoPredicateJoinsAMissingGoFile: a sweep of every cycle and
// regression predicate file finds no repo-root Go-source path that no longer
// exists.
func TestC1702_006_NoPredicateJoinsAMissingGoFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	missing, files, goTargets := staleGoJoins(t, root)
	t.Logf("swept %d predicate files, %d repo-root Go-source targets", files, goTargets)
	if files < 400 || goTargets < 100 {
		t.Fatalf("sweep examined %d predicate files / %d Go-source targets — too few, the glob or parser is broken", files, goTargets)
	}
	if len(missing) > 0 {
		t.Errorf("%d predicate path(s) point at a Go file that no longer exists:\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
}

// TestC1702_007_SweepDetectsAStaleJoin: the sweep reports a repo-root join to a
// missing Go file and ignores one that exists or that joins a non-root base.
func TestC1702_007_SweepDetectsAStaleJoin(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go", "live"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "live", "here.go"), []byte("package live\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `package cycle9

func TestC9_001(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_ = filepath.Join(root, "go", "live", "here.go")
	_ = filepath.Join(root, "go", "cmd", "evolve", "cmd_gone.go")
	_ = filepath.Join(t.TempDir(), "fixture.go")
}
`
	pkgDir := filepath.Join(root, "go", "acs", "cycle9")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	missing, files, goTargets := staleGoJoins(t, root)
	if files != 1 || goTargets != 2 {
		t.Fatalf("sweep examined %d files / %d targets, want 1 / 2", files, goTargets)
	}
	if len(missing) != 1 || !regexp.MustCompile(`TestC9_001\) -> go/cmd/evolve/cmd_gone\.go$`).MatchString(missing[0]) {
		t.Errorf("sweep reported %q, want exactly the cmd_gone.go join", missing)
	}
}

// fileReadCalls are the os functions through which an acsassert helper resolves a path.
var fileReadCalls = map[string]bool{"ReadFile": true, "Stat": true, "Lstat": true, "Open": true, "ReadDir": true}

// pathReaders parses the non-test sources of package acsassert and returns its
// exported functions that read a file, split by whether the signature can carry
// a failure message (a TB parameter or an error result) or can only return a value.
func pathReaders(t *testing.T) (messaging, silent []string) {
	t.Helper()
	srcs, err := filepath.Glob(filepath.Join(goDir(t), "pkg", "acsassert", "*.go"))
	if err != nil {
		t.Fatalf("glob acsassert sources: %v", err)
	}
	fset := token.NewFileSet()
	for _, src := range srcs {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, src, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", src, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil || !fd.Name.IsExported() || !readsFile(fd.Body) {
				continue
			}
			if carriesMessage(fd.Type) {
				messaging = append(messaging, fd.Name.Name)
			} else {
				silent = append(silent, fd.Name.Name)
			}
		}
	}
	sort.Strings(messaging)
	sort.Strings(silent)
	return messaging, silent
}

func readsFile(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && fileReadCalls[sel.Sel.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

func carriesMessage(ft *ast.FuncType) bool {
	isIdent := func(e ast.Expr, name string) bool {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == name
	}
	for _, p := range ft.Params.List {
		if isIdent(p.Type, "TB") {
			return true
		}
	}
	if ft.Results != nil {
		for _, r := range ft.Results.List {
			if isIdent(r.Type, "error") {
				return true
			}
		}
	}
	return false
}

// readerProbe calls one acsassert reader on path and returns whether it
// reported success, the failure text it produced, and its error, if it returns one.
type readerProbe func(path string) (ok bool, msg string, err error)

func tbProbe(call func(acsassert.TB, string) bool) readerProbe {
	return func(p string) (bool, string, error) {
		tb := &recordingTB{}
		ok := call(tb, p)
		return ok, strings.Join(tb.msgs, "\n"), nil
	}
}

// messagingProbes has one probe per acsassert reader that can report a failure message.
var messagingProbes = map[string]readerProbe{
	"FileExists":       tbProbe(func(tb acsassert.TB, p string) bool { return acsassert.FileExists(tb, p) }),
	"FileContains":     tbProbe(func(tb acsassert.TB, p string) bool { return acsassert.FileContains(tb, p, "needle") }),
	"FileNotContains":  tbProbe(func(tb acsassert.TB, p string) bool { return acsassert.FileNotContains(tb, p, "needle") }),
	"FileMatchesRegex": tbProbe(func(tb acsassert.TB, p string) bool { return acsassert.FileMatchesRegex(tb, p, `needle`) }),
	"JSONFieldEquals":  tbProbe(func(tb acsassert.TB, p string) bool { return acsassert.JSONFieldEquals(tb, p, "key", "value") }),
	"CountInGoFunc": func(p string) (bool, string, error) {
		if _, err := acsassert.CountInGoFunc(p, "Handler", "needle"); err != nil {
			return false, err.Error(), err
		}
		return true, "", nil
	},
}

func mentionsMoved(msg, path string) bool {
	return strings.Contains(strings.ToLower(strings.ReplaceAll(msg, path, "<path>")), "moved")
}

// TestC1702_008_EveryMessageCarryingReaderHintsAtRelocation: every exported
// acsassert function that reads a path and can report a failure (a TB parameter
// or an error result) fails a missing path with the path, the moved-file
// possibility, and, for an error result, an error chain that is still fs.ErrNotExist.
func TestC1702_008_EveryMessageCarryingReaderHintsAtRelocation(t *testing.T) {
	messaging, _ := pathReaders(t)
	probed := make([]string, 0, len(messagingProbes))
	for name := range messagingProbes {
		probed = append(probed, name)
	}
	sort.Strings(probed)
	if strings.Join(messaging, ",") != strings.Join(probed, ",") {
		t.Errorf("acsassert's message-carrying path readers are %v, but this predicate probes %v — every such reader must be probed, and every probe must name a real reader", messaging, probed)
	}
	gone := filepath.Join(t.TempDir(), "cmd", "evolve", "cmd_gone.go")
	for _, name := range probed {
		ok, msg, err := messagingProbes[name](gone)
		if ok {
			t.Errorf("%s(missing path) reported success", name)
			continue
		}
		if !strings.Contains(msg, gone) {
			t.Errorf("%s(missing path) does not name the path: %q", name, msg)
		}
		if !mentionsMoved(msg, gone) {
			t.Errorf("%s(missing path) does not name the moved-file possibility: %q", name, msg)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s(missing path) error no longer satisfies errors.Is(err, fs.ErrNotExist): %v", name, err)
		}
	}
}

// TestC1702_009_ErrorAndJSONReadersHintOnlyForNotExist: CountInGoFunc and
// JSONFieldEquals keep their plain failure for an existing path (missing
// function, unparsable source, invalid JSON, absent key, value mismatch, a
// directory) and report success without a message when satisfied.
func TestC1702_009_ErrorAndJSONReadersHintOnlyForNotExist(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.go")
	broken := filepath.Join(dir, "broken.go")
	doc := filepath.Join(dir, "doc.json")
	notJSON := filepath.Join(dir, "not.json")
	for p, body := range map[string]string{
		src:     "package x\n\nfunc Handler() { needle() }\n",
		broken:  "package x\n\nfunc (\n",
		doc:     `{"key":"value"}`,
		notJSON: `{`,
	} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := acsassert.CountInGoFunc(src, "Handler", "needle"); err != nil || n != 1 {
		t.Errorf("CountInGoFunc on a satisfied path = %d, %v; want 1, nil", n, err)
	}
	for _, c := range []struct{ name, path, fn string }{
		{"missing-function", src, "Gone"},
		{"unparsable-source", broken, "Handler"},
		{"directory", dir, "Handler"},
	} {
		_, err := acsassert.CountInGoFunc(c.path, c.fn, "needle")
		if err == nil {
			t.Errorf("CountInGoFunc/%s: nil error, want a failure", c.name)
			continue
		}
		if mentionsMoved(err.Error(), c.path) {
			t.Errorf("CountInGoFunc/%s: moved-file hint on an existing path: %v", c.name, err)
		}
	}
	tb := &recordingTB{}
	if !acsassert.JSONFieldEquals(tb, doc, "key", "value") || len(tb.msgs) != 0 {
		t.Errorf("JSONFieldEquals on a satisfied path: want true with no messages, got %q", tb.msgs)
	}
	for _, c := range []struct {
		name, path, dotPath string
		want                any
	}{
		{"invalid-json", notJSON, "key", "value"},
		{"absent-key", doc, "other", "value"},
		{"value-mismatch", doc, "key", "other"},
		{"directory", dir, "key", "value"},
	} {
		tb := &recordingTB{}
		if acsassert.JSONFieldEquals(tb, c.path, c.dotPath, c.want) {
			t.Errorf("JSONFieldEquals/%s: returned true, want a failure", c.name)
		}
		if len(tb.msgs) == 0 {
			t.Errorf("JSONFieldEquals/%s: logged no failure", c.name)
		}
		if strings.Contains(tb.hintText(c.path), "moved") {
			t.Errorf("JSONFieldEquals/%s: moved-file hint on an existing path: %q", c.name, tb.msgs)
		}
	}
}

// TestC1702_010_FollowUpInboxItemCoversTheSilentReaders: the acsassert path
// readers whose signature cannot carry a message (value-only results, no TB)
// are named in the acceptance of an inbox item that declares go/pkg/acsassert,
// loaded through the real inbox loader, so the gap is queued rather than dropped.
func TestC1702_010_FollowUpInboxItemCoversTheSilentReaders(t *testing.T) {
	_, silent := pathReaders(t)
	if len(silent) == 0 {
		t.Logf("every acsassert path reader can carry a message; no follow-up needed")
		return
	}
	root := acsassert.RepoRoot(t)
	var items []inboxbatch.Item
	for _, d := range []string{"inbox", filepath.Join("inbox", "consumed")} {
		its, _, err := inboxbatch.LoadDir(filepath.Join(root, ".evolve", d))
		if err != nil {
			t.Fatalf("load .evolve/%s: %v", d, err)
		}
		items = append(items, its...)
	}
	for _, it := range items {
		if it.ID == "acs-cycle50-predicates-point-at-a-moved-file" || !declaresAcsassert(it) {
			continue
		}
		acceptance := strings.Join(it.Acceptance, "\n")
		var unnamed []string
		for _, name := range silent {
			if !strings.Contains(acceptance, name) {
				unnamed = append(unnamed, name)
			}
		}
		if len(unnamed) == 0 {
			t.Logf("follow-up inbox item %s (%s) covers %v", it.ID, it.Path, silent)
			return
		}
	}
	t.Errorf("no inbox item declares go/pkg/acsassert with acceptance naming every value-only path reader %v — these cannot carry the moved-file hint without an API change; file the follow-up", silent)
}

func declaresAcsassert(it inboxbatch.Item) bool {
	for _, p := range it.DeclaredPaths() {
		if strings.Contains(p, "go/pkg/acsassert") {
			return true
		}
	}
	return false
}
