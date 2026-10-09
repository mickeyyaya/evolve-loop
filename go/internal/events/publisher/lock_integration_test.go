//go:build integration

package publisher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func holdChannelLock(t *testing.T, cfg Config, name string) {
	t.Helper()
	if err := os.MkdirAll(cfg.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	release, err := flock.Lock(filepath.Join(cfg.Root, name+".lock"))
	if err != nil {
		t.Fatalf("flock.Lock = %v", err)
	}
	t.Cleanup(release)
}

func TestPublisher_ALockPastItsDeadlineNeverHoldsTheDrain(t *testing.T) {
	cfg := testConfig(t, lossless("loop", ""))
	cfg.Log.LockDeadline = 100 * time.Millisecond
	holdChannelLock(t, cfg, "loop")
	p, err := newPublisher(cfg, openLog)
	if err != nil {
		t.Fatal(err)
	}
	center := signalcenter.New(signalcenter.WithPID(testPID))
	center.Subscribe(p.Listen)

	start := time.Now()
	center.Emit(loopEvent(0))
	center.Flush()
	held := time.Since(start)

	if held < cfg.Log.LockDeadline || held > cfg.Log.LockDeadline+5*time.Second {
		t.Fatalf("the drain was held %v, want about the lock deadline %v while another holder keeps the lock", held, cfg.Log.LockDeadline)
	}
	if n := p.Close(time.Millisecond); n != 1 {
		t.Fatalf("Close = %d, want 1 counted loss", n)
	}
}

func TestPublisher_OneLockCallWaitsForEachChannel(t *testing.T) {
	cfg := testConfig(t, lossless("loop", ""))
	cfg.Log.LockDeadline = time.Second
	holdChannelLock(t, cfg, "loop")
	p, err := newPublisher(cfg, openLog)
	if err != nil {
		t.Fatal(err)
	}

	p.Listen(loopEvent(1))
	start := time.Now()
	for seq := uint64(2); seq <= 5; seq++ {
		p.Listen(loopEvent(seq))
	}
	rest := time.Since(start)

	if rest >= cfg.Log.LockDeadline/2 {
		t.Fatalf("four appends behind a waiting lock call took %v, want each to fail at once with ErrLockPending", rest)
	}
	if n := p.Close(time.Millisecond); n != 5 {
		t.Fatalf("Close = %d, want 5 counted losses", n)
	}
}

func TestPublisher_CloseUnderAHeldLockReturnsTheCount(t *testing.T) {
	cfg := testConfig(t, lossless("loop", ""), bestEffort("signals", ""))
	cfg.Log.LockDeadline = 20 * time.Millisecond
	holdChannelLock(t, cfg, "loop")
	holdChannelLock(t, cfg, "signals")
	p, err := newPublisher(cfg, openLog)
	if err != nil {
		t.Fatal(err)
	}

	for seq := uint64(1); seq <= 3; seq++ {
		p.Listen(loopEvent(seq))
	}

	if n := p.Close(time.Second); n != 6 {
		t.Fatalf("Close = %d, want 6: three events lost in each of two channels whose lock another holder keeps", n)
	}
}
