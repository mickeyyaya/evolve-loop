//go:build acs

package cycle415

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC415_001_TddEngineerStripsAtLeast1500Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 64 {
		t.Errorf("tdd-engineer compaction saved only %d bytes (want ≥64: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

func TestC415_002_TddEngineerBehaviorAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"RED phase is proof of understanding",
		"Do NOT implement production code",
		"15-turn boundary",
		"challenge-token",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required anchor %q lost below ## Reference Index — must remain above marker in evolve-tdd-engineer.md", anchor)
		}
	}
}

func TestC415_003_TddEngineerBuriedRuleNegative(t *testing.T) {
	body := "Preamble content.\n\n## Reference Index\n\nDo NOT implement production code\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading")
	}
	if strings.Contains(stripped, "Do NOT implement production code") {
		t.Error("synthetic: rule buried below ## Reference Index survived strip — StripOnDemandSections broken")
	}
}

func TestC415_004_TriageStripsAtLeast1200Bytes(t *testing.T) {
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
	if saved < 64 {
		t.Errorf("triage compaction saved only %d bytes (want ≥64: the marker tail); ## Reference Index heading missing? (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

func TestC415_005_TriageRequiredSectionsAboveMarker(t *testing.T) {
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
	for _, section := range []string{
		"## top_n",
		"## deferred",
		"## dropped",
		"carryoverTodos",
		"## Rationale",
		"Operator-queue priority floor",
	} {
		if !strings.Contains(stripped, section) {
			t.Errorf("required section/rule %q lost below ## Reference Index — must stay above marker in evolve-triage.md", section)
		}
	}
}

func TestC415_006_TriageOperationalSectionsSurviveStrip(t *testing.T) {
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
	for _, kept := range []string{
		"Idempotency skip-list (v9.6.0+)",
		"Inbox ingestion (v9.5.0+)",
		"Reflection Authoring (v10.20.0+)",
	} {
		if !strings.Contains(stripped, kept) {
			t.Errorf("operational section %q lost below ## Reference Index — it must survive into dispatched triage prompts", kept)
		}
	}
}

func TestC415_007_AllAlwaysOnDocsHaveCompactMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	alwaysOn := []string{
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-orchestrator",
		"evolve-auditor",
		"evolve-builder",
		"evolve-scout",
	}
	for _, name := range alwaysOn {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "agents", name+".md"))
			if err != nil {
				t.Fatalf("read %s.md: %v", name, err)
			}
			_, body, err := prompts.ParseFrontmatter(string(data))
			if err != nil {
				t.Fatalf("parse frontmatter: %v", err)
			}
			if !bodyHasCompactMarker(body) {
				t.Errorf("RED: %s.md has no line-anchored ## Reference Index heading; add it so StripOnDemandSections compacts it every cycle", name)
			}
		})
	}
}

func TestC415_008_InlineMentionNotCountedAsMarker(t *testing.T) {
	body := "See ## Reference Index below for details.\nMore content.\n"
	if bodyHasCompactMarker(body) {
		t.Error("inline prose mention of ## Reference Index counted as compact marker — gate must be line-anchored")
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
