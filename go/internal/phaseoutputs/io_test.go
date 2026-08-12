package phaseoutputs

// io_test.go — the one sanctioned workspace reader, shared by both adapters
// (`evolve cycle outputs` and the loop's post-cycle signal emitter) so the two
// cannot drift into reading different files or classifying a read differently.
// Survey/CycleChainStatus/Signal stay pure; this file is their I/O companion.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadListing_NamesAndSizesTopLevelFilesOnly(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ws, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := LoadListing(ws)
	if err != nil {
		t.Fatal(err)
	}
	if got["build-report.md"] != 5 {
		t.Errorf("listing must carry byte sizes (Empty detection depends on it): %v", got)
	}
	if _, ok := got["subdir"]; ok {
		t.Error("directories are not artifacts and must not appear in the listing")
	}
}

func TestLoadShadowReading_TotalizesAllThreeReadOutcomes(t *testing.T) {
	t.Parallel()
	const file = "audit-chain-shadow.json"

	absent := LoadShadowReading(t.TempDir(), file)
	if absent.View != nil || absent.Corrupt {
		t.Errorf("no file on disk must read as the zero reading: %+v", absent)
	}

	corruptWS := t.TempDir()
	if err := os.WriteFile(filepath.Join(corruptWS, file), []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := LoadShadowReading(corruptWS, file)
	if !corrupt.Corrupt || corrupt.View != nil {
		t.Errorf("an existing-but-unparseable record must read Corrupt — classifying it as missing was a review finding: %+v", corrupt)
	}

	parsedWS := t.TempDir()
	rec, _ := json.Marshal(map[string]any{"chain_present": true})
	if err := os.WriteFile(filepath.Join(parsedWS, file), rec, 0o644); err != nil {
		t.Fatal(err)
	}
	parsed := LoadShadowReading(parsedWS, file)
	if parsed.View == nil || !parsed.View.ChainPresent {
		t.Errorf("a parseable record must surface its view: %+v", parsed)
	}
}

func TestLoadCompletedPhases_ReadsRunJSON(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	run, _ := json.Marshal(map[string]any{"completed_phases": []string{"scout", "build"}})
	if err := os.WriteFile(filepath.Join(ws, "run.json"), run, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCompletedPhases(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "scout" || got[1] != "build" {
		t.Errorf("completed phases = %v, want [scout build]", got)
	}
	if _, err := LoadCompletedPhases(t.TempDir()); err == nil {
		t.Error("a workspace with no run.json must error loudly — an aborted cycle is a fact, not an empty survey")
	}
}
