//go:build acs

package cycle2411

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePackage = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

const serialListFile = "parallel_serial_tests_test.go"

const serialEntriesAtBase = 1035

var routerCluster = []string{
	"TestLegacyAdvisorDispatch_MatchesTheGolden",
	"TestRouterDispatch_ARefusedRouteIsAnError",
}

var bridgeSessionsCluster = []string{
	"TestBridgeSessions_ADeadWritersSnapshotIsNotListedAsLive",
	"TestBridgeSessions_ASnapshotWhoseSessionIsGoneReadsGone",
	"TestBridgeSessions_AnUnreachableSocketWarnsAndKeepsTheSnapshotState",
	"TestBridgeSessions_EmptyProjectSaysSo",
	"TestBridgeSessions_ListsEachLivePaneWithItsLiveness",
	"TestBridgeSessions_LiveLoopSocketsShowUnwatchedSessions",
	"TestBridgeSessions_TableNamesEveryColumn",
	"TestBridgeSessions_WritesNothing",
}

var seatHealthCluster = []string{
	"TestWaveStatus_SeatHealth_ABusyPaneSilentForHoursReadsStalled",
	"TestWaveStatus_SeatHealth_ABusyPaneWithFreshOutputReadsWorking",
	"TestWaveStatus_SeatHealth_AClosedRunShowsNoSeat",
	"TestWaveStatus_SeatHealth_ADeadSessionTheLastOneOnItsSocketReadsZombie",
	"TestWaveStatus_SeatHealth_ADeadSessionWithAnOpenRunReadsZombie",
	"TestWaveStatus_SeatHealth_ADeliverableMakesTheSeatIdleEvenAtItsPrompt",
	"TestWaveStatus_SeatHealth_ADeliverableOfAnEarlierRoundNeverHidesTheStateOfThisRound",
	"TestWaveStatus_SeatHealth_ADeliverableOutranksADeadSession",
	"TestWaveStatus_SeatHealth_ADeliverableWrittenByThisRoundReadsIdle",
	"TestWaveStatus_SeatHealth_AGoneServerIsAZombieNotAWarning",
	"TestWaveStatus_SeatHealth_AHealthyListingRaisesNoWarning",
	"TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerNeverReadsZombie",
	"TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerWarnsTheOperatorAndKeepsTheSeat",
	"TestWaveStatus_SeatHealth_AnUnreachableSocketNeverReadsZombie",
	"TestWaveStatus_SeatHealth_AnUnreadablePaneWatchFileWarnsAndShowsNoSeat",
	"TestWaveStatus_SeatHealth_APaneAtItsPromptWithNoDeliverableReadsBlocked",
	"TestWaveStatus_SeatHealth_TheViewReadsTmuxThroughTheListSeamAndChangesNothing",
}

var rootEventsCluster = []string{
	"TestNewRoleSignalCenter_AnUnwritableEventsDirectoryNeverFailsTheRootAndWarnsOnce",
	"TestNewRoleSignalCenter_PublishesUnderItsRoleAndTheCloserDrainsTheQueues",
	"TestRootSignalCenter_ThePublisherLeavesTheCycleRecordUnchanged",
	"TestNewQualityCenter_AttachesThePublisherUnderTheQualityRole",
	"TestEmitIntegrationSignal_SendsTheEventThroughACenterOfTheIntegrationRole",
}

var seamGlobals = map[string]bool{
	"bridgeSessionsTmux": true, "bridgeLiveSockets": true, "bridgeSessionsNow": true, "attachRootEventsFn": true,
}

var envReaders = map[string]bool{"Environ": true, "Getenv": true, "LookupEnv": true}

var processStateWriters = map[string]bool{"Setenv": true, "Chdir": true, "Unsetenv": true, "Clearenv": true}

var assertionPrefixes = []string{"want", "assert", "Assert", "expect", "require", "must"}

var assertionMethods = map[string]bool{"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Fail": true, "FailNow": true}

func allMoved() []string {
	var all []string
	for _, group := range [][]string{routerCluster, bridgeSessionsCluster, seatHealthCluster, rootEventsCluster} {
		all = append(all, group...)
	}
	return all
}

type cmdEvolveTree struct {
	dir     string
	sources map[string]string
	fset    *token.FileSet
	files   map[string]*ast.File
	funcs   map[string][]*ast.FuncDecl
	methods map[string][]*ast.FuncDecl
	owner   map[*ast.FuncDecl]string
	imports map[string]map[string]bool
}

