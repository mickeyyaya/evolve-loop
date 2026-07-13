// composition_write_test.go — RED contract for WriteCompositionVerdict, the
// RUNG 0 producer (cycle-787; knowledge-base/research/merge-concurrency-2026).
// Cycle-786 landed the reader (ship.tryTrivialRebaseCarryForward) and the
// kernel verifier (verifyCompositionLine); nothing in the tree writes a
// composition-verdict line yet. These tests encode the writer's contract:
//
//   - round-trip: a written line is accepted by the existing kernel verify
//     and carries every field both consumer structs read;
//   - fail-closed at write time: a patch_id that does not recompute from the
//     supplied diffs, or a gate_results map not green on the full
//     ciparity.RequiredComposedGates set, is an error and appends NOTHING;
//   - empty/whitespace-only diffs are rejected (mirrors PatchID(nil) erroring).
//
// Builder contract: implement WriteCompositionVerdict in composition.go to
// turn these GREEN. Do not modify this file.
package ledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

// passingComposedGates returns the full required composed-tree gate set,
// all green — built from the SSOT list, not a second hardcoded copy.
func passingComposedGates() map[string]string {
	m := make(map[string]string, len(ciparity.RequiredComposedGates))
	for _, g := range ciparity.RequiredComposedGates {
		m[g] = "pass"
	}
	return m
}

// honestWriteInput builds a CompositionVerdictInput whose patch_id honestly
// recomputes from its diffs and whose gates are all green; artifacts persist
// under artifactDir.
func honestWriteInput(t *testing.T, artifactDir string) CompositionVerdictInput {
	t.Helper()
	honestID, err := PatchID([]byte(compTestDiff))
	if err != nil {
		t.Fatalf("PatchID(compTestDiff): %v", err)
	}
	return CompositionVerdictInput{
		Cycle:        787,
		LaneAuditRef: "lane-audit-artifact-sha256",
		PatchID:      honestID,
		AuditedBase:  "audited-base-sha",
		GitHead:      "composed-head-sha",
		TreeStateSHA: "composed-tree-state-sha",
		GateResults:  passingComposedGates(),
		AuditedDiff:  []byte(compTestDiff),
		ComposedDiff: []byte(compTestDiff),
		ArtifactDir:  artifactDir,
	}
}

// ledgerSize stats ledgerPath, treating "does not exist" as size 0 so the
// no-partial-write assertions cover both a fresh and a pre-populated ledger.
func ledgerSize(t *testing.T, ledgerPath string) int64 {
	t.Helper()
	fi, err := os.Stat(ledgerPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("stat %s: %v", ledgerPath, err)
	}
	return fi.Size()
}

// TestWriteCompositionVerdict_RoundTrip: write → read back → kernel verify.
// The written line must satisfy BOTH consumers: ship's compositionEntry
// (kind/method/lane_audit_ref/patch_id/audited_base/git_head/tree_state_sha/
// gate_results) and ledger's compositionFields (patch_id + both persisted
// diff artifact paths), and the existing Verify must accept it unchanged.
func TestWriteCompositionVerdict_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	in := honestWriteInput(t, dir)

	if err := WriteCompositionVerdict(ledgerPath, in); err != nil {
		t.Fatalf("WriteCompositionVerdict(honest input): %v", err)
	}

	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read back ledger: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want exactly 1 appended line, got %d:\n%s", len(lines), raw)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("appended line is not one JSON object: %v\n%s", err, lines[0])
	}
	for field, want := range map[string]string{
		"kind":           CompositionVerdictKind,
		"method":         "trivial-rebase",
		"lane_audit_ref": in.LaneAuditRef,
		"patch_id":       in.PatchID,
		"audited_base":   in.AuditedBase,
		"git_head":       in.GitHead,
		"tree_state_sha": in.TreeStateSHA,
	} {
		if got[field] != want {
			t.Errorf("field %q = %v, want %q", field, got[field], want)
		}
	}
	gates, ok := got["gate_results"].(map[string]any)
	if !ok {
		t.Fatalf("gate_results missing or not an object: %v", got["gate_results"])
	}
	for _, g := range ciparity.RequiredComposedGates {
		if gates[g] != "pass" {
			t.Errorf("gate_results[%q] = %v, want \"pass\"", g, gates[g])
		}
	}

	// The persisted artifacts must round-trip byte-for-byte: kernel verify
	// recomputes patch-id from these files, not from the caller's memory.
	for field, want := range map[string]string{
		"audited_diff_path":  compTestDiff,
		"composed_diff_path": compTestDiff,
	} {
		p, ok := got[field].(string)
		if !ok || p == "" {
			t.Fatalf("field %q missing from written line: %v", field, got[field])
		}
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("persisted artifact %s unreadable: %v", field, err)
		}
		if string(content) != want {
			t.Errorf("artifact %s content drifted from the supplied diff", field)
		}
	}

	// The whole point: the EXISTING kernel verifier accepts the writer's
	// output with zero changes on the verify side.
	if err := New(dir).Verify(context.Background()); err != nil {
		t.Fatalf("existing ledger Verify must accept a written composition-verdict: %v", err)
	}
}

