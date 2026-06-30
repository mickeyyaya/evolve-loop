//go:build acs

// Package cycle421 materializes the cycle-421 acceptance criteria for two prompt-compaction tasks:
//
//   - retro-phase-compaction-wiring (T1) — wire CompactPrompts into retro phase (only content
//     phase that loads an agent doc outside the BaseRunner compaction path), and rebalance
//     evolve-retrospective.md so ≥1500B is stripped per retro invocation.
//
//   - orchestrator-reference-index-rebalance (T2) — relocate on-demand sections of
//     evolve-orchestrator.md below ## Reference Index so ≥2000B is stripped; add
//     TestOrchestratorCompaction byte-floor + anchor-survival test mirroring cycle 415-417 pattern.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	retro-phase-compaction-wiring (T1):
//	  AC1  evolve-retrospective.md saves ≥1500B after StripOnDemandSections → C421_001 (RED)
//	  AC2  retro output-contract anchors survive strip (above marker)          → C421_002 (pre-existing GREEN)
//	  AC3  versioned sections absent from stripped body (negative)             → C421_003 (RED)
//	  AC4  retro.Config has CompactPrompts bool field                          → C421_004 (RED)
//	  AC5  CompactPrompts=true → bridge receives stripped prompt               → C421_005 (RED)
//	  AC6  CompactPrompts=false → prompt byte-identical to raw body            → C421_006 (pre-existing GREEN)
//
//	orchestrator-reference-index-rebalance (T2):
//	  AC1  evolve-orchestrator.md saves ≥2000B after StripOnDemandSections     → C421_007 (RED)
//	  AC2  stripped head ≥9000B (anti-over-strip guard)                        → C421_008 (pre-existing GREEN)
//	  AC3  gate-bearing anchors survive strip (above marker)                   → C421_009 (pre-existing GREEN)
//	  AC4  on-demand sections absent from stripped body (negative)             → C421_010 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C421_003 (versioned retro sections still above marker → fail);
//	          C421_010 (path/worktree/closure sections still above marker → fail).
//	Edge/OOD: C421_008 (boundary: head must be ≥9000B even if aggressive relocation);
//	          C421_006 (disabled-path must be byte-identical — catch accidental always-strip).
//	Semantic:  10 distinct dimensions: byte-delta retro / retro-anchor-survival /
//	           retro-versioned-absent / config-field / prompt-stripped / identity /
//	           byte-delta orchestrator / head-floor / gate-anchor / on-demand-absent.
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C421_001–C421_006), manual+checklist=0, unverifiable-remove=0 → total AC=6 ✓
//	T2: predicate=4 (C421_007–C421_010), manual+checklist=0, unverifiable-remove=0 → total AC=4 ✓
package cycle421

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the <repoRoot>/go module directory for subprocess test invocations.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// ===================== T1 — retro-phase-compaction-wiring =====================

// TestC421_001_RetroCompactionSavesBytes1500 asserts that StripOnDemandSections applied
// to the real evolve-retrospective.md body saves ≥1500 bytes — the minimum on-demand
// tail size required per scout AC.
// RED: only 571B is currently below ## Reference Index; 571 < 1500 → FAIL.
func TestC421_001_RetroCompactionSavesBytes1500(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 1500 {
		t.Errorf("RED: retro compaction saved only %d bytes (want ≥1500); relocate on-demand sections below ## Reference Index in evolve-retrospective.md (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC421_002_RetroCompactionOutputAnchorsAboveMarker asserts that required output-contract
// anchors survive prompts.StripOnDemandSections (remain above the ## Reference Index marker).
// Pre-existing GREEN: anchors are above line 204 marker; current stripping only removes 571B reference table.
// Regression guard: fires if Builder accidentally buries any output-contract anchor below the marker.
func TestC421_002_RetroCompactionOutputAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"## Core Principles",
		"retrospective-report.md",
		"failure-lesson",
		"handoff-retrospective.json",
		"challenge-token",
		"## Final checks before exit",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required output-contract anchor %q lost below ## Reference Index — must remain above marker in evolve-retrospective.md", anchor)
		}
	}
}

// TestC421_003_RetroCompactionVersionedAbsentAfterStrip_Negative asserts that
// versioned-feature sections are relocated BELOW the ## Reference Index marker (absent from stripped body).
// RED: sections are currently above line-204 marker → present in stripped body → FAIL.
// GREEN after Builder: sections moved below marker → absent in stripped body.
func TestC421_003_RetroCompactionVersionedAbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-retrospective.md"))
	if err != nil {
		t.Fatalf("read evolve-retrospective.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, versioned := range []string{
		"### 1.5 Read abnormal-events.jsonl (v46+)",
		"### 1.7 Read reflector synthesis (v10.20.0+)",
	} {
		if strings.Contains(stripped, versioned) {
			t.Errorf("RED: versioned section %q still appears above ## Reference Index — relocate it below the marker in evolve-retrospective.md", versioned)
		}
	}
}

