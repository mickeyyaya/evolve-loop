package shipwindow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindowtest"
)

func beatOptions(clock *shipwindowtest.Clock, beats *shipwindowtest.Beats, pid int) Options {
	return Options{Now: clock.Now, Ticker: beats.Ticker, OwnerPID: pid, Alive: func(int) bool { return true }, Poll: time.Millisecond}
}

func mustAcquire(t *testing.T, ctx context.Context, dir string, o Options) *Lease {
	t.Helper()
	l, err := Acquire(ctx, dir, o)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	return l
}

func TestAcquire_LiveHolderPastTTLWithHeartbeatIsNotBroken(t *testing.T) {
	dir := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	holder := mustAcquire(t, context.Background(), dir, beatOptions(clock, beats, 101))
	defer func() { _ = holder.Release() }()
	for i := 0; i < 8; i++ {
		clock.Advance(5 * time.Minute)
		beats.Tick(t, clock.Now())
	}

	var broken []StaleHolder
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	sibOpts := beatOptions(clock, beats, 202)
	sibOpts.OnStaleBreak = func(h StaleHolder) { broken = append(broken, h) }
	sibling, err := Acquire(ctx, dir, sibOpts)
	if err == nil {
		_ = sibling.Release()
		t.Fatal("a sibling took the lease 40 minutes in while the holder still beats")
	}
	if len(broken) != 0 {
		t.Fatalf("the sibling broke a live beating holder: %+v", broken)
	}
	if err := holder.Err(); err != nil {
		t.Fatalf("holder.Err() = %v, want nil while it holds the lease", err)
	}
	if got, want := beats.Period(), DefaultTTL/3; got != want {
		t.Fatalf("heartbeat period = %v, want TTL/3 = %v", got, want)
	}
}

func TestHeartbeat_ReleaseAndContextCancelStopTheBeat(t *testing.T) {
	for name, stop := range map[string]func(*Lease, context.CancelFunc){
		"release": func(l *Lease, _ context.CancelFunc) {
			if err := l.Release(); err != nil {
				t.Fatal(err)
			}
		},
		"cancel": func(_ *Lease, cancel context.CancelFunc) { cancel() },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			l := mustAcquire(t, ctx, dir, beatOptions(clock, beats, 101))
			stop(l, cancel)
			beats.WaitStopped(t)
			_ = l.Release()
		})
	}
}

func TestHeartbeat_AForeignTokenMarksTheLeaseLost(t *testing.T) {
	dir := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	lost := make(chan *LostError, 1)
	o := beatOptions(clock, beats, 101)
	o.OnLost = func(e *LostError) { lost <- e }
	l := mustAcquire(t, context.Background(), dir, o)
	foreign := `{"owner_pid":202,"heartbeat_at":"2026-11-21T00:00:00Z","token":"foreign"}` + "\n"
	if err := os.WriteFile(PathIn(dir), []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	clock.Advance(5 * time.Minute)
	beats.Tick(t, clock.Now())
	select {
	case e := <-lost:
		if e.Reason == "" {
			t.Fatal("OnLost carried no reason")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a beat that found a foreign token did not call OnLost")
	}
	if _, ok := l.Err().(*LostError); !ok {
		t.Fatalf("Err() = %v, want a *LostError", l.Err())
	}
	if raw, _ := os.ReadFile(PathIn(dir)); string(raw) != foreign {
		t.Fatalf("the lost holder touched the lease of the new owner: %q", raw)
	}
}

func TestHeartbeat_TheHardCapReleasesAndMarksLost(t *testing.T) {
	dir := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	lost := make(chan *LostError, 1)
	o := beatOptions(clock, beats, 101)
	o.OnLost = func(e *LostError) { lost <- e }
	l := mustAcquire(t, context.Background(), dir, o)
	clock.Advance(DefaultMaxHold)
	beats.Tick(t, clock.Now())
	select {
	case <-lost:
	case <-time.After(5 * time.Second):
		t.Fatal("a holder at the hard cap was not marked lost")
	}
	if _, err := os.Stat(PathIn(dir)); !os.IsNotExist(err) {
		t.Fatalf("the hard cap left the lease in place (stat err=%v)", err)
	}
	if _, ok := HeldErr(WithHeld(context.Background(), l)).(*LostError); !ok {
		t.Fatalf("HeldErr = %v, want a *LostError", HeldErr(WithHeld(context.Background(), l)))
	}
}

func TestLostError_NamesTheReason(t *testing.T) {
	if got := (&LostError{Reason: "why"}).Error(); got != "shipwindow: lease lost: why" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestLostError_Fields(t *testing.T) {
	f := (&LostError{Reason: "why"}).Fields()
	if len(f) != 2 || f["role"] != "holder" || f["reason"] != "why" {
		t.Fatalf("Fields() = %v, want role=holder and reason=why", f)
	}
}

func TestErr_ALeaseWithoutABeatAndAContextWithoutALeaseAreNeverLost(t *testing.T) {
	if err := (&Lease{}).Err(); err != nil {
		t.Fatalf("Err() = %v for a lease that never beat", err)
	}
	if err := HeldErr(context.Background()); err != nil {
		t.Fatalf("HeldErr = %v for a context with no lease", err)
	}
}

func TestHeartbeat_AFailedBeatWriteMarksTheLeaseLostWithTheStep(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file modes")
	}
	for name, tc := range map[string]struct {
		break_ func(t *testing.T, dir string)
		want   string
	}{
		"marshal": {func(t *testing.T, _ string) {
			prev := marshalRecord
			t.Cleanup(func() { marshalRecord = prev })
			marshalRecord = func(any) ([]byte, error) { return nil, errors.New("boom") }
		}, "heartbeat marshal"},
		"stage": {func(t *testing.T, dir string) {
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		}, "heartbeat stage"},
		"replace": {func(t *testing.T, _ string) {
			prev := renameLease
			t.Cleanup(func() { renameLease = prev })
			renameLease = func(string, string) error { return errors.New("boom") }
		}, "heartbeat replace"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
			lost := make(chan *LostError, 1)
			o := beatOptions(clock, beats, 101)
			o.OnLost = func(e *LostError) { lost <- e }
			l := mustAcquire(t, context.Background(), dir, o)
			tc.break_(t, dir)
			clock.Advance(5 * time.Minute)
			beats.Tick(t, clock.Now())
			select {
			case e := <-lost:
				if !strings.Contains(e.Reason, tc.want) {
					t.Fatalf("reason = %q, want %q", e.Reason, tc.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("a beat that could not write the heartbeat did not mark the lease lost")
			}
			_ = os.Chmod(dir, 0o755)
			if err := l.Release(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAcquire_AQueueThatCannotTakeATicketIsAnError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file modes")
	}
	dir := t.TempDir()
	queueDir := filepath.Join(dir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(queueDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(queueDir, 0o755) })
	if _, err := Acquire(context.Background(), dir, Options{}); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Acquire err = %v, want the enqueue failure", err)
	}
}

func TestRelease_ALeaseThatCannotBeRemovedIsAnError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file modes")
	}
	dir := t.TempDir()
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	l := mustAcquire(t, context.Background(), dir, beatOptions(clock, beats, 101))
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := l.Release(); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Release err = %v, want the remove failure", err)
	}
}
