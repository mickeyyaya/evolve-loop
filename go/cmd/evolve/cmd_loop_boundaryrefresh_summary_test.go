package main

// acs-predicate: config-check — T6/T7 are caller-existence checks; the value
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	logPath := filepath.Join(evolveDir, chainBoundaryRefreshLogFile)
	entries := []chainBoundaryRefreshLogEntry{
		{Batch: 1, AuthorizedClass: "boundary-refresh", Timestamp: "2026-08-05T01:00:00Z", OldSHA: "aaaa1111", NewSHA: "bbbb2222"},
		{Batch: 4, AuthorizedClass: "boundary-refresh", Timestamp: "2026-08-05T02:00:00Z", OldSHA: "bbbb2222", NewSHA: "cccc3333"},
		{Batch: 9, AuthorizedClass: "boundary-refresh", Timestamp: "2026-08-05T03:00:00Z", OldSHA: "cccc3333", NewSHA: "dddd4444"},
	}
	var buf []byte
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal fixture entry: %v", err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(logPath, buf, 0o644); err != nil {
		t.Fatalf("write fixture log: %v", err)
	}

	got, err := lastChainBoundaryRefreshLogEntry(evolveDir)
	if err != nil {
		t.Fatalf("lastChainBoundaryRefreshLogEntry: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("lastChainBoundaryRefreshLogEntry returned nil for a non-empty log")
	}
	want := entries[len(entries)-1]
	if got.Batch != want.Batch || got.OldSHA != want.OldSHA || got.NewSHA != want.NewSHA || got.Timestamp != want.Timestamp {
		t.Errorf("got %+v, want the LAST fixture entry %+v", *got, want)
	}
}

func TestLastChainBoundaryRefreshLogEntry_MissingFileIsNilNoError(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	got, err := lastChainBoundaryRefreshLogEntry(evolveDir)
	if err != nil {
		t.Errorf("missing log file must degrade to nil error (fail-open), got: %v", err)
	}
	if got != nil {
		t.Errorf("missing log file must return nil entry, got %+v", *got)
	}
}

func TestLastChainBoundaryRefreshLogEntry_EmptyFileIsNilNoError(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, chainBoundaryRefreshLogFile), nil, 0o644); err != nil {
		t.Fatalf("write empty fixture log: %v", err)
	}
	got, err := lastChainBoundaryRefreshLogEntry(evolveDir)
	if err != nil {
		t.Errorf("empty log file must degrade to nil error, got: %v", err)
	}
	if got != nil {
		t.Errorf("empty log file must return nil entry, got %+v", *got)
	}
}

func TestChainResult_MarshalOmitsBoundaryRefreshWhenNil(t *testing.T) {
	t.Parallel()
	res := chainResult{ChainMode: true, MaxBatches: 5, StopReason: "chain_inbox_empty"}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal chainResult: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal marshaled chainResult: %v", err)
	}
	if _, present := doc["boundary_refresh"]; present {
		t.Errorf("chainResult JSON must OMIT boundary_refresh when nil, got key present: %s", buf)
	}
}

func TestLoopResult_MarshalOmitsBoundaryRefreshWhenNil(t *testing.T) {
	t.Parallel()
	lr := loopResult{StopReason: "max_cycles_reached"}
	buf, err := json.Marshal(lr)
	if err != nil {
		t.Fatalf("marshal loopResult: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal marshaled loopResult: %v", err)
	}
	if _, present := doc["boundary_refresh"]; present {
		t.Errorf("loopResult JSON must OMIT boundary_refresh when nil, got key present: %s", buf)
	}
}

// acs-predicate: config-check — lastChainBoundaryRefreshLogEntry's own
func TestRunLoopChain_SetsBoundaryRefreshOnReExecStop(t *testing.T) {
	t.Parallel()
	n, err := acsassert.CountInGoFunc(filepath.Join("..", "..", "internal", "loopchain", "driver.go"), "boundary", "res.BoundaryRefresh")
	if err != nil {
		t.Fatalf("CountInGoFunc(Driver.boundary, res.BoundaryRefresh): %v", err)
	}
	if n < 1 {
		t.Errorf("Driver.boundary does not set res.BoundaryRefresh (count=%d); the boundary-refresh-log.jsonl audit record stays invisible to the chain summary JSON", n)
	}
}

// acs-predicate: config-check — see T6.
func TestRunLoopBatch_SetsBoundaryRefreshOnWaveBoundaryReExecStop(t *testing.T) {
	t.Parallel()
	n, err := acsassert.CountInGoFunc("cmd_loop_window.go", "prepareIteration", "b.result.BoundaryRefresh")
	if err != nil {
		t.Fatalf("CountInGoFunc(prepareIteration, b.result.BoundaryRefresh): %v", err)
	}
	if n < 1 {
		t.Errorf("runLoopBatch does not set lr.BoundaryRefresh (count=%d); the wave/fleet boundary's refresh events stay invisible to the loop summary JSON even though runLoopChain's are surfaced", n)
	}
}

func TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent(t *testing.T) {
	t.Parallel()
	for _, f := range []string{filepath.Join("..", "..", "internal", "loopchain", "driver.go"), "cmd_loop_outcome.go"} {
		if !strings.Contains(mustReadFile(t, f), `json:"boundary_refresh,omitempty"`) {
			t.Errorf("%s: expected a `json:\"boundary_refresh,omitempty\"` struct tag (nil-when-clean, mirrors spine_fail_opens)", f)
		}
	}
}

func mustReadFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
