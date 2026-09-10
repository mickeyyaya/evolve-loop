package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
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
	refs := o.taskItemRefs(map[string]string{"fleet_scope_paths": "a=/x/a.json b=/x/b.json"}, "/root", "")
	if len(refs) != 2 || refs[0] != (taskItemRef{"a", "/x/a.json"}) || refs[1] != (taskItemRef{"b", "/x/b.json"}) {
		t.Fatalf("fleet_scope_paths must win: %+v", refs)
	}
	if refs := o.taskItemRefs(map[string]string{"fleet_scope_paths": "ok=/x/ok.json broken"}, "/root", ""); len(refs) != 2 || refs[1] != (taskItemRef{id: "broken"}) {
		t.Fatalf("a malformed pair must render as an unresolved task, never be dropped: %+v", refs)
	}
	if refs := o.taskItemRefs(map[string]string{"fleet_scope_paths": "ok=/x/ok.json", "fleet_scope": "ok, refused=id"}, "/root", ""); len(refs) != 2 || refs[1] != (taskItemRef{id: "refused=id"}) {
		t.Fatalf("a scope id the producer could not encode must still render (unresolved), never vanish: %+v", refs)
	}
	refs = o.taskItemRefs(map[string]string{"fleet_scope": "c, d"}, "/root", "")
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
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopePathResolver(func(root, id string) string { return filepath.Join(root, id+".json") }))
	for _, paths := range []string{"", "renamed-work=/root/renamed-work.json postponed-item=/root/postponed-item.json"} {
		refs := o.taskItemRefs(map[string]string{"fleet_scope": "renamed-work,postponed-item", "fleet_scope_paths": paths}, "/root", ws)
		if len(refs) != 1 || refs[0] != (taskItemRef{"renamed-work", "/root/renamed-work.json"}) {
			t.Errorf("deferred work must stay out of the contract while decomposed scope remains bound; paths=%q refs=%+v", paths, refs)
		}
	}
}

// TestListACSPredicates_InventoriesTheCyclePackage runs the real `go test
// -list` against a throwaway module: the names come from the test files, and
// an absent package or an empty one is a loud note.
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
// wiring proof: through RunCycle with the scope-path resolver the composition
// root wires, the tdd, build and audit requests carry the block (build and
// audit with the predicate inventory); scout and triage do not. The phase half
// — each ComposePrompt rendering the key under "## Task Contract" — is pinned
// in phases/{tdd,build,audit}/task_contract_prompt_test.go.
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
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true, Context: map[string]string{"fleet_scope": "task-a"}}); err != nil {
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

// TestResume_TaskContractSeededOnTheResumeSurface — the crash-resume dispatch
// builder composes the same block (resume.go is the second surface).
func TestResume_TaskContractSeededOnTheResumeSurface(t *testing.T) {
	item := writeItem(t, t.TempDir(), "task-r", `{"id":"task-r","title":"Resumed","acceptance":["resume carries the contract"]}`)
	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictPASS})
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 9, Phase: string(PhaseBuild), WorkspacePath: RunWorkspacePath(root, 9)}}
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

// TestListACSPredicates_FailureBranchesAreLoud — a package that does not
// compile and one with no Test functions are both notes, never silence.
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

// Both prompt consumers must retain every pinned member, even when a resumed
// context still carries a partial or stale path disclosure.
func TestTaskContract_MultiSlugProjectionParity(t *testing.T) {
	for _, pin := range []bool{false, true} {
		t.Run(fmt.Sprint("pin=", pin), func(t *testing.T) {
			ws := t.TempDir()
			if pin {
				if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{"todo_ids":["a","b"]}`), 0o644); err != nil {
					t.Fatal(err)
				}
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

// TestLaneScopeIDs_ExplicitEmptyPinIsNoPin: a present-but-empty pin
// (`{"todo_ids":[]}`) is not an authoritative empty membership — treating it
// as one would wipe the triage-derived refs and make the Task Contract
// silently disappear (the "never a silent omission" doctrine). No producer
// writes it today; this pins the fallback so a future producer cannot.
func TestLaneScopeIDs_ExplicitEmptyPinIsNoPin(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, LaneScopeFile), []byte(`{"todo_ids":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LaneScopeIDs(ws); got != nil {
		t.Fatalf("explicit empty pin must fall back like an absent one; got %v", got)
	}
}

// TestContractTaskIDs — cycle-1620 salvage (architecture CRITICAL 1): the
// TDD->Build scope gate must bind to the SAME id set the Task Contract handed
// TDD — the lane pin when present, else the triage decision's top_n, minus the
// decision's deferrals — never to triage-report.md's markdown ## top_n, which
// is prose in triage's working-id namespace (decomposition sub-ids are the
// documented norm). ContractTaskIDs is that one projection; taskItemRefs
// derives its ids from the same readers, proven by parity below.
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

// TestContractTaskIDs_WhitespacePaddedIDsAreNotPhantomMembers pins the one
// behavior difference between the shared committedset projection and the
// inline code it replaced: a padded or blank id in the lane pin is trimmed and
// dropped rather than carried verbatim. Verbatim, " beta " matched nothing
// downstream — a phantom member that could never be satisfied.
func TestContractTaskIDs_WhitespacePaddedIDsAreNotPhantomMembers(t *testing.T) {
	ws := t.TempDir()
	writeWSFile(t, ws, LaneScopeFile, `{"todo_ids":["alpha"," beta ","","  "]}`)
	if got := ContractTaskIDs(ws); strings.Join(got, ",") != "alpha,beta" {
		t.Fatalf("ContractTaskIDs = %v, want the trimmed, non-blank ids", got)
	}
	// A padded DEFERRAL still subtracts the member it names.
	ws2 := t.TempDir()
	writeWSFile(t, ws2, LaneScopeFile, `{"todo_ids":["alpha","beta"]}`)
	writeWSFile(t, ws2, "triage-decision.json", `{"deferred":[{"id":" beta "}]}`)
	if got := ContractTaskIDs(ws2); strings.Join(got, ",") != "alpha" {
		t.Fatalf("ContractTaskIDs = %v, want beta deferred despite its padding", got)
	}
}
