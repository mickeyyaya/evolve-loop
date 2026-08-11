// cmd_ledger_anchor_wiring_test.go — cycle-1433 durable WIRING proof for the two
// ledger-fleet-concurrency-chain repairs.
//
// The ledger package owns the behavior; this file owns the seam. Both surfaces
// are operator-facing CLI, so a correct AnchorLine/Rebaseline that no subcommand
// or flag reaches is dead code — and the cycle-1433 ACS predicates that drive the
// compiled binary are cycle-scoped and retire. These tests call runLedger, the
// production dispatcher, so a dropped `--line-sha` flag or `rebaseline` case
// stays caught after cycle-1433's predicates are gone.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wiringNote = "operator sign-off: cycle-1433 wiring proof"

// writeWiringLedger materializes a ledger whose entry_seq=2 is carried by TWO
// distinct fork-sibling lines — the ambiguity the anchor half must refuse.
// Returns the dir and both seq-2 SHAs.
//
// It plants no chain break: whether the sealed prefix was damaged is the ledger
// package's contract (TestRebaseline_SealsDamagedPrefix), and what these tests
// prove is the SEAM — that the subcommand and flag reach that contract at all.
func writeWiringLedger(t *testing.T) (dir, firstSeq2, secondSeq2 string) {
	t.Helper()
	dir = t.TempDir()
	zero := strings.Repeat("0", 64)
	sha := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	g := `{"ts":"2026-05-01T00:00:00Z","cycle":1,"role":"orchestrator","kind":"phase","exit_code":0,"entry_seq":0,"prev_hash":"` + zero + `"}`
	a := fmt.Sprintf(`{"ts":"2026-05-01T00:01:00Z","cycle":1,"role":"scout","kind":"phase","exit_code":0,"entry_seq":1,"prev_hash":"%s"}`, sha(g))
	b1 := fmt.Sprintf(`{"ts":"2026-05-01T00:02:00Z","cycle":1,"role":"builder","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha(a))
	b2 := fmt.Sprintf(`{"ts":"2026-05-01T00:02:01Z","cycle":1,"role":"auditor","kind":"phase","exit_code":0,"entry_seq":2,"prev_hash":"%s"}`, sha(a))
	body := g + "\n" + a + "\n" + b1 + "\n" + b2 + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tip := fmt.Sprintf("2:%s", sha(b2))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, sha(b1), sha(b2)
}

// TestRunLedgerAnchor_AmbiguousSeqIsRefusedAndLineSHADisambiguates pins the
// anchor seam: the ambiguity refusal reaches the operator with an actionable
// --line-sha remedy, and that flag is parsed and forwarded to AnchorLine.
func TestRunLedgerAnchor_AmbiguousSeqIsRefusedAndLineSHADisambiguates(t *testing.T) {
	dir, _, second := writeWiringLedger(t)
	anchorPath := filepath.Join(dir, "ledger-anchor.json")

	var stdout, stderr bytes.Buffer
	if rc := runLedger([]string{"anchor", "2", "--evolve-dir", dir}, nil, &stdout, &stderr); rc == 0 {
		t.Errorf("`ledger anchor 2` returned 0 on a seq carried by two distinct lines:\n%s", stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "entry_seq=2") {
		t.Errorf("refusal does not name the offending seq:\n%s", out)
	}
	if !strings.Contains(out, "--line-sha") {
		t.Errorf("refusal does not name the --line-sha remedy — the ErrAmbiguousAnchorSeq hint is not wired into runLedgerAnchor:\n%s", out)
	}
	if _, err := os.Stat(anchorPath); err == nil {
		t.Error("a refused anchor still wrote ledger-anchor.json")
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runLedger([]string{"anchor", "2", "--line-sha", second, "--evolve-dir", dir}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("`ledger anchor 2 --line-sha <sha>` returned %d — the flag is not wired:\n%s", rc, stderr.String())
	}
	raw, err := os.ReadFile(anchorPath)
	if err != nil {
		t.Fatalf("anchor file not written: %v", err)
	}
	var rec struct {
		AnchorSeq     int    `json:"anchor_seq"`
		AnchorLineSHA string `json:"anchor_line_sha256"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.AnchorSeq != 2 || rec.AnchorLineSHA != second {
		t.Errorf("anchor rec = %+v, want seq=2 sha=%s — the flag value was dropped between the flag set and AnchorLine", rec, second)
	}
}

// TestRunLedgerRebaseline_SubcommandIsReachableAndGated pins the rebaseline seam:
// the subcommand dispatches (never "unknown subcommand"), the --note gate refuses
// from the command's own validation, and a gated call seals in one invocation.
func TestRunLedgerRebaseline_SubcommandIsReachableAndGated(t *testing.T) {
	t.Run("missing_note_is_refused_by_the_command_not_the_dispatcher", func(t *testing.T) {
		dir, _, _ := writeWiringLedger(t)
		var stdout, stderr bytes.Buffer
		rc := runLedger([]string{"rebaseline", "--evolve-dir", dir}, nil, &stdout, &stderr)
		if rc == 0 {
			t.Errorf("rebaseline without --note returned 0 — the operator gate is not enforced")
		}
		if strings.Contains(stderr.String(), "unknown subcommand") {
			t.Errorf("`rebaseline` is not dispatched at all — the refusal is the dispatcher's, not the gate's:\n%s", stderr.String())
		}
	})

	t.Run("gated_call_seals_and_verifies", func(t *testing.T) {
		dir, _, _ := writeWiringLedger(t)
		var stdout, stderr bytes.Buffer
		if rc := runLedger([]string{"rebaseline", "--evolve-dir", dir, "--note", wiringNote}, nil, &stdout, &stderr); rc != 0 {
			t.Fatalf("`ledger rebaseline` returned %d:\n%s", rc, stderr.String())
		}
		stdout.Reset()
		stderr.Reset()
		if rc := runLedger([]string{"verify", "--deep", "--evolve-dir", dir}, nil, &stdout, &stderr); rc != 0 {
			t.Errorf("verify --deep returned %d after one rebaseline:\n%s", rc, stderr.String())
		}
		raw, err := os.ReadFile(filepath.Join(dir, "ledger.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), wiringNote) {
			t.Error("the operator note is absent from the sealed ledger — the trust decision is unattributable")
		}
	})
}
