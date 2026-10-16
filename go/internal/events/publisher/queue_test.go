package publisher

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPublisher_QueueOverflowWritesOneGapWithTheExactSeqRange(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.QueueEvents = 2
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	<-gate.entered
	for seq := uint64(2); seq <= 6; seq++ {
		p.Listen(loopEvent(seq))
	}
	close(gate.release)

	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("Close = %d, want 0: the gap went out", n)
	}
	got := readChannel(t, cfg, "signals")
	assertRecords(t, got, []string{"seq:1", "gap:queue_full", "seq:2", "seq:3"})
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonQueueFull, signalcenter.SeverityWarn, 4, 6, 3)}) {
		t.Fatalf("gap = %+v, want queue_full WARN seq 4..6 dropped 3", g)
	}
}

func TestPublisher_QueueBytesBoundTheQueue(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	line, err := json.Marshal(channel.Record{Source: "loop", Signal: ptr(loopEvent(2))})
	if err != nil {
		t.Fatal(err)
	}
	cfg.QueueBytes = len(line) + 1
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	<-gate.entered
	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))
	close(gate.release)
	p.Close(time.Minute)

	got := readChannel(t, cfg, "signals")
	assertRecords(t, got, []string{"seq:1", "gap:queue_full", "seq:2"})
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonQueueFull, signalcenter.SeverityWarn, 3, 3, 1)}) {
		t.Fatalf("gap = %+v, want queue_full seq 3..3", g)
	}
}

func TestPublisher_AnEnqueueDeadlineWaitsForSpace(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.QueueEvents = 1
	cfg.EnqueueDeadline = time.Hour
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)
	p.Listen(loopEvent(1))
	<-gate.entered
	p.Listen(loopEvent(2))

	listened := make(chan struct{})
	go func() {
		p.Listen(loopEvent(3))
		close(listened)
	}()
	close(gate.release)
	<-listened
	p.Close(time.Minute)

	assertRecords(t, readChannel(t, cfg, "signals"), []string{"seq:1", "seq:2", "seq:3"})
}

func TestPublisher_AnEnqueueDeadlineThatPassesIsAQueueFullLoss(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.QueueEvents = 1
	cfg.EnqueueDeadline = time.Millisecond
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)
	p.Listen(loopEvent(1))
	<-gate.entered

	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))
	close(gate.release)
	p.Close(time.Minute)

	got := readChannel(t, cfg, "signals")
	assertRecords(t, got, []string{"seq:1", "gap:queue_full", "seq:2"})
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonQueueFull, signalcenter.SeverityWarn, 3, 3, 1)}) {
		t.Fatalf("gap = %+v, want queue_full seq 3..3", g)
	}
}

func TestPublisher_ALossDuringAFailedAppendKeepsTheOrderOfTheLosses(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.QueueEvents = 1
	gate, open := gated("signals")
	gate.fail = errors.New("eio")
	p := startPublisher(t, cfg, open)
	p.Listen(loopEvent(1))
	<-gate.entered

	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))
	close(gate.release)
	p.Close(time.Minute)

	got := readChannel(t, cfg, "signals")
	assertRecords(t, got, []string{"gap:write_error", "gap:queue_full", "seq:2"})
	want := []channel.Gap{
		wantGap(channel.ReasonWriteError, signalcenter.SeverityWarn, 1, 1, 1),
		wantGap(channel.ReasonQueueFull, signalcenter.SeverityWarn, 3, 3, 1),
	}
	if g := gapsOf(got); !reflect.DeepEqual(g, want) {
		t.Fatalf("gaps = %+v, want %+v: the loss of the failed batch is older", g, want)
	}
}

func TestPublisher_CloseDrainsTheQueueBeforeTheDeadline(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)
	p.Listen(loopEvent(1))
	<-gate.entered
	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))

	closed := make(chan int)
	go func() { closed <- p.Close(time.Hour) }()
	close(gate.release)

	if n := <-closed; n != 0 {
		t.Fatalf("Close = %d, want 0", n)
	}
	assertRecords(t, readChannel(t, cfg, "signals"), []string{"seq:1", "seq:2", "seq:3"})
}

