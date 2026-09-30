//go:build acs

package cycle412

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countSubstring(s, substr string) int {
	return strings.Count(s, substr)
}

func countLines(text string) int {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return len(lines) - 1
	}
	return len(lines)
}

func TestC412_001_NoLegacyScriptRefsInAnyPrompt(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		filepath.Join(root, "agents", "evolve-scout.md"),
		filepath.Join(root, "agents", "evolve-builder.md"),
		filepath.Join(root, "agents", "evolve-auditor.md"),
		filepath.Join(root, "agents", "evolve-tdd-engineer.md"),
		filepath.Join(root, "agents", "evolve-orchestrator.md"),
		filepath.Join(root, "agents", "evolve-triage.md"),
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		n := countSubstring(string(raw), "legacy/scripts")
		if n > 0 {
			t.Errorf("RED: %s still contains %d 'legacy/scripts' reference(s) — "+
				"all 18 dead refs must be removed and replaced with their in-process equivalents.\n"+
				"Each removed instruction must be replaced with the current Go orchestrator / "+
				"evolve CLI equivalent documented in its local v12.0.0 status block.",
				filepath.Base(f), n)
		}
		total += n
	}
	if total > 0 {
		t.Errorf("RED: total 'legacy/scripts' occurrences across all prompts: %d (must be 0)", total)
	}
}

func TestC412_002_CombinedWordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []struct {
		rel      string
		baseline int
	}{
		{"agents/evolve-scout.md", 1915},
		{"agents/evolve-builder.md", 2683},
		{"agents/evolve-auditor.md", 2263},
		{"agents/evolve-tdd-engineer.md", 2801},
		{"agents/evolve-orchestrator.md", 2358},
		{"agents/evolve-triage.md", 2106},
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, f.rel))
		if err != nil {
			t.Fatalf("cannot read %s: %v", f.rel, err)
		}
		total += countWords(string(raw))
	}
	const maxWords = 13900
	if total > maxWords {
		t.Errorf("RED: combined word count across 6 phase prompts = %d (Go baseline 14124) — "+
			"must be ≤ 13900 after removing dead legacy/scripts instructions and v12.0.0 disclaimers.\n"+
			"Expected reduction: ~300-500 words from ~20 dead-instruction occurrences + 5 disclaimer blocks.\n"+
			"Current count %d > 13900 — dead-ref removal not yet applied.",
			total, total)
	}
}

// acs-predicate: config-check
func TestC412_003_GateAnchorsPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)

	checks := []struct {
		file    string
		anchors []string
	}{
		{
			"agents/evolve-scout.md",
			[]string{"STOP CRITERION", "Gates (all six required)", "challenge-token"},
		},
		{
			"agents/evolve-builder.md",
			[]string{"STOP CRITERION", "AC-TABLE-BEGIN"},
		},
		{
			"agents/evolve-auditor.md",
			[]string{"STOP CRITERION", "EGPS Verdict Computation", "challenge-token"},
		},
		{
			"agents/evolve-tdd-engineer.md",
			[]string{"challenge-token"},
		},
		{
			"agents/evolve-orchestrator.md",
			[]string{"STOP CRITERION", "evolve guard phase"},
		},
		{
			"agents/evolve-triage.md",
			[]string{"challenge-token"},
		},
	}

	for _, c := range checks {
		path := filepath.Join(root, c.file)
		for _, anchor := range c.anchors {
			if !acsassert.FileContains(t, path, anchor) {
				t.Errorf("gate anchor %q was removed from %s — "+
					"legacy/scripts cleanup must not delete gate-critical instructions",
					anchor, c.file)
			}
		}
	}
}

func TestC412_004_NoFileGutted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		"agents/evolve-scout.md",
		"agents/evolve-builder.md",
		"agents/evolve-auditor.md",
		"agents/evolve-tdd-engineer.md",
		"agents/evolve-orchestrator.md",
		"agents/evolve-triage.md",
	}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("cannot read %s: %v", rel, err)
		}
		n := countLines(string(raw))
		const minLines = 100
		if n < minLines {
			t.Errorf("%s has only %d lines — floor is %d. "+
				"Dead-ref removal must not gut entire sections.",
				rel, n, minLines)
		}
	}
}

