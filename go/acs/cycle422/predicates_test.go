//go:build acs

// Package cycle422 materializes the cycle-422 acceptance criteria for two prompt-compaction tasks:
//
//   - intent-prompt-reference-index-expansion (T1) — relocate ## Output contract (INTENT_MODE),
//     ## Re-run behavior, and ## Reflection Authoring below ## Reference Index in
//     evolve-intent.md so ≥2200B is stripped per intent dispatch (up from current ~755B).
//
//   - triage-prompt-reference-index-expansion (T2) — relocate additional on-demand reference
//     (step-3b predicate-graph bash example, and other verbose on-demand content) below
//     ## Reference Index in evolve-triage.md so ≥4200B is stripped (up from current ~3209B).
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	intent-prompt-reference-index-expansion (T1):
//	  AC1  evolve-intent.md saves ≥2200B after StripOnDemandSections               → C422_001 (RED)
//	  AC2  ## Output contract (INTENT_MODE) absent from stripped body (negative)   → C422_002 (RED)
//	  AC3  ## Re-run behavior absent from stripped body (negative)                 → C422_003 (RED)
//	  AC4  ## Reflection Authoring detail absent from stripped body (negative)     → C422_004 (RED)
//	  AC5  Required anchors survive above marker (regression guard)                → C422_005 (pre-existing GREEN)
//	  AC6  Synthetic buried-anchor negative (non-vacuity proof)                    → C422_006 (pre-existing GREEN)
//
//	triage-prompt-reference-index-expansion (T2):
//	  AC1  evolve-triage.md saves ≥4200B after StripOnDemandSections               → C422_007 (RED)
//	  AC2  Decision anchors survive above marker (regression guard)                → C422_008 (pre-existing GREEN)
//	  AC3  Synthetic buried-anchor negative (non-vacuity proof)                    → C422_009 (pre-existing GREEN)
//	  AC4  On-demand bash example absent from stripped body (negative)             → C422_010 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C422_002/003/004 (on-demand sections still above marker → fail "absent");
//	          C422_006/009 (synthetic buried-rule — StripOnDemandSections must remove it);
//	          C422_010 (bash example still above marker → fail "absent").
//	Edge/OOD: C422_005 (anchor guard: fires if builder over-relocates required anchors);
//	          C422_008 (decision-anchor guard: fires if builder relocates process-critical content).
//	Semantic:  10 distinct dimensions: byte-delta-intent / output-contract-absent /
//	           rerun-absent / reflection-absent / intent-anchor-survival / synthetic-intent /
//	           byte-delta-triage / triage-anchor-survival / synthetic-triage / bash-example-absent.
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C422_001–C422_006), manual+checklist=0, unverifiable-remove=0 → total AC=6 ✓
//	T2: predicate=4 (C422_007–C422_010), manual+checklist=0, unverifiable-remove=0 → total AC=4 ✓
package cycle422

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ===================== T1 — intent-prompt-reference-index-expansion =====================

// TestC422_001_IntentCompactionSavesBytes2200 asserts that StripOnDemandSections applied to
// the real evolve-intent.md body saves ≥2200 bytes — the target floor after relocating
// ## Output contract (INTENT_MODE), ## Re-run behavior, and ## Reflection Authoring below
// the ## Reference Index marker.
// RED: only ~755B is currently below ## Reference Index; 755 < 2200 → FAIL.
func TestC422_001_IntentCompactionSavesBytes2200(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 2200 {
		t.Errorf("RED: intent compaction saved only %d bytes (want ≥2200); relocate ## Output contract (INTENT_MODE), ## Re-run behavior, ## Reflection Authoring below ## Reference Index in evolve-intent.md (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC422_002_IntentOutputContractAbsentAfterStrip_Negative asserts that the
// ## Output contract (INTENT_MODE) section is relocated BELOW the ## Reference Index
// marker (absent from the stripped body) in evolve-intent.md.
// RED: ## Output contract (INTENT_MODE) is currently ABOVE line-203 marker → present in
// stripped body → FAIL.
// GREEN after Builder: section moved below marker → intent-delta.md absent in stripped.
func TestC422_002_IntentOutputContractAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	// intent-delta.md is the delta-mode output file referenced only inside
	// ## Output contract (INTENT_MODE). Its presence in stripped body proves
	// the section was not relocated below the marker.
	if strings.Contains(stripped, "intent-delta.md") {
		t.Error("RED: 'intent-delta.md' still appears in stripped body — ## Output contract (INTENT_MODE) must be relocated below ## Reference Index in evolve-intent.md")
	}
}

// TestC422_003_IntentRerunBehaviorAbsentAfterStrip_Negative asserts that the
// ## Re-run behavior section is relocated BELOW the ## Reference Index marker
// (absent from the stripped body) in evolve-intent.md.
// RED: ## Re-run behavior is currently ABOVE line-203 marker → its content present in
// stripped body → FAIL.
// GREEN after Builder: section moved below marker → heading absent from stripped.
func TestC422_003_IntentRerunBehaviorAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	// "## Re-run behavior" is the line-anchored heading of the section.
	// Checking for the heading confirms the entire section was moved below the marker.
	if strings.Contains(stripped, "## Re-run behavior") {
		t.Error("RED: '## Re-run behavior' heading still appears in stripped body — ## Re-run behavior must be relocated below ## Reference Index in evolve-intent.md")
	}
}

// TestC422_004_IntentReflectionAuthoringAbsentAfterStrip_Negative asserts that the
// ## Reflection Authoring (v10.20.0+) detail is relocated BELOW the ## Reference Index
// marker (absent from the stripped body) in evolve-intent.md.
// RED: ## Reflection Authoring (v10.20.0+) is currently ABOVE line-203 marker → its
// sidecar reference (intent-reflection.yaml) present in stripped body → FAIL.
// GREEN after Builder: section moved below marker → intent-reflection.yaml absent.
func TestC422_004_IntentReflectionAuthoringAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	// intent-reflection.yaml is the sidecar file referenced only inside
	// ## Reflection Authoring (v10.20.0+). Its presence proves the section was not moved.
	if strings.Contains(stripped, "intent-reflection.yaml") {
		t.Error("RED: 'intent-reflection.yaml' still appears in stripped body — ## Reflection Authoring must be relocated below ## Reference Index in evolve-intent.md")
	}
}

