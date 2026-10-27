package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type scriptedSource struct {
	batches [][]reader.Item
	end     error
}

func (s *scriptedSource) Next(context.Context, time.Time) ([]reader.Item, error) {
	if len(s.batches) == 0 {
		return nil, s.end
	}
	next := s.batches[0]
	s.batches = s.batches[1:]
	return next, nil
}

func landedItem(commit string) reader.Item {
	e := &signalcenter.Event{Kind: signalcenter.KindShipLanded, Fields: map[string]string{"commit": commit}}
	return reader.Item{Channel: "ship", Next: 7, Record: channel.Record{Signal: e}}
}

func followRun(t *testing.T, root string, stderr *bytes.Buffer) *curatorRun {
	t.Helper()
	signals, closeSignals := newRoleSignalCenter("inbox", root, root+"/.evolve", stderr)
	t.Cleanup(closeSignals)
	return &curatorRun{req: curateRequest{root: root, follow: true}, cfg: policy.Policy{}.InboxCuratorConfig(), git: gitexec.Default(root), stderr: stderr, signals: signals}
}

func TestCuratorRun_FollowCuratesEachLandingAndSkipsOtherRecords(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	sha := f.land(t, "feat: backfill the pool")
	other := reader.Item{Channel: "ship", Record: channel.Record{Signal: &signalcenter.Event{Kind: signalcenter.KindShipError}}}
	src := &scriptedSource{batches: [][]reader.Item{{other, landedItem(sha), landedItem("not-a-sha")}}, end: reader.ErrHangup}
	var stdout, stderr bytes.Buffer

	rc := followRun(t, f.root, &stderr).followFrom(context.Background(), src, &stdout)

	if rc != 0 || strings.Count(stdout.String(), "consume rolling-pool (landing, deterministic)") != 1 || !f.pending(t) || !strings.Contains(stderr.String(), "WARN the landing at ship:7") {
		t.Errorf("rc = %d stdout = %q stderr = %q, want one deterministic proposal, nothing applied and a WARN for the bad record", rc, stdout.String(), stderr.String())
	}
}

func TestCuratorRun_FollowStopsOnAReadErrorOrACancel(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	var stderr bytes.Buffer
	run := followRun(t, root, &stderr)

	if rc := run.followFrom(context.Background(), &scriptedSource{end: errors.New("segment torn")}, &bytes.Buffer{}); rc != 2 || !strings.Contains(stderr.String(), "segment torn") {
		t.Errorf("a read error: rc = %d stderr = %q, want 2 naming it", rc, stderr.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rc := run.followFrom(ctx, &scriptedSource{end: ctx.Err()}, &bytes.Buffer{}); rc != 0 {
		t.Errorf("a cancel: rc = %d, want 0", rc)
	}
}

func TestLandingReader_ReadsTheShipChannelOfTheRoot(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	out, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = out.Close(); _ = w.Close() })
	since, _ := reader.ParseSince("all")

	src, err := landingReader(root, since, w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.Next(context.Background(), time.Now().Add(50*time.Millisecond)); !errors.Is(err, reader.ErrDeadline) {
		t.Errorf("Next on an empty ship channel = %v, want the deadline", err)
	}
	closed, _ := os.CreateTemp(t.TempDir(), "closed")
	_ = closed.Close()
	if _, err := landingReader(root, since, closed); err == nil {
		t.Error("landingReader with a closed output = nil error, want the wake error")
	}
}

func TestCmd_InboxCurate_FollowEndsWhenItsContextEnds(t *testing.T) {
	saved := curateFollowContext
	t.Cleanup(func() { curateFollowContext = saved })
	curateFollowContext = func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, cancel
	}
	root := plainCuratorRoot(t, `{}`)

	if rc, _, stderr := curateAt(root, "", "--follow"); rc != 0 {
		t.Errorf("rc = %d stderr = %q, want 0 when the context ends", rc, stderr)
	}
}

func TestCuratorRun_FollowWithAnOutputTheKernelCannotWatchExitsIO(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	_ = closed.Close()
	var stderr bytes.Buffer

	if rc := followRun(t, root, &stderr).follow(closed, &bytes.Buffer{}); rc != 2 || !strings.Contains(stderr.String(), "kernel wake") {
		t.Errorf("rc = %d stderr = %q, want 2 naming the wake", rc, stderr.String())
	}
}

func TestCurateFollowContext_EndsWhenItsStopRuns(t *testing.T) {
	t.Parallel()
	ctx, stop := curateFollowContext()
	stop()

	if ctx.Err() == nil {
		t.Error("the follow context is live after its stop, want it ended")
	}
}

func TestCuratorRun_ACurateWithAnUnreadableInboxIsAnError(t *testing.T) {
	t.Parallel()
	root := plainCuratorRoot(t, `{}`)
	if err := os.RemoveAll(root + "/.evolve/inbox"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/.evolve/inbox", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	if _, err := followRun(t, root, &stderr).curate(nil, true); err == nil {
		t.Error("curate over an inbox that is a file = nil error, want an error")
	}
}
