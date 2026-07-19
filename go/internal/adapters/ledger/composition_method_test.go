// composition_method_test.go — RED contract for the merge ladder RUNG 2
// `method` field (cycle-941, merge-rung2-scoped-review-core; knowledge-base/
// research/merge-concurrency-2026).
//
// RUNG 0 records every composition-verdict line with method:"trivial-rebase"
// unconditionally (composition.go). RUNG 2 composes after a scoped merge review
// resolves an overlapping change; its verdict must be distinguishable —
// method:"scoped-review" — so the ship-side reader and any audit can tell a
// trivial-rebase carry-forward from a reviewed composition. This adds a
// caller-supplied `Method` field to CompositionVerdictInput, defaulting to
// TrivialRebaseMethod when blank so existing RUNG 0 callers are byte-for-byte
// unchanged.
//
// RED at authoring: ScopedReviewMethod and CompositionVerdictInput.Method are
// undefined (compile failure). Builder contract: add the const + field and
// thread Method into the persisted line (defaulting empty → TrivialRebaseMethod).
// DO NOT modify this file. Reuses honestWriteInput/passingComposedGates from
// composition_write_test.go (same package).
package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readbackCompositionMethod reads the single appended composition-verdict line
// and returns its persisted `method` field.
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

// A5a: an explicit Method:"scoped-review" persists as method:"scoped-review",
// and the exported constant carries that exact string (writer and reader share
// one SSOT, mirroring TrivialRebaseMethod).
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

// A5b (no rung-0 regression): a caller that leaves Method blank persists the
// unchanged RUNG 0 default, method:"trivial-rebase".
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