// TestC421_004_RetroPhaseConfigHasCompactPrompts asserts that retro.Config has a
// CompactPrompts bool field, making the retro phase compaction-aware like every other
// BaseRunner-based content phase.
// acs-predicate: config-check (inherent Config-struct field presence — no other behavioral probe is
// possible before the field exists; B1 registry-class fix requires this structural check).
// RED: retro.Config has no CompactPrompts field → reflect FieldByName returns zero Value → FAIL.
func TestC421_004_RetroPhaseConfigHasCompactPrompts(t *testing.T) {
	ct := reflect.TypeOf(retro.Config{})
	f, ok := ct.FieldByName("CompactPrompts")
	if !ok {
		t.Fatal("RED: retro.Config has no CompactPrompts bool field — Builder must add it (pattern: all content-phase Configs carry this field to stay compaction-aware)")
	}
	if f.Type.Kind() != reflect.Bool {
		t.Fatalf("retro.Config.CompactPrompts is %v, want bool", f.Type)
	}
}

// TestC421_005_RetroPhaseCompactEnabledStripsBody invokes the behavioral unit test
// TestRetroPhase_CompactEnabled_StripsBody in the retro package, which asserts that
// setting CompactPrompts=true via reflect causes the bridge to receive a stripped prompt
// (on-demand tail absent).
// RED: retro.Config has no CompactPrompts field → unit test calls t.Fatal → FAIL.
func TestC421_005_RetroPhaseCompactEnabledStripsBody(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRetroPhase_CompactEnabled_StripsBody",
		"./internal/phases/retro/")
	if err != nil || code != 0 {
		t.Errorf("RED: TestRetroPhase_CompactEnabled_StripsBody failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// TestC421_006_RetroPhaseCompactDisabledBodyIdentical invokes the regression unit test
// TestRetroPhase_CompactDisabled_BodyIdentical in the retro package, which asserts that
// when CompactPrompts=false (default), the bridge receives the raw unstripped body.
// Pre-existing GREEN: current retro phase strips nothing; identity holds.
func TestC421_006_RetroPhaseCompactDisabledBodyIdentical(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-run", "TestRetroPhase_CompactDisabled_BodyIdentical",
		"./internal/phases/retro/")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: TestRetroPhase_CompactDisabled_BodyIdentical failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}

// ===================== T2 — orchestrator-reference-index-rebalance =====================

// TestC421_007_OrchestratorCompactionSavesBytes2000 asserts that StripOnDemandSections
// applied to the real evolve-orchestrator.md body saves ≥2000 bytes.
// RED: only 993B is currently below ## Reference Index (line 316); 993 < 2000 → FAIL.
func TestC421_007_OrchestratorCompactionSavesBytes2000(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 2000 {
		t.Errorf("RED: orchestrator compaction saved only %d bytes (want ≥2000); relocate on-demand sections below ## Reference Index in evolve-orchestrator.md (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC421_008_OrchestratorCompactionHeadFloor9000 asserts that the stripped orchestrator
// body (above-marker head) is at least 9000 bytes — a floor preventing over-relocation
// that would strip gate-bearing operational content.
// Pre-existing GREEN: current head is ~19032B >> 9000B; guard fires only on aggressive over-strip.
func TestC421_008_OrchestratorCompactionHeadFloor9000(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if len(stripped) < 9000 {
		t.Errorf("orchestrator head too small after strip: %d bytes (want ≥9000); Builder relocated too much — gate-bearing sections must stay above ## Reference Index", len(stripped))
	}
}

// TestC421_009_OrchestratorCompactionGateAnchorsAboveMarker asserts that gate-bearing
// anchors required every cycle survive prompts.StripOnDemandSections (above the ## Reference Index
// marker) in evolve-orchestrator.md.
// Pre-existing GREEN: all gate anchors are above line 316; current 993B strip leaves them intact.
// Regression guard: fires if Builder accidentally relocates a gate-bearing section below the marker.
func TestC421_009_OrchestratorCompactionGateAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"EGPS Verdict-of-Record",
		"Verdict Decision Tree",
		"STOP CRITERION",
		"Completion Gates",
		"Banned Post-Report Patterns",
		"Fast-Fail Abort",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("gate-bearing anchor %q lost below ## Reference Index — must remain above marker in evolve-orchestrator.md", anchor)
		}
	}
}

// TestC421_010_OrchestratorCompactionOnDemandSectionsAbsent_Negative asserts that
// on-demand/rarely-needed sections are relocated BELOW the ## Reference Index marker
// (absent from stripped body) in evolve-orchestrator.md.
// RED: sections currently above line-316 marker → present in stripped body → FAIL.
// GREEN after Builder: sections relocated below marker → absent in stripped body.
func TestC421_010_OrchestratorCompactionOnDemandSectionsAbsent_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read evolve-orchestrator.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, onDemand := range []string{
		"## Path conventions",
		"## Worktree contract",
		"## Closure-Mode Detection",
	} {
		if strings.Contains(stripped, onDemand) {
			t.Errorf("RED: on-demand section %q still appears above ## Reference Index — relocate it below the marker in evolve-orchestrator.md", onDemand)
		}
	}
}
