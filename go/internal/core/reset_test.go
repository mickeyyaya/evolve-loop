package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// reset_test.go — SealCycle abandons a stuck/unfinished cycle while
// PRESERVING its history (workspace + cycle-state snapshot + manifest) in a
// self-contained archive, advances lastCycleNumber so the number is never
// reused, and records an auditable ledger entry. Mirrors resume_test.go's
// table-driven + temp-dir + seam-injection style.

// recordingLedger is a core.Ledger that captures appended entries.
type recordingLedger struct{ entries []LedgerEntry }

func (r *recordingLedger) Append(_ context.Context, e LedgerEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

// sealFixture seeds .evolve/{cycle-state.json,state.json,runs/cycle-<id>}.
func sealFixture(t *testing.T, evolveDir string, cycleID int) (workspace string) {
	t.Helper()
	workspace = filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "scout-report.md"), []byte("partial\n"), 0o644); err != nil {
		t.Fatalf("seed workspace file: %v", err)
	}
	cs := map[string]any{
		"cycle_id":       cycleID,
		"phase":          "scout",
		"active_agent":   "scout",
		"workspace_path": workspace,
	}
	writeJSONFixture(t, filepath.Join(evolveDir, "cycle-state.json"), cs)
	// state.json carries a field (expected_ship_sha) that the typed core.State
	// struct does NOT model — the seal must preserve it (full-fidelity map write).
	st := map[string]any{
		"lastCycleNumber":   cycleID - 1,
		"version":           18,
		"currentBatch":      map[string]any{"cycleAccruedCostUSD": 239.2},
		"expected_ship_sha": "deadbeef-must-survive",
	}
	writeJSONFixture(t, filepath.Join(evolveDir, "state.json"), st)
	return workspace
}

func writeJSONFixture(t *testing.T, path string, body any) {
	t.Helper()
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func sealOpts(evolveDir string) SealOptions {
	return SealOptions{
		EvolveDir:   evolveDir,
		ProjectRoot: evolveDir,
		Reason:      "operator reset (test)",
		Now:         func() time.Time { return time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC) },
		GitHead:     func(string) (string, error) { return "testhead123", nil },
	}
}

func TestSealCycle_NothingToReset(t *testing.T) {
	t.Parallel()
	t.Run("missing cycle-state", func(t *testing.T) {
		ev := t.TempDir()
		_, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev))
		if !errors.Is(err, ErrNothingToReset) {
			t.Fatalf("want ErrNothingToReset, got %v", err)
		}
	})
	t.Run("zero cycle_id", func(t *testing.T) {
		ev := t.TempDir()
		writeJSONFixture(t, filepath.Join(ev, "cycle-state.json"), map[string]any{"cycle_id": 0})
		_, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev))
		if !errors.Is(err, ErrNothingToReset) {
			t.Fatalf("want ErrNothingToReset, got %v", err)
		}
	})
}