func TestC412_005_NoV12StatusDisclaimers(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		filepath.Join(root, "agents", "evolve-scout.md"),
		filepath.Join(root, "agents", "evolve-builder.md"),
		filepath.Join(root, "agents", "evolve-auditor.md"),
		filepath.Join(root, "agents", "evolve-tdd-engineer.md"),
		filepath.Join(root, "agents", "evolve-orchestrator.md"),
		filepath.Join(root, "agents", "evolve-triage.md"),
	}
	total := 0
	for _, f := range files {
		if !acsassert.FileNotContains(t, f, "v12.0.0 status") {
			n := countSubstring(func() string {
				raw, _ := os.ReadFile(f)
				return string(raw)
			}(), "v12.0.0 status")
			t.Errorf("RED: %s still contains %d 'v12.0.0 status' disclaimer(s) — "+
				"these cover-disclaimers must be deleted once their legacy/scripts triggers are removed.",
				filepath.Base(f), n)
			total += n
		}
	}
	if total > 0 {
		t.Errorf("RED: total 'v12.0.0 status' occurrences: %d (must be 0 after cleanup)", total)
	}
}

func countLinesContaining(text, substr string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, substr) {
			n++
		}
	}
	return n
}

func TestC412_006_ReferencePathLineCountAtMostTwo(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		refPath string
		current int
	}{
		{"agents/evolve-scout.md", "agents/evolve-scout-reference.md", 9},
		{"agents/evolve-builder.md", "agents/evolve-builder-reference.md", 12},
		{"agents/evolve-auditor.md", "agents/evolve-auditor-reference.md", 9},
	}
	for _, c := range checks {
		raw, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatalf("cannot read %s: %v", c.file, err)
		}
		n := countLinesContaining(string(raw), c.refPath)
		if n > 2 {
			t.Errorf("RED: %s has %q on %d lines (currently %d; must be ≤2 lines).\n"+
				"Collapse: declare the base path once as a section header; replace all other "+
				"inline occurrences with just the section name.",
				c.file, c.refPath, n, c.current)
		}
	}
}

// acs-predicate: config-check
func TestC412_007_AllScoutReferenceSectionsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-scout.md")
	sections := []string{
		"turn-budget-rationale",
		"mode-discovery-detail",
		"eval-integrity-rules",
		"eval-format-template",
		"output-template",
		"task-selection-tables",
		"project-digest-template",
	}
	for _, section := range sections {
		if !acsassert.FileContains(t, path, section) {
			t.Errorf("scout reference section %q was removed from agents/evolve-scout.md — "+
				"path deduplication must preserve every section name; only the repeated full-path "+
				"prefix on each row should be collapsed",
				section)
		}
	}
}

func TestC412_008_ReferencePointerNotAmputated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		refPath string
	}{
		{"agents/evolve-scout.md", "agents/evolve-scout-reference.md"},
		{"agents/evolve-builder.md", "agents/evolve-builder-reference.md"},
		{"agents/evolve-auditor.md", "agents/evolve-auditor-reference.md"},
	}
	for _, c := range checks {
		raw, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatalf("cannot read %s: %v", c.file, err)
		}
		n := countLinesContaining(string(raw), c.refPath)
		if n < 1 {
			t.Errorf("%s has 0 lines with %q — the reference file pointer was amputated.\n"+
				"Deduplication must keep ≥1 discoverable pointer (the base-path declaration row).",
				c.file, c.refPath)
		}
	}
}

func TestC412_009_ScoutBuilderAuditorWordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []struct {
		rel      string
		baseline int
	}{
		{"agents/evolve-scout.md", 1915},
		{"agents/evolve-builder.md", 2683},
		{"agents/evolve-auditor.md", 2263},
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, f.rel))
		if err != nil {
			t.Fatalf("cannot read %s: %v", f.rel, err)
		}
		total += countWords(string(raw))
	}
	const maxWords = 6860
	if total > maxWords {
		t.Errorf("RED: combined word count for scout+builder+auditor = %d (baseline 6861) — "+
			"must be < 6861 after collapsing Reference Index path repetition.\n"+
			"Current count %d == baseline — no Reference Index deduplication has been applied yet.",
			total, total)
	}
}
