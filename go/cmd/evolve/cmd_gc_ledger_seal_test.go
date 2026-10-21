package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func sealFixture(t *testing.T, mode string, entries int) (loopConfig, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gc":{"mode":"`+mode+`","ledger_seal_keep":3,"ledger_seal":"enforce"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	l := ledger.New(evolveDir)
	for i := 0; i < entries; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Cycle: i, Role: "builder", Kind: "phase_complete", Message: fmt.Sprintf("entry-%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	return loopConfig{EvolveDir: evolveDir}, t.TempDir()
}

func segments(t *testing.T, evolveDir string) []string {
	t.Helper()
	segs, _ := filepath.Glob(filepath.Join(evolveDir, "ledger-segments", "*.jsonl.gz"))
	return segs
}

func TestRunGCHook_EnforceSealsLedgerBeyondKeep(t *testing.T) {
	cfg, ws := sealFixture(t, "enforce", 12)
	var stderr bytes.Buffer
	runGCHook(cfg, ws, &stderr)
	if len(segments(t, cfg.EvolveDir)) != 1 || !strings.Contains(stderr.String(), "[gc] enforce: sealed the ledger (keep=3) and deep-verified it") {
		t.Fatalf("segments %v, stderr:\n%s\nwant one segment and the sealed line", segments(t, cfg.EvolveDir), stderr.String())
	}
	if err := ledger.New(cfg.EvolveDir).VerifyDeep(context.Background()); err != nil {
		t.Errorf("verify --deep after the seal: %v", err)
	}
}

func TestRunGCHook_ShadowNeverSeals(t *testing.T) {
	cfg, ws := sealFixture(t, "shadow", 12)
	if err := os.WriteFile(filepath.Join(cfg.EvolveDir, "policy.json"), []byte(`{"gc":{"mode":"enforce","ledger_seal_keep":3}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	runGCHook(cfg, ws, &stderr)
	if len(segments(t, cfg.EvolveDir)) != 0 || !strings.Contains(stderr.String(), "[gc] shadow: the ledger holds 12 lines; enforce seals all but the last 3") {
		t.Fatalf("segments %v, stderr:\n%s\nwant no segment and the report line: gc.mode enforce alone must not seal", segments(t, cfg.EvolveDir), stderr.String())
	}
	if got := ledgerSealMode("shadow", gc.Policy{LedgerSeal: "enforce"}); got != "shadow" {
		t.Errorf("ledgerSealMode under gc.mode shadow = %q, want shadow", got)
	}
}

func TestSealLedgerGC_ASmallLedgerIsLeftAloneAndAFailedSealIsAWarning(t *testing.T) {
	t.Parallel()
	cfg, _ := sealFixture(t, "enforce", 6)
	var stderr bytes.Buffer
	sealLedgerGC(cfg, "enforce", 3, &stderr)
	if len(segments(t, cfg.EvolveDir)) != 0 || stderr.Len() != 0 {
		t.Errorf("a ledger under twice the keep was sealed or reported: %v %q", segments(t, cfg.EvolveDir), stderr.String())
	}
	broken, _ := sealFixture(t, "enforce", 12)
	if err := os.WriteFile(filepath.Join(broken.EvolveDir, "ledger-segments"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	sealLedgerGC(broken, "enforce", 3, &stderr)
	if !strings.Contains(stderr.String(), "[gc] WARN: ledger seal failed:") {
		t.Errorf("stderr %q, want the seal failure", stderr.String())
	}
	tampered, _ := sealFixture(t, "enforce", 12)
	live := filepath.Join(tampered.EvolveDir, "ledger.jsonl")
	raw, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, bytes.Replace(raw, []byte("entry-11"), []byte("entry-XX"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	sealLedgerGC(tampered, "enforce", 3, &stderr)
	if !strings.Contains(stderr.String(), "[gc] WARN: ledger sealed but deep verify FAILED:") {
		t.Errorf("a tampered tail: stderr %q, want the deep verify failure", stderr.String())
	}
	dirLedger := loopConfig{EvolveDir: t.TempDir()}
	if err := os.Mkdir(filepath.Join(dirLedger.EvolveDir, "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	sealLedgerGC(dirLedger, "enforce", 3, &stderr)
	if !strings.Contains(stderr.String(), "[gc] WARN: ledger seal skipped:") {
		t.Errorf("an unreadable ledger: stderr %q, want the skip warning", stderr.String())
	}
	unreadable := loopConfig{EvolveDir: filepath.Join(t.TempDir(), "absent")}
	stderr.Reset()
	sealLedgerGC(unreadable, "enforce", 3, &stderr)
	if stderr.Len() != 0 {
		t.Errorf("an absent ledger: stderr %q, want nothing", stderr.String())
	}
}

func TestRunGCHook_AnUnreadablePolicyWarnsAndRunsTheShadowDefault(t *testing.T) {
	cfg, ws := sealFixture(t, "enforce", 12)
	if err := os.WriteFile(filepath.Join(cfg.EvolveDir, "policy.json"), []byte(`{"gc":`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	runGCHook(cfg, ws, &stderr)
	if !strings.Contains(stderr.String(), "[gc] WARN: policy load failed:") || len(segments(t, cfg.EvolveDir)) != 0 {
		t.Errorf("stderr:\n%s\nwant the policy warning and no seal under the shadow default", stderr.String())
	}
}
