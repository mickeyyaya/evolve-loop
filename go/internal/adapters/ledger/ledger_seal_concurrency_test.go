package ledger

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"
)

// ledger_seal_concurrency_test.go — Phase 2 / S2.1 (modularization campaign,
// ADR-0050). The append-vs-append cross-process race is already covered by
// TestAppend_TwoProcessStress. The UNTESTED, safety-critical windows on the
// hash chain are the other two mutators of ledger.jsonl:
//
//   - Seal vs a concurrent cross-process Append. Seal (seal.go) holds
//     ledger.lock to truncate the live file, RELEASES it, then re-acquires it
//     for the chained anchor Append. A foreign `evolve` process can grab
//     ledger.lock anywhere in that handoff. The chain must stay verifiable and
//     no append may be lost. RED-check: drop the inner flock.Lock(l.lockPath)
//     from Seal's sealLocked wrapper (keep only seal.lock) — the truncation then
//     races a foreign append and rewriteLive's tmp+rename drops it; childN
//     below goes < stressN. This proves the test asserts the lock handoff, not
//     merely "did not panic".
//   - Anchor vs a concurrent Seal. Anchor (anchor.go) takes NEITHER l.mu NOR a
//     flock; it reads the full chain while Seal rewrites it. It must never bind
//     ledger-anchor.json to a line the seal removed.
//
// Both assert an INVARIANT (VerifyDeep intact / anchor points at a real line),
// never a bare no-panic. seedLedger(t, n) (*FileLedger, string) is the shared
// helper from seal_test.go (returns the seeded ledger + its dir).

func TestSeal_ConcurrentWithCrossProcessAppend_ChainStaysVerifiable(t *testing.T) {
	if testing.Short() {
		t.Skip("two-process stress skipped in -short")
	}
	const seedN = 20
	l, dir := seedLedger(t, seedN)
	ctx := context.Background()

	// A separate OS process hammers Append into the shared dir (reusing the
	// cross-proc helper). Its only serialization with our Seal is the on-disk
	// ledger.lock — l.mu is invisible across processes.
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLedgerAppender$", "-test.v=false")
	cmd.Env = append(os.Environ(), stressDirEnv+"="+dir)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child appender: %v", err)
	}
	childDone := make(chan error, 1)
	go func() { childDone <- cmd.Wait() }()

	// Seal continuously for the whole lifetime of the child so a Seal's
	// truncate → release-ledger.lock → anchor-Append window keeps overlapping
	// the child's appends (no sleeps — drive overlap by looping until exit).
sealLoop:
	for {
		select {
		case err := <-childDone:
			if err != nil {
				t.Fatalf("child appender failed: %v", err)
			}
			break sealLoop
		default:
			if err := l.Seal(ctx, 5); err != nil {
				t.Fatalf("seal during concurrent cross-process append: %v", err)
			}
		}
	}

	// INVARIANT 1: the full segment+tail chain verifies end-to-end across every
	// seal boundary (residue check + per-segment anchor binding + chain walk).
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("VerifyDeep failed after concurrent seal+append: %v", err)
	}

	// Read the FULL chain — sealed segments + live tail (Iter is live-only).
	lines, err := l.gatherAllLines()
	if err != nil {
		t.Fatalf("gatherAllLines: %v", err)
	}
	seqs := make(map[int]bool, len(lines))
	childN, maxSeq := 0, -1
	for _, line := range lines {
		_, e, derr := decodeLedgerLine(line)
		if derr != nil {
			t.Fatalf("decode ledger line: %v", derr)
		}
		if seqs[e.EntrySeq] {
			t.Errorf("duplicate entry_seq %d — lost-update interleave between a seal and a foreign append", e.EntrySeq)
		}
		seqs[e.EntrySeq] = true
		if e.EntrySeq > maxSeq {
			maxSeq = e.EntrySeq
		}
		if e.Kind == "stress-child" {
			childN++
		}
	}

	// INVARIANT 2: every cross-process append survived (none dropped when a
	// foreign Append landed in the seal's lock-handoff window).
	if childN != stressN {
		t.Errorf("cross-process appends present = %d, want %d (an append was lost in the seal lock-handoff window)", childN, stressN)
	}
	// INVARIANT 3: entry_seq is unique and gapless 0..max — Append assigns
	// prevSeq+1 under ledger.lock, so a correct chain has no gap or collision
	// even with a concurrent sealer relocating lines into segments.
	if maxSeq != len(lines)-1 {
		t.Errorf("max entry_seq = %d for a %d-line chain — gap or duplicate seq", maxSeq, len(lines))
	}
	for i := 0; i <= maxSeq; i++ {
		if !seqs[i] {
			t.Errorf("entry_seq %d missing — gap in the chain", i)
			break
		}
	}
}

func TestAnchor_ConcurrentWithSeal_NoCorruptAnchor(t *testing.T) {
	if testing.Short() {
		t.Skip("stress skipped in -short")
	}
	const seedN = 40
	l, _ := seedLedger(t, seedN)
	ctx := context.Background()

	// Anchor (no lock) races Seal (which rewrites the chain into segments).
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 60; i++ {
			// A clean "no line with entry_seq" miss is acceptable when the target
			// line is mid-relocation; a corrupt anchor is not. Ignore the error,
			// assert the recorded anchor afterward.
			_ = l.Anchor(ctx, seedN/2, "concurrent-seal-stress")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 60; i++ {
			if err := l.Seal(ctx, 5); err != nil {
				t.Errorf("seal during concurrent anchor: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	// INVARIANT 1: the chain still verifies after the anchor/seal race.
	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("VerifyDeep failed after concurrent anchor+seal: %v", err)
	}
	// INVARIANT 2: any recorded epoch-anchor must bind a line that STILL exists
	// in the final chain — never a SHA a concurrent seal removed.
	anchorSHA := l.loadAnchorSHA()
	if anchorSHA == "" {
		return // no anchor landed (all attempts missed mid-relocation) — acceptable
	}
	lines, err := l.gatherAllLines()
	if err != nil {
		t.Fatalf("gatherAllLines: %v", err)
	}
	for _, line := range lines {
		if sha256Hex(line) == anchorSHA {
			return // anchor points at a real, present line — invariant holds
		}
	}
	t.Errorf("ledger-anchor.json binds SHA %s to a line absent from the final chain — anchor corrupted by a concurrent seal", anchorSHA)
}
