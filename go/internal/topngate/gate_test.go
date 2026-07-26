// Package topngate implements the build->audit BLOCKING gate that enforces
// Builder task-slug binding to triage-report.md's ## top_n (inbox
// builder-task-binding-topn-gate, weight 0.96, 7th recurrence: cycles 282,
// 310, 522, 575, 577, 599, 640). This file drives topNBindingGate.check
// directly (white-box, same package — mirrors internal/evalgate/gates_test.go
// for materializationGate).
package topngate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeTriageReport writes a triage-report.md ## top_n section listing the
// given slugs (prose form, matching agents/evolve-triage.md Step 4's real
// output shape).
func writeTriageReport(t *testing.T, workspace string, topN ...string) {
	t.Helper()
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

// writeBuildReport writes a build-report.md whose ## Task: line claims
// claimedSlug (matching agents/evolve-builder.md's contracted header shape).
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
		// POLICY CHANGE (operator-directed, cycles 916 + 1012): both recorded
		// fatal rejections discarded CORRECT work whose report merely labeled
		// the committed task differently — two LLM strings compared. The lane
		// is plan-driven by construction, so label drift WARNs loudly and the
		// binding authority is the committed set. Scope-based fraud
		// verification is the queued construction-level replacement.
		ws := t.TempDir()
		writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
		writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
		reason, block := topNBindingGate{}.check(core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
		if block {
			t.Fatalf("label drift must never block; got reason=%q", reason)
		}
		// The reason is POPULATED (block=false) so the reviewer's single
		// structured logf seam emits the advisory — testable, unlike a raw
		// stderr write inside the gate.
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
		writeTriageReport(t, ws) // no entries
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

// writeTDDReport writes a test-report.md whose "## Task:" line claims
// claimedSlug and whose "## Handoff to Builder" fenced JSON declares testFiles
// (matching agents/evolve-tdd.md Step 6's contracted deliverable shape).
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

// TestTDDScopeGate_LabelDriftIsAdvisory is the cycle-1073 crux. tddScopeGate's
// case 2 (non-empty committed top_n + an authored slug with zero overlap) is
// the SAME "two LLM-authored strings compared for exact equality" defect that
// #348 (cbd088a1) converted to an advisory for the sibling topNBindingGate,
// one phase later in the pipeline. Two recorded false rejections (cycles 916,
// 1012) discarded correct work over a label; the triage->TDD transition carries
// the identical risk and must warn, not block.
func TestTDDScopeGate_LabelDriftIsAdvisory(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeTDDReport(t, ws, "fix-token-resolver-transcript-source", "go/acs/cycle1073/predicates_test.go")
	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if block {
		t.Fatalf("label drift at triage->TDD must never block (mirrors #348's build-gate fix); got reason=%q", reason)
	}
	// The reason stays POPULATED at block=false so the reviewer's single
	// structured logf seam still emits the advisory (testable, unlike a raw
	// stderr write inside the gate).
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

// TestTDDScopeGate_EmptyTopNStillBlocks is the anti-overcorrection guard: case
// 1 (empty committed top_n + a non-empty authored set) is NOT a labelling
// dispute — no committed item exists that the authored files could be a
// differently-labelled response to — so it must stay fatal. A blanket
// "block=false" rewrite of check() would pass the advisory test above and fail
// here.
func TestTDDScopeGate_EmptyTopNStillBlocks(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws) // triage committed nothing
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

	t.Run("multiple top_n slugs: any member passes", func(t *testing.T) {
		ws := t.TempDir()
		writeTriageReport(t, ws, "a", "b")
		writeTDDReport(t, ws, "b", "go/acs/cycle1073/predicates_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if reason != "" || block {
			t.Errorf("member of top_n must pass; got reason=%q block=%v", reason, block)
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
		writeTriageReport(t, ws) // empty top_n
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

// --- cycle-1111: file-scope binding (tdd-file-scope-binding-check) -----------
//
// The slug check above compares two LLM-authored PROSE labels, which is why
// both drift cases are advisory. The gate's own comments (gate.go:73-75,
// 132-135) name the missing complement as "the queued construction-level
// check": the committed item's DECLARED file scope vs what TDD actually
// authored. A deliverable can carry the right label and touch nothing the
// committed item names — today that passes silently.
//
// This second check runs ALONGSIDE the slug check on the in-lane path (it
// never replaces or tightens it) and is ADVISORY (block=false), matching this
// gate family's fail-open convention: a scope mismatch has legitimate causes
// (shared helper, incidental file), so it warns and lets shadow evidence
// decide whether it ever becomes fatal.

// scoutTask is one "### Task N: <slug>" entry of scout-report.md's
// "## Selected Tasks" section, with the paths its "- **targetFiles:**" line
// declares.
type scoutTask struct {
	slug        string
	targetFiles []string
}

// writeScoutReportTasks writes a scout-report.md whose "## Selected Tasks"
// section carries one "### Task N: <slug>" header per task followed by the
// contracted "- **targetFiles:** `path` (prose), `path` (prose)" line. The
// backticks and trailing prose annotations are REAL (see this cycle's own
// scout-report.md:43) — the parser must take the backticked paths and ignore
// the annotations. A task with no targetFiles emits no such line at all.
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

// writeScoutReport is the single-task convenience form.
func writeScoutReport(t *testing.T, workspace, slug string, targetFiles ...string) {
	t.Helper()
	writeScoutReportTasks(t, workspace, scoutTask{slug: slug, targetFiles: targetFiles})
}

// TestTDDScopeGate_FileScopeDriftIsAdvisory is the cycle-1111 crux: the label
// matches the committed item exactly, so every existing check passes silently,
// yet the authored file lives in a tree the committed item never names. That
// is the wrong-work shape the slug check provably cannot see, and the one the
// gate's own comments defer to this check.
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
		// The normal, correct shape: Scout names the PRODUCTION file, TDD
		// authors the sibling _test.go. Same directory = same scope; flagging
		// it would make the advisory fire on every healthy cycle.
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
		writeScoutReport(t, ws, "committed-slug") // header, no targetFiles line
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
		// The decoy task declares exactly the tree TDD authored into. A parser
		// that grabs the first (or any) **targetFiles:** line instead of the
		// committed slug's own line passes this and misses real wrong-work.
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
		// Regression guard on the existing check: when the slug is out-of-lane
		// the gate reports LABEL drift (the pre-existing signal); the new check
		// must not swallow or rename it.
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
		// The advisory must survive the reviewer at the STRICTEST stage: a
		// block=false signal can never abort a cycle, only log.
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
		// Anti-overcorrection: the ONE fatal case must not be softened by the
		// new advisory path.
		ws := t.TempDir()
		writeTriageReport(t, ws) // triage committed nothing
		writeScoutReport(t, ws, "orphan-task", "go/internal/topngate/gate.go")
		writeTDDReport(t, ws, "orphan-task", "go/internal/topngate/gate_test.go")
		reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
		if !block {
			t.Fatalf("authoring under an EMPTY top_n must stay fatal; got reason=%q block=false", reason)
		}
	})
}