func TestPublisher_CloseCountsWhatIsLeftAtTheDeadline(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)
	p.Listen(loopEvent(1))
	<-gate.entered
	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))

	p.stopWriters(expired(t))
	close(gate.release)

	if n := p.closeBy(expired(t)); n != 2 {
		t.Fatalf("closeBy = %d, want 2: seq 2 and 3 were still queued at the deadline", n)
	}
	assertRecords(t, readChannel(t, cfg, "signals"), []string{"seq:1"})
	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("a second Close = %d, want 0: it writes the pending gap", n)
	}
	got := readChannel(t, cfg, "signals")
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonQueueFull, signalcenter.SeverityWarn, 2, 3, 2)}) {
		t.Fatalf("gap = %+v, want queue_full seq 2..3 dropped 2", g)
	}
}

func TestPublisher_AnEventAfterCloseIsACountedLoss(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	p := startPublisher(t, cfg, openLog)
	p.Close(time.Minute)

	p.Listen(loopEvent(1))

	if n := p.closeBy(expired(t)); n != 1 {
		t.Fatalf("closeBy = %d, want 1: the queue is closed", n)
	}
}

func TestPublisher_ABestEffortWriteErrorIsAWarnGap(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	_, open := scripted("signals", errors.New("eio"))
	p := startPublisher(t, cfg, open)

	p.Listen(loopEvent(1))
	p.Close(time.Minute)

	got := readChannel(t, cfg, "signals")
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonWriteError, signalcenter.SeverityWarn, 1, 1, 1)}) {
		t.Fatalf("gap = %+v, want write_error WARN seq 1..1", g)
	}
}

func TestPublisher_ABlockedBestEffortChannelNeverDelaysTheEmitterOrAnotherChannel(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""), bestEffort("signals", ""))
	gate, open := gated("signals")
	p := startPublisher(t, cfg, open)
	center := signalcenter.New(signalcenter.WithPID(testPID))
	center.Subscribe(p.Listen)

	for i := 0; i < 3; i++ {
		center.Emit(loopEvent(0))
	}
	center.Flush()

	assertRecords(t, readChannel(t, cfg, "loop"), []string{"seq:1", "seq:2", "seq:3"})
	<-gate.entered
	close(gate.release)
	p.Close(time.Minute)
	assertRecords(t, readChannel(t, cfg, "signals"), []string{"seq:1", "seq:2", "seq:3"})
}

func TestPublisher_ASlowReaderNeverDelaysAnAppend(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, lossless("loop", ""), bestEffort("signals", ""))
	p := startPublisher(t, cfg, openLog)
	p.Listen(loopEvent(1))
	l, err := channel.New(cfg.Root, "loop", cfg.Log)
	if err != nil {
		t.Fatal(err)
	}
	segs, err := l.Segments()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(segs[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := reader.Read(make([]byte, 8)); err != nil {
		t.Fatal(err)
	}

	for seq := uint64(2); seq <= 5; seq++ {
		p.Listen(loopEvent(seq))
	}

	assertRecords(t, readChannel(t, cfg, "loop"), []string{"seq:1", "seq:2", "seq:3", "seq:4", "seq:5"})
	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("Close = %d, want 0", n)
	}
	assertRecords(t, readChannel(t, cfg, "signals"), []string{"seq:1", "seq:2", "seq:3", "seq:4", "seq:5"})
}

func TestPublisher_CloseTwiceIsSafe(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	p := startPublisher(t, cfg, openLog)

	first, second := p.Close(time.Minute), p.Close(time.Minute)

	if first != 0 || second != 0 {
		t.Fatalf("Close = %d then %d, want 0 and 0", first, second)
	}
}

func ptr(e signalcenter.Event) *signalcenter.Event { return &e }

