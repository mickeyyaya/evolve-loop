package deliverable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestCountSalvageApplied_CountsOnlyOwnEventType(t *testing.T) {
	t.Parallel()
	const jsonl = `{"event_type":"salvage_applied","phase":"audit","pattern":"fenced-json","run":"111"}
{"event_type":"some_other_emitter","phase":"build","pattern":"fenced-json","run":"111"}

{"event_type":"salvage_applied","phase":"build","pattern":"trailing-comma","run":"222"}
`
	got, _, err := CountSalvageApplied(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("well-formed sidecar must not error: %v", err)
	}
	if got != 2 {
		t.Errorf("CountSalvageApplied = %d, want 2 — the foreign emitter and the blank line must not be counted", got)
	}
}

func TestCountSalvageApplied_EmptyAndTorn(t *testing.T) {
	t.Parallel()
	if got, malformed, err := CountSalvageApplied(strings.NewReader("\n\n")); err != nil || got != 0 || malformed != 0 {
		t.Errorf("empty sidecar: got (%d, %d, %v), want (0, 0, nil)", got, malformed, err)
	}
	got, malformed, err := CountSalvageApplied(strings.NewReader(`{"event_type":"salvage_app`))
	if err != nil {
		t.Errorf("a torn line must not brick the count: %v", err)
	}
	if got != 0 || malformed != 1 {
		t.Errorf("torn line: got (saved=%d, malformed=%d), want (0, 1) — the skip must be counted, not silent", got, malformed)
	}
}

func TestSalvageAppliedFile_IsTheFileTheGateWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordSalvageApplied(phasecontract.Roots{EvolveDir: dir}, "audit", SalvagePatternFencedJSON, nil)

	f, err := os.Open(filepath.Join(dir, SalvageAppliedFile))
	if err != nil {
		t.Fatalf("recordSalvageApplied did not write %s: %v", SalvageAppliedFile, err)
	}
	defer f.Close()
	got, _, err := CountSalvageApplied(f)
	if err != nil {
		t.Fatalf("fold the recorder's own output: %v", err)
	}
	if got != 1 {
		t.Errorf("saved = %d after one recorded salvage, want 1", got)
	}
}
