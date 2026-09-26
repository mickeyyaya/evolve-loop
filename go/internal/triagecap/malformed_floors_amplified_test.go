package triagecap

import (
	"os"
	"path/filepath"
	"testing"
)

// writeEmptyCompanion writes a 0-byte companion: present, so malformed rather than absent.
func writeEmptyCompanion(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeTruncatedCompanion cuts the JSON off mid-value, unlike writeMalformedCompanion's garbage tokens.
func writeTruncatedCompanion(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte(`{"committed_floors":`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMalformedFloorWarning_EmptyFileSurfacesCommitted(t *testing.T) {
	comp := writeEmptyCompanion(t, t.TempDir())
	warn := MalformedCommittedFloorWarning(comp)
	if warn == "" {
		t.Fatal("0-byte companion is present-but-malformed; MalformedCommittedFloorWarning must surface a warning (not treat it as absent)")
	}
}

func TestMalformedFloorWarning_EmptyFileSurfacesDeferred(t *testing.T) {
	comp := writeEmptyCompanion(t, t.TempDir())
	warn := MalformedDeferredFloorWarning(comp)
	if warn == "" {
		t.Fatal("0-byte companion is present-but-malformed; MalformedDeferredFloorWarning must surface a warning (not treat it as absent)")
	}
}

func TestMalformedFloorWarning_TruncatedJsonSurfacesCommitted(t *testing.T) {
	comp := writeTruncatedCompanion(t, t.TempDir())
	warn := MalformedCommittedFloorWarning(comp)
	if warn == "" {
		t.Fatal("truncated JSON companion must surface a MalformedCommittedFloorWarning")
	}
}

func TestMalformedFloorWarning_TruncatedJsonSurfacesDeferred(t *testing.T) {
	comp := writeTruncatedCompanion(t, t.TempDir())
	warn := MalformedDeferredFloorWarning(comp)
	if warn == "" {
		t.Fatal("truncated JSON companion must surface a MalformedDeferredFloorWarning")
	}
}

func TestBothMalformedFunctions_OnSameMalformedFile(t *testing.T) {
	comp := writeMalformedCompanion(t, t.TempDir())

	committedWarn := MalformedCommittedFloorWarning(comp)
	deferredWarn := MalformedDeferredFloorWarning(comp)

	if committedWarn == "" {
		t.Error("malformed companion must surface MalformedCommittedFloorWarning")
	}
	if deferredWarn == "" {
		t.Error("malformed companion must surface MalformedDeferredFloorWarning")
	}
}

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
