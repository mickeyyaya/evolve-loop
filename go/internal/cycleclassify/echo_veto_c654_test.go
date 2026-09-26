package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

// echoedReviewerLine is a verbatim adversarial-review prompt line that the normalizer keyword-matches as rate_limit.
const echoedReviewerLine = "unbounded allocation or recursion; TOCTOU / race windows; missing rate limits."

func passSentinel(phase string) string {
	return "<!-- evolve-verdict: {\"phase\":\"" + phase + "\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n"
}

// infraEventLine hand-writes the envelope shape phasestream.infraEnvelope emits.
func infraEventLine(marker, excerpt string) string {
	return "{\"schema_version\":\"1\",\"seq\":56,\"source\":{\"producer\":\"normalizer\",\"phase\":\"adversarial-review\"}," +
		"\"kind\":\"infra_failure\",\"severity\":\"INCIDENT\",\"data\":{\"marker\":\"" + marker +
		"\",\"source\":\"stderr\",\"excerpt\":\"" + excerpt + "\"}}\n"
}

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
