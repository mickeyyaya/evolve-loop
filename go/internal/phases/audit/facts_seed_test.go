package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/auditfacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

func writeLedger(t *testing.T, path string, led handoff.Ledger) {
	t.Helper()
	raw, err := json.Marshal(led)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(raw))
}

var writeFile = inboxtest.WriteFile

type factsFixture struct {
	req core.PhaseRequest
}

func newFactsFixture(t *testing.T) factsFixture {
	t.Helper()
	root := t.TempDir()
	repo := gittest.Fixture(t)
	worktree := repo.Dir
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-7")
	writeFile(t, filepath.Join(worktree, "go/lib/lib.go"), "package lib\n\nfunc Stable() int {\n\treturn 1\n}\n\nfunc Edited() int {\n\treturn 2\n}\n")
	writeFile(t, filepath.Join(worktree, "go/lib/lib_test.go"), "package lib\n\nimport \"testing\"\n\nfunc TestEdited(t *testing.T) {\n\tt.Fatal()\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	writeFile(t, filepath.Join(worktree, "go/lib/lib.go"), "package lib\n\nfunc Stable() int {\n\treturn 1\n}\n\nfunc Edited() int {\n\treturn 3\n}\n")
	writeFile(t, filepath.Join(worktree, "go/lib/lib_test.go"), "package lib\n\nimport \"testing\"\n\nfunc TestEdited(t *testing.T) {}\n")
	writeFile(t, filepath.Join(worktree, "go/lib/fresh.go"), "package lib\n\nfunc Fresh() {}\n")
	writeFile(t, filepath.Join(worktree, "agents/evolve-auditor.md"), "protected\n")
	writeFile(t, filepath.Join(worktree, "go/internal/guards/g.go"), "package guards\n\nfunc Guarded() {}\n")
	return factsFixture{req: core.PhaseRequest{
		Cycle: 7, ProjectRoot: root, Worktree: worktree, Workspace: workspace, WorktreeBaseSHA: base,
		Context: map[string]string{"keep": "me"},
	}}
}

func TestSeedContext_DerivesTheBlockFromTheRealRecordsAndTheWorktreeDiff(t *testing.T) {
	t.Parallel()
	f := newFactsFixture(t)
	writeFile(t, filepath.Join(f.req.Workspace, "acs-verdict.round1.json"), `{"red_count":2,"ship_eligible":false,"results":[{"ac_id":"AC-9","result":"red"}]}`)
	writeLedger(t, filepath.Join(f.req.Workspace, "handoff-file-name.json"), handoff.Ledger{Pair: "tdd-build", Check: &handoff.Check{Outcome: "pass"}})
	writeLedger(t, filepath.Join(f.req.Workspace, "handoff-triage-tdd.json"), handoff.Ledger{Check: &handoff.Check{Outcome: "stale"}})
	writeLedger(t, filepath.Join(f.req.Workspace, "handoff-nocheck.json"), handoff.Ledger{Pair: "nocheck"})
	writeFile(t, filepath.Join(f.req.Workspace, "handoff-bad.json"), `{"check":`)
	writeFile(t, filepath.Join(f.req.Workspace, "handoff-empty.json"), `{"check":{}}`)
	if err := os.MkdirAll(filepath.Join(f.req.Workspace, "handoff-dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.req.Workspace, core.RunStateFile), `{"cycle_id":7,"audit_fail_reasons":["gofmt: 1 file(s) are not gofmt -s clean"]}`)

	got := (hooks{}).SeedContext(f.req).Context[core.CtxKeyAuditFacts]

	for _, want := range []string{
		"- round: 2\n",
		"- gates_the_host_decides_after_your_report: " + strings.Join(core.AuditGateNames(), ", ") + "\n",
		"- acs_verdict_round1: red_count=2 ship_eligible=false red=[AC-9]\n",
		"- handoff_tdd-build: outcome=pass\n",
		"- handoff_triage-tdd: outcome=stale\n",
		"- audit_gate_fail_untrusted_json: \"gofmt: 1 file(s) are not gofmt -s clean\"\n",
		"- changed_unprotected_functions: go/lib/fresh.go:Fresh go/lib/lib.go:Edited\n",
		"- changed_tests: go/lib/lib_test.go:TestEdited\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("seeded block lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "handoff_nocheck") || strings.Contains(got, "handoff_file-name") || strings.Contains(got, "handoff-bad") || strings.Contains(got, "handoff_bad") || strings.Contains(got, "handoff_empty") || strings.Contains(got, "agents/evolve-auditor.md") || strings.Contains(got, "Guarded") {
		t.Errorf("an unreadable or empty handoff ledger and a protected file must not appear:\n%s", got)
	}
}

func TestSeedContext_RoundOneReadsTheLiveVerdictOrSaysNotRecorded(t *testing.T) {
	t.Parallel()
	f := newFactsFixture(t)
	if got := (hooks{}).SeedContext(f.req).Context[core.CtxKeyAuditFacts]; !strings.Contains(got, "- round: 1\n") || !strings.Contains(got, "- acs_verdict: not recorded\n") {
		t.Errorf("no records: want round 1 and not recorded:\n%s", got)
	}
	writeFile(t, filepath.Join(f.req.Workspace, "acs-verdict.json"), `{"red_count":0,"ship_eligible":true}`)
	if got := (hooks{}).SeedContext(f.req).Context[core.CtxKeyAuditFacts]; !strings.Contains(got, "- acs_verdict_round0: red_count=0 ship_eligible=true red=[]\n") {
		t.Errorf("the live verdict must be read when no archive exists:\n%s", got)
	}
}

func TestSeedContext_WithoutWorktreeOrBaseTheDiffFactsAreNotRecorded(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*core.PhaseRequest){
		"no worktree": func(r *core.PhaseRequest) { r.Worktree = "" },
		"no base":     func(r *core.PhaseRequest) { r.WorktreeBaseSHA = "" },
		"bad base":    func(r *core.PhaseRequest) { r.WorktreeBaseSHA = "deadbeef" },
		"not a repo":  func(r *core.PhaseRequest) { r.Worktree = t.TempDir() },
	} {
		f := newFactsFixture(t)
		mutate(&f.req)
		if got := (hooks{}).SeedContext(f.req).Context[core.CtxKeyAuditFacts]; !strings.Contains(got, "- changed_unprotected_functions: not recorded\n") {
			t.Errorf("%s: want not recorded:\n%s", name, got)
		}
	}
}

func TestSeedContext_DoesNotMutateTheCallersContextMap(t *testing.T) {
	t.Parallel()
	f := newFactsFixture(t)
	seeded := (hooks{}).SeedContext(f.req)
	if _, leaked := f.req.Context[core.CtxKeyAuditFacts]; leaked || len(f.req.Context) != 1 {
		t.Errorf("caller's map was written: %v", f.req.Context)
	}
	if seeded.Context["keep"] != "me" {
		t.Errorf("seeded request lost the existing context: %v", seeded.Context)
	}
	f.req.Context = nil
	if (hooks{}).SeedContext(f.req).Context[core.CtxKeyAuditFacts] == "" {
		t.Error("a nil Context must still receive the block")
	}
}

func TestComposePrompt_RendersTheSeededBlockBelowTheBoundaryAndRunsNothingItself(t *testing.T) {
	t.Parallel()
	f := newFactsFixture(t)
	bare := (hooks{}).ComposePrompt("PERSONA", f.req)
	if strings.Contains(bare, auditfacts.FactsHeading) {
		t.Fatalf("an unseeded request must carry no facts block:\n%s", bare)
	}
	if _, err := os.Stat(f.req.Workspace); err == nil {
		t.Error("ComposePrompt created the workspace")
	}
	seeded := (hooks{}).ComposePrompt("PERSONA", (hooks{}).SeedContext(f.req))
	if strings.Index(seeded, auditfacts.FactsHeading) < strings.Index(seeded, "## Cycle Context") {
		t.Errorf("the facts block must follow the cycle-context boundary:\n%s", seeded)
	}
	if got := runner.StaticPrefix(seeded); got != "PERSONA" {
		t.Errorf("static prefix = %q, want the persona only", got)
	}
}

func TestPhaseRun_EveryDispatchCarriesFactsAndRubricEvenWithExplanationVersionZero(t *testing.T) {
	t.Parallel()
	f := newFactsFixture(t)
	if err := os.MkdirAll(f.req.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	writeACSVerdict(t, f.req.Workspace, 0)
	fb := &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("# Auditor body")})
	f.req.ExplanationDocumentationVersion = 0

	_, _ = phase.Run(context.Background(), f.req)

	for _, want := range []string{auditfacts.FactsHeading, auditfacts.RegressionHeading, auditfacts.HonestyHeading, "go/lib/lib.go:Edited"} {
		if !strings.Contains(fb.gotReq.Prompt, want) {
			t.Errorf("dispatched prompt lacks %q", want)
		}
	}
}

func TestGateNames_EveryGateThatFailsTheAuditIsARemedyRowOfTheOneHome(t *testing.T) {
	t.Parallel()
	fails := func(core.PhaseRequest) ([]string, error) { return []string{"offender"}, nil }
	a := &auditClassification{hooks: hooks{
		gofmtCheck: fails, solutionCheck: fails, skillsDriftCheck: fails, goVetCheck: fails, acsDurableCheck: fails,
		apicoverEnforceCheck: fails, integrationTierCheck: fails, apicoverNewPkgGraduationCheck: fails,
	}}

	a.executeRepositoryGates()

	if len(a.overrodeBy) != 8 {
		t.Fatalf("overrodeBy = %v, want one label for each of the 8 repository gates", a.overrodeBy)
	}
	names := core.AuditGateNames()
	for _, label := range a.overrodeBy {
		if !slices.Contains(names, label) {
			t.Errorf("gate label %q is not in core.AuditGateNames() %v", label, names)
		}
	}
}

func TestPersonaReference_FinalDiffReviewIsAPointerToTheHostBlockNotACopyOfItsFormats(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "agents", "evolve-auditor-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "## Section: final-diff-review")
	if start < 0 {
		t.Fatal("the reference lacks the final-diff-review section")
	}
	section := body[start:]
	if end := strings.Index(section[1:], "\n## Section:"); end >= 0 {
		section = section[:end+1]
	}
	if !strings.Contains(section, "## Final Diff Review Sections (mandatory)") {
		t.Errorf("the section must point at the host block:\n%s", section)
	}
	for _, copied := range []string{"regression found or none", "honest or hollow", "is hollow"} {
		if strings.Contains(section, copied) {
			t.Errorf("the section restates %q, which auditfacts.Rubric owns:\n%s", copied, section)
		}
	}
	if !strings.Contains(auditfacts.Rubric, "regression found or none") {
		t.Error("the rubric must keep the formats")
	}
}
