package ledger

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"
)

func TestSeal_ConcurrentWithCrossProcessAppend_ChainStaysVerifiable(t *testing.T) {
	if testing.Short() {
		t.Skip("two-process stress skipped in -short")
	}
	const seedN = 20
	l, dir := seedLedger(t, seedN)
	ctx := context.Background()

	// A separate process: l.mu does not cross processes, so ledger.lock alone serializes it with Seal.
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLedgerAppender$", "-test.v=false")
	cmd.Env = append(os.Environ(), stressDirEnv+"="+dir)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child appender: %v", err)
	}
	childDone := make(chan error, 1)
	go func() { childDone <- cmd.Wait() }()

	// Seal until the child exits so each seal's lock handoff keeps overlapping its appends.
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

	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("VerifyDeep failed after concurrent seal+append: %v", err)
	}

	// Iter reads only the live tail; the appends under test are mostly sealed.
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

	if childN != stressN {
		t.Errorf("cross-process appends present = %d, want %d (an append was lost in the seal lock-handoff window)", childN, stressN)
	}
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

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 60; i++ {
			// A miss while the line is mid-relocation is fine; only the recorded anchor is asserted.
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

	if err := l.VerifyDeep(ctx); err != nil {
		t.Fatalf("VerifyDeep failed after concurrent anchor+seal: %v", err)
	}
	anchorSHA := l.loadAnchorSHA()
	if anchorSHA == "" {
		return // every attempt missed mid-relocation
	}
	lines, err := l.gatherAllLines()
	if err != nil {
		t.Fatalf("gatherAllLines: %v", err)
	}
	for _, line := range lines {
		if sha256Hex(line) == anchorSHA {
			return
		}
	}
	t.Errorf("ledger-anchor.json binds SHA %s to a line absent from the final chain — anchor corrupted by a concurrent seal", anchorSHA)
}
