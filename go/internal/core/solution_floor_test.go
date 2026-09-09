package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func documentSpec() config.DeliverableKindSpec {
	return config.DeliverableKindSpec{
		Root: "solutions", MinOptions: 2,
		RequiredFiles:      []string{"recommendation.md", "assumptions-and-evidence.md"},
		RequiredSections:   map[string][]string{"recommendation.md": {"Recommendation"}},
		ForbidPlaceholders: []string{"TBD"},
		EvidenceFile:       "assumptions-and-evidence.md",
	}
}

func writeWS(t *testing.T, ws, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// documentWorkspace declares a document cycle bound to one task the way the
// live pipeline does: triage's report header (authoritative kind) + its
// decision's top_n.
func documentWorkspace(t *testing.T, kind string) string {
	t.Helper()
	ws := t.TempDir()
	writeWS(t, ws, "scout-report.md", "# Scout\n<!-- challenge-token: x -->\ngoal_type: strategy-options\ndeliverable_kind: "+kind+"\n\n## Selected Tasks\n### Task 1: x\n")
	writeWS(t, ws, "triage-report.md", "<!-- challenge-token: x -->\n# Triage\n\ncycle_size_estimate: medium\ndeliverable_kind: "+kind+"\n\n## top_n\n- netflix-margin: x\n")
	writeWS(t, ws, "triage-decision.json", `{"top_n":[{"id":"netflix-margin"}],"deferred":[]}`)
	return ws
}

// TestSolutionFloorChecks — ADR-0099 slice 2: the build handoff floor judges a
// document cycle's solutions/<slug>/ deterministically (the ONE engine,
// internal/solutioncheck) and stays silent for code cycles. The kind and the
// bound slugs come from the kernel's own reads (report headers + triage
// decision), never from the builder's report.
func TestSolutionFloorChecks(t *testing.T) {
	fn := SolutionFloorChecks(documentSpec())
	ctx := context.Background()

	ws := documentWorkspace(t, "document")
	wt := t.TempDir() // builder delivered nothing
	fails := fn(ctx, ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: wt})
	if len(fails) == 0 || !strings.Contains(strings.Join(fails, "\n"), "solutions/netflix-margin") {
		t.Fatalf("document cycle with no deliverable must be rejected naming the slug; got %v", fails)
	}

	// A complete deliverable passes.
	for rel, body := range map[string]string{
		"options/1-a.md":              "# A\n\n+0.6pp see assumptions-and-evidence.md\n",
		"options/2-b.md":              "# B\n\n+0.5pp see assumptions-and-evidence.md\n",
		"recommendation.md":           "# R\n\n## Recommendation\n\nA, see assumptions-and-evidence.md\n",
		"assumptions-and-evidence.md": "# E\n\n## Assumptions\n\n- A1\n\n## Evidence\n\n- E1\n",
	} {
		p := filepath.Join(wt, "solutions", "netflix-margin", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if fails := fn(ctx, ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: wt}); len(fails) != 0 {
		t.Fatalf("complete deliverable must pass the floor; got %v", fails)
	}

	// A code cycle never consults the solution contract.
	if fails := fn(ctx, ReviewInput{Phase: string(PhaseBuild), Workspace: documentWorkspace(t, "code"), Worktree: t.TempDir()}); fails != nil {
		t.Fatalf("code cycle must be untouched by the solution floor; got %v", fails)
	}
	// Only the build phase is judged.
	if fails := fn(ctx, ReviewInput{Phase: string(PhaseTDD), Workspace: ws, Worktree: t.TempDir()}); fails != nil {
		t.Fatalf("non-build phase must be untouched; got %v", fails)
	}
}

// TestDocumentCycle: the ONE classification ship and the audit gate both
// defer to instead of each re-deriving it from router.Digest.
func TestDocumentCycle(t *testing.T) {
	if DocumentCycle("") {
		t.Error("empty workspace ⇒ false")
	}
	if DocumentCycle(documentWorkspace(t, "code")) {
		t.Error("code cycle ⇒ false")
	}
	if !DocumentCycle(documentWorkspace(t, "document")) {
		t.Error("document cycle ⇒ true")
	}
}

// TestSolutionViolations: the shared engine SolutionFloorChecks and the audit
// gate both call — nil for a code cycle, the stringified failures otherwise.
func TestSolutionViolations(t *testing.T) {
	ws := documentWorkspace(t, "document")
	wt := t.TempDir()
	if got := SolutionViolations(ws, wt, "", documentSpec()); len(got) == 0 || !strings.Contains(got[0], "solutions/netflix-margin") {
		t.Fatalf("document cycle with no deliverable must be rejected naming the slug; got %v", got)
	}
	if got := SolutionViolations(documentWorkspace(t, "code"), t.TempDir(), "", documentSpec()); got != nil {
		t.Fatalf("code cycle must be untouched; got %v", got)
	}
}

// TestChainBuildFloorChecks: the production engine composes the solution floor
// AFTER the existing checks — both lists reach the correction ladder.
func TestChainBuildFloorChecks(t *testing.T) {
	a := func(context.Context, ReviewInput) []string { return []string{"first"} }
	b := func(context.Context, ReviewInput) []string { return nil }
	c := func(context.Context, ReviewInput) []string { return []string{"third"} }
	got := ChainBuildFloorChecks(a, b, c)(context.Background(), ReviewInput{})
	if strings.Join(got, ",") != "first,third" {
		t.Errorf("chained failures = %v, want [first third]", got)
	}
}

// TestSolutionViolations_EmptyBindingIsLoud: a document cycle bound to no task
// (empty or absent triage decision) is a violation, never a silent clean pass.
func TestSolutionViolations_EmptyBindingIsLoud(t *testing.T) {
	ws := documentWorkspace(t, "document")
	writeWS(t, ws, "triage-decision.json", `{"top_n":[]}`)
	got := SolutionViolations(ws, t.TempDir(), "", documentSpec())
	if len(got) != 1 || !strings.Contains(got[0], "binds no task") {
		t.Fatalf("empty top_n must be one loud violation; got %v", got)
	}
	if err := os.Remove(filepath.Join(ws, "triage-decision.json")); err != nil {
		t.Fatal(err)
	}
	if got := SolutionViolations(ws, t.TempDir(), "", documentSpec()); len(got) != 1 {
		t.Fatalf("absent decision must be one loud violation; got %v", got)
	}
	// The floor projection carries it.
	if got := SolutionFloorChecks(documentSpec())(context.Background(), ReviewInput{Phase: string(PhaseBuild), Workspace: ws, Worktree: t.TempDir()}); len(got) != 1 {
		t.Fatalf("floor must surface the empty binding; got %v", got)
	}
}

// TestSeedTaskContract_DocumentCycle: the Task Contract block carries the
// item's deliverable kind and the expected solutions/<id>/ path, and skips the
// Go predicate inventory (there is no ACS suite for a document deliverable —
// the floor is `evolve solution check`).
func TestSeedTaskContract_DocumentCycle(t *testing.T) {
	dir := t.TempDir()
	item := writeItem(t, dir, "netflix-margin", `{"id":"netflix-margin","title":"Netflix margin","deliverable_kind":"document","acceptance":["two options compared"]}`)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.cfg.DeliverableKinds = map[string]config.DeliverableKindSpec{config.DeliverableKindDocument: documentSpec()}
	o.acsPredicates = func(context.Context, string, int) acsPredicates {
		return acsPredicates{names: []string{"TestShouldNotAppear"}}
	}
	ws := documentWorkspace(t, "document")
	cs := CycleState{CycleID: 7, WorkspacePath: ws}
	out := o.seedTaskContract(context.Background(), map[string]string{"fleet_scope_paths": "netflix-margin=" + item}, PhaseBuild, cs, dir)
	block := out[CtxKeyTaskContract]
	for _, want := range []string{"Deliverable kind: document", "solutions/netflix-margin/", "at least 2 candidate strategies", "evolve solution check"} {
		if !strings.Contains(block, want) {
			t.Errorf("block missing %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "TestShouldNotAppear") {
		t.Errorf("document cycle must not render the Go predicate inventory:\n%s", block)
	}
	// A code cycle keeps the inventory.
	code := o.seedTaskContract(context.Background(), map[string]string{"fleet_scope_paths": "netflix-margin=" + item}, PhaseBuild, CycleState{CycleID: 7, WorkspacePath: documentWorkspace(t, "code")}, dir)
	if !strings.Contains(code[CtxKeyTaskContract], "TestShouldNotAppear") {
		t.Errorf("code cycle must render the predicate inventory:\n%s", code[CtxKeyTaskContract])
	}
}

// TestBoundTaskIDs names the exported reader ship uses for the commit prefix.
func TestBoundTaskIDs(t *testing.T) {
	ws := documentWorkspace(t, "document")
	if ids := BoundTaskIDs(ws); len(ids) != 1 || ids[0] != "netflix-margin" {
		t.Errorf("BoundTaskIDs = %v", ids)
	}
	if ids := BoundTaskIDs(t.TempDir()); ids != nil {
		t.Errorf("absent decision ⇒ nil, got %v", ids)
	}
}

// TestSeedDomainDefault: .evolve/domain.json reaches scout/triage as the default
// kind; other phases and projects without the file are byte-identical.
func TestSeedDomainDefault(t *testing.T) {
	root := t.TempDir()
	if got := seedDomainDefault(map[string]string{"goal": "g"}, PhaseScout, root); got[CtxKeyDeliverableKindDefault] != "" {
		t.Errorf("no domain.json ⇒ no default key; got %v", got)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "domain.json"), []byte(`{"domain":"writing"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := seedDomainDefault(map[string]string{"goal": "g"}, PhaseScout, root); got[CtxKeyDeliverableKindDefault] != "document" || got["goal"] != "g" {
		t.Errorf("writing project ⇒ document default for scout; got %v", got)
	}
	if got := seedDomainDefault(map[string]string{}, PhaseBuild, root); got[CtxKeyDeliverableKindDefault] != "" {
		t.Errorf("build must not receive the default; got %v", got)
	}
}

// TestSeedDomainDefault_MalformedIsLoudNotSilent: a domain.json that exists but
// cannot be parsed must not silently make a writing project a code project.
func TestSeedDomainDefault_MalformedIsLoudNotSilent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "domain.json"), []byte(`{"domain": "writing",}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := seedDomainDefault(map[string]string{"goal": "g"}, PhaseScout, root)
	if _, present := got[CtxKeyDeliverableKindDefault]; present {
		t.Errorf("malformed domain.json must seed nothing (and WARN); got %v", got)
	}
}
