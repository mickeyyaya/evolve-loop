package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func writeItem(t *testing.T, dir, id, body string) string {
	t.Helper()
	p := filepath.Join(dir, id+".json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestComposeTaskContract_VerbatimAcceptanceAndLoudGaps(t *testing.T) {
	dir := t.TempDir()
	a := writeItem(t, dir, "task-a", `{"id":"task-a","title":"Title A","acceptance":["build-prompt.txt carries acceptance[] verbatim","go vet ./... green"]}`)
	b := writeItem(t, dir, "task-b", `{"id":"task-b","title":"No criteria"}`)
	c := writeItem(t, dir, "task-c", `{"id":"task-c","title":"Control chars","acceptance":["line one\u0001 with control"]}`)
	got := composeTaskContract([]taskItemRef{{"task-a", a}, {"task-b", b}, {"task-c", c}, {"task-d", ""}, {"task-e", filepath.Join(dir, "missing.json")}}, config.DeliverableKindSpec{Root: "solutions", MinOptions: 2})
	for _, want := range []string{
		"### task-a — Title A", "1. build-prompt.txt carries acceptance[] verbatim", "2. go vet ./... green",
		"### task-b — No criteria", "declares no acceptance[]", ".evolve/evals/task-b.md",
		"### task-c — Control chars", "(note: task-c.json: sanitized control characters", "1. line one  with control",
		"### task-d — inbox record not resolved", "### task-e — inbox record unreadable at",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "### task-a") > strings.Index(got, "### task-b") {
		t.Error("items must render in the bound order")
	}
}

func TestComposeTaskContract_SanitizedCriteriaAreNotClaimedVerbatim(t *testing.T) {
	dir := t.TempDir()
	item := writeItem(t, dir, "large", `{"id":"large","acceptance":["`+strings.Repeat("a", 700)+`"]}`)
	got := composeTaskContract([]taskItemRef{{"large", item}}, config.DeliverableKindSpec{Root: "solutions", MinOptions: 2})
	if strings.Contains(got, "Acceptance (verbatim") || !strings.Contains(got, "sanitized preview") || !strings.Contains(got, item) {
		t.Fatalf("altered criteria must name their source without claiming verbatim authority: %q", got)
	}
}

func TestTaskItemRefs_PathsThenScopeThenTriage(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(root, id string) string { return filepath.Join(root, id+".json") }))
	unresolving := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	pinned := func(ids string) string {
		ws := t.TempDir()
		writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":[`+ids+`]}`)
		return ws
	}
	refs := o.taskItemRefs(map[string]string{"fleet_scope_paths": "a=/x/a.json b=/x/b.json"}, "/root", pinned(`"a","b"`))
	if len(refs) != 2 || refs[0] != (taskItemRef{"a", "/x/a.json"}) || refs[1] != (taskItemRef{"b", "/x/b.json"}) {
		t.Fatalf("fleet_scope_paths must win: %+v", refs)
	}
	if refs := unresolving.taskItemRefs(map[string]string{"fleet_scope_paths": "ok=/x/ok.json broken"}, "/root", pinned(`"ok","broken"`)); len(refs) != 2 || refs[1] != (taskItemRef{id: "broken"}) {
		t.Fatalf("a malformed pair must render as an unresolved task, never be dropped: %+v", refs)
	}
	if refs := unresolving.taskItemRefs(map[string]string{"fleet_scope_paths": "ok=/x/ok.json", "fleet_scope": "ok, refused=id"}, "/root", pinned(`"ok","refused=id"`)); len(refs) != 2 || refs[1] != (taskItemRef{id: "refused=id"}) {
		t.Fatalf("a scope id the producer could not encode must still render (unresolved), never vanish: %+v", refs)
	}
	refs = o.taskItemRefs(map[string]string{"fleet_scope": "c, d"}, "/root", pinned(`"c","d"`))
	if len(refs) != 2 || refs[0].path != "/root/c.json" || refs[1].id != "d" {
		t.Fatalf("scope ids resolve through the scope-path resolver: %+v", refs)
	}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"e"},{"id":""},{"id":"f"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	refs = o.taskItemRefs(map[string]string{}, "/root", ws)
	if len(refs) != 2 || refs[0].id != "e" || refs[1].path != "/root/f.json" {
		t.Fatalf("no scope ⇒ the triage decision's top_n: %+v", refs)
	}
	if refs := o.taskItemRefs(map[string]string{}, "/root", t.TempDir()); len(refs) != 0 {
		t.Fatalf("nothing bound ⇒ nothing seeded: %+v", refs)
	}
}