// TestC422_005_IntentRequiredAnchorsAboveMarker asserts that required every-cycle
// behavior anchors survive prompts.StripOnDemandSections (remain above the ## Reference
// Index marker) in evolve-intent.md.
// Pre-existing GREEN: all listed anchors are above the line-203 marker in the current file.
// Regression guard: fires if Builder accidentally buries any anchor below the marker.
// NOTE: "Prior FAIL audit" is in ## Output contract today; Builder MUST add a brief inline
// mention above the marker when relocating that section, or this guard fires.
func TestC422_005_IntentRequiredAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"STOP CRITERION",
		"challenged_premise",
		"30-80 line",
		"EMERGENCY EXIT",
		"Prior FAIL audit",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required anchor %q lost below ## Reference Index — must remain above marker in evolve-intent.md (Builder: add brief inline mention if the anchor's source section was relocated)", anchor)
		}
	}
}

// TestC422_006_IntentSyntheticBuriedAnchorNegative asserts that a required rule placed
// below ## Reference Index in a synthetic body does NOT appear in the stripped output.
// Anti-gaming: proves StripOnDemandSections actually removes below-marker content so the
// byte-savings and absent-section tests cannot be gamed by a no-op strip implementation.
// Pre-existing GREEN: StripOnDemandSections correctly removes below-marker content.
func TestC422_006_IntentSyntheticBuriedAnchorNegative(t *testing.T) {
	body := "Preamble operational content.\n\n## Reference Index\n\nPrior FAIL audit buried here\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading — StripOnDemandSections broken")
	}
	if strings.Contains(stripped, "Prior FAIL audit buried here") {
		t.Error("synthetic: anchor buried below ## Reference Index survived strip — StripOnDemandSections must remove below-marker content")
	}
}

// ===================== T2 — triage-prompt-reference-index-expansion =====================

// TestC422_007_TriageCompactionSavesBytes4200 asserts that StripOnDemandSections applied to
// the real evolve-triage.md body saves ≥4200 bytes — the target floor after relocating
// additional on-demand reference sections below the ## Reference Index marker.
// RED: only ~3209B is currently below ## Reference Index; 3209 < 4200 → FAIL.
func TestC422_007_TriageCompactionSavesBytes4200(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 4200 {
		t.Errorf("RED: triage compaction saved only %d bytes (want ≥4200); relocate additional on-demand reference (step-3b detection example, verbose tables) below ## Reference Index in evolve-triage.md (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC422_008_TriageDecisionAnchorsAboveMarker asserts that required triage-decision
// anchors survive prompts.StripOnDemandSections (remain above the ## Reference Index
// marker) in evolve-triage.md.
// Pre-existing GREEN: all listed anchors are above the line-211 marker in the current file.
// Regression guard: fires if Builder accidentally buries a process-critical anchor.
func TestC422_008_TriageDecisionAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"challenge-token",
		"## top_n",
		"## deferred",
		"## dropped",
		"carryoverTodos",
		"## Rationale",
		"Operator-queue priority floor",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required decision anchor %q lost below ## Reference Index — must remain above marker in evolve-triage.md", anchor)
		}
	}
}

// TestC422_009_TriageSyntheticBuriedAnchorNegative asserts that a decision anchor placed
// below ## Reference Index in a synthetic body does NOT appear in the stripped output.
// Anti-gaming: proves StripOnDemandSections removes below-marker content so byte-savings
// and absent-section tests cannot be gamed by a no-op implementation.
// Pre-existing GREEN: StripOnDemandSections correctly removes below-marker content.
func TestC422_009_TriageSyntheticBuriedAnchorNegative(t *testing.T) {
	body := "Triage preamble.\n\n## Reference Index\n\nOperator-queue priority floor buried here\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading — StripOnDemandSections broken")
	}
	if strings.Contains(stripped, "Operator-queue priority floor buried here") {
		t.Error("synthetic: anchor buried below ## Reference Index survived strip — StripOnDemandSections must remove below-marker content")
	}
}

// TestC422_010_TriageOnDemandBashExampleAbsentAfterStrip_Negative asserts that the
// step-3b predicate-graph detection bash example is relocated BELOW the ## Reference Index
// marker (absent from the stripped body) in evolve-triage.md.
// RED: "predicate-graph-reachable" echo output (step-3b bash code, line ~122) is currently
// ABOVE line-211 marker → present in stripped body → FAIL.
// GREEN after Builder: bash example moved below marker → string absent in stripped.
func TestC422_010_TriageOnDemandBashExampleAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	// "predicate-graph-reachable" is the echo output in the step-3b bash detection example —
	// it only appears inside the bash code block and would not appear in an inline rule mention.
	if strings.Contains(stripped, "predicate-graph-reachable") {
		t.Error("RED: 'predicate-graph-reachable' (step-3b bash example) still appears in stripped body — the detection example must be relocated below ## Reference Index in evolve-triage.md")
	}
}
