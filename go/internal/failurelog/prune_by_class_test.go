package failurelog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPruneByClassification_RemovesMatches(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "classification": "infrastructure-systemic"},
		{"cycle": float64(2), "classification": "infrastructure-transient"},
		{"cycle": float64(3), "classification": "ship-gate-config"},
		{"cycle": float64(4), "classification": "code-audit-fail"}, // not in target — keep
	})
	res, err := PruneByClassification(path, []Classification{
		InfrastructureSystemic, InfrastructureTransient, ShipGateConfig,
	})
	if err != nil {
		t.Fatalf("PruneByClassification: %v", err)
	}
	if res.Before != 4 || res.After != 1 || res.Removed != 3 {
		t.Fatalf("result=%+v want {4,1,3}", res)
	}
	state := readState(t, path)
	kept := state["failedApproaches"].([]any)
	if len(kept) != 1 || kept[0].(map[string]any)["cycle"].(float64) != 4 {
		t.Fatalf("kept=%+v want only cycle 4", kept)
	}
}

func TestPruneByClassification_EmptyClasses(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "classification": "infrastructure-systemic"},
	})
	res, err := PruneByClassification(path, nil)
	if err != nil {
		t.Fatalf("PruneByClassification(nil): %v", err)
	}
	if res.Removed != 0 {
		t.Fatalf("empty classes should be no-op; removed=%d", res.Removed)
	}
}

func TestPruneByClassification_NoMatches(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "classification": "code-audit-fail"},
	})
	res, err := PruneByClassification(path, []Classification{InfrastructureSystemic})
	if err != nil {
		t.Fatalf("PruneByClassification: %v", err)
	}
	if res.Removed != 0 {
		t.Fatalf("no matches should yield removed=0; got %d", res.Removed)
	}
}

func TestPruneByClassification_KeepsLegacyAndNonObject(t *testing.T) {
	t.Parallel()
	// Mix: classification-less entry (legacy) + non-object + matching
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := map[string]any{
		"failedApproaches": []any{
			map[string]any{"cycle": float64(1)}, // no classification — keep
			"just-a-string",                      // non-object — keep
			map[string]any{"cycle": float64(2), "classification": "infrastructure-systemic"}, // match — drop
		},
	}
	b, _ := json.Marshal(state)
	_ = os.WriteFile(path, b, 0o644)

	res, err := PruneByClassification(path, []Classification{InfrastructureSystemic})
	if err != nil {
		t.Fatalf("PruneByClassification: %v", err)
	}
	if res.Removed != 1 {
		t.Fatalf("removed=%d want 1 (only the typed entry)", res.Removed)
	}
	kept := readState(t, path)["failedApproaches"].([]any)
	if len(kept) != 2 {
		t.Fatalf("kept=%d want 2 (legacy + string)", len(kept))
	}
}

func TestPruneByClassification_MissingStateOK(t *testing.T) {
	t.Parallel()
	res, err := PruneByClassification(filepath.Join(t.TempDir(), "nope.json"),
		[]Classification{InfrastructureSystemic})
	if err != nil {
		t.Fatalf("missing state should not error: %v", err)
	}
	if res.Before != 0 {
		t.Fatalf("res=%+v want zero", res)
	}
}

func TestPruneByClassification_BadJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	_ = os.WriteFile(path, []byte("{bad"), 0o644)
	if _, err := PruneByClassification(path, []Classification{InfrastructureSystemic}); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestPruneByClassification_EmptyEntries(t *testing.T) {
	t.Parallel()
	path := seedStateWithEntries(t, []map[string]any{})
	res, err := PruneByClassification(path, []Classification{InfrastructureSystemic})
	if err != nil {
		t.Fatalf("PruneByClassification: %v", err)
	}
	if res.Before != 0 {
		t.Fatalf("empty list should yield before=0; got %d", res.Before)
	}
}

// TestPruneByClassification_ReadStateError covers the `read state` error
// branch (non-NotExist read failure, e.g., permission denied).
func TestPruneByClassification_ReadStateError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod doesn't restrict reads")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	_ = os.WriteFile(path, []byte(`{}`), 0o644)
	_ = os.Chmod(path, 0o000)
	defer os.Chmod(path, 0o644)
	if _, err := PruneByClassification(path, []Classification{InfrastructureSystemic}); err == nil {
		t.Fatalf("expected read error from unreadable state.json")
	}
}

func TestPruneByClassification_AtomicWriteError(t *testing.T) {
	// NOT t.Parallel — mutates package-level atomicWriteJSON.
	prev := atomicWriteJSON
	defer func() { atomicWriteJSON = prev }()
	atomicWriteJSON = func(string, map[string]any) error {
		return errors.New("synthetic write error")
	}
	path := seedStateWithEntries(t, []map[string]any{
		{"cycle": float64(1), "classification": "infrastructure-systemic"},
	})
	if _, err := PruneByClassification(path, []Classification{InfrastructureSystemic}); err == nil {
		t.Fatalf("expected write error")
	}
}
