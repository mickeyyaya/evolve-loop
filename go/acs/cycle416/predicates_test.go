//go:build acs

package cycle416

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC416_001_IntentMdHasCompactMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !bodyHasCompactMarker(body) {
		t.Errorf("RED: evolve-intent.md has no line-anchored ## Reference Index heading.\n" +
			"Builder must add:\n" +
			"  ## Reference Index (Layer 3, on-demand)\n" +
			"above the ## Composition section (line 199) and below all behavior-bearing rules.\n" +
			"This closes the dead-wired compaction: CompactPrompts=true + intent.go:133 flag is\n" +
			"plumbed but 0 bytes stripped because the marker is absent.")
	}
}

func TestC416_002_IntentCompaction_SavesAtLeast500Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 500 {
		t.Errorf("RED: intent compaction saved only %d bytes (want ≥500).\n"+
			"Builder must place ## Reference Index (Layer 3, on-demand) before the reference-grade\n"+
			"sections (## Composition line 199, ## Reference line 205, ## Reflection Authoring line 214).\n"+
			"These sections total ~18 lines / ~990 bytes — 500 is a conservative floor.\n"+
			"body=%d stripped=%d", saved, len(body), len(stripped))
	}
}

func TestC416_003_IntentOperationalAnchors_AboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"IMKI",
		"STOP CRITERION",
		"challenged_premise",
		"Output contract",
		"INTENT_MODE",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required operational anchor %q lost below ## Reference Index — must remain above marker in evolve-intent.md", anchor)
		}
	}
}

func TestC416_004_IntentReferenceSection_AbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if strings.Contains(stripped, "## Composition") {
		t.Errorf("RED: reference section '## Composition' still appears above ## Reference Index.\n" +
			"Builder must place ## Reference Index (Layer 3, on-demand) ABOVE ## Composition (line 199)\n" +
			"so that compaction removes ## Composition, ## Reference, and ## Reflection Authoring\n" +
			"from the per-cycle prompt dispatch. These are reference-grade, not behavioral rules.")
	}
}

func TestC416_005_CompactionCoverageTestExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "prompts", "compaction_coverage_test.go")
	if !acsassert.FileExists(t, f) {
		t.Errorf("RED: go/internal/prompts/compaction_coverage_test.go does not exist.\n"+
			"Builder must ensure this file is written (TDD engineer contract from cycle-416).\n"+
			"File: %s", f)
	}
}

func TestC416_006_CompactionCoverageTest_CoversIntent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "prompts", "compaction_coverage_test.go")
	if !acsassert.FileContains(t, f, `"evolve-intent"`) {
		t.Errorf("RED: compaction_coverage_test.go does not contain \"evolve-intent\".\n"+
			"The coverage gate must enumerate all 7 per-cycle agents including evolve-intent.\n"+
			"File: %s", f)
	}
}

func TestC416_007_CompactionCoverageTest_CoversAllSevenAgents(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "prompts", "compaction_coverage_test.go")
	perCycleAgents := []string{
		"evolve-scout",
		"evolve-builder",
		"evolve-auditor",
		"evolve-orchestrator",
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-intent",
	}
	for _, name := range perCycleAgents {
		if !acsassert.FileContains(t, f, `"`+name+`"`) {
			t.Errorf("RED: compaction_coverage_test.go does not name %q in its per-cycle agent list.\n"+
				"The coverage gate must enumerate all 7 per-cycle agents to guard against silent\n"+
				"token re-inflation when a new agent ships without a ## Reference Index marker.\n"+
				"File: %s", name, f)
		}
	}
}

func TestC416_NEG_MarkerlessBody_CompactionIsNoOp(t *testing.T) {
	body := "# Agent\n\nOperational rules only.\n\n## Some Other Section\n\nContent.\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped != body {
		t.Errorf("markerless body modified by StripOnDemandSections — production code broken\n"+
			"body=%d stripped=%d diff=%d", len(body), len(stripped), len(body)-len(stripped))
	}
}

func bodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