func TestPublisher_ConcurrentEnqueuersShareTheSpaceOfOneTake(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.QueueEvents = 2
	cfg.EnqueueDeadline = time.Hour
	gate, open := gated("signals")
	gate.gates = 2
	p := startPublisher(t, cfg, open)
	t.Cleanup(func() { close(gate.release) })
	p.Listen(loopEvent(1))
	<-gate.entered
	p.Listen(loopEvent(2))
	p.Listen(loopEvent(3))
	waiters := []chan struct{}{make(chan struct{}), make(chan struct{})}
	for i, done := range waiters {
		go func(seq uint64, done chan struct{}) {
			p.Listen(loopEvent(seq))
			close(done)
		}(uint64(4+i), done)
	}

	gate.release <- struct{}{}
	<-gate.entered

	for _, done := range waiters {
		finishes(t, done, "an enqueuer while the queue has space and the writer is slow")
	}
	gate.release <- struct{}{}
	if n := p.Close(time.Minute); n != 0 {
		t.Fatalf("Close = %d, want 0", n)
	}
	if got := signalSeqs(readChannel(t, cfg, "signals")); len(got) != 5 {
		t.Fatalf("seqs = %v, want all 5", got)
	}
}

func TestPublisher_AListenAfterCloseNeverWaitsTheEnqueueDeadline(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.EnqueueDeadline = time.Hour
	p := startPublisher(t, cfg, openLog)
	p.Close(time.Minute)

	done := make(chan struct{})
	go func() {
		p.Listen(loopEvent(1))
		close(done)
	}()

	finishes(t, done, "Listen after Close")
	if n := p.closeBy(expired(t)); n != 1 {
		t.Fatalf("closeBy = %d, want 1 queue_full loss", n)
	}
}

func TestPublisher_ARecordThatDoesNotEncodeIsAWriteErrorLoss(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, bestEffort("signals", ""))
	p := startPublisher(t, cfg, openLog)
	p.routes[0].queue.marshal = func(any) ([]byte, error) { return nil, errors.New("unsupported value") }

	p.Listen(loopEvent(1))
	p.Close(time.Minute)

	got := readChannel(t, cfg, "signals")
	if g := gapsOf(got); !reflect.DeepEqual(g, []channel.Gap{wantGap(channel.ReasonWriteError, signalcenter.SeverityWarn, 1, 1, 1)}) {
		t.Fatalf("records = %+v, want one write_error gap and no signal", got)
	}
}

func fullQueueWithLongDeadline(t *testing.T) *queue {
	t.Helper()
	cfg := testConfig(t, bestEffort("signals", ""))
	cfg.QueueEvents = 1
	cfg.EnqueueDeadline = time.Hour
	q := newQueue(cfg)
	if reason, ok := q.push(channel.Record{}); !ok {
		t.Fatalf("first push = %q, want accepted", reason)
	}
	return q
}

func TestPublisher_ABlockedPutReturnsAtClose(t *testing.T) {
	t.Parallel()
	q := fullQueueWithLongDeadline(t)
	type outcome struct {
		reason string
		ok     bool
	}
	got := make(chan outcome, 1)
	go func() {
		reason, ok := q.push(channel.Record{})
		got <- outcome{reason, ok}
	}()
	time.Sleep(200 * time.Millisecond)

	q.closeQueue()

	done := make(chan struct{})
	var res outcome
	go func() {
		res = <-got
		close(done)
	}()
	finishes(t, done, "a put blocked on a full queue at close")
	if res.ok || res.reason != channel.ReasonQueueFull {
		t.Fatalf("blocked put = (%q, %v), want (%q, false)", res.reason, res.ok, channel.ReasonQueueFull)
	}
}

func TestPublisher_EveryBlockedPutReturnsAtClose(t *testing.T) {
	t.Parallel()
	q := fullQueueWithLongDeadline(t)
	dones := make([]chan struct{}, 3)
	for i := range dones {
		dones[i] = make(chan struct{})
		go func(done chan struct{}) {
			q.push(channel.Record{})
			close(done)
		}(dones[i])
	}
	time.Sleep(200 * time.Millisecond)

	q.closeQueue()

	for _, done := range dones {
		finishes(t, done, "one of several puts blocked on a full queue at close")
	}
}

func TestPublisher_APutAfterCloseIsRefusedWithoutWaiting(t *testing.T) {
	t.Parallel()
	q := fullQueueWithLongDeadline(t)
	q.closeQueue()

	done := make(chan struct{})
	var ok bool
	go func() {
		_, ok = q.push(channel.Record{})
		close(done)
	}()

	finishes(t, done, "a put after close")
	if ok {
		t.Fatalf("put after close was accepted")
	}
}
