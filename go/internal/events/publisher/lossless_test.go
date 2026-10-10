package publisher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPublisher_AWriteErrorIsACountedLossWithAnIncidentGap(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	p := startPublisher(t, cfg, openLog)
	blocker := filepath.Join(cfg.Root, "loop")
	if err := os.MkdirAll(cfg.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	p.Listen(loopEvent(3))

	got := readChannel(t, cfg, "loop")
	assertRecords(t, got, []string{"gap:write_error", "seq:3"})
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonWriteError, signalcenter.SeverityIncident, 1, 2, 2)}) {
		t.Fatalf("gap = %+v, want write_error INCIDENT seq 1..2 dropped 2", g)
	}
	if got[0].Source != "loop" {
		t.Fatalf("gap source = %q, want the role loop", got[0].Source)
	}
}

func TestPublisher_APendingLockIsALockDeadlineLoss(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	pending := fmt.Errorf("channel loop: lock: %w", channel.ErrLockPending)
	deadline := fmt.Errorf("channel loop: lock: %w", flock.ErrLockDeadline)
	log, open := scripted("loop", deadline, pending)
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))

	got := readChannel(t, cfg, "loop")
	assertRecords(t, got, []string{"gap:lock_deadline", "seq:3"})
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonLockDeadline, signalcenter.SeverityIncident, 1, 2, 2)}) {
		t.Fatalf("gap = %+v, want lock_deadline INCIDENT seq 1..2 dropped 2", g)
	}
	if len(log.batches) != 3 {
		t.Fatalf("append calls = %d, want 3: a pending lock is never retried", len(log.batches))
	}
}

func TestPublisher_EachReasonAndSeqRunGetsItsOwnGap(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	deadline := fmt.Errorf("lock: %w", flock.ErrLockDeadline)
	_, open := scripted("loop", errors.New("eio"), deadline, errors.New("enospc"))
	p := startPublisher(t, cfg, open)

	for seq := uint64(1); seq <= 4; seq++ {
		p.Listen(loopEvent(seq))
	}

	got := readChannel(t, cfg, "loop")
	assertRecords(t, got, []string{"gap:write_error", "gap:lock_deadline", "gap:write_error", "seq:4"})
	want := []channel.Gap{
		wantGap(channel.ReasonWriteError, signalcenter.SeverityIncident, 1, 1, 1),
		wantGap(channel.ReasonLockDeadline, signalcenter.SeverityIncident, 2, 2, 1),
		wantGap(channel.ReasonWriteError, signalcenter.SeverityIncident, 3, 3, 1),
	}
	if g := gapsOf(got); !reflect.DeepEqual(g, want) {
		t.Fatalf("gaps = %+v, want %+v", g, want)
	}
}

func TestPublisher_AFailedGapStaysPendingWithTheNewLoss(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	log, open := scripted("loop", errors.New("eio"), errors.New("eio"))
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))

	got := readChannel(t, cfg, "loop")
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonWriteError, signalcenter.SeverityIncident, 1, 2, 2)}) {
		t.Fatalf("gaps = %+v, want one write_error gap for seq 1..2", g)
	}
	if second := log.batches[1]; len(second) != 2 || second[0].Gap == nil || second[0].Gap.Dropped != 1 {
		t.Fatalf("second batch = %+v, want the pending gap of seq 1 and then seq 2", second)
	}
}

func TestPublisher_ARotationErrorIsNotALoss(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	_, open := scripted("loop", fmt.Errorf("channel loop: %w: eisdir", channel.ErrRotate))
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))

	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("Close = %d, want 0: the batch of a rotation error is on disk", n)
	}
	assertRecords(t, readChannel(t, cfg, "loop"), []string{"seq:1", "seq:2"})
}

func TestPublisher_CloseWritesAPendingGap(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	_, open := scripted("loop", errors.New("eio"))
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))

	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("Close = %d, want 0: the gap went out", n)
	}
	assertRecords(t, readChannel(t, cfg, "loop"), []string{"gap:write_error"})
}

func TestPublisher_CloseReturnsTheCountOfAGapThatDidNotGoOut(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""), lossless("ci", ""))
	_, open := scripted("loop", errors.New("eio"), errors.New("eio"), errors.New("eio"))
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	p.Listen(loopEvent(2))

	if n := p.Close(time.Minute); n != 2 {
		t.Fatalf("Close = %d, want 2: the gap write failed", n)
	}
}

func TestPublisher_CloseAfterTheDeadlineDoesNotTryTheGap(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""))
	log, open := scripted("loop", errors.New("eio"))
	p := startPublisher(t, cfg, open)
	p.Listen(loopEvent(1))

	n := p.closeBy(expired(t))

	if n != 1 || len(log.batches) != 1 {
		t.Fatalf("closeBy = %d after %d appends, want 1 and no gap append past the deadline", n, len(log.batches))
	}
}

func TestMergeLosses_OneGapForEachContiguousSeqRunInAnyOrder(t *testing.T) {
	t.Parallel()
	one := func(reason string, pid int, seq uint64) []loss {
		return []loss{{reason: reason, pid: pid, first: seq, last: seq}}
	}
	cases := []struct {
		name   string
		inputs [][]loss
		want   []loss
	}{
		{"drops 5 and 9 with 6 to 8 written", [][]loss{one("write_error", 1, 9), one("write_error", 1, 5)},
			[]loss{{"write_error", 1, 5, 5}, {"write_error", 1, 9, 9}}},
		{"a contiguous run in any order", [][]loss{one("queue_full", 1, 7), one("queue_full", 1, 5), one("queue_full", 1, 6)},
			[]loss{{"queue_full", 1, 5, 7}}},
		{"an overlap counts each seq once", [][]loss{{{"queue_full", 1, 5, 7}}, {{"queue_full", 1, 6, 9}}},
			[]loss{{"queue_full", 1, 5, 9}}},
		{"a run inside another keeps the outer end", [][]loss{{{"queue_full", 1, 5, 9}}, {{"queue_full", 1, 6, 7}}},
			[]loss{{"queue_full", 1, 5, 9}}},
		{"the runs of one pid join across another pid", [][]loss{one("write_error", 1, 5), one("write_error", 2, 6), one("write_error", 1, 6)},
			[]loss{{"write_error", 1, 5, 6}, {"write_error", 2, 6, 6}}},
		{"two reasons never join", [][]loss{one("write_error", 1, 5), one("lock_deadline", 1, 6)},
			[]loss{{"write_error", 1, 5, 5}, {"lock_deadline", 1, 6, 6}}},
		{"two pids never join", [][]loss{one("write_error", 2, 6), one("write_error", 1, 5)},
			[]loss{{"write_error", 1, 5, 5}, {"write_error", 2, 6, 6}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var forward, backward []loss
			for i := range tc.inputs {
				forward = mergeLosses(forward, tc.inputs[i])
				backward = mergeLosses(backward, tc.inputs[len(tc.inputs)-1-i])
			}

			if !reflect.DeepEqual(forward, tc.want) || !reflect.DeepEqual(backward, tc.want) {
				t.Fatalf("mergeLosses = %+v forward and %+v backward, want %+v", forward, backward, tc.want)
			}
			for _, l := range forward {
				if l.dropped() != int(l.last-l.first+1) {
					t.Fatalf("loss %+v dropped %d, want last-first+1", l, l.dropped())
				}
			}
		})
	}
}
