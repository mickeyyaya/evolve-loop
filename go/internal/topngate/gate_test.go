package topngate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeTriageReport also writes the matching triage-decision.json, which core.ContractTaskIDs reads.
func writeTriageReport(t *testing.T, workspace string, topN ...string) {
	t.Helper()
	writeTriageDecision(t, workspace, topN, nil)
	var b strings.Builder
	b.WriteString("## top_n (commit to THIS cycle)\n")
	for _, s := range topN {
		b.WriteString("- " + s + ": placeholder description — priority=H, evidence=x, source=inbox\n")
	}
	b.WriteString("\n## deferred (carry to NEXT cycle's carryoverTodos)\n")
	if err := os.WriteFile(filepath.Join(workspace, "triage-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write triage-report: %v", err)
	}
}

func writeBuildReport(t *testing.T, workspace, claimedSlug string) {
	t.Helper()
	body := "# Build Report\n\n## Task: " + claimedSlug + "\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write build-report: %v", err)
	}
}

func TestTopNBindingGate(t *testing.T) {
	t.Run("in-lane slug passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
		writeBuildReport(t, ws, "statefile-rmw-flock-single-source")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("in-lane slug must pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("out-of-lane slug is ADVISORY: WARN + pass (2026-07-22)", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
		writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if block {
			t.Fatalf("label drift must never block; got reason=%q", reason)
		}
		if !strings.Contains(reason, "label drift") || !strings.Contains(reason, "fix-token-resolver-transcript-source") || !strings.Contains(reason, "statefile-rmw-flock-single-source") {
			t.Fatalf("advisory reason must name the drift and both slug sets; got %q", reason)
		}
	})

	t.Run("multiple top_n slugs: any member passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a", "b")
		writeBuildReport(t, ws, "b")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("member of top_n must pass; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no triage report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeBuildReport(t, ws, "anything")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing triage-report.md must fail open (ambiguity, not a certain violation); got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no build report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing build-report.md must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("empty top_n → fail-open (nothing committed to bind against)", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws)
		writeBuildReport(t, ws, "anything")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if reason != "" || block {
			t.Errorf("empty top_n must fail open; got reason=%q block=%v", reason, block)
		}
	})
}

func TestTopNBindingGate_AppliesToBuildOnly(t *testing.T) {
	g := topNBindingGate{}
	if !g.appliesTo(string(core.PhaseBuild)) {
		t.Error("must apply to the build phase (reviews build-report.md right after build completes, before audit)")
	}
	for _, p := range []string{
		string(core.PhaseScout), string(core.PhaseTriage), string(core.PhaseTDD),
		string(core.PhaseAudit), string(core.PhaseShip),
	} {
		if g.appliesTo(p) {
			t.Errorf("must NOT apply to phase %q", p)
		}
	}
}

func writeTDDReport(t *testing.T, workspace, claimedSlug string, testFiles ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("# TDD Report\n\n## Task: " + claimedSlug + "\n\n## RED Run Output\n\n```\nFAIL\n```\n\n## Handoff to Builder\n\n```json\n{\"testFiles\": [")
	for i, f := range testFiles {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("\"" + f + "\"")
	}
	b.WriteString("], \"redRunConfirmed\": true}\n```\n")
	if err := os.WriteFile(filepath.Join(workspace, "test-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write test-report: %v", err)
	}
}

func TestTDDScopeGate_LabelDriftIsAdvisory(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeTDDReport(t, ws, "fix-token-resolver-transcript-source", "go/acs/cycle1073/predicates_test.go")
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if block {
		t.Fatalf("label drift at triage->TDD must never block (mirrors #348's build-gate fix); got reason=%q", reason)
	}
	if !strings.Contains(reason, "label drift") {
		t.Errorf("advisory reason must be labelled %q so operators can grep it; got %q", "label drift", reason)
	}
	if !strings.Contains(reason, "fix-token-resolver-transcript-source") {
		t.Errorf("advisory reason must name the CLAIMED slug; got %q", reason)
	}
	if !strings.Contains(reason, "statefile-rmw-flock-single-source") {
		t.Errorf("advisory reason must name the COMMITTED top_n set; got %q", reason)
	}
}

