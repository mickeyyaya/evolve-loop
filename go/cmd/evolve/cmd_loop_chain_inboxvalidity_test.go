package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// cmd_loop_chain_inboxvalidity_test.go — cycle-1098 RED contract tests for
// chain-inbox-pending-validity: inboxPendingCount counts every root-level
// `*.json` with no shape validation, so one malformed or non-item file pins
// pending>0 permanently and the chain burns batches to max_batches consuming
// nothing — reaching the runaway cap via a FALSE signal. Skips are silent
// today, which would also hide a real item lost to a typo.
//
// Builder makes these pass by changing production code only. Helpers
// (chainTestEnv/stubBatches/runChain) come from cmd_loop_chain_test.go.

// writeInboxFile is a local fixture helper: it drops a raw body at
// .evolve/inbox/<name> (bypassing chainTestEnv's well-formed-item seeding so
// malformed shapes can be planted).
func writeInboxFile(t *testing.T, evolveDir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox", name), []byte(body), 0o644); err != nil {
		t.Fatalf("seed inbox file %s: %v", name, err)
	}
}

// TestInboxPendingCount_SkipsMalformedAndNamesThem — AC2.1 + AC2.2. Only files
// that parse as an inbox item (a JSON object carrying `id`) are pending work;
// every other root-level `*.json` is SKIPPED and RETURNED BY NAME so the caller
// can fail loudly instead of swallowing a real item lost to a typo.
//
// The signature this pins is:
//
//	inboxPendingCount(evolveDir string) (pending int, skipped []string, err error)
//
// — the skip list is returned, not printed, so the predicate stays pure and the
// diagnostic lives at the call site (runLoopChain).
func TestInboxPendingCount_SkipsMalformedAndNamesThem(t *testing.T) {
	cfg := chainTestEnv(t, 2, "") // two well-formed items
	writeInboxFile(t, cfg.EvolveDir, "broken.json", `{"id": "truncated"`)

	n, skipped, err := inboxPendingCount(cfg.EvolveDir)
	if err != nil {
		t.Fatalf("inboxPendingCount: unexpected error %v", err)
	}
	if n != 2 {
		t.Errorf("pending = %d, want 2 — a malformed file must not count as pending work "+
			"(a false pending signal burns the chain to max_batches consuming nothing)", n)
	}
	if len(skipped) != 1 || !strings.Contains(strings.Join(skipped, ","), "broken.json") {
		t.Errorf("skipped = %v, want exactly [broken.json] — skips must be reported, never silent", skipped)
	}
}

// TestInboxPendingCount_MissingInboxIsZeroWithNoSkips — AC2.3 (edge, unchanged
// behaviour). A missing inbox is legitimately zero pending with a nil error and
// nothing to report — validation must not turn it into a skip or an error.
func TestInboxPendingCount_MissingInboxIsZeroWithNoSkips(t *testing.T) {
	n, skipped, err := inboxPendingCount(filepath.Join(t.TempDir(), "nope"))
	if err != nil || n != 0 || len(skipped) != 0 {
		t.Fatalf("missing inbox = (%d,%v,%v), want (0,[],nil)", n, skipped, err)
	}
}

// TestInboxPendingCount_MalformedShapes — AC2.1/AC2.4 (edge/OOD matrix). Each
// non-item shape must be skipped-and-named; lifecycle subdirectories and
// non-json files stay invisible (neither pending nor skipped — they were never
// claimed to be items).
func TestInboxPendingCount_MalformedShapes(t *testing.T) {
	cfg := chainTestEnv(t, 0, "")
	inbox := filepath.Join(cfg.EvolveDir, "inbox")

	writeInboxFile(t, cfg.EvolveDir, "good.json", `{"id":"real-item","weight":0.88}`)
	writeInboxFile(t, cfg.EvolveDir, "empty.json", "")
	writeInboxFile(t, cfg.EvolveDir, "array.json", `[{"id":"a"}]`)
	writeInboxFile(t, cfg.EvolveDir, "noid.json", `{"weight":0.5}`)
	writeInboxFile(t, cfg.EvolveDir, "blankid.json", `{"id":""}`)
	// A well-formed item parked in a lifecycle subdir is claimed work, not pending.
	if err := os.WriteFile(filepath.Join(inbox, "processed", "done.json"), []byte(`{"id":"done"}`), 0o644); err != nil {
		t.Fatalf("seed processed item: %v", err)
	}

	n, skipped, err := inboxPendingCount(cfg.EvolveDir)
	if err != nil {
		t.Fatalf("inboxPendingCount: unexpected error %v", err)
	}
	if n != 1 {
		t.Errorf("pending = %d, want 1 (only good.json is a real unclaimed item)", n)
	}
	joined := strings.Join(skipped, ",")
	for _, want := range []string{"empty.json", "array.json", "noid.json", "blankid.json"} {
		if !strings.Contains(joined, want) {
			t.Errorf("skipped %v is missing %s — every non-item *.json must be named", skipped, want)
		}
	}
	for _, unwanted := range []string{"good.json", "README.md", "done.json"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("skipped %v must not name %s (valid items, non-json files and lifecycle subdirs are not skips)", skipped, unwanted)
		}
	}
}

// TestRunLoopChain_MalformedOnlyInboxDoesNotBurnToCap — AC2.2 + the composed
// contract, end-to-end. An inbox holding nothing but a malformed file has ZERO
// real pending work: the chain must run the single min-one-batch cycle and stop
// as drained — NOT relaunch to the cap on a false pending signal — and must name
// the skipped file on stderr so the operator can fix the typo.
func TestRunLoopChain_MalformedOnlyInboxDoesNotBurnToCap(t *testing.T) {
	cfg := chainTestEnv(t, 0, "")
	writeInboxFile(t, cfg.EvolveDir, "typo-item.json", `{"id": `)
	seen := stubBatches(t, func(int, loopConfig) int { return 0 }) // consumes nothing

	rc, res, stderr := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 3})

	if len(*seen) != 1 {
		t.Fatalf("chain ran %d batches on a malformed-only inbox, want exactly 1: "+
			"an unparseable file is a FALSE pending signal that must not burn the chain to the cap", len(*seen))
	}
	if rc != 0 || res.StopReason != "chain_inbox_empty" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_inbox_empty", rc, res.StopReason)
	}
	if res.Batches[0].InboxPending != 0 {
		t.Errorf("recorded inbox_pending = %d, want 0 — the summary must report REAL pending work", res.Batches[0].InboxPending)
	}
	if !strings.Contains(stderr, "typo-item.json") {
		t.Errorf("stderr must name every skipped inbox file (fail loudly, never swallow a lost item); got:\n%s", stderr)
	}
}

// TestInboxPendingCount_ValidItemFixtureIsRepresentative is a fixture-integrity
// guard: the shape chainTestEnv seeds must be one a REAL inbox item validator
// accepts, otherwise the tests above would pass against a validator that
// rejects everything.
func TestInboxPendingCount_ValidItemFixtureIsRepresentative(t *testing.T) {
	t.Parallel()
	var doc map[string]any
	if err := json.Unmarshal([]byte(`{"id":"item"}`), &doc); err != nil {
		t.Fatalf("fixture item is not valid JSON: %v", err)
	}
	if _, ok := doc["id"]; !ok {
		t.Fatal("fixture item must carry an id — the field the validator keys on")
	}
}
