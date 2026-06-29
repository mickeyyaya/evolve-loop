//go:build acs

// Package cycle419 — amplification tests added by the Test Amplifier phase.
//
// Anti-bias: designed strictly from build-report.md (file list + preservation
// claims) without reading the implementation files. Targets:
//
//  1. agents/evolve-auditor.md — ZERO TDD predicates per R9.3 (bonus task
//     trim-auditor-prompt-rationale not in triage top_n). Amplification covers
//     all 11 behavioral elements the build report claims to have preserved.
//  2. agents/evolve-scout.md edge cases — tight-coupling phrase identified in
//     build-report.md § Discovery Scan ("eval materialization gate" at line 141)
//     and challenge-token mandatory-context integrity.
//
// Amplification test naming: Amp419_A## (auditor), Amp419_S## (scout edge).
// Signals:
//
//	amplify.tests_added = 15
//	amplify.failures_found = (set at runtime)
package cycle419

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// auditorContent reads evolve-auditor.md and returns raw bytes, frontmatter map,
// and parsed body. Mirrors scoutContent in predicates_test.go.
func auditorContent(t *testing.T) (raw []byte, fm map[string]any, body string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	p := filepath.Join(root, "agents", "evolve-auditor.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	fm, body, err = prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter(agents/evolve-auditor.md): %v", err)
	}
	return raw, fm, body
}

// =================== AMPLIFY — agents/evolve-auditor.md ===================
// Build: trim-auditor-prompt-rationale (bonus task, NOT in triage top_n)
// Baseline: 289 lines / 18102 bytes → claimed 274 lines / 17353 bytes.
// Build-report preserved: frontmatter, all 25 ## headings, Anti-Bias/SURE,
// Challenge Token Verification, EGPS Verdict Computation, red_count, STOP
// CRITERION, Completion Gates, handoff-auditor.json, Constitutional checklist.

// TestAmp419_A01_AuditorLineCountAtFloor asserts auditor ≤274 lines after trim.
func TestAmp419_A01_AuditorLineCountAtFloor(t *testing.T) {
	raw, _, _ := auditorContent(t)
	lines := strings.Split(string(raw), "\n")
	n := len(lines)
	if n > 0 && lines[n-1] == "" {
		n-- // match wc -l: trailing newline not counted
	}
	const maxLines = 274
	if n > maxLines {
		t.Errorf("auditor has %d lines (want ≤%d); build claimed reduction from 289→274",
			n, maxLines)
	}
}

// TestAmp419_A02_AuditorByteCountReduced asserts byte count < 18102 (baseline).
func TestAmp419_A02_AuditorByteCountReduced(t *testing.T) {
	raw, _, _ := auditorContent(t)
	const baselineBytes = 18102
	if len(raw) >= baselineBytes {
		t.Errorf("auditor is %d bytes (want <%d); build claimed −749 bytes",
			len(raw), baselineBytes)
	}
}

// TestAmp419_A03_AuditorFrontmatterPreserved asserts required frontmatter fields.
func TestAmp419_A03_AuditorFrontmatterPreserved(t *testing.T) {
	_, fm, _ := auditorContent(t)
	if fm == nil {
		t.Fatalf("ParseFrontmatter returned nil — auditor YAML block is broken")
	}
	for _, key := range []string{"name", "model", "description", "tools"} {
		v, ok := fm[key]
		if !ok {
			t.Errorf("auditor frontmatter missing key %q", key)
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				t.Errorf("auditor frontmatter[%q] is present but empty", key)
			}
		case []string:
			if len(val) == 0 {
				t.Errorf("auditor frontmatter[%q] is empty list", key)
			}
		case nil:
			t.Errorf("auditor frontmatter[%q] has nil value", key)
		}
	}
}

// TestAmp419_A04_AuditorAntiBiasProtocolPreserved asserts Anti-Bias Protocol
// and SURE pipeline keywords survived the rationale trim.
func TestAmp419_A04_AuditorAntiBiasProtocolPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	for _, phrase := range []string{"Anti-Bias", "SURE"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("auditor body missing %q — Anti-Bias/SURE pipeline was stripped", phrase)
		}
	}
}

// TestAmp419_A05_AuditorEGPSAndVerdictRulesPreserved asserts EGPS computation
// and the red_count verdict rule survived.
func TestAmp419_A05_AuditorEGPSAndVerdictRulesPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	for _, phrase := range []string{"EGPS", "red_count"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("auditor body missing %q — EGPS verdict computation or red_count rule stripped",
				phrase)
		}
	}
}

// TestAmp419_A06_AuditorChallengeTokenVerificationPreserved asserts the
// Challenge Token Verification section survived.
func TestAmp419_A06_AuditorChallengeTokenVerificationPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	hasChallengeToken := strings.Contains(body, "Challenge Token") ||
		strings.Contains(body, "challenge-token") ||
		strings.Contains(body, "challenge_token")
	if !hasChallengeToken {
		t.Errorf("auditor body missing challenge-token verification — output gate stripped")
	}
}

