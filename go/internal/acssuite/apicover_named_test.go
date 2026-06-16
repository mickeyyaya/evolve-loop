package acssuite

import (
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
