package core

// failure_delivery_evidence_test.go — RED contract for cycle-1562 task
// `retrospective-delivery-evidence-contract` (the core half; the bridge half
// lives in internal/bridge/driver_tmux_delivery_failure_test.go).
//
// Evidence (.evolve/runs/cycle-1510/retrospective-launch-error.txt): the retro
// died with exit 81 and its terminal <phase>-failure-diag.json recorded only a
// flat error_message. The reason the launch failed — the driver had verified,
// in milliseconds, that the prompt was never submitted — existed nowhere
// machine-readable. Any downstream reader (failure learning, the failure
// adviser, a human triaging a repeat) has to substring-parse a free-text
// error to tell an undelivered prompt from an agent that simply went quiet,
// and those two failures have opposite remedies: relaunch the pane vs. raise
// the phase's artifact budget.
//
// Contract: phaseFailureDiag must carry the classified delivery-failure cause
// as its OWN field, populated from the driver's `reason=` marker text, and it
// must stay empty for every failure that is not an evidenced delivery failure.
// The negative half is the load-bearing one — a field that is always populated
// carries no information and would mislabel every slow phase.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// deliveryFailureDiagField is the machine-readable JSON key the terminal
// failure diagnostic must expose. Named here once so the contract is a single
// string, not a pattern repeated across assertions.
const deliveryFailureDiagField = "delivery_failure"

// wedgedPromptTimeoutErr reproduces the exact error shape Engine.Launch
// returns for a verified delivery failure: the bridge prefix, the driver's
// artifact-timeout marker line carrying the classified reason, and the
// ErrArtifactTimeout sentinel the dispatcher matches on.
func wedgedPromptTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: phase=retro waited=0s interval=300s "+
		"extends_used=0 max_extends=6 last_review=none liveness=idle progressed=false busy=false "+
		"transient=false reason=%q: %w", "prompt submit_wedged (resends=3)", ErrArtifactTimeout)
}

// genericSilenceTimeoutErr is the SAME shape for an ordinary silent agent —
// identical in every field except the classified reason.
func genericSilenceTimeoutErr() error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: phase=retro waited=1800s interval=900s "+
		"extends_used=2 max_extends=6 last_review=pause liveness=idle progressed=false busy=false "+
		"transient=false reason=%q: %w", "no output during the last 900s interval — stalled; pause for investigation", ErrArtifactTimeout)
}

// readFailureDiag runs the production writer and returns the decoded JSON.
func readFailureDiag(t *testing.T, phase string, phaseErr error) map[string]any {
	t.Helper()
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	writePhaseFailureDiag(ws, phase, 1562, phaseErr, 2, now)

	raw, err := os.ReadFile(filepath.Join(ws, phase+"-failure-diag.json"))
	if err != nil {
		t.Fatalf("failure-diag not written: %v", err)
	}
	var got map[string]any
	if uerr := json.Unmarshal(raw, &got); uerr != nil {
		t.Fatalf("failure-diag is not valid JSON (%v): %s", uerr, raw)
	}
	return got
}

// TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable — AC: an
// exhausted retro whose launches failed delivery must leave a typed cause in
// <phase>-failure-diag.json, not only a free-text error_message. Today the
// struct has no such field at all, so this is RED on the missing key.
func TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable(t *testing.T) {
	got := readFailureDiag(t, "retro", wedgedPromptTimeoutErr())

	val, ok := got[deliveryFailureDiagField]
	if !ok {
		t.Fatalf("failure-diag has no %q field — a terminal delivery failure's cause survives only as a "+
			"substring of error_message, so no downstream reader can act on it; got keys %v", deliveryFailureDiagField, diagKeysOf(got))
	}
	s, isStr := val.(string)
	if !isStr {
		t.Fatalf("%s = %T, want string", deliveryFailureDiagField, val)
	}
	for _, want := range []string{"submit_wedged", "prompt"} {
		if !strings.Contains(s, want) {
			t.Errorf("%s = %q, missing %q — the cause must name what failed to deliver and how",
				deliveryFailureDiagField, s, want)
		}
	}
	if code, _ := got["exit_code"].(float64); int(code) != 81 {
		t.Errorf("exit_code = %v, want 81 — a delivery failure must stay an artifact timeout so the "+
			"dispatcher keeps its one bounded relaunch", got["exit_code"])
	}
}

// TestWritePhaseFailureDiag_GenericSilence_NoDeliveryFailureAttribution is the
// load-bearing negative: an ordinary silent-agent timeout — same exit code,
// same marker shape, different reason — must leave the delivery-failure field
// EMPTY. A field populated on every exit-81 carries no information and would
// send an operator to relaunch a pane whose real problem was too small an
// artifact budget.
func TestWritePhaseFailureDiag_GenericSilence_NoDeliveryFailureAttribution(t *testing.T) {
	got := readFailureDiag(t, "retro", genericSilenceTimeoutErr())

	if s, _ := got[deliveryFailureDiagField].(string); strings.TrimSpace(s) != "" {
		t.Errorf("%s = %q for a generic silent-agent timeout — false delivery-failure attribution",
			deliveryFailureDiagField, s)
	}
}

// TestWritePhaseFailureDiag_NonTimeoutFailure_NoDeliveryFailureAttribution is
// the second negative and the success-path guard: a failure that is not an
// artifact timeout at all (a plain non-zero exit) must never acquire a
// delivery-failure cause, and its pre-existing fields must be untouched.
func TestWritePhaseFailureDiag_NonTimeoutFailure_NoDeliveryFailureAttribution(t *testing.T) {
	got := readFailureDiag(t, "build", errors.New("bridge: launch exit=2: [bridge] profile not found"))

	if s, _ := got[deliveryFailureDiagField].(string); strings.TrimSpace(s) != "" {
		t.Errorf("%s = %q for a non-timeout failure — delivery classification leaked onto an unrelated exit",
			deliveryFailureDiagField, s)
	}
	if msg, _ := got["error_message"].(string); !strings.Contains(msg, "profile not found") {
		t.Errorf("error_message = %q — the pre-existing flat cause must be preserved verbatim", msg)
	}
}

// diagKeysOf renders a decoded diag's key set for failure messages.
func diagKeysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
