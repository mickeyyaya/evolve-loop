package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// cmd_loop_chain_test.go — the fake-runner harness for the outer batch-chaining
// loop (cycle 1075). The batch dispatcher is replaced via runLoopBatchFn so the
// BOUNDARY decisions (start another batch? stop, and why?) are exercised
// end-to-end without spawning cycles: every test below asserts on the chain's
// own summary + the number of batches the fake actually saw.

// chainTestEnv seeds a temp project with an inbox holding `items` pending
// todos and an optional policy.json body.
func chainTestEnv(t *testing.T, items int, policyJSON string) loopConfig {
	t.Helper()
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("seed inbox: %v", err)
	}
	// A lifecycle subdirectory and a non-json file must NOT count as pending.
	if err := os.MkdirAll(filepath.Join(inbox, "processed"), 0o755); err != nil {
		t.Fatalf("seed processed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "README.md"), []byte("not a todo"), 0o644); err != nil {
		t.Fatalf("seed non-json: %v", err)
	}
	for i := 0; i < items; i++ {
		p := filepath.Join(inbox, fmt.Sprintf("2026-07-23T00-0%d-00Z-item-%d.json", i, i))
		if err := os.WriteFile(p, []byte(`{"id":"item"}`), 0o644); err != nil {
			t.Fatalf("seed inbox item: %v", err)
		}
	}
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatalf("write policy.json: %v", err)
		}
	}
	return loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir, GoalHash: "deadbeef", MaxCycles: 1, ChainMode: true}
}

// stubBatches installs a scripted batch runner and returns a pointer to the
// slice of configs it was called with (one entry per batch).
func stubBatches(t *testing.T, fn func(batch int, cfg loopConfig) int) *[]loopConfig {
	t.Helper()
	seen := &[]loopConfig{}
	prev := runLoopBatchFn
	runLoopBatchFn = func(cfg loopConfig, _ io.Reader, _ io.Writer, _ io.Writer) int {
		*seen = append(*seen, cfg)
		return fn(len(*seen), cfg)
	}
	t.Cleanup(func() { runLoopBatchFn = prev })
	return seen
}

// consumeOneInboxItem deletes one pending inbox todo — the fake batch's stand-in
// for a cycle that consumed a task.
func consumeOneInboxItem(t *testing.T, cfg loopConfig) {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(cfg.EvolveDir, "inbox"))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(cfg.EvolveDir, "inbox", e.Name())); err != nil {
			t.Fatalf("consume inbox item: %v", err)
		}
		return
	}
}

func runChain(t *testing.T, cfg loopConfig, cc policy.ChainConfig) (int, chainResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runLoopChain(cfg, cc, nil, &stdout, &stderr)
	var res chainResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("chain summary is not JSON: %v\nstdout:\n%s", err, stdout.String())
	}
	return rc, res, stderr.String()
}

// TestRunLoopChain_InboxDrainStartsNextBatchThenCleanExit — AC1. Two pending
// todos, one consumed per batch: the chain must start batch 2 with no external
// invocation and then exit CLEAN (rc=0) once the inbox is empty.
func TestRunLoopChain_InboxDrainStartsNextBatchThenCleanExit(t *testing.T) {
	cfg := chainTestEnv(t, 2, "")
	seen := stubBatches(t, func(_ int, c loopConfig) int {
		consumeOneInboxItem(t, c)
		return 0
	})

	rc, res, stderr := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(*seen) != 2 {
		t.Fatalf("chain ran %d batches, want 2 (one per pending inbox item); stderr=%s", len(*seen), stderr)
	}
	if rc != 0 {
		t.Errorf("drained inbox must exit clean, got rc=%d", rc)
	}
	if res.StopReason != "chain_inbox_empty" {
		t.Errorf("stop reason = %q, want chain_inbox_empty", res.StopReason)
	}
	if len(res.Batches) != 2 || res.Batches[1].Batch != 2 {
		t.Errorf("chain summary must record both batches, got %+v", res.Batches)
	}
}

// TestRunLoopChain_EmptyInboxExitsWithoutRunningABatch — AC1 edge case: a chain
// launched against an already-empty inbox must not run a batch at all.
func TestRunLoopChain_EmptyInboxExitsWithoutRunningABatch(t *testing.T) {
	cfg := chainTestEnv(t, 0, "")
	seen := stubBatches(t, func(int, loopConfig) int { return 0 })

	rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(*seen) != 0 {
		t.Fatalf("empty inbox must run zero batches, ran %d", len(*seen))
	}
	if rc != 0 || res.StopReason != "chain_inbox_empty" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_inbox_empty", rc, res.StopReason)
	}
}

