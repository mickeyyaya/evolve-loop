//go:build acs

// Package cycle419 materializes the cycle-419 acceptance criteria for the
// committed top_n task:
//
//   - trim-scout-prompt-redundancy (T1) — collapse the run-on §9 eval-
//     materialization paragraph (restates gate #6 evals-materialized) and the
//     multi-cycle war-story in the challenge-token block, without removing any
//     required output section, gate, or frontmatter field.
//
// Deferred (zero predicates per R9.3): trim-auditor-prompt-rationale is NOT in
// the triage top_n (triage-report.md lists only trim-scout-prompt-redundancy).
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	trim-scout-prompt-redundancy (T1):
//	  AC1  agents/evolve-scout.md ≤ 204 lines (was 219)                  → C419_001 (RED)
//	  AC2  byte count strictly < 14941 (baseline 14941)                   → C419_002 (RED)
//	  AC3  frontmatter fields preserved (name/model/description/tools)    → C419_003 (pre-existing GREEN)
//	  AC4  all 11 required output-section names still instructed           → C419_004 (pre-existing GREEN)
//	  AC5  all 6 gates + challenge-token rule present                      → C419_005 (pre-existing GREEN)
//	  AC6  anti-gaming floor ≥150 lines (content not gutted)              → C419_006 (pre-existing GREEN)
//	  AC7  no duplicate ## headings in body                                → C419_007 (pre-existing GREEN)
//	  AC8  go test ./internal/prompts/... ./internal/phases/scout/... green → C419_008 (pre-existing GREEN)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C419_006 — anti-gaming floor (catches over-deletion that games AC1);
//	          "eval materialization" mandate must survive even when §9 prose is compacted.
//	Edge/OOD: C419_002 — boundary: 14941 bytes is the exact baseline; 14941 ≥ 14941 → fails.
//	Semantic:  8 distinct dimensions: line-count / byte-count / frontmatter-fields /
//	           output-section-names / gate-presence / floor-lines / heading-dedup / test-suite.
//
// 1:1 enforcement:
//
//	T1: predicate=8 (C419_001–C419_008), manual+checklist=0, unverifiable-remove=0 → total AC=8 ✓
//	T2 (not top_n): predicate=0 per R9.3 ✓
package cycle419

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// scoutContent reads evolve-scout.md and returns raw bytes, frontmatter map,
// and parsed body. Calls prompts.ParseFrontmatter — the production SSOT.
func scoutContent(t *testing.T) (raw []byte, fm map[string]any, body string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	p := filepath.Join(root, "agents", "evolve-scout.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	fm, body, err = prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter(agents/evolve-scout.md): %v", err)
	}
	return raw, fm, body
}

// goDir returns <repoRoot>/go — the module root for subprocess invocations.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// ===================== T1 — trim-scout-prompt-redundancy =====================

// TestC419_001_ScoutLineCountAtFloor asserts agents/evolve-scout.md has ≤204
// lines after the prose trim.
// BEHAVIORAL: reads raw bytes, counts newline-separated lines.
//
// RED baseline: 219 lines > 204 → fails until Builder removes ≥15 lines by
// collapsing the §9 run-on paragraph and challenge-token multi-cycle war-story.
func TestC419_001_ScoutLineCountAtFloor(t *testing.T) {
	raw, _, _ := scoutContent(t)
	lines := strings.Split(string(raw), "\n")
	n := len(lines)
	if n > 0 && lines[n-1] == "" {
		n-- // match wc -l behavior: trailing newline does not count
	}
	const maxLines = 204
	if n > maxLines {
		t.Errorf("RED: agents/evolve-scout.md has %d lines (want ≤%d).\n"+
			"Builder must remove ≥%d lines by collapsing the §9 eval-materialization\n"+
			"run-on paragraph and the challenge-token multi-cycle war-story. (current: %d → target: ≤%d)",
			n, maxLines, n-maxLines, n, maxLines)
	}
}

// TestC419_002_ScoutByteCountAtFloor asserts agents/evolve-scout.md is strictly
// less than 14941 bytes after the trim.
// BEHAVIORAL: reads raw bytes and counts.
//
// RED baseline: file is exactly 14941 bytes; 14941 < 14941 is false → fails.
// Edge: the boundary is exact — even a one-byte reduction satisfies this.
func TestC419_002_ScoutByteCountAtFloor(t *testing.T) {
	raw, _, _ := scoutContent(t)
	const maxBytes = 14941
	if len(raw) >= maxBytes {
		t.Errorf("RED: agents/evolve-scout.md is %d bytes (want <%d).\n"+
			"Builder must shrink below the 14941-byte baseline.\n"+
			"Current: %d bytes (need to remove ≥1 byte of redundant prose).",
			len(raw), maxBytes, len(raw))
	}
}

