package coherence

import (
	"strings"
	"testing"
)

func TestCheckPhaseOrder_SinglePhaseHasNothingToOrder(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	xaWriteTiming(t, ws, xaPhase{"build", "2026-10-01T10:00:00Z", "2026-10-01T10:01:00Z"})
	got := checkPhaseOrder(ws)
	if got.Status != InvariantIndeterminate {
		t.Fatalf("status = %q, want indeterminate for one timed phase", got.Status)
	}
}

func TestCheckPhaseOrder_PhaseWithoutTimestampsIsSkipped(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	xaWriteTiming(t, ws,
		xaPhase{"scout", "", ""},
		xaPhase{"build", "2026-10-01T10:00:00Z", "2026-10-01T10:01:00Z"},
		xaPhase{"audit", "2026-10-01T10:02:00Z", "2026-10-01T10:03:00Z"},
	)
	got := checkPhaseOrder(ws)
	if got.Status != InvariantOK {
		t.Fatalf("status = %q (%s), want ok: an untimed phase is not part of the chain", got.Status, got.Evidence)
	}
	if !strings.Contains(got.Evidence, "2-phase chain") {
		t.Errorf("evidence = %q, want it to count the two timed phases", got.Evidence)
	}
}

func TestCheckPhaseOrder_PhaseEndingBeforeItStartsIsViolated(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	xaWriteTiming(t, ws,
		xaPhase{"build", "2026-10-01T10:00:00Z", "2026-10-01T09:00:00Z"},
		xaPhase{"audit", "2026-10-01T10:01:00Z", "2026-10-01T10:02:00Z"},
	)
	got := checkPhaseOrder(ws)
	if got.Status != InvariantViolated || !strings.Contains(got.Evidence, "ends at") {
		t.Fatalf("status = %q evidence = %q, want violated naming the reversed phase", got.Status, got.Evidence)
	}
}

func TestCheckPhaseOrder_ChainThatRunsBackwardsIsViolated(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	xaWriteTiming(t, ws,
		xaPhase{"build", "2026-10-01T10:00:00Z", "2026-10-01T10:01:00Z"},
		xaPhase{"scout", "2026-10-01T09:00:00Z", "2026-10-01T09:01:00Z"},
	)
	got := checkPhaseOrder(ws)
	if got.Status != InvariantViolated || !strings.Contains(got.Evidence, "runs backwards") {
		t.Fatalf("status = %q evidence = %q, want violated for a backwards chain", got.Status, got.Evidence)
	}
}

func TestCheckPhaseOrder_AuditBeforeBuildIsViolated(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	xaWriteTiming(t, ws,
		xaPhase{"audit", "2026-10-01T09:00:00Z", "2026-10-01T09:01:00Z"},
		xaPhase{"build", "2026-10-01T09:30:00Z", "2026-10-01T09:31:00Z"},
	)
	got := checkPhaseOrder(ws)
	if got.Status != InvariantViolated || !strings.Contains(got.Evidence, "cannot precede the build") {
		t.Fatalf("status = %q evidence = %q, want violated for an audit before its build", got.Status, got.Evidence)
	}
}