// TestRunLoopChain_QuotaExhaustionDefersInsteadOfRelaunching — AC2. rc=5 is the
// batch's QUOTA-PAUSE contract (derived from core.allFamiliesQuotaExhausted).
// The chain must NOT start another batch into the drained families: it stops,
// propagates rc=5, and points at the intact checkpoint — even though the inbox
// still holds work it would otherwise chain on.
func TestRunLoopChain_QuotaExhaustionDefersInsteadOfRelaunching(t *testing.T) {
	cfg := chainTestEnv(t, 3, "")
	seen := stubBatches(t, func(int, loopConfig) int { return 5 })

	rc, res, stderr := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(*seen) != 1 {
		t.Fatalf("quota wall must stop the chain after the walled batch, ran %d batches", len(*seen))
	}
	if rc != 5 {
		t.Errorf("rc = %d, want 5 (resumable quota-pause propagated, not swallowed)", rc)
	}
	if res.StopReason != "chain_quota_defer" {
		t.Errorf("stop reason = %q, want chain_quota_defer", res.StopReason)
	}
	if !strings.Contains(stderr, "DEFERRING, not relaunching") || !strings.Contains(stderr, "evolve loop --resume") {
		t.Errorf("quota stop must announce the deferral + resume path; stderr=%s", stderr)
	}
	if n, _ := inboxPendingCount(cfg.EvolveDir); n != 3 {
		t.Errorf("quota deferral must leave the inbox untouched, pending=%d want 3", n)
	}
}

// TestRunLoopChain_MaxBatchesCapHalts — AC3. With a never-draining inbox the
// runaway backstop is the only thing that stops the chain: it must halt at
// EXACTLY the cap, not one batch either side of it.
func TestRunLoopChain_MaxBatchesCapHalts(t *testing.T) {
	cfg := chainTestEnv(t, 5, "")
	seen := stubBatches(t, func(int, loopConfig) int { return 0 }) // consumes nothing

	rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 3})

	if len(*seen) != 3 {
		t.Fatalf("chain ran %d batches, want exactly the cap of 3", len(*seen))
	}
	if rc != 0 || res.StopReason != "chain_max_batches" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_max_batches", rc, res.StopReason)
	}
}

// TestRunLoopChain_LoopStopFileBrakeHalts — AC4. `.evolve/loop-stop` dropped
// while batch 1 runs must halt the chain at the next boundary even though the
// inbox still has work and the cap is far away.
func TestRunLoopChain_LoopStopFileBrakeHalts(t *testing.T) {
	cfg := chainTestEnv(t, 5, "")
	seen := stubBatches(t, func(batch int, c loopConfig) int {
		if batch == 1 {
			if err := os.WriteFile(filepath.Join(c.EvolveDir, chainBrakeFile), nil, 0o644); err != nil {
				t.Fatalf("engage brake: %v", err)
			}
		}
		return 0
	})

	rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(*seen) != 1 {
		t.Fatalf("operator brake must stop the chain after the in-flight batch, ran %d", len(*seen))
	}
	if rc != 0 || res.StopReason != "chain_operator_brake" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_operator_brake", rc, res.StopReason)
	}
}

// TestRunLoopChain_FleetWidthPreservedAcrossBatches — AC6. Fleet width is a
// hard operator commitment; a chain that re-derives per-batch settings can
// silently narrow it. Every batch must receive the IDENTICAL config, and the
// recorded lane count must be stable batch N → N+1.
func TestRunLoopChain_FleetWidthPreservedAcrossBatches(t *testing.T) {
	cfg := chainTestEnv(t, 3, `{"fleet":{"count":3}}`)
	seen := stubBatches(t, func(_ int, c loopConfig) int {
		consumeOneInboxItem(t, c)
		return 0
	})

	_, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(res.Batches) < 2 {
		t.Fatalf("need at least two batches to compare width, got %d", len(res.Batches))
	}
	for _, b := range res.Batches {
		if b.FleetCount != 3 {
			t.Errorf("batch %d ran with %d lanes, want the operator-committed 3 (width must never narrow across the chain)", b.Batch, b.FleetCount)
		}
	}
	for i, c := range *seen {
		if !reflect.DeepEqual(c, (*seen)[0]) {
			t.Errorf("batch %d received a mutated config (%+v) — the chain must hand every batch the same resolved config", i+1, c)
		}
	}
}

