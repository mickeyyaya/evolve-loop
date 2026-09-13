package failurediag

// delivery_test.go — unit 02 (ADR-0103, design decomposition/02-failure-diagnostics.md):
// the delivery-failure classifier, pinned in the unit with an injected
// timeout predicate (the unit never imports core's sentinel). RED first.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// errTimeout stands in for core.ErrArtifactTimeout: the unit only ever sees
// the predicate, never the sentinel.
var errTimeout = errors.New("core: bridge artifact timeout")

func isTimeout(err error) bool { return errors.Is(err, errTimeout) }

func typedErr(cause, reason string) error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: cause=%s reason=%q phase=retro: %w", cause, reason, errTimeout)
}

func legacyErr(reason string) error {
	return fmt.Errorf("bridge: launch exit=81: artifact-timeout: phase=retro waited=0s interval=300s "+
		"extends_used=0 max_extends=6 last_review=none liveness=idle progressed=false busy=false "+
		"transient=false reason=%q: %w", reason, errTimeout)
}

func TestWireTokens_AreTheDecodersSpellings(t *testing.T) {
	if MarkerPrefix != "artifact-timeout: " || CauseField != "cause=" || ReasonField != "reason=" || CauseSubmitWedged != "submit_wedged" || ExitCodeArtifactTimeout != 81 {
		t.Fatalf("the decoder's spellings of the wire tokens (the bridge's producer copies are re-pointed here in follow-up F2): %q %q %q %q %d", MarkerPrefix, CauseField, ReasonField, CauseSubmitWedged, ExitCodeArtifactTimeout)
	}
}

func TestDeliveryFailureCause_PrefersTypedMarkerField(t *testing.T) {
	want := `prompt submit_wedged (resends=3) with "quoted" evidence`
	if got := DeliveryFailureCause(typedErr("submit_wedged", want), isTimeout); got != want {
		t.Fatalf("the typed marker's quoted reason is returned exactly (escapes walked): %q", got)
	}
}

func TestDeliveryFailureCause_DoesNotParseCauseTextInsideReason(t *testing.T) {
	if got := DeliveryFailureCause(typedErr("review_stop", "operator quoted cause=submit_wedged while stopping"), isTimeout); got != "" {
		t.Fatalf("the host-owned cause token is authoritative; reviewer prose cannot forge it: %q", got)
	}
}

func TestDeliveryFailureCause_ReasonMustImmediatelyFollowCause(t *testing.T) {
	err := fmt.Errorf("bridge: artifact-timeout: cause=submit_wedged phase=retro reason=%q: %w", "prompt submit_wedged", errTimeout)
	if got := DeliveryFailureCause(err, isTimeout); got != "submit_wedged" {
		t.Fatalf("a reason= that does not immediately follow the cause is not read; the bare cause is returned: %q", got)
	}
}

func TestDeliveryFailureCause_TypedWedgedReasonWithoutTheTokenReturnsTheBareCause(t *testing.T) {
	if got := DeliveryFailureCause(typedErr("submit_wedged", "resent thrice"), isTimeout); got != "submit_wedged" {
		t.Fatalf("a wedged cause whose reason lacks the token returns the bare cause: %q", got)
	}
	followedByOtherFields := fmt.Errorf("bridge: artifact-timeout: cause=submit_wedged phase=retro: %w", errTimeout)
	if got := DeliveryFailureCause(followedByOtherFields, isTimeout); got != "submit_wedged" {
		t.Fatalf("a wedged cause followed by other fields but no reason= returns the bare cause: %q", got)
	}
	markerEndsAtTheCause := fmt.Errorf("%w: artifact-timeout: cause=submit_wedged", errTimeout)
	if got := DeliveryFailureCause(markerEndsAtTheCause, isTimeout); got != "submit_wedged" {
		t.Fatalf("a marker that ends at the cause token returns the bare cause: %q", got)
	}
}

func TestDeliveryFailureCause_LegacyReasonMarkerStillClassifies(t *testing.T) {
	got := DeliveryFailureCause(legacyErr("prompt submit_wedged (resends=3)"), isTimeout)
	if !strings.Contains(got, "submit_wedged") || !strings.Contains(got, "prompt") {
		t.Fatalf("the pre-cause= marker still classifies through its quoted reason: %q", got)
	}
	if got := DeliveryFailureCause(legacyErr("no output during the last 900s interval — stalled; pause for investigation"), isTimeout); got != "" {
		t.Fatalf("a generic silence is not a delivery failure: %q", got)
	}
}

func TestDeliveryFailureCause_GatesOnTheInjectedPredicate(t *testing.T) {
	never := func(error) bool { return false }
	if got := DeliveryFailureCause(typedErr("submit_wedged", "prompt submit_wedged"), never); got != "" {
		t.Fatalf("a perfect marker is not classified unless the predicate says the error is a timeout: %q", got)
	}
	wrapped := fmt.Errorf("phase retro: %w", typedErr("submit_wedged", "prompt submit_wedged (resends=2)"))
	if got := DeliveryFailureCause(wrapped, isTimeout); got != "prompt submit_wedged (resends=2)" {
		t.Fatalf("wrapping is matched through the predicate: %q", got)
	}
}

func TestDeliveryFailureCause_MalformedAndTruncatedMarkersAreEmpty(t *testing.T) {
	truncated := strings.Repeat("x", 1024) + "…"
	for name, err := range map[string]error{
		"no marker":          fmt.Errorf("bridge: launch exit=81: %w", errTimeout),
		"marker, no reason":  fmt.Errorf("bridge: artifact-timeout: phase=retro: %w", errTimeout),
		"unquoted value":     fmt.Errorf("bridge: artifact-timeout: reason=prompt submit_wedged: %w", errTimeout),
		"unterminated quote": fmt.Errorf("bridge: artifact-timeout: reason=\"prompt submit_wedged: %w", errTimeout),
		"cut at 1024 runes":  fmt.Errorf("bridge: artifact-timeout: reason=%s: %w", strconv.Quote("prompt submit_wedged " + truncated)[:1000], errTimeout),
	} {
		if got := DeliveryFailureCause(err, isTimeout); got != "" {
			t.Errorf("%s: malformed markers classify as nothing, silently: %q", name, got)
		}
	}
}