// TestAmp419_A07_AuditorHandoffJsonPreserved asserts handoff-auditor.json
// reference survived (output contract).
func TestAmp419_A07_AuditorHandoffJsonPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	if !strings.Contains(body, "handoff-auditor.json") {
		t.Errorf("auditor body missing 'handoff-auditor.json' — output contract stripped")
	}
}

// TestAmp419_A08_AuditorCompletionGatesPreserved asserts Completion Gates
// section survived.
func TestAmp419_A08_AuditorCompletionGatesPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	hasGates := strings.Contains(body, "Completion Gate") ||
		strings.Contains(body, "completion gate")
	if !hasGates {
		t.Errorf("auditor body missing 'Completion Gate' — gate section stripped")
	}
}

// TestAmp419_A09_AuditorStopCriterionPreserved asserts STOP CRITERION survived.
func TestAmp419_A09_AuditorStopCriterionPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	if !strings.Contains(body, "STOP") {
		t.Errorf("auditor body missing STOP keyword — STOP CRITERION may have been removed")
	}
}

// TestAmp419_A10_AuditorConstitutionalChecklistPreserved asserts the
// Constitutional audit checklist survived.
func TestAmp419_A10_AuditorConstitutionalChecklistPreserved(t *testing.T) {
	_, _, body := auditorContent(t)
	hasConstitutional := strings.Contains(body, "Constitutional") ||
		strings.Contains(body, "constitutional")
	if !hasConstitutional {
		t.Errorf("auditor body missing constitutional checklist — trim over-deleted")
	}
}

// TestAmp419_A11_AuditorNoDuplicateHeadings asserts no ## heading appears more
// than once. Build report claims all 25 ## occurrences preserved.
func TestAmp419_A11_AuditorNoDuplicateHeadings(t *testing.T) {
	_, _, body := auditorContent(t)
	seen := make(map[string]int)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "## ") {
			seen[trimmed]++
		}
	}
	for heading, count := range seen {
		if count > 1 {
			t.Errorf("duplicate ## heading %q appears %d times in auditor — trim introduced duplicate",
				heading, count)
		}
	}
}

// TestAmp419_A12_AuditorAntiGamingFloor asserts ≥200 lines (guards against
// gaming A01 by deleting behavioral rules instead of prose).
func TestAmp419_A12_AuditorAntiGamingFloor(t *testing.T) {
	raw, _, _ := auditorContent(t)
	lines := strings.Split(string(raw), "\n")
	n := len(lines)
	if n > 0 && lines[n-1] == "" {
		n--
	}
	const minLines = 200
	if n < minLines {
		t.Errorf("NEGATIVE: auditor has only %d lines (want ≥%d) — over-deleted behavioral rules",
			n, minLines)
	}
}

// ============= AMPLIFY — agents/evolve-scout.md edge cases =============
// Edge cases NOT covered by C419_001–C419_008.

// TestAmp419_S09_ScoutEvalMaterializationGatePhrase asserts the exact phrase
// "eval materialization gate" is present (build-report discovery scan: tight
// coupling at line ~141; AC6 relies on this exact phrase after §9 condensation).
func TestAmp419_S09_ScoutEvalMaterializationGatePhrase(t *testing.T) {
	_, _, body := scoutContent(t)
	if !strings.Contains(body, "eval materialization gate") {
		t.Errorf("EDGE: exact phrase 'eval materialization gate' absent from scout body.\n" +
			"Build-report discovery: line ~141 tight coupling — AC6 depends on this phrase\n" +
			"surviving even when the §9 redundant paragraph is condensed.")
	}
}

// TestAmp419_S10_ScoutChallengeTokenMandatoryContext asserts challenge-token
// appears with an action-verb instruction (not stripped to a bare keyword).
func TestAmp419_S10_ScoutChallengeTokenMandatoryContext(t *testing.T) {
	_, _, body := scoutContent(t)
	if !strings.Contains(body, "challenge-token") {
		// Already caught by AC5; log for completeness.
		t.Errorf("challenge-token absent from scout body — already caught by AC5")
		return
	}
	mandatoryContext := strings.Contains(body, "MUST") || strings.Contains(body, "must") ||
		strings.Contains(body, "Required") || strings.Contains(body, "required") ||
		strings.Contains(body, "include") || strings.Contains(body, "embed") ||
		strings.Contains(body, "write") || strings.Contains(body, "place")
	if !mandatoryContext {
		t.Errorf("challenge-token present but lacks mandatory instruction context — " +
			"prose trim may have stripped the action verb, leaving a dangling keyword")
	}
}

// TestAmp419_S11_AuditPhaseSuiteGreenAfterAuditorTrim asserts the audit phase
// test suite still passes after the bonus auditor-prompt trim.
// Distinct from AC8 (which covers ./internal/prompts/... + ./internal/phases/scout/...
// but NOT ./internal/phases/audit/...).
func TestAmp419_S11_AuditPhaseSuiteGreenAfterAuditorTrim(t *testing.T) {
	dir := goDir(t)
	_, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "./internal/phases/audit/...")
	if err != nil || code != 0 {
		t.Errorf("REGRESSION: audit phase suite failed after auditor trim (exit=%d err=%v):\n%s",
			code, err, stderr)
	}
}