func TestTaskItemRefs_DeferredScopeIsNotMandatory(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"sub-task-a"}],"deferred":[{"id":"postponed-item","reason":"needs another fix"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":["renamed-work","postponed-item"]}`)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(root, id string) string { return filepath.Join(root, id+".json") }))
	for _, paths := range []string{"", "renamed-work=/root/renamed-work.json postponed-item=/root/postponed-item.json"} {
		refs := o.taskItemRefs(map[string]string{"fleet_scope": "renamed-work,postponed-item", "fleet_scope_paths": paths}, "/root", ws)
		if len(refs) != 1 || refs[0] != (taskItemRef{"renamed-work", "/root/renamed-work.json"}) {
			t.Errorf("deferred work must stay out of the contract while decomposed scope remains bound; paths=%q refs=%+v", paths, refs)
		}
	}
}

func TestListACSPredicates_InventoriesTheCyclePackage(t *testing.T) {
	wt := t.TempDir()
	mod := filepath.Join(wt, "go")
	if err := os.MkdirAll(filepath.Join(mod, "acs", "cycle7"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/tmp\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "acs", "cycle7", "predicates_test.go"), []byte("//go:build acs\n\npackage cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_First(t *testing.T) {}\nfunc TestC7_002_Second(t *testing.T) {}\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := listACSPredicates(context.Background(), wt, 7)
	if got.note != "" || strings.Join(got.names, ",") != "TestC7_001_First,TestC7_002_Second" {
		t.Fatalf("predicates = %+v", got)
	}
	if absent := listACSPredicates(context.Background(), wt, 8); len(absent.names) != 0 || !strings.Contains(absent.note, "no ./acs/cycle8 package") {
		t.Fatalf("absent package must be a loud note: %+v", absent)
	}
	if none := listACSPredicates(context.Background(), "", 7); !strings.Contains(none.note, "no worktree") {
		t.Fatalf("no worktree: %+v", none)
	}
	rendered := renderPredicates(got)
	if !strings.Contains(rendered, "- TestC7_001_First") || !strings.Contains(rendered, "every one must be GREEN") {
		t.Fatalf("rendered = %q", rendered)
	}
}

// TestDispatch_TaskContractReachesTDDBuildAndAudit is the core half of the
// wiring proof; the phase half — each ComposePrompt rendering the key under
// "## Task Contract" — is pinned in
// phases/{tdd,build,audit}/task_contract_prompt_test.go.
func TestDispatch_TaskContractReachesTDDBuildAndAudit(t *testing.T) {
	dir := t.TempDir()
	item := writeItem(t, dir, "task-a", `{"id":"task-a","title":"Title A","acceptance":["the build prompt carries this sentence verbatim"]}`)
	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithScopePathResolver(func(_, id string) string {
		if id == "task-a" {
			return item
		}
		return ""
	}))
	o.acsPredicates = func(_ context.Context, _ string, cycle int) acsPredicates {
		return acsPredicates{names: []string{"TestC1_001_Fake"}}
	}
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true, Env: map[string]string{ipcenv.FleetScopeKey: "task-a"}, Context: map[string]string{"fleet_scope": "task-a"}}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	for _, p := range []Phase{PhaseTDD, PhaseBuild, PhaseAudit} {
		fr := runners[p].(*fakeRunner)
		if len(fr.requests) == 0 {
			t.Fatalf("%s never dispatched", p)
		}
		got := fr.requests[0].Context[CtxKeyTaskContract]
		if !strings.HasPrefix(got, taskContractPreamble) || !strings.Contains(got, "### task-a — Title A") || !strings.Contains(got, "1. the build prompt carries this sentence verbatim") {
			t.Errorf("%s request must carry the preamble and the verbatim acceptance, got %q", p, got)
		}
	}
	for _, p := range []Phase{PhaseBuild, PhaseAudit} {
		if got := runners[p].(*fakeRunner).requests[0].Context[CtxKeyTaskContract]; !strings.Contains(got, "### ACS predicates") || !strings.Contains(got, "- TestC1_001_Fake") {
			t.Errorf("%s runs after tdd and must carry the predicate inventory, got %q", p, got)
		}
	}
	if got := runners[PhaseTDD].(*fakeRunner).requests[0].Context[CtxKeyTaskContract]; strings.Contains(got, "### ACS predicates") {
		t.Error("tdd runs before the predicates exist; its block must not claim an inventory")
	}
	for _, p := range []Phase{PhaseScout, PhaseTriage} {
		if got := runners[p].(*fakeRunner).requests[0].Context[CtxKeyTaskContract]; got != "" {
			t.Errorf("%s must not receive the Task Contract, got %q", p, got)
		}
	}
}

func TestResume_TaskContractSeededOnTheResumeSurface(t *testing.T) {
	item := writeItem(t, t.TempDir(), "task-r", `{"id":"task-r","title":"Resumed","acceptance":["resume carries the contract"]}`)
	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictPASS})
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 9, Phase: string(PhaseBuild), WorkspacePath: RunWorkspacePath(root, 9)}}
	if err := os.MkdirAll(RunWorkspacePath(root, 9), 0o755); err != nil {
		t.Fatal(err)
	}
	writeWSFile(t, RunWorkspacePath(root, 9), LaneScopeFile, `{"todo_ids":["task-r"]}`)
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithScopePathResolver(func(_, id string) string { return item }))
	o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{note: "fake inventory"} }
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, Context: map[string]string{"fleet_scope": "task-r"}}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	build := runners[PhaseBuild].(*fakeRunner)
	if len(build.requests) == 0 || !strings.Contains(build.requests[0].Context[CtxKeyTaskContract], "1. resume carries the contract") {
		t.Fatalf("resumed build must carry the block: %+v", build.requests)
	}
}

