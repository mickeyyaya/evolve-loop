package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readbackCompositionMethod(t *testing.T, ledgerPath string) string {
	t.Helper()
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read back ledger %s: %v", ledgerPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want exactly 1 appended line, got %d:\n%s", len(lines), raw)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("appended line is not one JSON object: %v\n%s", err, lines[0])
	}
	m, _ := got["method"].(string)
	return m
}

func TestWriteCompositionVerdict_MethodScopedReview(t *testing.T) {
	if ScopedReviewMethod != "scoped-review" {
		t.Errorf("ScopedReviewMethod = %q, want %q", ScopedReviewMethod, "scoped-review")
	}

	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	in := honestWriteInput(t, dir)
	in.Method = ScopedReviewMethod

	if err := WriteCompositionVerdict(ledgerPath, in); err != nil {
		t.Fatalf("WriteCompositionVerdict(scoped-review): %v", err)
	}
	if got := readbackCompositionMethod(t, ledgerPath); got != "scoped-review" {
		t.Errorf("persisted method = %q, want %q", got, "scoped-review")
	}
}

func TestWriteCompositionVerdict_MethodDefaultsTrivialRebase(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	in := honestWriteInput(t, dir) // Method left zero-value

	if err := WriteCompositionVerdict(ledgerPath, in); err != nil {
		t.Fatalf("WriteCompositionVerdict(default method): %v", err)
	}
	if got := readbackCompositionMethod(t, ledgerPath); got != TrivialRebaseMethod {
		t.Errorf("blank Method persisted as %q, want default %q (rung-0 regression)", got, TrivialRebaseMethod)
	}
}
