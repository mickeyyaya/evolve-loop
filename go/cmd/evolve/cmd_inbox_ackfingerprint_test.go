package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeInboxItemFixture(t *testing.T, path string, fields map[string]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("{\n")
	i := 0
	for k, v := range fields {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString(`  "` + k + `": ` + jsonQuote(v))
		i++
	}
	b.WriteString("\n}\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// jsonQuote is a minimal string literal quoter sufficient for the fixed
// ASCII fixture text used in these tests (no embedded newlines/quotes).
func jsonQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func TestRunInbox_AckFingerprint_WritesLedgerFromRealItem(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)
	itemPath := filepath.Join(projectRoot, ".evolve", "inbox", "consumed", "2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json")
	writeInboxItemFixture(t, itemPath, map[string]string{
		"id":          "pipeline-defect-pipeline-blocker",
		"consumed_by": "console-2026-08-05: fingerprint ship|unknown|76d0f4fca190 = root cause fixed",
	})

	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"ack-fingerprint", itemPath}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}

	raw, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("resolved-fingerprints.json not written: %v", err)
	}
	if !strings.Contains(string(raw), "ship|unknown|76d0f4fca190") {
		t.Fatalf("ledger must contain the acked fingerprint, got %s", raw)
	}
	if !strings.Contains(stdout.String()+stderr.String(), "acknowledged") {
		t.Errorf("operator-facing output must confirm the ack, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunInbox_AckFingerprint_FallsBackToNotesField(t *testing.T) {
	// Omits consumed_by to simulate an item consumed before a narrative
	// exists, exercising the notes fallback.
	projectRoot := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)
	itemPath := filepath.Join(projectRoot, ".evolve", "inbox", "consumed", "pipeline-defect-pipeline-blocker.json")
	writeInboxItemFixture(t, itemPath, map[string]string{
		"id":    "pipeline-defect-pipeline-blocker",
		"notes": `Auto-filed. Evidence: failure fingerprint "ship|unknown|76d0f4fca190" recurred 3x`,
	})

	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"ack-fingerprint", itemPath}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("resolved-fingerprints.json not written: %v", err)
	}
	if !strings.Contains(string(raw), "ship|unknown|76d0f4fca190") {
		t.Fatalf("ledger must contain the acked fingerprint via notes fallback, got %s", raw)
	}
}

func TestRunInbox_AckFingerprint_MissingItemReturnsNonZero(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)
	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"ack-fingerprint", filepath.Join(projectRoot, "does-not-exist.json")}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("missing item path must return a nonzero exit code")
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".evolve", "resolved-fingerprints.json")); err == nil {
		t.Fatalf("a failed ack must not write a ledger file")
	}
}

func TestRunInbox_AckFingerprint_NoFingerprintInItemReturnsNonZero(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", projectRoot)
	itemPath := filepath.Join(projectRoot, ".evolve", "inbox", "consumed", "no-fp.json")
	writeInboxItemFixture(t, itemPath, map[string]string{
		"id":          "some-other-item",
		"consumed_by": "closed as duplicate, no fingerprint recorded",
	})
	var stdout, stderr bytes.Buffer
	rc := runInbox([]string{"ack-fingerprint", itemPath}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("an item with no parseable fingerprint must return a nonzero exit code")
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".evolve", "resolved-fingerprints.json")); err == nil {
		t.Fatalf("a failed ack must not write a ledger file")
	}
}