func TestListACSPredicates_FailureBranchesAreLoud(t *testing.T) {
	wt := t.TempDir()
	mod := filepath.Join(wt, "go")
	for _, d := range []string{"acs/cycle11", "acs/cycle12"} {
		if err := os.MkdirAll(filepath.Join(mod, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/tmp\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "acs", "cycle11", "broken_test.go"), []byte("//go:build acs\n\npackage cycle11\n\nimport \"testing\"\n\nfunc TestC11_001(t *testing.T) { undefined() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "acs", "cycle12", "empty_test.go"), []byte("//go:build acs\n\npackage cycle12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := listACSPredicates(context.Background(), wt, 11)
	if len(broken.names) != 0 || !strings.Contains(broken.note, "`go test -list` failed for ./acs/cycle11") {
		t.Fatalf("compile failure must be a loud note with no names: %+v", broken)
	}
	empty := listACSPredicates(context.Background(), wt, 12)
	if len(empty.names) != 0 || !strings.Contains(empty.note, "declares no Test functions") {
		t.Fatalf("empty package must be a loud note: %+v", empty)
	}
}

func TestTaskContract_MultiSlugProjectionParity(t *testing.T) {
	for _, pin := range []bool{false, true} {
		t.Run(fmt.Sprint("pin=", pin), func(t *testing.T) {
			ws := t.TempDir()
			if pin {
				if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{"todo_ids":["a","b"]}`), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				writeWSFile(t, ws, "triage-decision.json", `{"top_n":[{"id":"a"},{"id":"b"}]}`)
			}
			a := writeItem(t, ws, "a", `{"id":"a","acceptance":["deliver a"]}`)
			b := writeItem(t, ws, "b", `{"id":"b","acceptance":["deliver b"]}`)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(_, id string) string {
				if id == "a" {
					return a
				}
				return b
			}))
			o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{} }
			base := map[string]string{"fleet_scope": "a,b", "fleet_scope_paths": "a=" + a}
			if pin {
				base["fleet_scope"] = "a"
			}
			for _, phase := range []Phase{PhaseTDD, PhaseBuild} {
				got := o.seedTaskContract(context.Background(), base, phase, CycleState{WorkspacePath: ws}, ws)[CtxKeyTaskContract]
				if !strings.Contains(got, "### a —") || !strings.Contains(got, "### b —") || strings.Index(got, "### a —") > strings.Index(got, "### b —") {
					t.Fatalf("%s lost ordered members: %s", phase, got)
				}
			}
		})
	}
}

func TestLaneScopeIDs(t *testing.T) {
	ws := t.TempDir()
	if got := LaneScopeIDs(ws); got != nil {
		t.Fatalf("absent pin = %v", got)
	}
	if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{"todo_ids":["b","a"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LaneScopeIDs(ws); strings.Join(got, ",") != "b,a" {
		t.Fatalf("ordered pin = %v", got)
	}
	if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{broken`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LaneScopeIDs(ws); got != nil {
		t.Fatalf("malformed pin = %v", got)
	}
}

func TestDispatch_TaskContractMultiSlugLiveAndResume(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(fmt.Sprint("resume=", resume), func(t *testing.T) {
			root := t.TempDir()
			ws := RunWorkspacePath(root, 9)
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{"todo_ids":["a","b"]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			a := writeItem(t, ws, "a", `{"id":"a","acceptance":["deliver a"]}`)
			b := writeItem(t, ws, "b", `{"id":"b","acceptance":["deliver b"]}`)
			runners := buildRunners(map[Phase]string{PhaseAudit: VerdictPASS})
			st := &fakeStorage{state: State{LastCycleNumber: 8}, cycleState: CycleState{CycleID: 9, Phase: string(PhaseTDD), WorkspacePath: ws}}
			if resume {
				st.state.LastCycleNumber = 9
			}
			o := NewOrchestrator(st, &fakeLedger{}, runners, WithScopePathResolver(func(_, id string) string {
				if id == "a" {
					return a
				}
				return b
			}))
			o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{} }
			req := CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true, Context: map[string]string{"fleet_scope": "a", "fleet_scope_paths": "a=" + a}}
			var err error
			if resume {
				_, err = o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseTDD), CycleID: 9})
			} else {
				_, err = o.RunCycle(context.Background(), req)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, phase := range []Phase{PhaseTDD, PhaseBuild} {
				requests := runners[phase].(*fakeRunner).requests
				if len(requests) == 0 {
					t.Fatalf("%s never reached", phase)
				}
				contract := requests[0].Context[CtxKeyTaskContract]
				if !strings.Contains(contract, "1. deliver a") || !strings.Contains(contract, "1. deliver b") {
					t.Fatalf("%s lost a member: %s", phase, contract)
				}
			}
		})
	}
}

func TestLaneScopeIDs_ExplicitEmptyPinIsNoPin(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{"todo_ids":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LaneScopeIDs(ws); got != nil {
		t.Fatalf("explicit empty pin must fall back like an absent one; got %v", got)
	}
}

func TestContractTaskIDs(t *testing.T) {
	decision := `{"top_n":[{"id":"alpha"},{"id":"beta"},{"id":"gamma"}],"deferred":[{"id":"gamma"}]}`
	t.Run("no pin ⇒ decision top_n minus deferred", func(t *testing.T) {
		ws := t.TempDir()
		writeWSFile(t, ws, "triage-decision.json", decision)
		if got := ContractTaskIDs(ws); strings.Join(got, ",") != "alpha,beta" {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("pin wins over the decision, deferrals still subtract", func(t *testing.T) {
		ws := t.TempDir()
		writeWSFile(t, ws, "triage-decision.json", decision)
		writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":["renamed-work","gamma"]}`)
		if got := ContractTaskIDs(ws); strings.Join(got, ",") != "renamed-work" {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("nothing bound ⇒ nil", func(t *testing.T) {
		if got := ContractTaskIDs(t.TempDir()); got != nil {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("parity with the Task Contract's own refs", func(t *testing.T) {
		for _, pin := range []string{"", `{"todo_ids":["renamed-work","gamma"]}`} {
			ws := t.TempDir()
			writeWSFile(t, ws, "triage-decision.json", decision)
			if pin != "" {
				writeWSFile(t, ws, LaneScopeFile, pin)
			}
			o := &Orchestrator{}
			var refIDs []string
			for _, r := range o.taskItemRefs(map[string]string{}, t.TempDir(), ws) {
				refIDs = append(refIDs, r.id)
			}
			if want := ContractTaskIDs(ws); strings.Join(refIDs, ",") != strings.Join(want, ",") {
				t.Fatalf("pin=%q: taskItemRefs ids %v != ContractTaskIDs %v", pin, refIDs, want)
			}
		}
	})
}

func writeWSFile(t *testing.T, ws, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestContractTaskIDs_WhitespacePaddedIDsAreNotPhantomMembers(t *testing.T) {
	ws := t.TempDir()
	writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":["alpha"," beta ","","  "]}`)
	if got := ContractTaskIDs(ws); strings.Join(got, ",") != "alpha,beta" {
		t.Fatalf("ContractTaskIDs = %v, want the trimmed, non-blank ids", got)
	}
	ws2 := t.TempDir()
	writeWSFile(t, ws2, LaneScopeFile, `{"todo_ids":["alpha","beta"]}`)
	writeWSFile(t, ws2, "triage-decision.json", `{"deferred":[{"id":" beta "}]}`)
	if got := ContractTaskIDs(ws2); strings.Join(got, ",") != "alpha" {
		t.Fatalf("ContractTaskIDs = %v, want beta deferred despite its padding", got)
	}
}

// contractParityFixture is one on-disk binding plus the dispatch context the
// Task Contract is composed from. want is the committed set committedset
// projects for the binding, in order; the context never changes it.
type contractParityFixture struct {
	name     string
	pin      string // lane-scope.json body; "" = absent
	decision string // triage-decision.json body; "" = absent
	ctx      map[string]string
	want     []string
}

// contractParityFixtures is the shared table: the Task Contract's refs, the
// block seedTaskContract renders and ContractTaskIDs must name the same ids for
// every row.
var contractParityFixtures = []contractParityFixture{
	{name: "pin", pin: `{"todo_ids":["alpha","beta"]}`, ctx: map[string]string{"fleet_scope": "alpha,beta"}, want: []string{"alpha", "beta"}},
	{name: "pin_wins_over_decision_and_deferral_subtracts", pin: `{"todo_ids":["renamed-work","gamma"]}`, decision: `{"top_n":[{"id":"alpha"}],"deferred":[{"id":"gamma"}]}`, ctx: map[string]string{"fleet_scope": "renamed-work,gamma"}, want: []string{"renamed-work"}},
	{name: "pin_ignores_scope_path_ids_it_does_not_name", pin: `{"todo_ids":["alpha"]}`, ctx: map[string]string{"fleet_scope": "alpha,stray", "fleet_scope_paths": "alpha=/x/alpha.json stray=/x/stray.json"}, want: []string{"alpha"}},
	{name: "decision_only", decision: `{"top_n":[{"id":"alpha"},{"id":"beta"}]}`, want: []string{"alpha", "beta"}},
	{name: "decision_with_deferral", decision: `{"top_n":[{"id":"alpha"},{"id":"beta"},{"id":"gamma"}],"deferred":[{"id":"gamma"}]}`, want: []string{"alpha", "beta"}},
	{name: "empty", want: nil},
	{name: "explicit_empty_pin_falls_through_to_decision", pin: `{"todo_ids":[]}`, decision: `{"top_n":[{"id":"alpha"}]}`, want: []string{"alpha"}},
	{name: "malformed_pin_falls_through_to_decision", pin: `{broken`, decision: `{"top_n":[{"id":"alpha"}]}`, want: []string{"alpha"}},
	{name: "stale_scope_without_pin_yields_to_decision", decision: `{"top_n":[{"id":"new-id"}]}`, ctx: map[string]string{"fleet_scope": "old-id"}, want: []string{"new-id"}},
	{name: "decomposed_decision_without_pin_ignores_scope", decision: `{"top_n":[{"id":"sub-task-a"}],"deferred":[{"id":"postponed-item"}]}`, ctx: map[string]string{"fleet_scope": "renamed-work,postponed-item"}, want: []string{"sub-task-a"}},
	{name: "scope_paths_without_binding_are_not_members", ctx: map[string]string{"fleet_scope": "a,b", "fleet_scope_paths": "a=/x/a.json b=/x/b.json"}, want: nil},
	{name: "scope_without_binding_is_not_members", ctx: map[string]string{"fleet_scope": "a"}, want: nil},
	{name: "scope_paths_add_no_member_to_a_decision", decision: `{"top_n":[{"id":"alpha"}]}`, ctx: map[string]string{"fleet_scope_paths": "alpha=/x/alpha.json stray=/x/stray.json"}, want: []string{"alpha"}},
	{name: "explicit_empty_commitment_is_not_resurrected_from_scope", decision: `{"top_n":[]}`, ctx: map[string]string{"fleet_scope": "old-id"}, want: nil},
	{name: "malformed_decision_with_scope_binds_nothing", decision: `{broken`, ctx: map[string]string{"fleet_scope": "alpha"}, want: nil},
	{name: "padded_pin_ids_are_trimmed_and_blanks_dropped", pin: `{"todo_ids":["alpha"," beta ","","  "]}`, want: []string{"alpha", "beta"}},
	{name: "blank_only_pin_falls_through_to_decision", pin: `{"todo_ids":["  "]}`, decision: `{"top_n":[{"id":"alpha"}]}`, want: []string{"alpha"}},
	{name: "padded_deferral_still_subtracts", pin: `{"todo_ids":["alpha","beta"]}`, decision: `{"deferred":[{"id":" beta "}]}`, want: []string{"alpha"}},
	{name: "padded_top_n_ids_are_trimmed", decision: `{"top_n":[{"id":" alpha "},{"id":"beta"}]}`, want: []string{"alpha", "beta"}},
}

func (f contractParityFixture) workspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if f.pin != "" {
		writeWSFile(t, ws, LaneScopeFile, f.pin)
	}
	if f.decision != "" {
		writeWSFile(t, ws, "triage-decision.json", f.decision)
	}
	return ws
}

func refIDs(refs []taskItemRef) []string {
	var ids []string
	for _, r := range refs {
		ids = append(ids, r.id)
	}
	return ids
}

// blockIDs returns the ids of the "### <id> — " headers of a rendered Task
// Contract block, in order.
func blockIDs(block string) []string {
	var ids []string
	for _, line := range strings.Split(block, "\n") {
		rest, ok := strings.CutPrefix(line, "### ")
		if !ok {
			continue
		}
		if id, _, ok := strings.Cut(rest, " — "); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func TestTaskItemRefs_ContractTaskIDsParity(t *testing.T) {
	for _, f := range contractParityFixtures {
		t.Run(f.name, func(t *testing.T) {
			ws := f.workspace(t)
			root := t.TempDir()
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(root, id string) string { return filepath.Join(root, id+".json") }))
			want := ContractTaskIDs(ws)
			if strings.Join(want, ",") != strings.Join(f.want, ",") {
				t.Fatalf("fixture drifted from committedset: ContractTaskIDs = %q, fixture want %q", want, f.want)
			}
			ctx := map[string]string{}
			for k, v := range f.ctx {
				ctx[k] = v
			}
			if got := refIDs(o.taskItemRefs(ctx, root, ws)); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
				t.Errorf("taskItemRefs ids %q != ContractTaskIDs %q", got, want)
			}
			block := o.seedTaskContract(context.Background(), ctx, PhaseTDD, CycleState{WorkspacePath: ws}, root)[CtxKeyTaskContract]
			if got := blockIDs(block); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
				t.Errorf("rendered Task Contract names %q, ContractTaskIDs %q:\n%s", got, want, block)
			}
		})
	}
}

func TestTaskItemRefs_ScopePathsResolvePathsNeverMembership(t *testing.T) {
	resolver := WithScopePathResolver(func(_, id string) string { return "/resolver/" + id + ".json" })
	cases := []struct {
		name, pin, decision string
		ctx                 map[string]string
		noResolver          bool
		want                []taskItemRef
	}{
		{
			name: "pin_with_partial_and_stale_disclosure",
			pin:  `{"todo_ids":["a","b","c"]}`,
			ctx:  map[string]string{"fleet_scope": "a,b,c", "fleet_scope_paths": "a=/pairs/a.json x=/pairs/x.json"},
			want: []taskItemRef{{"a", "/pairs/a.json"}, {"b", "/resolver/b.json"}, {"c", "/resolver/c.json"}},
		},
		{
			name:     "decision_with_stale_disclosure",
			decision: `{"top_n":[{"id":"a"},{"id":"b"}]}`,
			ctx:      map[string]string{"fleet_scope_paths": "a=/pairs/a.json x=/pairs/x.json"},
			want:     []taskItemRef{{"a", "/pairs/a.json"}, {"b", "/resolver/b.json"}},
		},
		{
			name:     "pair_path_wins_over_resolver",
			decision: `{"top_n":[{"id":"a"}]}`,
			ctx:      map[string]string{"fleet_scope_paths": "a=/pairs/a.json"},
			want:     []taskItemRef{{"a", "/pairs/a.json"}},
		},
		{
			name:       "unplaceable_member_renders_unresolved_never_dropped",
			pin:        `{"todo_ids":["a","b"]}`,
			ctx:        map[string]string{"fleet_scope_paths": "a=/pairs/a.json"},
			noResolver: true,
			want:       []taskItemRef{{"a", "/pairs/a.json"}, {id: "b"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := contractParityFixture{pin: c.pin, decision: c.decision}.workspace(t)
			var opts []Option
			if !c.noResolver {
				opts = append(opts, resolver)
			}
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
			got := o.taskItemRefs(c.ctx, "/root", ws)
			if len(got) != len(c.want) {
				t.Fatalf("refs = %+v, want %+v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("refs[%d] = %+v, want %+v (all: %+v)", i, got[i], c.want[i], got)
				}
			}
		})
	}
}

// TestSeedTaskContract_RendersIdenticallyForPinDecisionAndDeferral pins the
// complete block, byte for byte, for the three production binding shapes.
// A disclosed pair and the resolver point at different records for alpha so
// the render also proves which one placed it.
func TestSeedTaskContract_RendersIdenticallyForPinDecisionAndDeferral(t *testing.T) {
	dir := t.TempDir()
	alphaScope := writeItem(t, dir, "alpha-scope", `{"id":"alpha","title":"Alpha (scope record)","acceptance":["alpha criterion one","alpha criterion two"]}`)
	writeItem(t, dir, "alpha", `{"id":"alpha","title":"Alpha (resolver record)","acceptance":["alpha resolver criterion"]}`)
	writeItem(t, dir, "beta", `{"id":"beta","title":"Beta title","acceptance":["beta criterion"]}`)
	writeItem(t, dir, "gamma", `{"id":"gamma","title":"Gamma title","acceptance":["gamma criterion"]}`)
	const verbatim = "Acceptance (verbatim from the inbox item — the auditor grades against exactly these):\n"
	alphaFromScope := "### alpha — Alpha (scope record)\n" + verbatim + "1. alpha criterion one\n2. alpha criterion two\n\n"
	alphaFromResolver := "### alpha — Alpha (resolver record)\n" + verbatim + "1. alpha resolver criterion\n\n"
	beta := "### beta — Beta title\n" + verbatim + "1. beta criterion\n\n"
	gamma := "### gamma — Gamma title\n" + verbatim + "1. gamma criterion\n\n"
	cases := []struct {
		name, pin, decision string
		ctx                 map[string]string
		body                string
	}{
		{
			name: "lane_pin",
			pin:  `{"todo_ids":["alpha","beta"]}`,
			ctx:  map[string]string{"fleet_scope": "alpha,beta", "fleet_scope_paths": "alpha=" + alphaScope},
			body: alphaFromScope + beta,
		},
		{
			name:     "decision_only",
			decision: `{"top_n":[{"id":"alpha"},{"id":"gamma"}]}`,
			ctx:      map[string]string{},
			body:     alphaFromResolver + gamma,
		},
		{
			name:     "deferral_under_a_pin",
			pin:      `{"todo_ids":["alpha","beta","gamma"]}`,
			decision: `{"top_n":[{"id":"sub-x"}],"deferred":[{"id":"beta"}]}`,
			ctx:      map[string]string{"fleet_scope": "alpha,beta,gamma", "fleet_scope_paths": "alpha=" + alphaScope + " beta=" + filepath.Join(dir, "beta.json")},
			body:     alphaFromScope + gamma,
		},
		{
			name:     "deferral_in_a_decision",
			decision: `{"top_n":[{"id":"alpha"},{"id":"beta"}],"deferred":[{"id":"beta"}]}`,
			ctx:      map[string]string{},
			body:     alphaFromResolver,
		},
	}
	inventory := acsPredicates{names: []string{"TestC1_001_Fake"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := contractParityFixture{pin: c.pin, decision: c.decision}.workspace(t)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(_, id string) string { return filepath.Join(dir, id+".json") }))
			o.acsPredicates = func(context.Context, string, int) acsPredicates { return inventory }
			for phase, want := range map[Phase]string{
				PhaseTDD:   taskContractPreamble + c.body,
				PhaseAudit: taskContractPreamble + c.body + renderPredicates(inventory),
			} {
				if got := o.seedTaskContract(context.Background(), c.ctx, phase, CycleState{WorkspacePath: ws}, dir)[CtxKeyTaskContract]; got != want {
					t.Errorf("%s block changed:\n--- got\n%s\n--- want\n%s", phase, got, want)
				}
			}
		})
	}
}
