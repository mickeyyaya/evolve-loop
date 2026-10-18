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

func chainTestEnv(t *testing.T, items int, policyJSON string) loopConfig {
	t.Helper()
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("seed inbox: %v", err)
	}
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
	if n, _, _ := inboxPendingCount(cfg.EvolveDir); n != 3 {
		t.Errorf("quota deferral must leave the inbox untouched, pending=%d want 3", n)
	}
}

func TestRunLoopChain_MaxBatchesCapHalts(t *testing.T) {
	cfg := chainTestEnv(t, 5, "")
	seen := stubBatches(t, func(int, loopConfig) int { return 0 })

	rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 3})

	if len(*seen) != 3 {
		t.Fatalf("chain ran %d batches, want exactly the cap of 3", len(*seen))
	}
	if rc != 0 || res.StopReason != "chain_max_batches" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_max_batches", rc, res.StopReason)
	}
}

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

func withoutHooks(c loopConfig) loopConfig {
	c.announce, c.retire = nil, nil
	return c
}

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
		if !reflect.DeepEqual(withoutHooks(c), withoutHooks((*seen)[0])) {
			t.Errorf("batch %d received a mutated config (%+v) — the chain must hand every batch the same resolved config", i+1, c)
		}
	}
}

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

func TestInboxPendingCount(t *testing.T) {
	t.Parallel()
	cfg := chainTestEnv(t, 4, "")
	n, skipped, err := inboxPendingCount(cfg.EvolveDir)
	if err != nil || n != 4 || len(skipped) != 0 {
		t.Fatalf("inboxPendingCount = (%d,%v,%v), want (4,[],nil) — subdirs and non-json files must not count, and valid items must not be skipped", n, skipped, err)
	}
	n, skipped, err = inboxPendingCount(filepath.Join(t.TempDir(), "nope"))
	if err != nil || n != 0 || len(skipped) != 0 {
		t.Fatalf("missing inbox = (%d,%v,%v), want (0,[],nil)", n, skipped, err)
	}
}

func TestParseLoopArgs_UntilInboxEmpty(t *testing.T) {
	t.Parallel()
	cfg, rc := parseLoopArgs([]string{"--goal-text", "g", "--until-inbox-empty"}, os.Stderr)
	if rc != 0 || !cfg.ChainMode {
		t.Fatalf("--until-inbox-empty: rc=%d ChainMode=%v, want 0/true", rc, cfg.ChainMode)
	}
	cfg, rc = parseLoopArgs([]string{"--goal-text", "g"}, os.Stderr)
	if rc != 0 || cfg.ChainMode {
		t.Fatalf("no flag: rc=%d ChainMode=%v, want 0/false", rc, cfg.ChainMode)
	}
}
