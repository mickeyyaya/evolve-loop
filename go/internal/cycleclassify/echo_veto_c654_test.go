package cycleclassify

// RED regression tests for cycle-654 top_n task `infra-classifier-echo-veto`
// (the fix-of-record for lesson cycle-641-infra-incident-classifier-matches-
// echoed-prompt-keywords, byte-for-byte recurred in cycle-642). These encode the
// SOURCE-OF-TRUTH gate on cycle discard (retro recommendation #2): when a phase's
// driver exited 0 AND its deliverable sentinel is PASS AND the only infra_failure
// event's excerpt is a verbatim echo of the injected prompt text, cycleclassify
// MUST NOT return ClassInfrastructure. The paired negative test proves the fix is
// not a blanket disable of infra detection: a genuine runtime signal (non-zero
// exit, excerpt NOT in the prompt) MUST still classify as infrastructure.
//
// Both exercise the real system under test — cycleclassify.Classify over an
// on-disk cycle-642-shape workspace — and assert on its returned Classification.
// Neither is a source-grep. TestC654_001 is RED today (Classify's Pass 2
// scanEventsForInfra returns ClassInfrastructure on the keyword-only event,
// ignoring the PASS deliverable + exit-0 + prompt-echo). TestC654_002 is a
// GREEN-now regression guard (genuine infra already vetoes) that the fix must
// keep green.
//
// Ported verbatim (renamed C653→C654) from the fix-of-record RED suite preserved
// in .evolve/worktrees/cycle-21f9f7ae-653; do not author duplicate C653 copies.

import (
	"os"
	"path/filepath"
	"testing"
)

// echoedReviewerLine is verbatim adversarial-review-prompt.txt:46 — the
// Reviewer's own exploit checklist the tmux driver echoes into the pane/stderr,
// which the normalizer keyword-matched as marker:"rate_limit" in cycle-641/642.
const echoedReviewerLine = "unbounded allocation or recursion; TOCTOU / race windows; missing rate limits."

// passSentinel is a canonical v1 verdict sentinel for a PASS deliverable.
func passSentinel(phase string) string {
	return "<!-- evolve-verdict: {\"phase\":\"" + phase + "\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
}

// infraEventLine is one normalizer infra_failure INCIDENT envelope (the shape
// phasestream.infraEnvelope emits: kind + severity + data{marker,source,excerpt}).
func infraEventLine(marker, excerpt string) string {
	return "{\"schema_version\":\"1\",\"seq\":56,\"source\":{\"producer\":\"normalizer\",\"phase\":\"adversarial-review\"}," +
		"\"kind\":\"infra_failure\",\"severity\":\"INCIDENT\",\"data\":{\"marker\":\"" + marker +
		"\",\"source\":\"stderr\",\"excerpt\":\"" + excerpt + "\"}}\n"
}

// llmCallLine is one llm-calls.ndjson record carrying the phase's driver exit code.
func llmCallLine(phase string, exitCode int) string {
	return "{\"ts\":\"2026-07-10T10:00:00Z\",\"agent\":\"adversarial-review\",\"phase\":\"" + phase +
		"\",\"cli\":\"claude-tmux\",\"model\":\"deep\",\"attempt\":1,\"source\":\"result\"," +
		"\"duration_ms\":1000,\"exit_code\":" + itoa(exitCode) + "}\n"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return ws
}

// TestC654_001_PassDeliverableExit0EchoNotInfraVeto — AC1 (source-of-truth gate).
// A cycle-642-shape workspace: adversarial-review driver exited 0, its deliverable
// carries a PASS sentinel, and the only infra_failure event's excerpt is a verbatim
// echo of the injected prompt. Classify MUST NOT veto this to infrastructure.
// RED today: scanEventsForInfra returns ClassInfrastructure on the keyword alone.
func TestC654_001_PassDeliverableExit0EchoNotInfraVeto(t *testing.T) {
	ws := writeFixture(t, map[string]string{
		"adversarial-review-report.md":     "# Adversarial Review\n\nseverity_max=LOW exploit_count=0\n" + passSentinel("adversarial-review"),
		"adversarial-review-prompt.txt":    "Hunt for: " + echoedReviewerLine + "\n",
		"adversarial-review-events.ndjson": infraEventLine("rate_limit", echoedReviewerLine),
		"llm-calls.ndjson":                 llmCallLine("adversarial-review", 0),
	})
	got := Classify(ws).Class
	if got == ClassInfrastructure {
		t.Errorf("Classify vetoed a PASS deliverable / exit-0 phase to %q on a prompt-echo infra_failure; "+
			"deliverable-PASS + driver-exit-0 must be source-of-truth (cycle-641/642 lesson)", got)
	}
}

// TestC654_002_GenuineInfraStillVetoes — AC2 (negative axis / anti-no-op guard).
// A genuine runtime infra signal (driver exited non-zero; the infra_failure
// excerpt is a real provider error line, NOT a substring of the prompt) MUST
// still classify as infrastructure. GREEN today; the AC1 fix must keep it green
// (proving the guard is scoped to prompt-echo + clean-exit, not a blanket disable).
func TestC654_002_GenuineInfraStillVetoes(t *testing.T) {
	const genuine = "api error: 429 Too Many Requests from provider (retry-after 60)"
	ws := writeFixture(t, map[string]string{
		"adversarial-review-report.md":     "# Adversarial Review\n\nseverity_max=LOW exploit_count=0\n" + passSentinel("adversarial-review"),
		"adversarial-review-prompt.txt":    "Review the change for security defects.\n",
		"adversarial-review-events.ndjson": infraEventLine("api_429", genuine),
		"llm-calls.ndjson":                 llmCallLine("adversarial-review", 85),
	})
	if got := Classify(ws).Class; got != ClassInfrastructure {
		t.Errorf("genuine runtime infra (exit 85, non-echo 429 error) was not classified infrastructure: got %q", got)
	}
}
