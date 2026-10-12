//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const compositionBackfillDiff = "diff --git a/a.txt b/a.txt\nindex 1111111..2222222 100644\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-one\n+two\n"

func compositionGreenGates() map[string]string {
	out := map[string]string{}
	for _, g := range ciparity.RequiredComposedGates {
		out[g] = "pass"
	}
	return out
}

func TestWriteCompositionVerdict_PassesEveryFieldToTheChainedLedger(t *testing.T) {
	t.Parallel()
	ledgerPath := filepath.Join(t.TempDir(), "ledger.jsonl")
	patchID, err := ledger.PatchID([]byte(compositionBackfillDiff))
	if err != nil {
		t.Fatal(err)
	}
	in := core.CompositionVerdictInput{
		Cycle: 812, Method: "trivial-rebase", LaneAuditRef: "lane-ref", PatchID: patchID,
		AuditedBase: "base-sha", GitHead: "head-sha", TreeStateSHA: "tree-sha", AuditedTreeSHA: "audited-tree-sha",
		GateResults: compositionGreenGates(), AuditedDiff: []byte(compositionBackfillDiff), ComposedDiff: []byte(compositionBackfillDiff),
	}

	if err := writeCompositionVerdict(ledgerPath, in); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &rec); err != nil {
		t.Fatalf("ledger %q: %v", raw, err)
	}
	for _, want := range []string{"lane-ref", patchID, "base-sha", "head-sha", "tree-sha", "audited-tree-sha", "trivial-rebase"} {
		if !strings.Contains(string(raw), `"`+want+`"`) {
			t.Errorf("ledger line %s lacks %q", raw, want)
		}
	}
	if rec["cycle"] != float64(812) {
		t.Errorf("cycle = %v, want 812", rec["cycle"])
	}
	if gates, _ := rec["gate_results"].(map[string]any); len(gates) != len(ciparity.RequiredComposedGates) {
		t.Errorf("gate_results = %v, want every required gate", rec["gate_results"])
	}
}

func TestWriteCompositionVerdict_TheLedgerWriterRefusesARedGate(t *testing.T) {
	t.Parallel()
	ledgerPath := filepath.Join(t.TempDir(), "ledger.jsonl")
	gates := compositionGreenGates()
	gates["test"] = "fail"

	err := writeCompositionVerdict(ledgerPath, core.CompositionVerdictInput{
		GateResults: gates, AuditedDiff: []byte(compositionBackfillDiff), ComposedDiff: []byte(compositionBackfillDiff),
	})

	if err == nil || !strings.Contains(err.Error(), "composed-tree gates not green: test") {
		t.Errorf("err = %v, want the red-gate refusal", err)
	}
	if _, statErr := os.Stat(ledgerPath); !os.IsNotExist(statErr) {
		t.Errorf("a refused verdict wrote the ledger (stat err=%v)", statErr)
	}
}

func TestComposedGatesTo_AnUnmappedRequiredGateIsLeftAbsent(t *testing.T) {
	prev := composedGateTargets
	t.Cleanup(func() { composedGateTargets = prev })
	trimmed := maps.Clone(prev)
	delete(trimmed, "apicover")
	composedGateTargets = trimmed
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	makefile := "build:\n\t@true\ntest:\n\t@true\ntest-acs-durable:\n\t@true\napicover-enforce:\n\t@true\n"
	if err := os.WriteFile(filepath.Join(worktree, "go", "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	results := composedGatesTo(&log)(context.Background(), worktree)

	if _, ok := results["apicover"]; ok {
		t.Errorf("results = %v, want no entry for the unmapped gate", results)
	}
	for _, g := range []string{"compile", "test", "acs"} {
		if results[g].Status != "pass" {
			t.Errorf("results[%s] = %+v, want pass", g, results[g])
		}
	}
	status := map[string]string{}
	for g, o := range results {
		status[g] = o.Status
	}
	if missing := ciparity.MissingComposedGates(status); len(missing) != 1 || missing[0] != "apicover" {
		t.Errorf("missing = %v, want the absent gate to fail closed", missing)
	}
}
