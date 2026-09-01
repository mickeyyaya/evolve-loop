package acssuite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseGoTestJSON_SkipCarriesSkipExitCode names the acssuite.SkipExitCode
// const and pins the real branch (acssuite.go:436): a t.Skip'd predicate maps to
// a Result whose ExitCode is the TAP/automake SKIP convention, the value the
// audit/ship gate reads to count it neither red nor green.
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

// TestWriteVerdict_LandsAtVerdictFilename names acssuite.VerdictFilename and
// pins the belief the const exists for: the writer and every external reader/
// retirement path (core/audit_round_artifacts.go, cycle-1603) agree on ONE
// spelling because WriteVerdict itself derives its destination from the const.
func TestWriteVerdict_LandsAtVerdictFilename(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteVerdict(dir, Verdict{Cycle: 1603, Verdict: "PASS"})
	if err != nil {
		t.Fatalf("WriteVerdict: %v", err)
	}
	if filepath.Base(path) != VerdictFilename {
		t.Errorf("verdict written to %q, want basename %q", path, VerdictFilename)
	}
	if _, err := os.Stat(filepath.Join(dir, "runs", "cycle-1603", VerdictFilename)); err != nil {
		t.Errorf("verdict not at the canonical VerdictFilename path: %v", err)
	}
}
