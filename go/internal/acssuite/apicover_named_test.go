package acssuite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
)

func TestParseGoTestJSON_SkipCarriesSkipExitCode(t *testing.T) {
	raw := goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Skip", "skip"))
	results := parseGoTestJSON(strings.NewReader(raw), 9)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if got := results[0]; got.ResultStr != "skip" || got.ExitCode != SkipExitCode {
		t.Errorf("skip result = {result:%q exit:%d}, want {skip %d}", got.ResultStr, got.ExitCode, SkipExitCode)
	}
}

func TestWriteVerdict_LandsAtVerdictFilename(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteVerdict(dir, Verdict{Cycle: 1603, Verdict: "PASS"})
	if err != nil {
		t.Fatalf("WriteVerdict: %v", err)
	}
	if filepath.Base(path) != acsverdict.Filename {
		t.Errorf("verdict written to %q, want basename %q", path, acsverdict.Filename)
	}
	if _, err := os.Stat(filepath.Join(dir, "runs", "cycle-1603", acsverdict.Filename)); err != nil {
		t.Errorf("verdict not at the canonical acsverdict.Filename path: %v", err)
	}
}
