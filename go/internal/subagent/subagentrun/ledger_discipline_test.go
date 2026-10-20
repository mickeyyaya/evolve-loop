package subagentrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestAppendLedger_HoldsTheSharedLedgerLockWhileItAppends(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockHeld := false
	probe := WithLedgerOpener(func(p string) (io.WriteCloser, error) {
		release, held, err := flock.TryLock(filepath.Join(dir, "ledger.lock"))
		if err != nil {
			t.Errorf("probe the ledger lock: %v", err)
		}
		if release != nil {
			release()
		}
		lockHeld = held
		return openAppend(p)
	})

	if op, err := writer(t, probe).appendLedger(filepath.Join(dir, "ledger.jsonl"), ledgerEntry{Cycle: 1, Role: "scout"}); err != nil {
		t.Fatalf("appendLedger %s: %v", op, err)
	}

	if !lockHeld {
		t.Fatal("the append ran without ledger.lock held; adapters/ledger serializes on that lock, so the two writers can interleave")
	}
}

func TestAppendLedger_ThroughALinkedLedgerWritesTheTipBesideTheRealLedger(t *testing.T) {
	t.Parallel()
	host := t.TempDir()
	lane := t.TempDir()
	real := filepath.Join(host, "ledger.jsonl")
	if err := os.WriteFile(real, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(lane, "ledger.jsonl")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	if op, err := writer(t).appendLedger(link, ledgerEntry{Cycle: 1, Role: "scout"}); err != nil {
		t.Fatalf("appendLedger %s: %v", op, err)
	}

	if _, err := os.Stat(filepath.Join(host, "ledger.tip")); err != nil {
		t.Fatalf("the tip must advance beside the real ledger: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lane, "ledger.tip")); err == nil {
		t.Fatal("a tip beside the link forks the chain: the host tip stays stale")
	}
}

func TestAppendLedger_ConcurrentWritersKeepOneUnbrokenChain(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	const writers = 16
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if op, err := writer(t).appendLedger(path, ledgerEntry{Cycle: i, Role: "scout"}); err != nil {
				t.Errorf("appendLedger %s: %v", op, err)
			}
		}(i)
	}
	wg.Wait()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != writers {
		t.Fatalf("got %d lines, want %d", len(lines), writers)
	}
	prev := LedgerZeroSeed
	for i, line := range lines {
		var e struct {
			Seq  int    `json:"entry_seq"`
			Prev string `json:"prev_hash"`
		}
		if err := json.Unmarshal([]byte(line), &e); err != nil || e.Seq != i || e.Prev != prev {
			t.Fatalf("line %d breaks the chain (seq %d prev %s, want %d %s; err %v)", i, e.Seq, e.Prev, i, prev, err)
		}
		prev = SHA256Hex(line)
	}
}

func TestRenderLedgerLine_EscapesTheArtifactHashAndThePrevHash(t *testing.T) {
	t.Parallel()
	hostile := `x"y\z`

	line := renderLedgerLine(ledgerEntry{ArtifactSHA256: hostile}, "2026-10-20T00:00:00Z", 1, hostile)

	var got struct {
		Artifact string `json:"artifact_sha256"`
		Prev     string `json:"prev_hash"`
	}
	if err := json.Unmarshal([]byte(line), &got); err != nil || got.Artifact != hostile || got.Prev != hostile {
		t.Fatalf("line must stay valid JSON with both values intact: %v %+v\n%s", err, got, line)
	}
}

func TestAppendChainedLine_ANilOpenerAppendsToTheLedgerFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger.jsonl")

	op, err := AppendChainedLine(path, func(entrySeq int, prevHash string) string {
		return `{"entry_seq":0,"prev_hash":"` + prevHash + `"}`
	}, nil)

	if err != nil {
		t.Fatalf("AppendChainedLine %s: %v", op, err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != `{"entry_seq":0,"prev_hash":"`+LedgerZeroSeed+`"}`+"\n" {
		t.Fatalf("ledger = %q", raw)
	}
}

func TestAppendLedger_InterleavedWithTheFileLedgerTheChainVerifies(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	fileLedger := ledger.New(dir)
	const rounds = 8
	var wg sync.WaitGroup
	for i := 0; i < rounds; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			if op, err := writer(t).appendLedger(path, ledgerEntry{Cycle: i, Role: "scout"}); err != nil {
				t.Errorf("appendLedger %s: %v", op, err)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			if err := fileLedger.Append(context.Background(), core.LedgerEntry{Cycle: i, Role: "orchestrator", Kind: "phase"}); err != nil {
				t.Errorf("FileLedger.Append: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if err := fileLedger.Verify(context.Background()); err != nil {
		t.Fatalf("the two writers forked the chain: %v", err)
	}
}

func TestAppendLedger_AfterARotationTheSequenceFollowsTheTip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	tail := `{"entry_seq":41,"prev_hash":"x"}`
	if err := os.WriteFile(path, []byte(tail+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte("41:"+SHA256Hex(tail)), 0o644); err != nil {
		t.Fatal(err)
	}

	if op, err := writer(t).appendLedger(path, ledgerEntry{Cycle: 1, Role: "scout"}); err != nil {
		t.Fatalf("appendLedger %s: %v", op, err)
	}

	raw, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	var e struct {
		Seq  int    `json:"entry_seq"`
		Prev string `json:"prev_hash"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &e); err != nil || e.Seq != 42 || e.Prev != SHA256Hex(tail) {
		t.Fatalf("seq %d prev %s (err %v); want 42 and the tip hash, as adapters/ledger would write", e.Seq, e.Prev, err)
	}
}

func TestAppendLedger_AMalformedTipFailsTheChainLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte("not-a-tip"), 0o644); err != nil {
		t.Fatal(err)
	}

	op, err := writer(t).appendLedger(filepath.Join(dir, "ledger.jsonl"), ledgerEntry{Cycle: 1})

	if err == nil || op != "chain_link" {
		t.Fatalf("op %q err %v; a malformed tip must stop the append", op, err)
	}
}

func TestReplaceTip_ARenameOntoANonEmptyDirectoryIsTipRename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tipPath := filepath.Join(dir, "ledger.tip")
	if err := os.MkdirAll(filepath.Join(tipPath, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}

	op, err := replaceTip(tipPath, "0:abc\n")

	if err == nil || op != "tip_rename" {
		t.Fatalf("op %q err %v, want tip_rename", op, err)
	}
	if _, statErr := os.Stat(fmt.Sprintf("%s.tmp.%d", tipPath, os.Getpid())); statErr == nil {
		t.Fatal("a failed rename must remove its tmp file")
	}
}