func loadTree(t *testing.T) *cmdEvolveTree {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve")
	sources, err := paralleltest.Sources(dir)
	if err != nil {
		t.Fatal(err)
	}
	tree := &cmdEvolveTree{dir: dir, sources: sources, fset: token.NewFileSet(), files: map[string]*ast.File{},
		funcs: map[string][]*ast.FuncDecl{}, methods: map[string][]*ast.FuncDecl{},
		owner: map[*ast.FuncDecl]string{}, imports: map[string]map[string]bool{}}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		file, err := parser.ParseFile(tree.fset, name, sources[name], 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		tree.files[name] = file
		tree.imports[name] = importNames(file)
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			tree.owner[fd] = name
			if fd.Recv == nil {
				tree.funcs[fd.Name.Name] = append(tree.funcs[fd.Name.Name], fd)
			} else {
				tree.methods[fd.Name.Name] = append(tree.methods[fd.Name.Name], fd)
			}
		}
	}
	return tree
}

func importNames(file *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = true
	}
	return out
}

func (tree *cmdEvolveTree) topLevelTest(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()
	for _, fd := range tree.funcs[name] {
		if strings.HasSuffix(tree.owner[fd], "_test.go") {
			return fd
		}
	}
	t.Errorf("%s: no such top-level test in %s", name, tree.dir)
	return nil
}

type reach func(owner string, method bool) bool

func (tree *cmdEvolveTree) callees(fd *ast.FuncDecl, allow reach) []*ast.FuncDecl {
	imports := tree.imports[tree.owner[fd]]
	var out []*ast.FuncDecl
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var candidates []*ast.FuncDecl
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			candidates = tree.funcs[fun.Name]
		case *ast.SelectorExpr:
			if id, isIdent := fun.X.(*ast.Ident); isIdent && imports[id.Name] {
				return true
			}
			candidates = tree.methods[fun.Sel.Name]
		}
		_, isMethodCall := call.Fun.(*ast.SelectorExpr)
		for _, c := range candidates {
			if allow(tree.owner[c], isMethodCall) {
				out = append(out, c)
			}
		}
		return true
	})
	return out
}

func (tree *cmdEvolveTree) closure(root *ast.FuncDecl, allow reach) []*ast.FuncDecl {
	seen := map[*ast.FuncDecl]bool{root: true}
	queue := []*ast.FuncDecl{root}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range tree.callees(cur, allow) {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	out := make([]*ast.FuncDecl, 0, len(seen))
	for fd := range seen {
		out = append(out, fd)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name.Name < out[j].Name.Name })
	return out
}

func (tree *cmdEvolveTree) where(fd *ast.FuncDecl) string {
	return tree.owner[fd] + ":" + fd.Name.Name
}

func firstStatementIsParallel(fd *ast.FuncDecl) bool {
	if len(fd.Body.List) == 0 {
		return false
	}
	stmt, ok := fd.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Parallel"
}

func processStateMutations(fd *ast.FuncDecl, includeProcessWriters bool) []string {
	var found []string
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && seamGlobals[id.Name] {
					found = append(found, "assigns the seam "+id.Name)
				}
			}
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok && includeProcessWriters && processStateWriters[sel.Sel.Name] {
				found = append(found, "calls "+sel.Sel.Name)
			}
		}
		return true
	})
	return found
}

func environmentReads(fd *ast.FuncDecl) []string {
	var found []string
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && envReaders[sel.Sel.Name] {
			if id, isIdent := sel.X.(*ast.Ident); isIdent && id.Name == "os" {
				found = append(found, "reads the process environment through os."+sel.Sel.Name)
			}
		}
		return true
	})
	return found
}

func assertionCount(fd *ast.FuncDecl) int {
	count := 0
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if assertionMethods[fun.Sel.Name] || hasAssertionPrefix(fun.Sel.Name) {
				count++
			}
		case *ast.Ident:
			if hasAssertionPrefix(fun.Name) {
				count++
			}
		}
		return true
	})
	return count
}

func hasAssertionPrefix(name string) bool {
	for _, prefix := range assertionPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func mapLiteralKeys(t *testing.T, file *ast.File, varName string) map[string]bool {
	t.Helper()
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			if len(vs.Names) != 1 || vs.Names[0].Name != varName || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			keys := map[string]bool{}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := kv.Key.(*ast.BasicLit); ok {
					if s, err := strconv.Unquote(key.Value); err == nil {
						keys[s] = true
					}
				}
			}
			return keys
		}
	}
	t.Fatalf("var %s: no map literal found", varName)
	return nil
}

func (tree *cmdEvolveTree) serialKeys(t *testing.T) map[string]bool {
	t.Helper()
	return mapLiteralKeys(t, tree.files[serialListFile], "serialTests")
}

