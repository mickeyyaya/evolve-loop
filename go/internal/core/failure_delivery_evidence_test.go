package core

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

func typedWedgedTimeoutErr(cause, reason string) error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: cause=%s reason=%q phase=retro: %w",
		cause, reason, ErrArtifactTimeout)
}

func TestDeliveryFailureCause_PrefersTypedMarkerField(t *testing.T) {
	want := `prompt submit_wedged (resends=3) with "quoted" evidence`
	err := typedWedgedTimeoutErr("submit_wedged", want)
	if got := DeliveryFailureCause(err); got != want {
		t.Fatalf("DeliveryFailureCause() = %q, want %q from the typed marker", got, want)
	}
}

func TestDeliveryFailureCause_DoesNotParseCauseTextInsideReason(t *testing.T) {
	err := typedWedgedTimeoutErr("review_stop", "operator quoted cause=submit_wedged while stopping")
	if got := DeliveryFailureCause(err); got != "" {
		t.Fatalf("DeliveryFailureCause() = %q, want empty for actual cause=review_stop", got)
	}
}

func readFailureDiag(t *testing.T, phase string, phaseErr error) map[string]any {
	t.Helper()
	ws := t.TempDir()
	now := func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	(&Orchestrator{now: now}).writePhaseFailureDiag(ws, phase, 1562, phaseErr, 2)

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

func TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable(t *testing.T) {
	if cause := DeliveryFailureCause(wedgedPromptTimeoutErr()); !strings.Contains(cause, "submit_wedged") {
		t.Fatalf("DeliveryFailureCause() = %q, want classified submit_wedged reason", cause)
	}
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

func TestWritePhaseFailureDiag_GenericSilence_NoDeliveryFailureAttribution(t *testing.T) {
	got := readFailureDiag(t, "retro", genericSilenceTimeoutErr())

	if s, _ := got[deliveryFailureDiagField].(string); strings.TrimSpace(s) != "" {
		t.Errorf("%s = %q for a generic silent-agent timeout — false delivery-failure attribution",
			deliveryFailureDiagField, s)
	}
}

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

func diagKeysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
