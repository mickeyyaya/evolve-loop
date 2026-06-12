package triagecap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAmplifiedCommittedFloorCount_CountsDuplicateDeclarationsExactly(t *testing.T) {
	dir := t.TempDir()
	companion := filepath.Join(dir, TriageDecisionName())
	data := `{
		"top_n": [{"id": "declarative-floor-counter"}],
		"committed_floors": [
			"go/internal/triagecap",
			"go/internal/triagecap",
			"schemas/handoff"
		]
	}`
	if err := os.WriteFile(companion, []byte(data), 0o600); err != nil {
		t.Fatalf("write companion: %v", err)
	}

	got := CommittedFloorCount(
		"prose intentionally has no floor package mentions",
		companion,
		[]string{"go/internal/triagecap", "schemas/handoff"},
	)
	if got != 3 {
		t.Fatalf("duplicate declared floors must count exactly as declarations: got %d, want 3", got)
	}
}

func TestAmplifiedCommittedFloorCount_FallsBackWhenDeclarationFieldAbsent(t *testing.T) {
	dir := t.TempDir()
	companion := filepath.Join(dir, TriageDecisionName())
	if err := os.WriteFile(companion, []byte(`{"top_n":[{"id":"legacy-artifact"}]}`), 0o600); err != nil {
		t.Fatalf("write companion: %v", err)
	}

	want := CountCommittedFloors(proseFloors3, knownPkgsFixture)
	if want == 0 {
		t.Fatalf("invalid fallback fixture: legacy prose counter saw zero floors")
	}

	got := CommittedFloorCount(proseFloors3, companion, knownPkgsFixture)
	if got != want {
		t.Fatalf("missing committed_floors must fall back to prose counter: got %d, want %d", got, want)
	}
}

func TestAmplifiedCommittedFloorCount_FallsBackOnMalformedCompanion(t *testing.T) {
	dir := t.TempDir()
	companion := filepath.Join(dir, TriageDecisionName())
	if err := os.WriteFile(companion, []byte(`{"committed_floors": [`), 0o600); err != nil {
		t.Fatalf("write companion: %v", err)
	}

	want := CountCommittedFloors(proseFloors3, knownPkgsFixture)
	if want == 0 {
		t.Fatalf("invalid fallback fixture: legacy prose counter saw zero floors")
	}

	got := CommittedFloorCount(proseFloors3, companion, knownPkgsFixture)
	if got != want {
		t.Fatalf("malformed companion must fail open to prose counter: got %d, want %d", got, want)
	}
}
