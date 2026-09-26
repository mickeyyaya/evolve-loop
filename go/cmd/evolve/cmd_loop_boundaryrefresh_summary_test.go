package main

// acs-predicate: config-check — T6/T7 are caller-existence checks; the value
// they surface is already pinned behaviorally by T1-T5.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- T1/T2/T3: lastChainBoundaryRefreshLogEntry ---

// T1 (positive): three appended JSONL records — the helper must return the
// LAST one, with every field intact, not the first (a common off-by-one when
// hand-rolling a tail read) and not a merge/aggregate of all three.
func TestLastChainBoundaryRefreshLogEntry_ReturnsMostRecentEntry(t *testing.T) {
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

// T2 (edge): no log file at all — must be a quiet (nil, nil), never an
// error. A boundary refresh that has simply never happened yet on this
// plane is the overwhelmingly common case and must not spam stderr/fail a
// summary emission.
func TestLastChainBoundaryRefreshLogEntry_MissingFileIsNilNoError(t *testing.T) {
	evolveDir := t.TempDir() // no boundary-refresh-log.jsonl written
	got, err := lastChainBoundaryRefreshLogEntry(evolveDir)
	if err != nil {
		t.Errorf("missing log file must degrade to nil error (fail-open), got: %v", err)
	}
	if got != nil {
		t.Errorf("missing log file must return nil entry, got %+v", *got)
	}
}

// T3 (edge): a present-but-empty log file (e.g. truncated, or created but
// never appended to) also degrades to (nil, nil) — the same fail-open
// contract as a missing file, not a parse error.
func TestLastChainBoundaryRefreshLogEntry_EmptyFileIsNilNoError(t *testing.T) {
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

// --- T4/T5: exact-omission marshal assertions ---

// T4 (negative): chainResult with a nil BoundaryRefresh must OMIT the
// "boundary_refresh" key entirely from the marshaled JSON, not emit
// `"boundary_refresh": null`. A consumer that checks key-presence (not
// merely non-null) to decide "did a refresh happen this run" must see a
// clean, absent key on every ordinary run.
func TestChainResult_MarshalOmitsBoundaryRefreshWhenNil(t *testing.T) {
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

// T5 mirrors T4 for loopResult (the non-chain wave/fleet summary).
func TestLoopResult_MarshalOmitsBoundaryRefreshWhenNil(t *testing.T) {
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

// --- T6/T7: wiring proofs (structural, config-check waived) ---

// T6: the chain must populate res.BoundaryRefresh at the same call site
// that already sets res.StopReason = "chain_boundary_refresh_reexec" —
// otherwise the durable audit record the mechanism already writes to
// boundary-refresh-log.jsonl stays permanently invisible to the JSON
// summary an operator/dossier consumer actually reads. That call site is
// loopchain.(*Driver).boundary (the Result runLoopChain prints IS the
// summary schema), so the proof reads the leaf.
// See ADR-0103.
//
// acs-predicate: config-check — lastChainBoundaryRefreshLogEntry's own
// correctness is proven by T1-T3; this only proves the chain calls it.
func TestRunLoopChain_SetsBoundaryRefreshOnReExecStop(t *testing.T) {
	n, err := acsassert.CountInGoFunc(filepath.Join("..", "..", "internal", "loopchain", "driver.go"), "boundary", "res.BoundaryRefresh")
	if err != nil {
		t.Fatalf("CountInGoFunc(Driver.boundary, res.BoundaryRefresh): %v", err)
	}
	if n < 1 {
		t.Errorf("Driver.boundary does not set res.BoundaryRefresh (count=%d); the boundary-refresh-log.jsonl audit record stays invisible to the chain summary JSON", n)
	}
}

// T7 mirrors T6 for runLoopBatch's wave/fleet boundary stage
// (cmd_loop_window.go), the non-chain caller maybeRefreshChainBoundary also
// fires from. Both callers of the same refresh mechanism must surface the
// same summary field — surfacing only the chain-mode caller would silently
// leave the plain `evolve loop --max-cycles N` / fleet path's refresh
// events unobservable.
//
// acs-predicate: config-check — see T6.
func TestRunLoopBatch_SetsBoundaryRefreshOnWaveBoundaryReExecStop(t *testing.T) {
	n, err := acsassert.CountInGoFunc("cmd_loop_window.go", "prepareIteration", "b.result.BoundaryRefresh")
	if err != nil {
		t.Fatalf("CountInGoFunc(prepareIteration, b.result.BoundaryRefresh): %v", err)
	}
	if n < 1 {
		t.Errorf("runLoopBatch does not set lr.BoundaryRefresh (count=%d); the wave/fleet boundary's refresh events stay invisible to the loop summary JSON even though runLoopChain's are surfaced", n)
	}
}

// sanity guard against a degenerate "field exists but is spelled wrong"
// implementation slipping past T4/T5 (which only assert absence-when-nil,
// not the exact field name): confirm the json tag string itself is present
// verbatim in the struct source, so a rename in one place (struct tag) but
// not the other (T6/T7's population call) cannot both silently pass. The
// chain summary's struct is loopchain.Result (chainResult aliases it).
func TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent(t *testing.T) {
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
