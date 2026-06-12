package triagecap

// malformed_floors_amplified_test.go — cycle-308 adversarial amplification
// for MalformedCommittedFloorWarning and MalformedDeferredFloorWarning
// (companion-malformed-must-surface task).
//
// Targets gaps in the TDD contract: empty-file (0 bytes), truncated JSON, and
// cross-function consistency on the same malformed companion.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeEmptyCompanion writes a 0-byte triage-decision.json.
// An empty file is "present" (stat succeeds) but is not valid JSON — the
// three-case spec treats this as "present-but-malformed", not "absent".
func writeEmptyCompanion(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeTruncatedCompanion writes a triage-decision.json that starts valid but
// is cut off mid-field — a different malformed pattern from writeMalformedCompanion
// which has embedded garbage tokens.
func writeTruncatedCompanion(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte(`{"committed_floors":`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Empty-file boundary
// ---------------------------------------------------------------------------

// TestMalformedFloorWarning_EmptyFileSurfacesCommitted: a 0-byte companion is
// present but invalid JSON — MalformedCommittedFloorWarning must surface a
// non-empty warning (the "absent" path requires a missing file, not an empty one).
func TestMalformedFloorWarning_EmptyFileSurfacesCommitted(t *testing.T) {
	comp := writeEmptyCompanion(t, t.TempDir())
	warn := MalformedCommittedFloorWarning(comp)
	if warn == "" {
		t.Fatal("0-byte companion is present-but-malformed; MalformedCommittedFloorWarning must surface a warning (not treat it as absent)")
	}
}

// TestMalformedFloorWarning_EmptyFileSurfacesDeferred: same 0-byte companion
// must surface via MalformedDeferredFloorWarning too.
func TestMalformedFloorWarning_EmptyFileSurfacesDeferred(t *testing.T) {
	comp := writeEmptyCompanion(t, t.TempDir())
	warn := MalformedDeferredFloorWarning(comp)
	if warn == "" {
		t.Fatal("0-byte companion is present-but-malformed; MalformedDeferredFloorWarning must surface a warning (not treat it as absent)")
	}
}

// ---------------------------------------------------------------------------
// Truncated JSON boundary
// ---------------------------------------------------------------------------

// TestMalformedFloorWarning_TruncatedJsonSurfacesCommitted: a companion whose
// JSON is cut off mid-value is a different malformed pattern (no embedded
// non-JSON tokens) but still invalid — must surface a warning.
func TestMalformedFloorWarning_TruncatedJsonSurfacesCommitted(t *testing.T) {
	comp := writeTruncatedCompanion(t, t.TempDir())
	warn := MalformedCommittedFloorWarning(comp)
	if warn == "" {
		t.Fatal("truncated JSON companion must surface a MalformedCommittedFloorWarning")
	}
}

// TestMalformedFloorWarning_TruncatedJsonSurfacesDeferred: truncated companion
// must surface via MalformedDeferredFloorWarning too.
func TestMalformedFloorWarning_TruncatedJsonSurfacesDeferred(t *testing.T) {
	comp := writeTruncatedCompanion(t, t.TempDir())
	warn := MalformedDeferredFloorWarning(comp)
	if warn == "" {
		t.Fatal("truncated JSON companion must surface a MalformedDeferredFloorWarning")
	}
}

// ---------------------------------------------------------------------------
// Cross-function consistency
// ---------------------------------------------------------------------------

// TestBothMalformedFunctions_OnSameMalformedFile: a single malformed companion
// must cause BOTH MalformedCommittedFloorWarning and MalformedDeferredFloorWarning
// to surface simultaneously. This pins the invariant that a corrupt companion
// does not selectively hide itself from one function while surfacing in the other.
func TestBothMalformedFunctions_OnSameMalformedFile(t *testing.T) {
	comp := writeMalformedCompanion(t, t.TempDir()) // defined in malformed_floors_test.go

	committedWarn := MalformedCommittedFloorWarning(comp)
	deferredWarn := MalformedDeferredFloorWarning(comp)

	if committedWarn == "" {
		t.Error("malformed companion must surface MalformedCommittedFloorWarning")
	}
	if deferredWarn == "" {
		t.Error("malformed companion must surface MalformedDeferredFloorWarning")
	}
}

// TestMalformedFloorWarning_ProseCountUnaffected_EmptyFile: a 0-byte companion
// must not break CommittedFloorCount or DeferredFloorPackagesDecl — prose fallback
// must activate and produce the expected count. This extends the TDD "prose fallback
// preserved" assertion to the empty-file malformed variant.
func TestMalformedFloorWarning_ProseCountUnaffected_EmptyFile(t *testing.T) {
	wantProseCount := CountCommittedFloors(proseFloors3, knownPkgsFixture)
	if wantProseCount != 3 {
		t.Fatalf("fixture precondition: prose count = %d, want 3", wantProseCount)
	}

	comp := writeEmptyCompanion(t, t.TempDir())
	if got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture); got != wantProseCount {
		t.Errorf("CommittedFloorCount on 0-byte companion = %d, want %d (prose fallback must activate)", got, wantProseCount)
	}
}
