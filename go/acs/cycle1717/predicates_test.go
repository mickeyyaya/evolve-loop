//go:build acs

// Package cycle1717 materialises the acceptance criteria for
// task-contract-refs-parity: the Task Contract's refs (core.taskItemRefs) and
// the committed set (core.ContractTaskIDs) are ONE projection — committedset —
// and ctx fleet_scope_paths only places a committed member's inbox record.
//
// 001 drives the real core.Orchestrator.RunCycle with a request-context fleet
// scope and no lane pin while triage decides a different id, and asserts every
// dispatched tdd/build/audit Task Contract names exactly ContractTaskIDs.
// 002-004 bind the frozen in-package core tests (taskItemRefs is unexported)
// by their `--- PASS:` markers from ONE -race, -run-narrowed run of
// ./internal/core; 003 also walks taskItemRefs's static call graph; 004 runs
// go vet.
//
// Reachability probe (cycle-644 rule): internal/core already imports
// committedset (task_contract.go), and acs/cycle1717 -> core/fixtures is a
// leaf import, so no pin here can demand an import cycle.
package cycle1717

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const corePkg = "./internal/core"

// parityRows is the frozen shared table of TestTaskItemRefs_ContractTaskIDsParity.
var parityRows = []string{
	"pin",
	"pin_wins_over_decision_and_deferral_subtracts",
	"pin_ignores_scope_path_ids_it_does_not_name",
	"decision_only",
	"decision_with_deferral",
	"empty",
	"explicit_empty_pin_falls_through_to_decision",
	"malformed_pin_falls_through_to_decision",
	"stale_scope_without_pin_yields_to_decision",
	"decomposed_decision_without_pin_ignores_scope",
	"scope_paths_without_binding_are_not_members",
	"scope_without_binding_is_not_members",
	"scope_paths_add_no_member_to_a_decision",
	"explicit_empty_commitment_is_not_resurrected_from_scope",
	"malformed_decision_with_scope_binds_nothing",
	"padded_pin_ids_are_trimmed_and_blanks_dropped",
	"blank_only_pin_falls_through_to_decision",
	"padded_deferral_still_subtracts",
	"padded_top_n_ids_are_trimmed",
}

var pathRows = []string{
	"pin_with_partial_and_stale_disclosure",
	"decision_with_stale_disclosure",
	"pair_path_wins_over_resolver",
	"unplaceable_member_renders_unresolved_never_dropped",
}

var renderRows = []string{"lane_pin", "decision_only", "deferral_under_a_pin", "deferral_in_a_decision"}

// renderingTests are the existing tests that pin the rendered block for a lane
// pin, a decision-only cycle and a deferral, on both dispatch surfaces.
var renderingTests = []string{
	"TestSeedTaskContract_RendersIdenticallyForPinDecisionAndDeferral",
	"TestTaskItemRefs_PathsThenScopeThenTriage",
	"TestTaskItemRefs_DeferredScopeIsNotMandatory",
	"TestTaskContract_MultiSlugProjectionParity",
	"TestDispatch_TaskContractMultiSlugLiveAndResume",
	"TestDispatch_TaskContractReachesTDDBuildAndAudit",
	"TestResume_TaskContractSeededOnTheResumeSurface",
	"TestContractTaskIDs",
	"TestContractTaskIDs_WhitespacePaddedIDsAreNotPhantomMembers",
	"TestSeedTaskContract_DocumentCycle",
	"TestComposeTaskContract_VerbatimAcceptanceAndLoudGaps",
}

// ---------------------------------------------------------------------------
// One bound run of the frozen core tests, shared by 002-004.
// ---------------------------------------------------------------------------

type boundRun struct {
	out  string
	code int
}

