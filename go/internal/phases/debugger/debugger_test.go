package debugger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeDecision stages a debug-decision.json under dir. An empty body
// means "write no file" so the missing-file path can be exercised.
func writeDecision(t *testing.T, dir, body string) {
	t.Helper()
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, decisionFilename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestClassify is the load-bearing behavioral test: it pins the
// action→verdict→signals mapping and, critically, the SAFE-DEFAULT
// (BLOCK/FAIL) on any parse failure or unknown action. Never RESHIP on
// a malformed file.
func TestClassify(t *testing.T) {
	cases := []struct {
		name          string
		body          string // "" = no file
		wantVerdict   string
		wantAction    string // expected Signals["debugger.action"]; "" = key absent
		wantRerun     string // expected Signals["debugger.rerun_phase"]; "" = key absent
		wantRootCause string // expected Signals["debugger.root_cause"]; "" = not asserted
	}{
		{
			name:          "reship maps to PASS with ship next",
			body:          `{"action":"RESHIP","fix_applied":"re-ran ff-merge","root_cause":"stale ref","reasoning":"safe retry"}`,
			wantVerdict:   core.VerdictPASS,
			wantAction:    actionReship,
			wantRootCause: "stale ref",
		},
		{
			name:        "rerun_phase audit maps to PASS carrying rerun_phase signal",
			body:        `{"action":"RERUN_PHASE","rerun_phase":"audit","root_cause":"stale audit binding","reasoning":"head moved"}`,
			wantVerdict: core.VerdictPASS,
			wantAction:  actionRerunPhase,
			wantRerun:   "audit",
		},
		{
			name:        "block maps to FAIL",
			body:        `{"action":"BLOCK","root_cause":"integrity breach","reasoning":"tamper detected"}`,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "missing file is a safe block (FAIL)",
			body:        "",
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "malformed JSON is a safe block (FAIL)",
			body:        `{"action": "RESHIP", `,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "unknown action string is a safe block (FAIL)",
			body:        `{"action":"YOLO","reasoning":"???"}`,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "empty action is a safe block (FAIL)",
			body:        `{"action":"","reasoning":"forgot"}`,
			wantVerdict: core.VerdictFAIL,
			wantAction:  actionBlock,
		},
		{
			name:        "rerun_phase with no phase named falls back to audit",
			body:        `{"action":"RERUN_PHASE","root_cause":"precondition","reasoning":"redo"}`,
			wantVerdict: core.VerdictPASS,
			wantAction:  actionRerunPhase,
			wantRerun:   string(core.PhaseAudit),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDecision(t, dir, tc.body)

			verdict, signals, _ := Classify(dir)

			if verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q", verdict, tc.wantVerdict)
			}
			if got := signals[signalAction]; got != tc.wantAction {
				t.Errorf("Signals[%q] = %q, want %q", signalAction, got, tc.wantAction)
			}
			if tc.wantRerun != "" {
				if got := signals[signalRerunPhase]; got != tc.wantRerun {
					t.Errorf("Signals[%q] = %q, want %q", signalRerunPhase, got, tc.wantRerun)
				}
			}
			if tc.wantRootCause != "" {
				if got := signals[signalRootCause]; got != tc.wantRootCause {
					t.Errorf("Signals[%q] = %q, want %q", signalRootCause, got, tc.wantRootCause)
				}
			}
		})
	}
}

// TestClassifyNeverReshipOnParseFailure is an explicit safety pin: a
// parse failure must NEVER yield a RESHIP signal, even partially. This
// is the integrity-critical invariant of the phase.
func TestClassifyNeverReshipOnParseFailure(t *testing.T) {
	dir := t.TempDir()
	writeDecision(t, dir, `{"action":"RESHIP"`) // truncated JSON

	verdict, signals, _ := Classify(dir)
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict = %q, want FAIL on parse failure", verdict)
	}
	if signals[signalAction] == actionReship {
		t.Fatalf("parse failure produced a RESHIP action — must default to BLOCK")
	}
}
