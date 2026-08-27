package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runid_stamp_test.go — cycle-1571 H1. writeSubprocessLedger builds its ledger
// line as a hand-formatted JSON literal (field order mirrors the retired bash
// writer). That literal had no run_id, so every `evolve subagent run` auditor
// entry was invisible to ship's run-scoped binding lookup after PR #503.

func fixedNowFn() func() time.Time {
	return func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }
}

func writeAndRead(t *testing.T, e subprocessLedger) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := writeSubprocessLedger(p, e, fixedNowFn()); err != nil {
		t.Fatalf("writeSubprocessLedger: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// TestWriteSubprocessLedger_StampsRunID is the primary pin: the entry carries
// the run identity, so a run-scoped reader can bind it.
func TestWriteSubprocessLedger_StampsRunID(t *testing.T) {
	t.Parallel()
	line := writeAndRead(t, subprocessLedger{
		Cycle: 1519, Role: "auditor", ExitCode: 0, RunID: "01M09657TDN6Q1VMJK1XKYR376",
	})
	if !strings.Contains(line, `"run_id":"01M09657TDN6Q1VMJK1XKYR376"`) {
		t.Errorf("ledger line carries no run_id — a run-scoped binding lookup can never match it.\nline: %s", line)
	}
}

// TestWriteSubprocessLedger_RunIDReadableByBindingReaders decodes the line the
// way ship.findLatestAudit and cmd/evolve.latestAuditEntry actually do. Pinning
// the raw substring alone would pass on a malformed line; this proves the wire
// contract those two readers key on.
func TestWriteSubprocessLedger_RunIDReadableByBindingReaders(t *testing.T) {
	t.Parallel()
	line := writeAndRead(t, subprocessLedger{
		Cycle: 1519, Role: "auditor", RunID: "01RUNIDVALUE",
	})
	var got struct {
		Kind  string `json:"kind"`
		Role  string `json:"role"`
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("ledger line is not valid JSON after adding run_id: %v\nline: %s", err, line)
	}
	if got.Kind != "agent_subprocess" || got.Role != "auditor" {
		t.Errorf("kind/role = %q/%q, want agent_subprocess/auditor", got.Kind, got.Role)
	}
	if got.RunID != "01RUNIDVALUE" {
		t.Errorf("decoded run_id = %q, want 01RUNIDVALUE", got.RunID)
	}
}

// TestWriteSubprocessLedger_OmitsRunIDWhenUnresolved: parity with core's
// `json:"run_id,omitempty"`. An unresolved identity must leave the key ABSENT,
// not present-and-empty — a legacy-shaped entry, distinguishable from a
// deliberate blank, and byte-identical to what this writer emitted before.
func TestWriteSubprocessLedger_OmitsRunIDWhenUnresolved(t *testing.T) {
	t.Parallel()
	line := writeAndRead(t, subprocessLedger{Cycle: 1519, Role: "auditor"})
	if strings.Contains(line, "run_id") {
		t.Errorf("an unresolved run id must OMIT the key (omitempty parity), got: %s", line)
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(line), &probe); err != nil {
		t.Fatalf("omitted-run_id line is not valid JSON: %v\nline: %s", err, line)
	}
}

// TestWriteSubprocessLedger_RunIDWithFormatVerbsStaysValid — go-review CRITICAL.
// The run_id fragment is spliced into a fmt.Sprintf FORMAT STRING, so a '%' in
// the value is reinterpreted as a verb: it consumes the next argument, shifting
// every subsequent field and producing a line that is not even valid JSON —
// silently, since writeSubprocessLedger still returns nil.
//
// jsonStringEscape handles \ " \n \r \t but NOT '%', and it cannot: escaping is
// the wrong layer. Every other field is safe only because it is passed as an
// ARGUMENT, where fmt never rescans for verbs. run_id must be too.
//
// Not hypothetical by construction: RunIDFromWorkspace reads this value off
// disk with no validation, and its own contract says the writer may run outside
// the orchestrator process — i.e. the value is not guaranteed to come from
// MintRunID's crockford-base32 alphabet.
func TestWriteSubprocessLedger_RunIDWithFormatVerbsStaysValid(t *testing.T) {
	t.Parallel()
	for _, runID := range []string{`run%s-weird`, `run%d`, `a"b`, `100%`, `%!x(MISSING)`} {
		line := writeAndRead(t, subprocessLedger{
			Cycle: 1519, Role: "auditor", Model: "opus", ExitCode: 0, RunID: runID,
		})
		var got struct {
			Role  string `json:"role"`
			Model string `json:"model"`
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("run_id %q corrupted the ledger line into invalid JSON: %v\nline: %s", runID, err, line)
			continue
		}
		if got.RunID != runID {
			t.Errorf("run_id round-trip = %q, want %q\nline: %s", got.RunID, runID, line)
		}
		// Argument shifting shows up as neighbouring fields taking each other's
		// values, so pin them too — a valid-JSON check alone would miss it.
		if got.Role != "auditor" || got.Model != "opus" {
			t.Errorf("run_id %q shifted adjacent fields: role=%q model=%q\nline: %s", runID, got.Role, got.Model, line)
		}
	}
}
