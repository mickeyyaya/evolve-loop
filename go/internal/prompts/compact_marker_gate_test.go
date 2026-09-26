package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTddEngineerCompaction(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 64 {
		t.Errorf("tdd-engineer compaction saved only %d bytes (want ≥64: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
	for _, anchor := range []string{
		"RED phase is proof of understanding",
		"Do NOT implement production code",
		"15-turn boundary",
		"challenge-token",
		"MUST exercise the system under test",
		"degenerate-predicate failure mode",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required anchor %q lost below ## Reference Index — must stay above marker in evolve-tdd-engineer.md", anchor)
		}
	}
}

func TestTddEngineerCompaction_BuriedRuleNegative(t *testing.T) {
	body := "Preamble content.\n\n## Reference Index\n\nDo NOT implement production code\n"
	stripped := StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading")
	}
	if strings.Contains(stripped, "Do NOT implement production code") {
		t.Error("synthetic: rule buried below ## Reference Index survived strip — StripOnDemandSections broken")
	}
}

func TestTriageCompaction(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 64 {
		t.Errorf("triage compaction saved only %d bytes (want ≥64: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
	for _, section := range []string{
		"## top_n",
		"## deferred",
		"## dropped",
		"carryoverTodos",
		"## Rationale",
		"Operator-queue priority floor",
		"Idempotency skip-list",
		"Inbox ingestion",
	} {
		if !strings.Contains(stripped, section) {
			t.Errorf("required section/rule %q lost below ## Reference Index — must stay above marker in evolve-triage.md", section)
		}
	}
}

func TestAlwaysOnPhaseDocsHaveCompactMarker(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-orchestrator",
		"evolve-auditor",
		"evolve-builder",
		"evolve-scout",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, "agents", name+".md"))
			if err != nil {
				t.Fatalf("read %s.md: %v", name, err)
			}
			_, body, err := ParseFrontmatter(string(raw))
			if err != nil {
				t.Fatalf("parse frontmatter: %v", err)
			}
			if !bodyHasCompactMarker(body) {
				t.Errorf("%s.md has no line-anchored ## Reference Index heading — add it so StripOnDemandSections compacts it on every cycle dispatch", name)
			}
		})
	}
}

func TestAlwaysOnPhaseDocsHaveCompactMarker_InlineMentionRejected(t *testing.T) {
	body := "See ## Reference Index below for details.\nMore content.\n"
	if bodyHasCompactMarker(body) {
		t.Error("inline prose mention of ## Reference Index counted as compact marker — gate must be line-anchored")
	}
}

// bodyHasCompactMarker must agree with StripOnDemandSections on what counts as the heading.
func bodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