var (
	boundOnce sync.Once
	bound     boundRun
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func coreBoundRun(t *testing.T) boundRun {
	t.Helper()
	dir := goDir(t)
	boundOnce.Do(func() {
		names := append([]string{"TestTaskItemRefs_ContractTaskIDsParity", "TestTaskItemRefs_ScopePathsResolvePathsNeverMembership"}, renderingTests...)
		cmd := exec.Command("go", "test", "-race", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", corePkg)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		bound = boundRun{out: string(out)}
		if err != nil {
			bound.code = -1
			if ee, ok := err.(*exec.ExitError); ok {
				bound.code = ee.ExitCode()
			}
		}
	})
	if strings.Contains(bound.out, "[build failed]") || strings.Contains(bound.out, "[setup failed]") {
		t.Fatalf("RED: %s does not compile:\n%s", corePkg, trimTail(bound.out))
	}
	return bound
}

// requirePass fails unless the bound run reports `--- PASS: <name> (` for the
// test and every listed subtest (a renamed, deleted or skipped row is RED).
func requirePass(t *testing.T, run boundRun, test string, rows ...string) {
	t.Helper()
	names := []string{test}
	for _, r := range rows {
		names = append(names, test+"/"+r)
	}
	for _, n := range names {
		if !strings.Contains(run.out, "--- PASS: "+n+" (") {
			t.Errorf("RED: binding test %s did NOT pass (failing, renamed or missing)", n)
		}
	}
	if t.Failed() {
		t.Logf("bound run (exit %d):\n%s", run.code, trimTail(run.out))
	}
}

func trimTail(s string) string {
	const max = 6000
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}

// ---------------------------------------------------------------------------
// 001 — the production dispatch path.
// ---------------------------------------------------------------------------

type tempWorktree struct{ path string }

func (w *tempWorktree) Create(string, int) (string, error) { return w.path, nil }
func (w *tempWorktree) Cleanup(string, string) error       { return nil }

// gitRepo returns a temp dir holding one empty commit (the Build explanation
// contract needs a resolvable base SHA). Every git call is -C-anchored.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "base"}} {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.email=acs@evolve", "-c", "user.name=acs"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func writeItem(t *testing.T, dir, id, body string) string {
	t.Helper()
	p := filepath.Join(dir, id+".json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// decidingTriage writes triage's structured decision into the cycle workspace,
// as the real triage phase does, then records the dispatch.
type decidingTriage struct {
	*fixtures.FakeRunner
	decision string
}

func (r *decidingTriage) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return core.PhaseResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "triage-decision.json"), []byte(r.decision), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	return r.FakeRunner.Run(ctx, req)
}

// contractObserver snapshots the committed set from the dispatched workspace
// at dispatch time, beside the Task Contract block the request carried.
type contractObserver struct {
	*fixtures.FakeRunner
	mu        sync.Mutex
	blocks    []string
	committed [][]string
}

func (r *contractObserver) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.mu.Lock()
	r.blocks = append(r.blocks, req.Context[core.CtxKeyTaskContract])
	r.committed = append(r.committed, core.ContractTaskIDs(req.Workspace))
	r.mu.Unlock()
	return r.FakeRunner.Run(ctx, req)
}

// headerIDs returns the ids of a rendered block's "### <id> — " headers.
func headerIDs(block string) []string {
	var ids []string
	for _, line := range strings.Split(block, "\n") {
		if rest, ok := strings.CutPrefix(line, "### "); ok {
			if id, _, ok := strings.Cut(rest, " — "); ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func TestC1717_001_DispatchedTaskContractNamesTheCommittedSetNotTheStaleScope(t *testing.T) {
	items := t.TempDir()
	paths := map[string]string{
		"committed-item":   writeItem(t, items, "committed-item", `{"id":"committed-item","title":"Committed item","acceptance":["the committed item's criterion"]}`),
		"stale-scope-item": writeItem(t, items, "stale-scope-item", `{"id":"stale-scope-item","title":"Stale scope item","acceptance":["the stale scope item's criterion"]}`),
	}
	runners := fixtures.BuildRunners(nil)
	runners[core.PhaseTriage] = &decidingTriage{
		FakeRunner: &fixtures.FakeRunner{PhaseName: string(core.PhaseTriage)},
		decision:   `{"top_n":[{"id":"committed-item"}],"deferred":[]}`,
	}
	observed := map[core.Phase]*contractObserver{}
	for _, p := range []core.Phase{core.PhaseTDD, core.PhaseBuild, core.PhaseAudit} {
		observed[p] = &contractObserver{FakeRunner: runners[p].(*fixtures.FakeRunner)}
		runners[p] = observed[p]
	}
	o := core.NewOrchestrator(&fixtures.FakeStorage{}, &fixtures.FakeLedger{}, runners,
		core.WithScopePathResolver(func(_, id string) string { return paths[id] }),
		core.WithWorktreeProvisioner(&tempWorktree{path: gitRepo(t)}))
	// A request-context fleet scope with no EVOLVE_FLEET_SCOPE env pins nothing
	// to disk (cyclerun's legacy snapshot path), so it goes stale once triage
	// commits to a different id.
	if _, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "c1717",
		Context: map[string]string{"fleet_scope": "stale-scope-item"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	dispatched := 0
	for _, p := range []core.Phase{core.PhaseTDD, core.PhaseBuild, core.PhaseAudit} {
		obs := observed[p]
		if len(obs.blocks) == 0 {
			continue
		}
		dispatched++
		block, committed := obs.blocks[0], obs.committed[0]
		if strings.Join(committed, ",") != "committed-item" {
			t.Fatalf("%s: ContractTaskIDs(workspace) = %q at dispatch, want [committed-item] — fixture did not bind", p, committed)
		}
		if got := headerIDs(block); strings.Join(got, ",") != strings.Join(committed, ",") {
			t.Errorf("RED: %s Task Contract names %q, ContractTaskIDs %q:\n%s", p, got, committed, block)
		}
		if !strings.Contains(block, "1. the committed item's criterion") {
			t.Errorf("RED: %s Task Contract lacks the committed item's verbatim acceptance:\n%s", p, block)
		}
		if strings.Contains(block, "stale-scope-item") || strings.Contains(block, "the stale scope item's criterion") {
			t.Errorf("RED: %s Task Contract binds the stale context scope:\n%s", p, block)
		}
	}
	if observed[core.PhaseBuild].blocks == nil || observed[core.PhaseAudit].blocks == nil {
		t.Fatalf("build/audit never dispatched (%d contract phases reached) — assertions would be vacuous", dispatched)
	}
}

// ---------------------------------------------------------------------------
// 002 — AC1: the parity guard exists, covers the shared table, and holds.
// ---------------------------------------------------------------------------

func TestC1717_002_ParityGuardHoldsForEverySharedFixture(t *testing.T) {
	requirePass(t, coreBoundRun(t), "TestTaskItemRefs_ContractTaskIDsParity", parityRows...)
}

// ---------------------------------------------------------------------------
// 003 — AC2: membership from committedset; fleet_scope_paths places records only.
// ---------------------------------------------------------------------------

// coreFunc is one func/method body of package core with the import names its
// file declares.
type coreFunc struct {
	body    *ast.BlockStmt
	imports map[string]bool
}

// parseCore maps each func/method name in package core's non-test sources to
// its bodies.
func parseCore(t *testing.T) map[string][]coreFunc {
	t.Helper()
	dir := filepath.Join(goDir(t), "internal", "core")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	funcs := map[string][]coreFunc{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		imports := map[string]bool{}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			local := filepath.Base(path)
			if imp.Name != nil {
				local = imp.Name.Name
			}
			imports[local] = true
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				funcs[fd.Name.Name] = append(funcs[fd.Name.Name], coreFunc{body: fd.Body, imports: imports})
			}
		}
	}
	return funcs
}

// reach walks the same-package call graph from root and reports every core
// func/method it reaches and every imported package it references.
func reach(funcs map[string][]coreFunc, root string) (reached, pkgs map[string]bool) {
	reached, pkgs = map[string]bool{root: true}, map[string]bool{}
	queue := []string{root}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, fn := range funcs[name] {
			ast.Inspect(fn.body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if id, ok := x.X.(*ast.Ident); ok && fn.imports[id.Name] {
						pkgs[id.Name] = true
						return false
					}
				case *ast.Ident:
					if _, ok := funcs[x.Name]; ok && !reached[x.Name] {
						reached[x.Name] = true
						queue = append(queue, x.Name)
					}
				}
				return true
			})
		}
	}
	return reached, pkgs
}