// TestWriteCompositionVerdict_RejectsPatchIDMismatch: fail-closed at write
// time. A claimed patch_id that does not recompute from the supplied diffs,
// or a gate_results map missing/failing any required composed gate, is an
// error AND appends nothing — asserted by ledger byte-length, not just the
// error return (a fake that errors after writing must fail here).
func TestWriteCompositionVerdict_RejectsPatchIDMismatch(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CompositionVerdictInput)
	}{
		{"forged-patch-id", func(in *CompositionVerdictInput) {
			in.PatchID = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
		}},
		{"composed-diff-drifted-from-patch-id", func(in *CompositionVerdictInput) {
			in.ComposedDiff = []byte(compTestDriftedDiff)
		}},
		{"missing-required-gate", func(in *CompositionVerdictInput) {
			delete(in.GateResults, "acs")
		}},
		{"failed-required-gate", func(in *CompositionVerdictInput) {
			in.GateResults["test"] = "fail"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ledgerPath := filepath.Join(dir, "ledger.jsonl")
			// Pre-populate so "appends nothing" is a real byte-length
			// delta check, not just "file still absent".
			preexisting := []byte(`{"kind":"unrelated"}` + "\n")
			if err := os.WriteFile(ledgerPath, preexisting, 0o644); err != nil {
				t.Fatalf("seed ledger: %v", err)
			}
			before := ledgerSize(t, ledgerPath)

			in := honestWriteInput(t, dir)
			tc.mutate(&in)

			if err := WriteCompositionVerdict(ledgerPath, in); err == nil {
				t.Fatal("WriteCompositionVerdict must reject the tampered/ungated input")
			}
			if after := ledgerSize(t, ledgerPath); after != before {
				t.Fatalf("rejected write must append NOTHING: ledger grew %d → %d bytes", before, after)
			}
		})
	}
}

// TestWriteCompositionVerdict_EmptyDiff: an empty or whitespace-only diff for
// either artifact is rejected before anything is persisted (mirrors
// PatchID(nil) erroring rather than returning a forgeable empty identity).
// The whitespace-only case exists precisely to catch a writer that
// special-cases "" but not "   \n".
func TestWriteCompositionVerdict_EmptyDiff(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*CompositionVerdictInput)
	}{
		{"nil-audited-diff", func(in *CompositionVerdictInput) { in.AuditedDiff = nil }},
		{"empty-composed-diff", func(in *CompositionVerdictInput) { in.ComposedDiff = []byte("") }},
		{"whitespace-only-audited-diff", func(in *CompositionVerdictInput) { in.AuditedDiff = []byte("   \n") }},
		{"whitespace-only-composed-diff", func(in *CompositionVerdictInput) { in.ComposedDiff = []byte(" \t\n\n") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ledgerPath := filepath.Join(dir, "ledger.jsonl")
			in := honestWriteInput(t, dir)
			tc.mutate(&in)

			if err := WriteCompositionVerdict(ledgerPath, in); err == nil {
				t.Fatal("WriteCompositionVerdict must reject an empty/whitespace-only diff")
			}
			if size := ledgerSize(t, ledgerPath); size != 0 {
				t.Fatalf("rejected write must not create/append to the ledger, got %d bytes", size)
			}
		})
	}
}
