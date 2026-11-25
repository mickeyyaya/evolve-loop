package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

func scopeOf(enforce bool) *core.AuditRoundScope {
	prev := reportdoc.Finding{ID: "M1", Severity: "MEDIUM", Title: "the brief omits the failing predicate at b.go:5", Location: "b.go:5"}
	return &core.AuditRoundScope{
		Round: 2, Enforce: enforce, Tree: "tree-2",
		KnownIDs: []string{core.HostFindingID(prev)},
		Previous: []core.ScopeFinding{{HostID: core.HostFindingID(prev), Severity: "MEDIUM", Title: prev.Title, Location: "b.go:5"}},
		Hunks:    []convergence.Hunk{{File: "b.go", From: 30, To: 35}},
	}
}

func promptFor(t *testing.T, root string, req core.PhaseRequest) string {
	t.Helper()
	req.Cycle, req.ProjectRoot, req.Workspace = 2035, root, root
	return hooks{}.ComposePrompt("PERSONA", req)
}

func TestPromptScopeBlockFromRound2AtEnforce(t *testing.T) {
	t.Parallel()
	root := projectRootWithPolicy(t, `{}`)
	got := promptFor(t, root, core.PhaseRequest{AuditRound: 2, AuditScope: scopeOf(true)})
	for _, want := range []string{
		"## Round scope",
		core.HostFindingID(reportdoc.Finding{Title: "the brief omits the failing predicate at b.go:5"}),
		"MEDIUM", "b.go:5", "b.go:30-35",
		"audit-round-dispositions.json", "## Filed (late)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("round-2 enforce prompt lacks %q", want)
		}
	}
	if round1 := promptFor(t, root, core.PhaseRequest{AuditRound: 1}); strings.Contains(round1, "## Round scope") {
		t.Error("the round-1 prompt carries a scope block")
	}
}

func TestPromptNamesTheLateFileBarOfTheAuditRepairLoop(t *testing.T) {
	t.Parallel()
	root := projectRootWithPolicy(t, `{}`)
	got := promptFor(t, root, core.PhaseRequest{AuditRound: 2, AuditScope: scopeOf(true)})
	bar := string(convergence.LateFileBar(convergence.LoopAuditRepair))
	for _, want := range []string{"below " + bar + ", goes under `## Filed (late)`", "at " + bar + " or above, outside the hunks, goes under `## Issues`"} {
		if !strings.Contains(got, want) {
			t.Errorf("the round-2 prompt lacks %q", want)
		}
	}
}

func TestShadowDoesNotChangePrompt(t *testing.T) {
	t.Parallel()
	root := projectRootWithPolicy(t, `{}`)
	shadow := promptFor(t, root, core.PhaseRequest{AuditRound: 2, AuditScope: scopeOf(false)})
	plain := promptFor(t, root, core.PhaseRequest{AuditRound: 2})
	if shadow != plain {
		t.Fatalf("a shadow scope changed the prompt:\n%s\n---\n%s", shadow, plain)
	}
}

func TestPersonaScopesFullChecklistToRound1(t *testing.T) {
	t.Parallel()
	persona := auditorPersona(t)
	const pointer = "Run the full checklist unless the host gives a Round scope block"
	if !strings.Contains(persona, pointer) {
		t.Fatalf("the persona lacks the pointer line %q", pointer)
	}
	lower := strings.ToLower(persona)
	for at := 0; ; {
		i := strings.Index(lower[at:], "always run full checklist when")
		if i < 0 {
			break
		}
		start := at + i
		if !strings.HasSuffix(strings.TrimSpace(lower[:start]), "unless the host gives a round scope block,") {
			t.Fatalf("an unconditional full-checklist rule survives at %q", persona[max(0, start-60):start+60])
		}
		at = start + 1
	}
}

func filedClassification(t *testing.T, scope *core.AuditRoundScope, report string, tree string) *auditClassification {
	t.Helper()
	a := &auditClassification{
		hooks:    hooks{auditTree: func(core.PhaseRequest) string { return tree }},
		artifact: report,
		req:      core.PhaseRequest{AuditRound: 2, AuditScope: scope},
		verdict:  core.VerdictPASS, verdictFound: true,
	}
	a.checkLateFiling()
	return a
}

const filedOK = "## Verdict\nPASS\n\n## Filed (late)\n### M2 (MEDIUM) — a fresh defect in untouched code at b.go:20\n"

func TestCheckOnlyForcesFail(t *testing.T) {
	t.Parallel()
	for name, report := range map[string]string{
		"a known id":            "## Filed (late)\n### M1 (MEDIUM) — the brief omits the failing predicate at b.go:5\n",
		"a location in hunks":   "## Filed (late)\n### M2 (MEDIUM) — a defect at b.go:31\n",
		"a HIGH":                "## Filed (late)\n### H2 (HIGH) — a fresh serious defect at b.go:20\n",
		"a CRITICAL":            "## Filed (late)\n### C2 (CRITICAL) — a fresh severe defect at b.go:20\n",
		"no location":           "## Filed (late)\n### M2 (MEDIUM) — a fresh defect with no place\n",
		"the previous location": "## Filed (late)\n### M2 (MEDIUM) — a reworded copy at b.go:5\n",
	} {
		t.Run(name, func(t *testing.T) {
			a := filedClassification(t, scopeOf(true), report, "tree-2")
			if a.verdict != core.VerdictFAIL || len(a.overrodeBy) == 0 {
				t.Fatalf("verdict = %s, want FAIL from %s", a.verdict, name)
			}
		})
	}
}

