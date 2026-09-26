package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realConsumedByNarrative is a consumed_by string in the exact shape a real
// consumed inbox item carries; fixtures use that shape, never a synthetic one.
const realConsumedByNarrative = "console-2026-08-05: fingerprint ship|unknown|76d0f4fca190 = root cause fixed"

const incidentFingerprint = "ship|unknown|76d0f4fca190"

// writeConsumedItem drops one JSON item into .evolve/inbox/consumed/, the
// operator-managed terminal ledger.
func writeConsumedItem(t *testing.T, evolveDir, name, body string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem reconciles when the P0
// naming the fingerprint already sits in consumed/ before the ledger exists:
// the breaker must not halt, and must materialize the ledger so the
// projection is durable rather than recomputed silently. The fixture's kind
// is "pipeline-repair" — production items never carry "pipeline-defect" — so
// a kind-gated implementation must fail this predicate.
func TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")
	writeConsumedItem(t, evolveDir, "2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json",
		`{"id":"pipeline-blocker","kind":"pipeline-repair","consumed_by":"`+realConsumedByNarrative+`"}`)

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr, testRootSignals(t, &stderr)); halted {
		t.Fatalf("a fingerprint whose P0 is already consumed must be reconciled into the ledger and excluded — this is the live cycle-1335 state that re-halted three times; stderr=%q", stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("the reconciler must MATERIALIZE the ledger as a projection of the consumed corpus, not recompute it invisibly: %v", err)
	}
	if !strings.Contains(string(raw), incidentFingerprint) {
		t.Fatalf("ledger must carry the reconciled fingerprint, got %s", raw)
	}
}

// TestBlockerBreakerHalt_ReconcilesItemWithNoKindFromNotes proves the gate
// is parse-success and nothing else: an item carrying NO kind field at all,
// whose fingerprint lives in the auto-filed notes field (the shape an item
// has before a consumed_by narrative is written), is still reconciled.
func TestBlockerBreakerHalt_ReconcilesItemWithNoKindFromNotes(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")
	writeConsumedItem(t, evolveDir, "no-kind.json",
		`{"id":"x","notes":"boot breaker tripped on fingerprint \"`+incidentFingerprint+`\" three times"}`)

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr, testRootSignals(t, &stderr)); halted {
		t.Fatalf("reconciliation must gate on parse-success, not on an item `kind` vocabulary that matches zero live items; stderr=%q", stderr.String())
	}
}

// TestBlockerBreakerHalt_ReconcileDoesNotWeakenRuleB is the negative half:
// a consumed corpus that acks a DIFFERENT fingerprint leaves the halting one
// untouched. The exclusion stays scoped to one named fingerprint at a time —
// never a blanket Rule B disable.
func TestBlockerBreakerHalt_ReconcileDoesNotWeakenRuleB(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")
	writeConsumedItem(t, evolveDir, "other.json",
		`{"id":"other","kind":"pipeline-repair","consumed_by":"console: fingerprint build|guard-abort|deadbeef01 = unrelated"}`)

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr, testRootSignals(t, &stderr)); !halted {
		t.Fatal("consuming an UNRELATED fingerprint must not excuse the halting one — reconciliation is per-fingerprint, never a blanket Rule B disable")
	}
}

// TestBlockerBreakerHalt_ReconcileSurvivesUnreadableItem pins the
// fail-loud-but-never-block contract: a corrupt file in consumed/ must WARN
// and let the sweep continue to the good item beside it. A reconciler defect
// must never become a new boot blocker.
func TestBlockerBreakerHalt_ReconcileSurvivesUnreadableItem(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")
	writeConsumedItem(t, evolveDir, "00-corrupt.json", `{"id":"broken",`)
	writeConsumedItem(t, evolveDir, "01-good.json",
		`{"id":"pipeline-blocker","kind":"pipeline-repair","consumed_by":"`+realConsumedByNarrative+`"}`)

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr, testRootSignals(t, &stderr)); halted {
		t.Fatalf("one corrupt consumed item must not abort the sweep or block the boot — the good item beside it still acks; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "00-corrupt.json") {
		t.Errorf("a skipped item must be named in a WARN, never swallowed silently; stderr=%q", stderr.String())
	}
}

// TestBlockerBreakerHalt_NoConsumedDirIsQuiet is the zero-value edge: with no
// consumed/ directory, reconciliation is a silent no-op and the breaker
// halts exactly as it always did.
func TestBlockerBreakerHalt_NoConsumedDirIsQuiet(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr, testRootSignals(t, &stderr)); !halted {
		t.Fatal("with nothing consumed, the breaker must halt exactly as before the fix")
	}
	if strings.Contains(stderr.String(), "WARN") && strings.Contains(stderr.String(), "consumed") {
		t.Errorf("an absent consumed/ directory is the normal case and must not WARN; stderr=%q", stderr.String())
	}
}