func TestSealCycle_HappyPath(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	workspace := sealFixture(t, ev, 108)
	led := &recordingLedger{}

	res, err := SealCycle(context.Background(), led, sealOpts(ev))
	if err != nil {
		t.Fatalf("SealCycle: %v", err)
	}

	// Result.
	if res.SealedCycleID != 108 || res.NextCycle != 109 {
		t.Fatalf("sealed=%d next=%d, want 108/109", res.SealedCycleID, res.NextCycle)
	}
	if res.SealedPhase != "scout" {
		t.Errorf("sealed phase = %q, want scout", res.SealedPhase)
	}

	// History sealed: original workspace gone, archive holds the workspace file
	// + a cycle-state snapshot + a reset manifest.
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Errorf("original workspace should be moved into the archive; stat err=%v", err)
	}
	for _, want := range []string{"scout-report.md", "cycle-state.snapshot.json", "reset-manifest.json"} {
		if _, err := os.Stat(filepath.Join(res.ArchiveDir, want)); err != nil {
			t.Errorf("archive missing %s: %v", want, err)
		}
	}

	// cycle-state.json cleared (the abandon commit point).
	if _, err := os.Stat(filepath.Join(ev, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("cycle-state.json should be removed; stat err=%v", err)
	}

	// state.json: lastCycleNumber advanced, batch zeroed, unknown field PRESERVED.
	sm := readJSONMap(t, filepath.Join(ev, "state.json"))
	if got := intFromAny(sm["lastCycleNumber"]); got != 108 {
		t.Errorf("lastCycleNumber = %d, want 108", got)
	}
	cb, _ := sm["currentBatch"].(map[string]any)
	if got := floatFromAny(cb["cycleAccruedCostUSD"]); got != 0 {
		t.Errorf("currentBatch.cycleAccruedCostUSD = %v, want 0", got)
	}
	if got := strFromAny(sm["expected_ship_sha"]); got != "deadbeef-must-survive" {
		t.Errorf("expected_ship_sha must be preserved through the seal; got %q", got)
	}

	// Auditable ledger entry.
	if len(led.entries) != 1 {
		t.Fatalf("want 1 ledger entry, got %d", len(led.entries))
	}
	e := led.entries[0]
	if e.Cycle != 0 || e.CycleLabel != "reset-seal-cycle-108" {
		t.Errorf("ledger entry cycle=%d label=%q, want 0/reset-seal-cycle-108", e.Cycle, e.CycleLabel)
	}
	if e.Kind != "reset" {
		t.Errorf("ledger kind = %q, want reset", e.Kind)
	}

	// Manifest content sanity.
	man := readJSONMap(t, filepath.Join(res.ArchiveDir, "reset-manifest.json"))
	if intFromAny(man["sealed_cycle"]) != 108 || strFromAny(man["git_head"]) != "testhead123" {
		t.Errorf("manifest mismatch: %+v", man)
	}
}

func TestSealCycle_DryRunMutatesNothing(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	workspace := sealFixture(t, ev, 108)
	led := &recordingLedger{}

	opts := sealOpts(ev)
	opts.DryRun = true
	res, err := SealCycle(context.Background(), led, opts)
	if err != nil {
		t.Fatalf("dry-run SealCycle: %v", err)
	}
	if !res.DryRun || res.SealedCycleID != 108 || res.NextCycle != 109 {
		t.Fatalf("dry-run result = %+v", res)
	}
	// Nothing mutated.
	if _, err := os.Stat(workspace); err != nil {
		t.Errorf("workspace must be untouched in dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ev, "cycle-state.json")); err != nil {
		t.Errorf("cycle-state.json must survive dry-run: %v", err)
	}
	if _, err := os.Stat(res.ArchiveDir); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create the archive dir")
	}
	if len(led.entries) != 0 {
		t.Errorf("dry-run must not append to the ledger; got %d", len(led.entries))
	}
	sm := readJSONMap(t, filepath.Join(ev, "state.json"))
	if intFromAny(sm["lastCycleNumber"]) != 107 {
		t.Errorf("dry-run must not bump lastCycleNumber")
	}
}

func TestSealCycle_EmptyWorkspaceStillSeals(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	// cycle-state present but the workspace dir was never created.
	workspace := filepath.Join(ev, "runs", "cycle-108")
	writeJSONFixture(t, filepath.Join(ev, "cycle-state.json"), map[string]any{
		"cycle_id": 108, "phase": "scout", "workspace_path": workspace,
	})
	writeJSONFixture(t, filepath.Join(ev, "state.json"), map[string]any{"lastCycleNumber": 107})

	res, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev))
	if err != nil {
		t.Fatalf("SealCycle (empty workspace): %v", err)
	}
	// Archive dir is created even with no workspace, holding the snapshot+manifest.
	for _, want := range []string{"cycle-state.snapshot.json", "reset-manifest.json"} {
		if _, err := os.Stat(filepath.Join(res.ArchiveDir, want)); err != nil {
			t.Errorf("archive missing %s: %v", want, err)
		}
	}
}

func TestSealCycle_RefusesWorkspaceOutsideRoots(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere", "cycle-108")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	writeJSONFixture(t, filepath.Join(ev, "cycle-state.json"), map[string]any{
		"cycle_id": 108, "phase": "scout", "workspace_path": outside,
	})
	writeJSONFixture(t, filepath.Join(ev, "state.json"), map[string]any{"lastCycleNumber": 107})

	_, err := SealCycle(context.Background(), &recordingLedger{}, sealOpts(ev))
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("want refusal for out-of-root workspace_path, got %v", err)
	}
	// The out-of-root directory must be untouched (not renamed).
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Errorf("out-of-root dir must not be moved; stat err=%v", statErr)
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}