func TestCheckLetsAValidFilingStandAndTouchesNoVerdict(t *testing.T) {
	t.Parallel()
	for _, verdict := range []string{core.VerdictPASS, core.VerdictWARN, core.VerdictFAIL} {
		a := &auditClassification{
			hooks: hooks{auditTree: func(core.PhaseRequest) string { return "tree-2" }}, artifact: filedOK,
			req: core.PhaseRequest{AuditRound: 2, AuditScope: scopeOf(true)}, verdict: verdict,
		}
		a.checkLateFiling()
		if a.verdict != verdict || len(a.overrodeBy) != 0 {
			t.Errorf("a valid filing moved %s to %s (%v)", verdict, a.verdict, a.overrodeBy)
		}
	}
}

func TestCheckNeverLiftsAFail(t *testing.T) {
	t.Parallel()
	reports := []string{filedOK, "## Verdict\nPASS\n", "## Filed (late)\n### H2 (HIGH) — x at b.go:20\n", ""}
	for _, scope := range []*core.AuditRoundScope{scopeOf(true), scopeOf(false), nil} {
		for _, tree := range []string{"tree-2", "other"} {
			for i := 0; i < 2; i++ {
				for _, report := range reports {
					a := &auditClassification{
						hooks: hooks{auditTree: func(core.PhaseRequest) string { return tree }}, artifact: report,
						req: core.PhaseRequest{AuditRound: 2, AuditScope: scope}, verdict: core.VerdictFAIL,
					}
					a.checkLateFiling()
					if a.verdict != core.VerdictFAIL {
						t.Fatalf("scope %v tree %s report %q lifted FAIL to %s", scope, tree, report, a.verdict)
					}
				}
			}
		}
	}
}

func TestCheckIgnoresShadowAndRoundOne(t *testing.T) {
	t.Parallel()
	for name, scope := range map[string]*core.AuditRoundScope{"shadow": scopeOf(false), "round 1": nil} {
		bad := "## Filed (late)\n### H2 (HIGH) — x at b.go:20\n"
		if a := filedClassification(t, scope, bad, "tree-2"); a.verdict != core.VerdictPASS {
			t.Errorf("%s: verdict = %s, want the check to stay out", name, a.verdict)
		}
	}
}

func TestCheckFailsClosedWhenTheTreeMovedUnderAFiling(t *testing.T) {
	t.Parallel()
	a := filedClassification(t, scopeOf(true), filedOK, "a-different-tree")
	if a.verdict != core.VerdictFAIL || !strings.Contains(strings.Join(overrides(a), " "), "tree") {
		t.Fatalf("verdict %s overrides %v, want FAIL naming the moved tree", a.verdict, a.overrodeBy)
	}
	if clean := filedClassification(t, scopeOf(true), "## Verdict\nPASS\n", "a-different-tree"); clean.verdict != core.VerdictPASS {
		t.Fatal("a moved tree with no filed entry must not fail the audit")
	}
}

func overrides(a *auditClassification) []string { return a.overrodeBy }

func TestPromptScopeBlockNamesAnEmptyScope(t *testing.T) {
	t.Parallel()
	root := projectRootWithPolicy(t, `{}`)
	empty := &core.AuditRoundScope{Round: 2, Enforce: true}
	got := promptFor(t, root, core.PhaseRequest{AuditRound: 2, AuditScope: empty})
	if strings.Count(got, "- none") != 2 {
		t.Fatalf("the empty scope block lacks its two none lines:\n%s", got)
	}
}

func TestNewHooksWiresTheTreeReaderTheCheckUses(t *testing.T) {
	t.Parallel()
	h := newHooks(Config{})
	if h.auditTree == nil {
		t.Fatal("the production hooks have no tree reader, so the check could never see a moved tree")
	}
	if got := h.auditTree(core.PhaseRequest{}); got != "" {
		t.Fatalf("a request with no worktree has tree %q", got)
	}
}

func TestAuditWiresTheLedgerResolverAsTheRoundEvidenceResolver(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if core.RoundEvidenceResolver == nil {
		t.Fatal("the audit package must wire core.RoundEvidenceResolver")
	}
	req := defectledger.Request{ProjectRoot: root, Worktree: root}
	for _, evidence := range []string{"b.go", "b.go:1", "b.go; reran tests", "missing.go", "words only", "../b.go"} {
		wantOK, _ := resolveEvidence(evidence, req)
		if gotOK, _ := core.RoundEvidenceResolver(evidence, req); gotOK != wantOK {
			t.Errorf("evidence %q: the round resolver says %v, the ledger resolver says %v", evidence, gotOK, wantOK)
		}
	}
}
