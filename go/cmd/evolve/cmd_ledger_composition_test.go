// cmd_ledger_composition_test.go — cycle-786 TDD contract, AC3:
// "TestCompositionVerdict_KernelRecomputesPatchId (tampered entry rejected);
// ledger verify covers new entry kind" (inbox
// merge-rung0-trivial-rebase-carryforward).
//
// Contract: `evolve ledger verify` must, for every kind="composition-verdict"
// entry, kernel-recompute the patch-id of BOTH persisted diff artifacts
// (audited_diff_path, composed_diff_path — via `git patch-id --stable`) and
// require each to equal the entry's recorded patch_id. A forged match (the
// entry claims a patch_id its own recorded diffs do not hash to) is tampering
// and must break verify with exit 2, exactly like a chain break. Deterministic,
// zero LLM tokens — re-derivable by anyone from the entry alone.
//
// RED status at authoring (cycle 786): the tampered subtests FAIL — today
// Verify only walks the hash chain and ignores unknown entry kinds, so a
// tampered composition-verdict entry exits 0. The valid-entry test is a
// pre-existing GREEN guard pinning that the checker never over-rejects an
// honest entry.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// compAuditedDiff / compDriftedDiff are two valid unified diffs with DIFFERENT
// patch-ids (the second changes the added line's content).
const compAuditedDiff = `diff --git a/fixture.txt b/fixture.txt
--- a/fixture.txt
+++ b/fixture.txt
@@ -1 +1,2 @@
 fixture line 1
+lane change
`

const compDriftedDiff = `diff --git a/fixture.txt b/fixture.txt
--- a/fixture.txt
+++ b/fixture.txt
@@ -1 +1,2 @@
 fixture line 1
+semantically different change
`

// compPatchID pipes a diff through `git patch-id --stable`.
func compPatchID(t *testing.T, dir, diff string) string {
	t.Helper()
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git patch-id --stable: %v", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		t.Fatalf("git patch-id produced no output for diff:\n%s", diff)
	}
	return fields[0]
}

// compFixture writes a .evolve dir whose ledger holds one composition-verdict
// entry over the two given diff artifacts, claiming claimedPatchID. Entries are
// written pre-v8.37 style (no prev_hash key) so the hash-chain walk is
// trivially intact and any verify failure is attributable to the
// composition-verdict checker alone.
func compFixture(t *testing.T, auditedDiff, composedDiff, claimedPatchID string) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	runDir := filepath.Join(evolveDir, "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", runDir, err)
	}
	auditedPath := filepath.Join(runDir, "audited.diff")
	composedPath := filepath.Join(runDir, "composed.diff")
	if err := os.WriteFile(auditedPath, []byte(auditedDiff), 0o644); err != nil {
		t.Fatalf("write audited.diff: %v", err)
	}
	if err := os.WriteFile(composedPath, []byte(composedDiff), 0o644); err != nil {
		t.Fatalf("write composed.diff: %v", err)
	}
	entry := map[string]any{
		"ts":                 "2026-07-13T16:00:00Z",
		"cycle":              1,
		"kind":               "composition-verdict",
		"method":             "trivial-rebase",
		"lane_audit_ref":     "0000000000000000000000000000000000000000000000000000000000000000",
		"patch_id":           claimedPatchID,
		"audited_base":       "1111111111111111111111111111111111111111",
		"new_base":           "2222222222222222222222222222222222222222",
		"git_head":           "2222222222222222222222222222222222222222",
		"audited_diff_path":  auditedPath,
		"composed_diff_path": composedPath,
		"gate_results":       map[string]string{"compile": "pass", "test": "pass", "acs": "pass", "apicover": "pass"},
	}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	return evolveDir
}

func runLedgerVerifyRC(t *testing.T, evolveDir string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runLedger([]string{"verify", "--evolve-dir", evolveDir}, nil, &stdout, &stderr)
	return rc, stderr.String()
}

// TestCompositionVerdict_KernelRecomputesPatchId: a composition-verdict entry
// whose recorded patch_id is NOT what its own persisted diffs recompute to is
// tampered and must break `evolve ledger verify` (exit 2). Two forgeries:
//
//   - drifted-composed: composed.diff's patch-id differs from the claimed
//     (audited) patch_id — a drift smuggled past the fast path.
//   - forged-patch-id: both diffs agree with each other but the entry claims
//     a fabricated patch_id — a hand-edited ledger line.
func TestCompositionVerdict_KernelRecomputesPatchId(t *testing.T) {
	auditedID := compPatchID(t, t.TempDir(), compAuditedDiff)
	cases := []struct {
		name                      string
		auditedDiff, composedDiff string
		claimedPatchID            string
	}{
		{"drifted-composed-diff", compAuditedDiff, compDriftedDiff, auditedID},
		{"forged-patch-id", compAuditedDiff, compAuditedDiff, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := compFixture(t, tc.auditedDiff, tc.composedDiff, tc.claimedPatchID)
			rc, errOut := runLedgerVerifyRC(t, evolveDir)
			if rc != 2 {
				t.Fatalf("tampered composition-verdict entry (%s) must break ledger verify: got rc=%d (want 2), stderr=%q", tc.name, rc, errOut)
			}
		})
	}
}

// TestCompositionVerdict_ValidEntryVerifies: an honest entry — recorded
// patch_id equals the kernel-recomputed patch-id of both persisted diffs —
// keeps the ledger green (exit 0). Guard against over-rejection: the checker
// must validate composition-verdict entries, not ban them.
func TestCompositionVerdict_ValidEntryVerifies(t *testing.T) {
	honestID := compPatchID(t, t.TempDir(), compAuditedDiff)
	evolveDir := compFixture(t, compAuditedDiff, compAuditedDiff, honestID)
	rc, errOut := runLedgerVerifyRC(t, evolveDir)
	if rc != 0 {
		t.Fatalf("honest composition-verdict entry must verify: got rc=%d, stderr=%q", rc, errOut)
	}
}
