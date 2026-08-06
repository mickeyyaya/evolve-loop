package main

// cmd_loop_boundaryrefresh_summary_test.go — RED tests (cycle 1330, inbox
// item auto-refresh-binary-at-boundary, chronicle
// docs/chronicle/2026-08-binary-lag.md "Remaining scope" item 2: "surfacing
// refresh events in the dossier/loop summary").
//
// PRIOR STATE (verified live in this worktree, not assumed from the scout
// report — the scout's own knowledge of "already shipped" traced main, not
// this worktree, and this worktree's chain-boundary mechanism is a SEPARATE,
// already-complete lineage from cycles 1314/1320/1323/1325):
//   - maybeRefreshChainBoundary (cmd_loop_chain.go) is fully implemented,
//     wired into BOTH runLoopChain (cmd_loop_chain.go:538) and runLoopBatch's
//     wave/fleet loop (cmd_loop.go:552), and covered by 18 existing GREEN
//     tests (cmd_loop_chain_boundaryrefresh_test.go,
//     _hardening_test.go, cmd_loop_wave_boundaryrefresh_wiring_test.go).
//     Remaining-scope item 1 (the chain-boundary hook itself) is therefore
//     PRE-EXISTING GREEN — this file adds no coverage for it.
//   - Every successful boundary refresh already appends a durable audit
//     record to .evolve/boundary-refresh-log.jsonl
//     (appendChainBoundaryRefreshLog, cmd_loop_chain.go) BEFORE the re-exec —
//     the "from stamp / to stamp" data chronicle item 2 asks to surface
//     already exists on disk. What is missing is the LAST MILE: that record
//     is never read back into the chain/loop summary JSON an operator (or a
//     dossier consumer) actually looks at. `chainResult` sets StopReason =
//     "chain_boundary_refresh_reexec" / loopResult sets StopReason =
//     "loop_boundary_refresh_reexec", but neither says WHICH commits were
//     involved without a separate grep of the JSONL file.
//
// THE GAP THIS CYCLE CLOSES (undefined until the Builder adds it — every
// reference below to a not-yet-existing symbol IS this file's RED evidence;
// no new detection/rebuild logic is invented, per
// never_duplicate_centralize_via_design_patterns — this is pure plumbing
// over the log file that already exists):
//
//	// lastChainBoundaryRefreshLogEntry reads
//	// <evolveDir>/boundary-refresh-log.jsonl and returns the LAST
//	// (most-recently-appended) entry. Best-effort, mirroring
//	// spineFailOpenRollup's nil-when-clean shape: a missing file, an empty
//	// file, or a read error all resolve to (nil, nil) — the field is simply
//	// absent from the summary, never a hard failure (a summary emission must
//	// not itself become a new failure mode for an already-successful boundary
//	// refresh).
//	func lastChainBoundaryRefreshLogEntry(evolveDir string) (*chainBoundaryRefreshLogEntry, error)
//
//	// chainResult gains:
//	BoundaryRefresh *chainBoundaryRefreshLogEntry `json:"boundary_refresh,omitempty"`
//	// populated at the SAME call site that already sets
//	// res.StopReason = "chain_boundary_refresh_reexec" (cmd_loop_chain.go).
//
//	// loopResult (cmd_loop_outcome.go) gains the identical field, populated
//	// at the wave-boundary call site that already sets
//	// lr.StopReason = "loop_boundary_refresh_reexec" (cmd_loop.go).
//
// Predicate strategy (cycle-85 degenerate-predicate ban: every predicate
// below exercises the real system under test, never a source-grep alone):
//
//	positive   — the helper returns the LAST of several appended entries
//	             with its fields intact (T1).
//	edge       — a missing log file, and an empty-but-present log file, both
//	             degrade to (nil, nil) rather than an error (T2, T3).
//	negative   — a nil BoundaryRefresh is OMITTED from marshaled JSON, not
//	             merely null (exact-omission assertion, T4/T5) — the
//	             cycle-131-class "the field is technically there but useless"
//	             failure mode.
//	wiring     — runLoopChain / runLoopBatch actually populate the new field
//	             at their respective re-exec-stop call sites, waived per
//	             acsassert's config-check convention because the field's own
//	             correctness (does it hold the right OldSHA/NewSHA) is proven
//	             by T1 above, and WHERE it must be called from is a structural
//	             fact a behavioral end-to-end drive would just duplicate at
//	             much higher fixture cost (T6/T7, mirrors
//	             TestRunLoop_CallsMaybeRefreshChainBoundaryAtWaveBoundary).
//
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

// T4 (negative — the cycle-131-class failure mode): chainResult with a nil
// BoundaryRefresh must OMIT the "boundary_refresh" key entirely from the
// marshaled JSON, not emit `"boundary_refresh": null`. A consumer that
// checks key-presence (not merely non-null) to decide "did a refresh
// happen this run" must see a clean, absent key on every ordinary run.
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

// T6: runLoopChain must populate res.BoundaryRefresh at the same call site
// that already sets res.StopReason = "chain_boundary_refresh_reexec" —
// otherwise the durable audit record the mechanism already writes to
// boundary-refresh-log.jsonl stays permanently invisible to the JSON
// summary an operator/dossier consumer actually reads.
//
// acs-predicate: config-check — lastChainBoundaryRefreshLogEntry's own
// correctness is proven by T1-T3; this only proves runLoopChain calls it.
func TestRunLoopChain_SetsBoundaryRefreshOnReExecStop(t *testing.T) {
	n, err := acsassert.CountInGoFunc("cmd_loop_chain.go", "runLoopChain", "BoundaryRefresh")
	if err != nil {
		t.Fatalf("CountInGoFunc(runLoopChain, BoundaryRefresh): %v", err)
	}
	if n < 1 {
		t.Errorf("runLoopChain does not reference BoundaryRefresh (count=%d); the boundary-refresh-log.jsonl audit record stays invisible to the chain summary JSON", n)
	}
}

// T7 mirrors T6 for runLoopBatch's wave/fleet boundary (cmd_loop.go), the
// non-chain caller maybeRefreshChainBoundary also fires from (cycle 1325
// wiring). Both callers of the SAME refresh mechanism must surface the
// SAME summary field — surfacing only the chain-mode caller would silently
// leave the plain `evolve loop --max-cycles N` / fleet path's refresh
// events unobservable, exactly the asymmetry cycle-1325 closed for the
// trigger wiring itself.
//
// acs-predicate: config-check — see T6.
func TestRunLoopBatch_SetsBoundaryRefreshOnWaveBoundaryReExecStop(t *testing.T) {
	n, err := acsassert.CountInGoFunc("cmd_loop.go", "runLoopBatch", "lr.BoundaryRefresh")
	if err != nil {
		t.Fatalf("CountInGoFunc(runLoopBatch, lr.BoundaryRefresh): %v", err)
	}
	if n < 1 {
		t.Errorf("runLoopBatch does not set lr.BoundaryRefresh (count=%d); the wave/fleet boundary's refresh events stay invisible to the loop summary JSON even though runLoopChain's are surfaced", n)
	}
}

// sanity guard against a degenerate "field exists but is spelled wrong"
// implementation slipping past T4/T5 (which only assert absence-when-nil,
// not the exact field name): confirm the json tag string itself is present
// verbatim in the struct source, so a rename in one place (struct tag) but
// not the other (T6/T7's population call) cannot both silently pass.
func TestChainResultAndLoopResult_BoundaryRefreshJSONTagPresent(t *testing.T) {
	for _, f := range []string{"cmd_loop_chain.go", "cmd_loop_outcome.go"} {
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