// TestC419_003_ScoutFrontmatterPreserved asserts that ParseFrontmatter returns
// a non-nil map containing the required identity and behavior-granting fields.
// BEHAVIORAL: calls prompts.ParseFrontmatter and inspects the returned map.
//
// Pre-existing GREEN: frontmatter intact before Builder runs.
// Regression guard: ensures the trim never corrupts the YAML fence or fields.
func TestC419_003_ScoutFrontmatterPreserved(t *testing.T) {
	_, fm, _ := scoutContent(t)
	if fm == nil {
		t.Fatalf("ParseFrontmatter returned nil map — YAML frontmatter fence is broken")
	}
	for _, key := range []string{"name", "model", "description", "tools"} {
		v, ok := fm[key]
		if !ok {
			t.Errorf("frontmatter missing key %q — trim corrupted the YAML block", key)
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				t.Errorf("frontmatter[%q] is present but empty", key)
			}
		case []string:
			if len(val) == 0 {
				t.Errorf("frontmatter[%q] is an empty list", key)
			}
		case nil:
			t.Errorf("frontmatter[%q] has nil value", key)
		}
	}
}

// TestC419_004_ScoutOutputSectionsPreserved asserts that all eleven required
// output-section names are still present in the parsed body.
// BEHAVIORAL: calls prompts.ParseFrontmatter and searches the returned body for
// each section name. If any is absent the trim broke the output contract.
//
// Pre-existing GREEN: all 11 sections present before Builder runs.
// Regression guard: catches a trim that accidentally deletes a section directive.
func TestC419_004_ScoutOutputSectionsPreserved(t *testing.T) {
	_, _, body := scoutContent(t)
	for _, section := range []string{
		"Discovery Summary",
		"Key Findings",
		"Research",
		"Research → Implementation Map",
		"Hypotheses",
		"Beyond-the-Ask Hypotheses",
		"Selected Tasks",
		"Acceptance Criteria Summary",
		"Carryover Decisions",
		"Deferred",
		"Decision Trace",
	} {
		if !strings.Contains(body, section) {
			t.Errorf("required output-section name %q absent from parsed body — trim over-deleted", section)
		}
	}
}

// TestC419_005_ScoutGatesAndTokenPreserved asserts that all six STOP-CRITERION
// gates and the challenge-token requirement are still present in the parsed body.
// BEHAVIORAL: calls prompts.ParseFrontmatter and checks the returned body.
//
// Pre-existing GREEN: gates + token present before Builder runs.
// Regression guard: trim must not delete gate table rows or the token mandate.
func TestC419_005_ScoutGatesAndTokenPreserved(t *testing.T) {
	_, _, body := scoutContent(t)
	for _, gate := range []string{
		"system-health-complete",
		"inbox-audit-complete",
		"backlog-complete",
		"build-plan-written",
		"research-cache-section",
		"evals-materialized",
	} {
		if !strings.Contains(body, gate) {
			t.Errorf("STOP-CRITERION gate %q absent from parsed body — trim must not delete gate rows", gate)
		}
	}
	if !strings.Contains(body, "challenge-token") {
		t.Errorf("challenge-token requirement absent from parsed body — trim must preserve the challenge-token mandate")
	}
}

// TestC419_006_ScoutAntiGamingFloor_Negative asserts that agents/evolve-scout.md
// has ≥150 lines (guards against gaming AC1 by deleting behavioral rules).
// BEHAVIORAL: line count arithmetic + body presence check.
//
// Pre-existing GREEN: 219 lines ≥ 150 before Builder runs.
// Negative sentinel: if Builder deletes rules rather than dedup prose → fails here.
func TestC419_006_ScoutAntiGamingFloor_Negative(t *testing.T) {
	raw, _, body := scoutContent(t)
	lines := strings.Split(string(raw), "\n")
	n := len(lines)
	if n > 0 && lines[n-1] == "" {
		n--
	}
	const minLines = 150
	if n < minLines {
		t.Errorf("NEGATIVE: agents/evolve-scout.md has only %d lines (want ≥%d).\n"+
			"Builder over-deleted — trim dedup prose only, not behavioral rules.", n, minLines)
	}
	// The §9 eval-materialization mandate must survive compaction of its repetition.
	if !strings.Contains(body, "eval materialization") {
		t.Errorf("NEGATIVE: 'eval materialization' absent from body — Builder stripped the §9 mandate\n" +
			"(must only collapse the redundant repetition that duplicates gate #6, not the mandate itself)")
	}
}

// TestC419_007_ScoutNoDuplicateHeadings asserts that no ## heading appears more
// than once in the parsed body.
// BEHAVIORAL: parses headings from body, detects duplicates algorithmically.
//
// Pre-existing GREEN: no duplicate ## headings in current file.
// Regression guard: trim that collapses sections must not introduce a duplicate.
func TestC419_007_ScoutNoDuplicateHeadings(t *testing.T) {
	_, _, body := scoutContent(t)
	seen := make(map[string]int)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "## ") {
			seen[trimmed]++
		}
	}
	for heading, count := range seen {
		if count > 1 {
			t.Errorf("duplicate ## heading %q appears %d times — trim must not introduce duplicate headings", heading, count)
		}
	}
}

// TestC419_008_ScoutPromptsAndPhaseSuiteGreen asserts that the prompts and
// scout phase test suites remain green after the trim.
// BEHAVIORAL: runs real `go test` subprocess and asserts exit code 0.
//
// Pre-existing GREEN: both suites currently pass.
// Regression guard: file corruption or frontmatter malformation caught here.
func TestC419_008_ScoutPromptsAndPhaseSuiteGreen(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"./internal/prompts/...", "./internal/phases/scout/...")
	if err != nil || code != 0 {
		t.Errorf("RED/REGRESSION: prompts+scout test suite failed (exit=%d, err=%v):\n%s",
			code, err, stderr)
	}
}
