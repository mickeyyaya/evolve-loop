package main

// cmd_loop_blockerbreaker_reconcile_test.go — RED contract for Defect A of
// the cycle-1335 incident (fault-localization-report.md E6, Defect A).
//
// The defect, verified on live state: .evolve/resolved-fingerprints.json
// DOES NOT EXIST, while the P0 that named the halting fingerprint sits
// consumed at .evolve/inbox/consumed/ with a consumed_by narrative that
// core.ParseConsumptionFingerprint parses correctly TODAY. The extraction
// logic exists and works; nothing non-interactive ever calls it. The only
// writer is the operator-invoked `evolve inbox ack-fingerprint`.
//
// The fix makes the ledger a PROJECTION of the consumed corpus (the repo's
// single-source-with-projection convention, cited by name at
// postship.go:110-113 / ADR-0047): blockerBreakerHalt reconciles
// .evolve/inbox/consumed/ into the ledger before loading it. This is the
// only variant that self-heals the CURRENT live state, where the P0 is
// already consumed and the ledger does not exist.
//
// Two load-bearing constraints, both verified against live data:
//
//  1. Gate on PARSE-SUCCESS, never on `kind`. Enumerating every live inbox
//     item, kind:"pipeline-defect" matches ZERO items; the incident's own P0
//     and the driving item are both kind:"pipeline-repair". A kind-gated
//     implementation passes every fixture test and never fires in production
//     — the unit-green/live-green trap that produced this incident.
//  2. A reconciler defect must not become a new boot blocker: per-item
//     errors WARN and the sweep continues.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realConsumedByNarrative is the consumed_by string carried verbatim by the
// live incident item
// (.evolve/inbox/consumed/2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json).
// Fixtures use the live shape, never a synthetic one.
const realConsumedByNarrative = "console-2026-08-05: fingerprint ship|unknown|76d0f4fca190 = root cause fixed"

// incidentFingerprint is the live fingerprint from the cycle-1335 incident,
// carried verbatim by the digests of cycles 1326/1328/1329 on the real tree.
const incidentFingerprint = "ship|unknown|76d0f4fca190"

// writeConsumedItem drops one JSON item into .evolve/inbox/consumed/ — the
// operator-managed terminal ledger that holds 41 live items and that no Go
// code writes today.
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

// TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem replays the live
// state exactly: the three incident digests are on disk, the P0 naming the
// fingerprint is ALREADY sitting in consumed/, and the ledger does not
// exist. The breaker must reconcile and not halt — and must materialize the
// ledger, so the projection is durable rather than recomputed silently.
//
// The fixture's kind is "pipeline-repair" (the real live value), NOT the
// "pipeline-defect" that matches zero production items: a kind-gated
// implementation must fail this predicate.
func TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")
	writeConsumedItem(t, evolveDir, "2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json",
		`{"id":"pipeline-blocker","kind":"pipeline-repair","consumed_by":"`+realConsumedByNarrative+`"}`)

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr); halted {
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
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr); halted {
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
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr); !halted {
		t.Fatal("consuming an UNRELATED fingerprint must not excuse the halting one — reconciliation is per-fingerprint, never a blanket Rule B disable")
	}
}

// TestBlockerBreakerHalt_ReconcileSurvivesUnreadableItem pins the
// fail-loud-but-never-block contract: a corrupt file in consumed/ must WARN
// and let the sweep continue to the good item beside it. A reconciler defect
// must never become a new boot blocker (the failure mode this whole cycle
// exists to remove).
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
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr); halted {
		t.Fatalf("one corrupt consumed item must not abort the sweep or block the boot — the good item beside it still acks; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "00-corrupt.json") {
		t.Errorf("a skipped item must be named in a WARN, never swallowed silently; stderr=%q", stderr.String())
	}
}

// TestBlockerBreakerHalt_NoConsumedDirIsQuiet is the zero-value edge: a tree
// with no consumed/ directory at all reconciles to nothing, silently, and
// leaves the pre-fix behavior byte-identical.
func TestBlockerBreakerHalt_NoConsumedDirIsQuiet(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 1326, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1328, incidentFingerprint, "gate-block")
	writeDigestFixture(t, evolveDir, 1329, incidentFingerprint, "gate-block")

	var stderr bytes.Buffer
	if _, halted := blockerBreakerHalt(evolveDir, root, 1325, &stderr); !halted {
		t.Fatal("with nothing consumed, the breaker must halt exactly as before the fix")
	}
	if strings.Contains(stderr.String(), "WARN") && strings.Contains(stderr.String(), "consumed") {
		t.Errorf("an absent consumed/ directory is the normal case and must not WARN; stderr=%q", stderr.String())
	}
}
