package phasetiming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Diagnostics is the phase's OWN structured notes (severity + message), carried
// on the C1 record so a phase that returns FAIL by its own Classify (triage's
// protected-surface admission rejection, cycles 1634/1636) seals with the
// reason it gave, not with a synthesized "phase-infra class" marker. omitempty:
// a legacy log without the key parses to nil — absent, never fabricated.
func TestEntry_DiagnosticsRideTheRecordAndAreOmittedWhenAbsent(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	in := []Entry{{Phase: "triage", Verdict: "FAIL", Diagnostics: []cyclestate.Diagnostic{{Severity: "error", Message: "top_n card names protected surface"}}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"diagnostics":[{"severity":"error","message":"top_n card names protected surface"}]`) {
		t.Errorf("diagnostics not persisted under `diagnostics`: %s", raw)
	}
	if err := os.WriteFile(Path(ws), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	back, err := Read(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || len(back[0].Diagnostics) != 1 || back[0].Diagnostics[0].Message != "top_n card names protected surface" {
		t.Errorf("diagnostics did not survive the round trip: %+v", back)
	}
	raw, err = json.Marshal([]Entry{{Phase: "scout", Verdict: "PASS"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "diagnostics") {
		t.Errorf("an entry without diagnostics must not emit the key (legacy logs stay byte-identical): %s", raw)
	}
	legacy := filepath.Join(ws, FileName)
	if err := os.WriteFile(legacy, []byte(`[{"phase":"build","duration_ms":1,"verdict":"FAIL","cost_usd":0,"attempt_count":1}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	back, err = Read(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].Diagnostics != nil {
		t.Errorf("a legacy log must parse with nil diagnostics, got %+v", back)
	}
}
