package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	got := composeTaskContract([]taskItemRef{{"task-a", a}, {"task-b", b}, {"task-c", c}, {"task-d", ""}, {"task-e", filepath.Join(dir, "missing.json")}})
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
