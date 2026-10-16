package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func writeInboxFile(t *testing.T, evolveDir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox", name), []byte(body), 0o644); err != nil {
		t.Fatalf("seed inbox file %s: %v", name, err)
	}
}

func TestInboxPendingCount_SkipsMalformedAndNamesThem(t *testing.T) {
	t.Parallel()
	cfg := chainTestEnv(t, 2, "")
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

func TestInboxPendingCount_MissingInboxIsZeroWithNoSkips(t *testing.T) {
	t.Parallel()
	n, skipped, err := inboxPendingCount(filepath.Join(t.TempDir(), "nope"))
	if err != nil || n != 0 || len(skipped) != 0 {
		t.Fatalf("missing inbox = (%d,%v,%v), want (0,[],nil)", n, skipped, err)
	}
}

func TestInboxPendingCount_MalformedShapes(t *testing.T) {
	t.Parallel()
	cfg := chainTestEnv(t, 0, "")
	inbox := filepath.Join(cfg.EvolveDir, "inbox")

	writeInboxFile(t, cfg.EvolveDir, "good.json", `{"id":"real-item","weight":0.88}`)
	writeInboxFile(t, cfg.EvolveDir, "empty.json", "")
	writeInboxFile(t, cfg.EvolveDir, "array.json", `[{"id":"a"}]`)
	writeInboxFile(t, cfg.EvolveDir, "noid.json", `{"weight":0.5}`)
	writeInboxFile(t, cfg.EvolveDir, "blankid.json", `{"id":""}`)
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

func TestRunLoopChain_MalformedOnlyInboxDoesNotBurnToCap(t *testing.T) {
	cfg := chainTestEnv(t, 0, "")
	writeInboxFile(t, cfg.EvolveDir, "typo-item.json", `{"id": `)
	seen := stubBatches(t, func(int, loopConfig) int { return 0 })

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