func TestC1717_003_TaskItemRefsMembershipComesFromCommittedset(t *testing.T) {
	requirePass(t, coreBoundRun(t), "TestTaskItemRefs_ScopePathsResolvePathsNeverMembership", pathRows...)

	funcs := parseCore(t)
	if _, ok := funcs["taskItemRefs"]; !ok {
		t.Fatalf("RED: core has no taskItemRefs to analyse")
	}
	reached, pkgs := reach(funcs, "taskItemRefs")
	if !pkgs["committedset"] {
		t.Errorf("RED: taskItemRefs's call graph never reaches committedset — its ids are not derived from the committed-set projection")
	}
	for _, legacy := range []string{"BoundTaskIDs", "LaneScopeIDs", "deferredTaskIDs"} {
		if reached[legacy] {
			t.Errorf("RED: taskItemRefs still reaches the legacy reader %s — membership must come from committedset only", legacy)
		}
	}
	if _, ok := funcs["deferredTaskIDs"]; ok {
		t.Errorf("RED: deferredTaskIDs is still declared — the duplicate deferral reader must be folded into committedset.Deferred")
	}
}

// ---------------------------------------------------------------------------
// 004 — AC3: identical rendering, vet and -race green.
// ---------------------------------------------------------------------------

func TestC1717_004_TaskContractRendersIdenticallyAndCoreIsVetAndRaceGreen(t *testing.T) {
	vet := exec.Command("go", "vet", corePkg, "./internal/committedset")
	vet.Dir = goDir(t)
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("RED: go vet %s ./internal/committedset: %v\n%s", corePkg, err, trimTail(string(out)))
	}
	run := coreBoundRun(t)
	requirePass(t, run, renderingTests[0], renderRows...)
	for _, name := range renderingTests[1:] {
		requirePass(t, run, name)
	}
	if strings.Contains(run.out, "WARNING: DATA RACE") {
		t.Errorf("RED: the race detector fired in %s:\n%s", corePkg, trimTail(run.out))
	}
	if run.code != 0 {
		t.Errorf("RED: the bound -race run of %s exited %d:\n%s", corePkg, run.code, trimTail(run.out))
	}
}
