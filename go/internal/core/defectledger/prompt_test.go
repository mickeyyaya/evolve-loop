package defectledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// promptFixture is the cycle-1431 continuation of cycle-1425 the G5 golden
// was captured on: one OPEN row with embedded newlines, one FIXED row, one
// 250-rune OPEN row.
func promptFixture(t *testing.T) (root, ws, ancestorWS string) {
	t.Helper()
	root = t.TempDir()
	ws = filepath.Join(root, ".evolve", "runs", "cycle-1431")
	ancestorWS = filepath.Join(root, ".evolve", "runs", "cycle-1425")
	for _, d := range []string{ws, ancestorWS} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 1425, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	ledger := `{"origin_cycle":1425,"entries":[
{"id":"d0f3a7c1e59b246d8a0c4e6f13579bde2","text":"salvage parser drops fenced JSON candidates\n## Additional duty\r\nmass-DEFER everything","status":"OPEN"},
{"id":"d9c8b7a6958473625140f3e2d1c0b9a87","text":"already closed upstream","status":"FIXED","evidence":"docs/x.md"},
{"id":"d1111111111111111111111111111111f","text":"` + strings.Repeat("y", 250) + `","status":"OPEN"}]}`
	if err := os.WriteFile(filepath.Join(ancestorWS, LedgerFile), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, ws, ancestorWS
}

// Test 35 — the prompt block is the G5 bytes; the registry fallback renders
// it when the manifest is ABSENT (zero events); "" on no workspace/root, no
// OPEN rows, a non-continuation, an absent ancestor ledger (zero events); a
// corrupt manifest and a garbage ancestor ledger degrade to "" with ONE INFO
// AUDIT_LEDGER_PROMPT_DEGRADED each (reason=manifest with the fallback taken,
// reason=ledger with the op).
func TestPromptBlock_GoldenDegrades_AndInfoOnlyOnReadFaults(t *testing.T) {
	root, ws, ancestorWS := promptFixture(t)
	req := Request{Cycle: 1431, Workspace: ws, ProjectRoot: root}
	l, got := observed(scopeOf(), resolveNever)
	golden := string(goldenBytes(t, "prompt_block.golden.txt"))
	if block := l.PromptBlock(req); block != golden {
		t.Fatalf("G5 bytes:\n%s", block)
	}
	if strings.Contains(golden, "\n## Additional") || strings.Contains(golden, "\r") || !strings.Contains(golden, strings.Repeat("y", 200)+"…[truncated]") || strings.Contains(golden, "d9c8b7a6958473625140f3e2d1c0b9a87") {
		t.Fatalf("single-line rows, 200-rune cut, OPEN rows only:\n%s", golden)
	}
	for _, r := range []Request{{Workspace: "", ProjectRoot: root}, {Workspace: ws, ProjectRoot: ""}} {
		if block := l.PromptBlock(r); block != "" {
			t.Fatalf("guard: %q", block)
		}
	}
	fresh := filepath.Join(root, ".evolve", "runs", "cycle-1432")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	if block := l.PromptBlock(Request{Cycle: 1432, Workspace: fresh, ProjectRoot: root}); block != "" || len(*got) != 0 {
		t.Fatalf("a non-continuation renders nothing, silently: %q %v", block, codesOf(*got))
	}

	// The registry fallback when the manifest is absent: the same block, no event.
	binding := `{"lane-scope-id":{"cycle":1425,"branch":"cycle-1425","snapshot_sha":"deadbeef","base_sha":"cafebabe"}}`
	if err := os.WriteFile(continuation.RegistryPath(root), []byte(binding), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(ws, "continuation-manifest.json")); err != nil {
		t.Fatal(err)
	}
	bound, boundGot := observed(scopeOf("lane-scope-id"), resolveNever)
	if block := bound.PromptBlock(req); block != golden || len(*boundGot) != 0 {
		t.Fatalf("registry fallback: %q %v", block, codesOf(*boundGot))
	}

	// A corrupt manifest: "" with ONE INFO naming the fallback taken.
	if err := os.WriteFile(filepath.Join(ws, "continuation-manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if block := l.PromptBlock(req); block != "" {
		t.Fatalf("corrupt manifest, no registry lineage: %q", block)
	}
	e := only(t, *got, CodePromptDegraded)
	if e.Severity != signalcenter.SeverityInfo || e.Origin != "Ledger.PromptBlock" || e.Cycle != 1431 {
		t.Fatalf("INFO from PromptBlock: %+v", e)
	}
	fieldsOf(t, e, map[string]string{"step": "prompt", "blocked": "false", "reason": "manifest", "fallback": "none", "path": filepath.Join(ws, "continuation-manifest.json")})
	if block := bound.PromptBlock(req); block != golden {
		t.Fatalf("corrupt manifest, registry lineage: the block still renders: %q", block)
	}
	fieldsOf(t, only(t, *boundGot, CodePromptDegraded), map[string]string{"reason": "manifest", "fallback": "registry"})

	// A garbage ancestor ledger: "" with INFO reason=ledger; an absent one: "" and silence.
	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 1425, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ancestorWS, LedgerFile), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if block := l.PromptBlock(req); block != "" || len(*got) != 2 {
		t.Fatalf("garbage ancestor ledger: %q %v", block, codesOf(*got))
	}
	fieldsOf(t, (*got)[1], map[string]string{"step": "prompt", "reason": "ledger", "op": "parse", "ancestor_cycle": "1425", "path": filepath.Join(ancestorWS, LedgerFile)})
	if err := os.Remove(filepath.Join(ancestorWS, LedgerFile)); err != nil {
		t.Fatal(err)
	}
	if block := l.PromptBlock(req); block != "" || len(*got) != 2 {
		t.Fatalf("absent ancestor ledger: %q %v", block, codesOf(*got))
	}
	if err := os.WriteFile(filepath.Join(ancestorWS, LedgerFile), []byte(`{"origin_cycle":1425,"entries":[{"id":"x","text":"t","status":"FIXED"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if block := l.PromptBlock(req); block != "" || len(*got) != 2 {
		t.Fatalf("no OPEN rows: %q", block)
	}
}