func TestTDDScopeGate_EmptyTopNStillBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws)
	writeTDDReport(t, ws, "orphan-task", "go/acs/cycle1073/predicates_test.go")
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if !block {
		t.Fatalf("orphan authoring under an EMPTY top_n must stay a hard block; got reason=%q block=false", reason)
	}
	if !strings.Contains(reason, "orphan-task") || !strings.Contains(reason, "EMPTY") {
		t.Errorf("fatal reason must name the claimed slug and the empty commitment; got %q", reason)
	}
}

func TestTDDScopeGate(t *testing.T) {
	t.Run("in-lane slug passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "tdd-topn-scope-gate")
		writeTDDReport(t, ws, "tdd-topn-scope-gate", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("in-lane authoring must pass silently; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("multiple top_n slugs: partial declaration blocks", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a", "b")
		writeTDDReport(t, ws, "b", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if !strings.Contains(reason, "scope-mismatch") || !block {
			t.Errorf("partial commitment must block; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no triage report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTDDReport(t, ws, "anything", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing triage-report.md is ambiguity, not a certain violation; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("no test report → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("missing test-report.md must fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("authored nothing → fail-open no-op PASS", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws)
		writeTDDReport(t, ws, "orphan-task")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("an empty authored set is the compliant no-op deliverable; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("unparseable claim with authored files → fail-open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeTDDReport(t, ws, "", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("no parseable claim is ambiguous → fail open; got reason=%q block=%v", reason, block)
		}
	})
}

func TestTDDScopeGate_AppliesToTDDOnly(t *testing.T) {
	g := tddScopeGate{}
	if !g.appliesTo(string(core.PhaseTDD)) {
		t.Error("must apply to the tdd phase (reviews test-report.md at the triage->TDD transition)")
	}
	for _, p := range []string{
		string(core.PhaseScout), string(core.PhaseTriage), string(core.PhaseBuild),
		string(core.PhaseAudit), string(core.PhaseShip),
	} {
		if g.appliesTo(p) {
			t.Errorf("must NOT apply to phase %q", p)
		}
	}
}

type scoutTask struct {
	slug        string
	targetFiles []string
}

// writeScoutReportTasks follows each backticked path with prose, as real scout
// reports do, so the parser must ignore the annotations.
func writeScoutReportTasks(t *testing.T, workspace string, tasks ...scoutTask) {
	t.Helper()
	var b strings.Builder
	b.WriteString("# Scout Report — Cycle 1111\n\n## Key Findings\n\n1. placeholder\n\n## Selected Tasks\n\n")
	for i, task := range tasks {
		b.WriteString("### Task " + string(rune('1'+i)) + ": " + task.slug + "\n\nPlaceholder task narrative.\n\n")
		if len(task.targetFiles) > 0 {
			b.WriteString("- **targetFiles:** ")
			for j, f := range task.targetFiles {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString("`" + f + "` (prose annotation the parser must ignore)")
			}
			b.WriteString("\n")
		}
		b.WriteString("- **complexity:** M\n- **dependsOn:** []\n\n")
	}
	b.WriteString("## Acceptance Criteria Summary\n\n- placeholder\n")
	if err := os.WriteFile(filepath.Join(workspace, "scout-report.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write scout-report: %v", err)
	}
}

func writeScoutReport(t *testing.T, workspace, slug string, targetFiles ...string) {
	t.Helper()
	writeScoutReportTasks(t, workspace, scoutTask{slug: slug, targetFiles: targetFiles})
}

func TestTDDScopeGate_FileScopeDriftIsAdvisory(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "tdd-file-scope-binding-check")
	writeScoutReport(t, ws, "tdd-file-scope-binding-check", "go/internal/topngate/gate.go", "go/internal/topngate/gate_test.go")
	writeTDDReport(t, ws, "tdd-file-scope-binding-check", "go/internal/tokenresolver/resolver_test.go")
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if block {
		t.Fatalf("file-scope drift must be ADVISORY like every other signal in this gate family; got block=true reason=%q", reason)
	}
	if !strings.Contains(reason, "file scope") {
		t.Fatalf("advisory reason must carry the greppable %q marker so operators can find it in the reviewer's logf seam; got %q", "file scope", reason)
	}
	if !strings.Contains(reason, "go/internal/tokenresolver/resolver_test.go") {
		t.Errorf("advisory reason must name the AUTHORED file(s); got %q", reason)
	}
	if !strings.Contains(reason, "go/internal/topngate/gate.go") {
		t.Errorf("advisory reason must name the committed item's DECLARED targetFiles; got %q", reason)
	}
	if !strings.Contains(reason, "tdd-file-scope-binding-check") {
		t.Errorf("advisory reason must name the committed slug the scope was read for; got %q", reason)
	}
}

func TestTDDScopeGate_FileScopeBinding(t *testing.T) {
	t.Run("exact targetFiles match passes silently", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "committed-slug", "go/internal/topngate/gate_test.go")
		writeTDDReport(t, ws, "committed-slug", "go/internal/topngate/gate_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("an exact scope match must stay silent; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("authored test file beside a declared target passes silently", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "committed-slug", "go/internal/topngate/gate.go")
		writeTDDReport(t, ws, "committed-slug", "go/internal/topngate/gate_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("a sibling file in a declared target's directory is in scope; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("one overlapping file among several authored is enough", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "committed-slug", "go/internal/topngate/gate.go")
		writeTDDReport(t, ws, "committed-slug", "go/acs/cycle1111/predicates_test.go", "go/internal/topngate/gate_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("overlap is an ANY relation, not an ALL relation; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("missing scout-report.md fails open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeTDDReport(t, ws, "committed-slug", "go/internal/tokenresolver/resolver_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("no declared scope to compare against is ambiguity → fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("task present but no targetFiles line fails open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "committed-slug")
		writeTDDReport(t, ws, "committed-slug", "go/internal/tokenresolver/resolver_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("an empty declared scope must fail open (both sets must be non-empty to compare); got reason=%q block=%v", reason, block)
		}
	})

	t.Run("committed slug absent from scout-report fails open", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "some-other-task", "go/internal/other/other.go")
		writeTDDReport(t, ws, "committed-slug", "go/internal/tokenresolver/resolver_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("no scope declared for the committed slug → nothing to compare → fail open; got reason=%q block=%v", reason, block)
		}
	})

	t.Run("scope is read per-slug, never borrowed from a sibling task", func(t *testing.T) {
		// The decoy declares exactly the tree TDD authored into, so a parser
		// that reads any task's targetFiles line would stay silent.
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReportTasks(t, ws,
			scoutTask{slug: "committed-slug", targetFiles: []string{"go/internal/topngate/gate.go"}},
			scoutTask{slug: "decoy-task", targetFiles: []string{"go/internal/tokenresolver/resolver.go"}},
		)
		writeTDDReport(t, ws, "committed-slug", "go/internal/tokenresolver/resolver_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if block {
			t.Fatalf("file-scope drift must never block; got reason=%q", reason)
		}
		if !strings.Contains(reason, "file scope") {
			t.Errorf("the decoy task's scope must not satisfy the committed slug's check; want a file-scope advisory, got %q", reason)
		}
	})

	t.Run("label drift keeps its own advisory, not the file-scope one", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "committed-slug", "go/internal/topngate/gate.go")
		writeTDDReport(t, ws, "different-label", "go/internal/topngate/gate_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if block {
			t.Fatalf("label drift must never block; got reason=%q", reason)
		}
		if !strings.Contains(reason, "label drift") {
			t.Errorf("the pre-existing label-drift advisory must survive unchanged; got %q", reason)
		}
	})

	t.Run("enforce stage approves the file-scope advisory end to end", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "committed-slug")
		writeScoutReport(t, ws, "committed-slug", "go/internal/topngate/gate.go")
		writeTDDReport(t, ws, "committed-slug", "go/internal/tokenresolver/resolver_test.go")
		res := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if !res.Approve {
			t.Fatalf("file-scope drift must approve even at enforce; got Approve=false reason=%q", res.Reason)
		}
	})

	t.Run("empty top_n still blocks regardless of declared scope", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws)
		writeScoutReport(t, ws, "orphan-task", "go/internal/topngate/gate.go")
		writeTDDReport(t, ws, "orphan-task", "go/internal/topngate/gate_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if !block {
			t.Fatalf("authoring under an EMPTY top_n must stay fatal; got reason=%q block=false", reason)
		}
	})
}

func writeTriageDecision(t *testing.T, workspace string, topN, deferred []string) {
	t.Helper()
	ids := func(in []string) []map[string]string {
		out := make([]map[string]string, 0, len(in))
		for _, s := range in {
			out = append(out, map[string]string{"id": s})
		}
		return out
	}
	body, err := json.Marshal(map[string]any{"top_n": ids(topN), "deferred": ids(deferred)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "triage-decision.json"), body, 0o644); err != nil {
		t.Fatalf("write triage-decision: %v", err)
	}
}