func (tree *cmdEvolveTree) guardConfig(t *testing.T) paralleltest.Config {
	t.Helper()
	serial := map[string]string{}
	for key := range tree.serialKeys(t) {
		serial[key] = "listed"
	}
	return paralleltest.Config{Serial: serial, GlobalSetters: mapLiteralKeys(t, tree.files["parallel_default_test.go"], "processGlobalSetters")}
}

type testEvent struct {
	Action  string  `json:"Action"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type testRun struct {
	events []testEvent
	tail   string
}

func (r testRun) paused(name string) bool { return r.has(name, "pause") }

func (r testRun) passed(name string) bool { return r.has(name, "pass") }

func (r testRun) has(name, action string) bool {
	for _, e := range r.events {
		if e.Test == name && e.Action == action {
			return true
		}
	}
	return false
}

func (r testRun) elapsed(name string) float64 {
	for _, e := range r.events {
		if e.Test == name && e.Action == "pass" {
			return e.Elapsed
		}
	}
	return 0
}

func runNamedTests(t *testing.T, names []string, extraEnv []string) testRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-run", pattern, cmdEvolvePackage)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.WaitDelay = 10 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()
	var run testRun
	var text strings.Builder
	sc := bufio.NewScanner(&stdout)
	sc.Buffer(nil, 16*1024*1024)
	for sc.Scan() {
		var e testEvent
		if line := sc.Bytes(); len(line) > 0 && line[0] == '{' && json.Unmarshal(line, &e) == nil {
			run.events = append(run.events, e)
			text.WriteString(e.Output)
		}
	}
	run.tail = acsassert.OutputTail(text.String()+stderr.String(), 30)
	return run
}

func ambientRouterEnv() []string {
	return []string{"EVOLVE_CLI=codex-tmux", "EVOLVE_ROUTER_CLI=codex-tmux"}
}

func stateWritingClusters() []string {
	var names []string
	names = append(names, bridgeSessionsCluster...)
	names = append(names, seatHealthCluster...)
	names = append(names, rootEventsCluster...)
	return names
}

func inFiles(files ...string) reach {
	set := map[string]bool{}
	for _, f := range files {
		set[f] = true
	}
	return func(owner string, _ bool) bool { return set[owner] }
}

func anyFunctionAnyMethodIn(files ...string) reach {
	set := map[string]bool{}
	for _, f := range files {
		set[f] = true
	}
	return func(owner string, method bool) bool { return !method || set[owner] }
}

func (tree *cmdEvolveTree) productionVarInits() map[string][]ast.Node {
	out := map[string][]ast.Node{}
	for name, file := range tree.files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if i < len(vs.Values) {
						out[id.Name] = append(out[id.Name], vs.Values[i])
					} else if len(vs.Values) == 1 {
						out[id.Name] = append(out[id.Name], vs.Values[0])
					}
				}
			}
		}
	}
	return out
}

func (tree *cmdEvolveTree) productionReach(t *testing.T) map[*ast.FuncDecl]bool {
	t.Helper()
	seen := map[*ast.FuncDecl]bool{}
	seenVars := map[string]bool{}
	varInits := tree.productionVarInits()
	type work struct {
		node    ast.Node
		imports map[string]bool
	}
	var queue []work
	for _, fd := range tree.funcs["main"] {
		if !strings.HasSuffix(tree.owner[fd], "_test.go") {
			seen[fd] = true
			queue = append(queue, work{fd.Body, tree.imports[tree.owner[fd]]})
		}
	}
	if len(queue) == 0 {
		t.Fatal("no production main function")
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		ast.Inspect(cur.node, func(n ast.Node) bool {
			var candidates []*ast.FuncDecl
			switch node := n.(type) {
			case *ast.Ident:
				candidates = tree.funcs[node.Name]
				if inits, ok := varInits[node.Name]; ok && !seenVars[node.Name] {
					seenVars[node.Name] = true
					for _, init := range inits {
						queue = append(queue, work{init, cur.imports})
					}
				}
			case *ast.SelectorExpr:
				if id, ok := node.X.(*ast.Ident); !ok || !cur.imports[id.Name] {
					candidates = tree.methods[node.Sel.Name]
				}
			}
			for _, c := range candidates {
				if !strings.HasSuffix(tree.owner[c], "_test.go") && !seen[c] {
					seen[c] = true
					queue = append(queue, work{c.Body, tree.imports[tree.owner[c]]})
				}
			}
			return true
		})
	}
	return seen
}

func (tree *cmdEvolveTree) closureMentions(entry *ast.FuncDecl, allow reach, ident string) bool {
	for _, fd := range tree.closure(entry, allow) {
		found := false
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == ident {
				found = true
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}
