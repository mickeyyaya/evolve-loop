//go:build acs

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

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

// acs-predicate: config-check (inherent Config-struct field presence — no other behavioral probe is
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
