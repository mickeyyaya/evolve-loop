// composition_test.go — unit pins for the composition-verdict kernel
// checker (cycle-786). The end-to-end `evolve ledger verify` exit-code
// contract lives in cmd/evolve/cmd_ledger_composition_test.go; these tests
// name the exported API (apicover) and exercise Verify directly.
package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const compTestDiff = `diff --git a/fixture.txt b/fixture.txt
--- a/fixture.txt
+++ b/fixture.txt
@@ -1 +1,2 @@
 fixture line 1
+lane change
`

const compTestDriftedDiff = `diff --git a/fixture.txt b/fixture.txt
--- a/fixture.txt
+++ b/fixture.txt
@@ -1 +1,2 @@
 fixture line 1
+semantically different change
`

// TestPatchID_StableContentIdentity pins ledger.PatchID: equal diffs share a
// patch-id, semantically different diffs do not, and an empty diff errors
// rather than returning an empty (forgeable) identity.
func TestPatchID_StableContentIdentity(t *testing.T) {
	a, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatalf("PatchID(diff): %v", err)
	}
	b, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatalf("PatchID(diff) second call: %v", err)
	}
	if a != b {
		t.Fatalf("PatchID not stable: %s vs %s", a, b)
	}
	drifted, err := PatchID([]byte(compTestDriftedDiff))
	if err != nil {
		t.Fatalf("PatchID(drifted): %v", err)
	}
	if drifted == a {
		t.Fatalf("PatchID gave the same identity to semantically different diffs: %s", a)
	}
	if _, err := PatchID(nil); err == nil {
		t.Fatal("PatchID(empty) must error, not return an empty identity")
	}
}

// compositionLedger writes a ledger whose single entry is a
// CompositionVerdictKind line over two persisted diffs claiming
// claimedPatchID, and returns the FileLedger.
func compositionLedger(t *testing.T, auditedDiff, composedDiff, claimedPatchID string) *FileLedger {
	t.Helper()
	dir := t.TempDir()
	auditedPath := filepath.Join(dir, "audited.diff")
	composedPath := filepath.Join(dir, "composed.diff")
	if err := os.WriteFile(auditedPath, []byte(auditedDiff), 0o644); err != nil {
		t.Fatalf("write audited.diff: %v", err)
	}
	if err := os.WriteFile(composedPath, []byte(composedDiff), 0o644); err != nil {
		t.Fatalf("write composed.diff: %v", err)
	}
	line, err := json.Marshal(map[string]any{
		"ts":                 "2026-07-13T00:00:00Z",
		"cycle":              1,
		"kind":               CompositionVerdictKind,
		"method":             "trivial-rebase",
		"patch_id":           claimedPatchID,
		"audited_diff_path":  auditedPath,
		"composed_diff_path": composedPath,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	return New(dir)
}

// TestVerify_CompositionVerdictKind_KernelRecompute names
// ledger.CompositionVerdictKind and pins the Verify-side contract: an honest
// entry (recorded patch_id == recomputed patch-id of both diffs) verifies;
// a drifted composed diff and a forged patch_id both break the chain with
// core.ErrLedgerChainBroken.
func TestVerify_CompositionVerdictKind_KernelRecompute(t *testing.T) {
	honestID, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatalf("PatchID: %v", err)
	}

	if err := compositionLedger(t, compTestDiff, compTestDiff, honestID).Verify(context.Background()); err != nil {
		t.Fatalf("honest composition-verdict entry must verify: %v", err)
	}

	cases := []struct {
		name                      string
		auditedDiff, composedDiff string
		claimedPatchID            string
		wantMsg                   string
	}{
		{"drifted-composed-diff", compTestDiff, compTestDriftedDiff, honestID, "composed_diff_path"},
		{"forged-patch-id", compTestDiff, compTestDiff, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "audited_diff_path"},
		{"missing-patch-id", compTestDiff, compTestDiff, "", "no patch_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := compositionLedger(t, tc.auditedDiff, tc.composedDiff, tc.claimedPatchID).Verify(context.Background())
			if !errors.Is(err, core.ErrLedgerChainBroken) {
				t.Fatalf("tampered entry must break the chain with ErrLedgerChainBroken, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error should name the offending field (%q): %v", tc.wantMsg, err)
			}
		})
	}
}

// TestVerify_CompositionVerdict_MissingArtifactBreaksChain: deleting a
// persisted diff artifact defeats kernel recompute — tampering by removal
// must break verify, not silently pass.
func TestVerify_CompositionVerdict_MissingArtifactBreaksChain(t *testing.T) {
	honestID, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatalf("PatchID: %v", err)
	}
	l := compositionLedger(t, compTestDiff, compTestDiff, honestID)
	if err := os.Remove(filepath.Join(filepath.Dir(l.ledgerPath), "composed.diff")); err != nil {
		t.Fatalf("remove composed.diff: %v", err)
	}
	if err := l.Verify(context.Background()); !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Fatalf("missing diff artifact must break the chain, got: %v", err)
	}
}
