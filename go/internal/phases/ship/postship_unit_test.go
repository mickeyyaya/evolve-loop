package ship

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Given a state.json with several pre-existing fields and a
// cycle-state.json:cycle_id, advancing the counter must (a) set
// lastCycleNumber = cycle_id and (b) leave EVERY other field intact.
func TestAdvanceLastCycleNumber_AdvancesAndPreservesSiblings(t *testing.T) {
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	mustWriteState(t, filepath.Join(evolve, "state.json"), map[string]any{
		"lastCycleNumber":       float64(5),
		"expected_ship_sha":     "deadbeef",
		"expected_ship_version": "12.2.2",
		"currentBatch":          map[string]any{"goalHash": "abc123"},
	})
	mustWriteState(t, filepath.Join(evolve, "cycle-state.json"), map[string]any{
		"cycle_id": float64(8),
	})

	opts := &Options{ProjectRoot: root}
	res := &RunResult{}
	if err := advanceLastCycleNumber(opts, res); err != nil {
		t.Fatalf("advanceLastCycleNumber: %v", err)
	}

	got, err := readStateMap(filepath.Join(evolve, "state.json"))
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if n, _ := stateInt(got, "lastCycleNumber"); n != 8 {
		t.Errorf("lastCycleNumber = %d, want 8", n)
	}
	if stateString(got, "expected_ship_sha") != "deadbeef" {
		t.Errorf("expected_ship_sha dropped during counter advance: %v", got["expected_ship_sha"])
	}
	if stateString(got, "expected_ship_version") != "12.2.2" {
		t.Errorf("expected_ship_version dropped: %v", got["expected_ship_version"])
	}
	if _, ok := got["currentBatch"]; !ok {
		t.Errorf("currentBatch nested object dropped during advance")
	}
}

// TestAdvanceLastCycleNumber_NoCycleIDIsNoop: a cycle-state.json without
// cycle_id must leave state.json untouched (bash silently skips).
func TestAdvanceLastCycleNumber_NoCycleIDIsNoop(t *testing.T) {
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	mustWriteState(t, filepath.Join(evolve, "state.json"), map[string]any{"lastCycleNumber": float64(5)})
	mustWriteState(t, filepath.Join(evolve, "cycle-state.json"), map[string]any{"other": "x"})

	opts := &Options{ProjectRoot: root}
	if err := advanceLastCycleNumber(opts, &RunResult{}); err != nil {
		t.Fatalf("advanceLastCycleNumber: %v", err)
	}
	got, _ := readStateMap(filepath.Join(evolve, "state.json"))
	if n, _ := stateInt(got, "lastCycleNumber"); n != 5 {
		t.Errorf("lastCycleNumber should be unchanged at 5; got %d", n)
	}
}

// TestRepinPostCycle_NoopWhenSHAMatches: when the binary's current SHA
// already equals state.json:expected_ship_sha, repin is a no-op.
func TestRepinPostCycle_NoopWhenSHAMatches(t *testing.T) {
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	binPath := filepath.Join(root, "evolve-bin")
	writeFile(t, binPath, "binary-content-v1")
	curSHA, err := sha256File(binPath)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	mustWriteState(t, filepath.Join(evolve, "state.json"), map[string]any{
		"expected_ship_sha": curSHA,
		"sentinel":          "keep-me",
	})

	opts := &Options{ProjectRoot: root, ShipBinaryPath: binPath}
	res := &RunResult{}
	if err := repinPostCycle(opts, res); err != nil {
		t.Fatalf("repinPostCycle: %v", err)
	}
	for _, l := range res.Logs {
		if strings.Contains(l, "TOFU") {
			t.Errorf("unexpected TOFU repin log when SHA matched: %q", l)
		}
	}
}

// TestRepinPostCycle_RepinsOnSHAChange: when the shipped commit changed
// the binary (current SHA != stored), repin writes the new SHA and logs.
func TestRepinPostCycle_RepinsOnSHAChange(t *testing.T) {
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	binPath := filepath.Join(root, "evolve-bin")
	writeFile(t, binPath, "binary-content-v2")
	newSHA, err := sha256File(binPath)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	mustWriteState(t, filepath.Join(evolve, "state.json"), map[string]any{
		"expected_ship_sha": "stale-sha-from-prior-cycle",
		"sentinel":          "keep-me",
	})

	opts := &Options{ProjectRoot: root, ShipBinaryPath: binPath}
	res := &RunResult{}
	if err := repinPostCycle(opts, res); err != nil {
		t.Fatalf("repinPostCycle: %v", err)
	}
	got, _ := readStateMap(filepath.Join(evolve, "state.json"))
	if stateString(got, "expected_ship_sha") != newSHA {
		t.Errorf("expected_ship_sha not repinned; got %v want %s", got["expected_ship_sha"], newSHA)
	}
	if stateString(got, "sentinel") != "keep-me" {
		t.Errorf("repin dropped sibling field 'sentinel'")
	}
	if !anyContains(res.Logs, "TOFU") {
		t.Errorf("expected a TOFU repin log line; got %v", res.Logs)
	}
}

// TestPromoteInbox_NoCycleIDIsNoop: without cycle_id, promote returns nil
// silently (no triage lookup attempted).
func TestPromoteInbox_NoCycleIDIsNoop(t *testing.T) {
	root := t.TempDir()
	mustWriteState(t, filepath.Join(root, ".evolve", "cycle-state.json"), map[string]any{"x": "y"})
	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
}

// TestPromoteInbox_NoTriageDecisionStillDrains: cycle_id present but no
// triage-decision.json ⇒ promote-to-processed is skipped (logged) but the
// residual drain STILL runs (no early-return strand).
func TestPromoteInbox_NoTriageDecisionStillDrains(t *testing.T) {
	root := t.TempDir()
	mustWriteState(t, filepath.Join(root, ".evolve", "cycle-state.json"), map[string]any{"cycle_id": float64(8)})
	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}
	if !anyContains(res.Logs, "promote-to-processed skipped") {
		t.Errorf("expected promote-skipped INFO log; got %v", res.Logs)
	}
	if !anyContains(res.Logs, "inbox lifecycle drain complete") {
		t.Errorf("residual drain must still run when triage-decision.json is absent; got %v", res.Logs)
	}
}

func mustWriteState(t *testing.T, path string, m map[string]any) {
	t.Helper()
	if err := writeStateMap(path, m); err != nil {
		t.Fatalf("writeStateMap %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func anyContains(logs []string, sub string) bool {
	for _, l := range logs {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