// TestRunLoopChain_BatchErrorStopsChain — a fatal batch outcome (preflight
// failure, unfinished cycle, ADR-0072 halt) must stop the chain and propagate
// the code, while an rc=3 batch (completed with absorbed failures) must NOT
// halt the queue.
func TestRunLoopChain_BatchErrorStopsChain(t *testing.T) {
	t.Run("rc=2 halts and propagates", func(t *testing.T) {
		cfg := chainTestEnv(t, 5, "")
		seen := stubBatches(t, func(int, loopConfig) int { return 2 })
		rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})
		if len(*seen) != 1 || rc != 2 || res.StopReason != "chain_batch_error" {
			t.Errorf("batches=%d rc=%d reason=%q, want 1/2/chain_batch_error", len(*seen), rc, res.StopReason)
		}
	})
	t.Run("rc=3 keeps chaining", func(t *testing.T) {
		cfg := chainTestEnv(t, 2, "")
		seen := stubBatches(t, func(_ int, c loopConfig) int {
			consumeOneInboxItem(t, c)
			return 3
		})
		rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})
		if len(*seen) != 2 || res.StopReason != "chain_inbox_empty" {
			t.Errorf("absorbed-failure batches must keep the queue moving: batches=%d rc=%d reason=%q", len(*seen), rc, res.StopReason)
		}
	})
}

// TestChainStartDecision pins the pre-batch precedence: the brake outranks a
// drained inbox and the cap, so an operator instruction is never masked by a
// coincidentally-empty inbox.
func TestChainStartDecision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		n, max     int
		pending    int
		brake      bool
		wantReason string
		wantStop   bool
	}{
		{"work pending under cap runs", 0, 3, 2, false, "", false},
		{"empty inbox stops clean", 1, 3, 0, false, "chain_inbox_empty", true},
		{"cap reached stops", 3, 3, 5, false, "chain_max_batches", true},
		{"brake outranks pending work", 0, 3, 5, true, "chain_operator_brake", true},
		{"brake outranks empty inbox", 0, 3, 0, true, "chain_operator_brake", true},
		{"last batch under cap still runs", 2, 3, 1, false, "", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reason, stop := chainStartDecision(tc.n, tc.max, tc.pending, tc.brake)
			if reason != tc.wantReason || stop != tc.wantStop {
				t.Fatalf("chainStartDecision(%d,%d,%d,%v) = (%q,%v), want (%q,%v)",
					tc.n, tc.max, tc.pending, tc.brake, reason, stop, tc.wantReason, tc.wantStop)
			}
		})
	}
}

// TestChainContinueDecision pins the rc→chain-move mapping, including the
// never-stop-the-queue semantics of rc=3.
func TestChainContinueDecision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rc         int
		wantReason string
		wantExit   int
		wantStop   bool
	}{
		{0, "", 0, false},
		{3, "", 3, false},
		{5, "chain_quota_defer", 5, true},
		{2, "chain_batch_error", 2, true},
		{4, "chain_batch_error", 4, true},
		{130, "chain_batch_error", 130, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("rc=%d", tc.rc), func(t *testing.T) {
			t.Parallel()
			reason, exit, stop := chainContinueDecision(tc.rc)
			if reason != tc.wantReason || exit != tc.wantExit || stop != tc.wantStop {
				t.Fatalf("chainContinueDecision(%d) = (%q,%d,%v), want (%q,%d,%v)",
					tc.rc, reason, exit, stop, tc.wantReason, tc.wantExit, tc.wantStop)
			}
		})
	}
}

// TestInboxPendingCount pins that only unclaimed top-level todos count, and
// that a missing inbox is zero rather than an error.
func TestInboxPendingCount(t *testing.T) {
	cfg := chainTestEnv(t, 4, "")
	n, err := inboxPendingCount(cfg.EvolveDir)
	if err != nil || n != 4 {
		t.Fatalf("inboxPendingCount = (%d,%v), want (4,nil) — subdirs and non-json files must not count", n, err)
	}
	n, err = inboxPendingCount(filepath.Join(t.TempDir(), "nope"))
	if err != nil || n != 0 {
		t.Fatalf("missing inbox = (%d,%v), want (0,nil)", n, err)
	}
}

// TestParseLoopArgs_UntilInboxEmpty pins the CLI opt-in: the parameter sets
// chain mode and its absence leaves it off (chaining is never a silent
// default).
func TestParseLoopArgs_UntilInboxEmpty(t *testing.T) {
	cfg, rc := parseLoopArgs([]string{"--goal-text", "g", "--until-inbox-empty"}, os.Stderr)
	if rc != 0 || !cfg.ChainMode {
		t.Fatalf("--until-inbox-empty: rc=%d ChainMode=%v, want 0/true", rc, cfg.ChainMode)
	}
	cfg, rc = parseLoopArgs([]string{"--goal-text", "g"}, os.Stderr)
	if rc != 0 || cfg.ChainMode {
		t.Fatalf("no flag: rc=%d ChainMode=%v, want 0/false", rc, cfg.ChainMode)
	}
}
