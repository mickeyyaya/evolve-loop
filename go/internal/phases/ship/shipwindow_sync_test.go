package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindowtest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const syncCycle = 41

type syncFixture struct {
	plane, peer, origin *gittest.Repo
	opts                *Options
	events              *[]signalcenter.Event
	center              *signalcenter.Center
}

func newSyncFixture(t *testing.T) syncFixture {
	t.Helper()
	f := syncFixture{plane: gittest.Fixture(t), origin: gittest.Bare(t)}
	f.plane.Write("base.txt", "base\n")
	f.plane.Write(".evolve/inbox/item.json", `{"id":"item"}`+"\n")
	f.plane.Git("add", "-A")
	f.plane.Git("commit", "-q", "-m", "base")
	f.plane.Git("remote", "add", "origin", f.origin.Dir)
	f.plane.Git("push", "-q", "origin", "main")
	f.peer = gittest.Clone(t, f.origin.Dir)
	f.center, f.events = recordingCenter()
	f.opts = &Options{Class: ClassCycle, ProjectRoot: f.plane.Dir, ActiveWorktree: t.TempDir(), CycleID: syncCycle, RunID: "run-41", Signals: f.center}
	return f
}

func (f syncFixture) peerCommits(t *testing.T, rel, body string) string {
	t.Helper()
	f.peer.Write(rel, body)
	f.peer.Git("add", "-A")
	f.peer.Git("commit", "-q", "-m", "origin moves "+rel)
	f.peer.Git("push", "-q", "origin", "main")
	return f.peer.Git("rev-parse", "HEAD")
}

func heldWindow(t *testing.T, root string) context.Context {
	t.Helper()
	lease, err := shipwindow.Acquire(context.Background(), filepath.Join(root, ".evolve"), shipwindow.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Release() })
	return shipwindow.WithHeld(context.Background(), lease)
}

func movedMain() error {
	return shipErr(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip, "a peer moved main")
}

func (f syncFixture) head() string { return f.plane.Git("rev-parse", "HEAD") }

func TestShip_OriginCommitAfterAuditIsFastForwardedUnderTheWindowAndSignalled(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	before := f.head()
	tip := f.peerCommits(t, "peer.txt", "from origin\n")
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if got := f.head(); got != tip {
		t.Fatalf("plane main %s, want origin tip %s: the sync must fast-forward a strictly behind main", got, tip)
	}
	f.center.Flush()
	var fired *signalcenter.Event
	for i, e := range *f.events {
		if e.Code == codeShipMainFastForwarded {
			fired = &(*f.events)[i]
		}
	}
	if fired == nil || fired.Fields["old"] != before || fired.Fields["new"] != tip || fired.Fields["cycle"] != "41" {
		t.Fatalf("signal %+v, want %s with old, new and cycle", fired, codeShipMainFastForwarded)
	}
	if !containsLog(*res, "fast-forwarded main") {
		t.Errorf("logs=%v", res.Logs)
	}
}

func TestShip_ADivergedLocalMainIsRefusedWithoutResetOrForce(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.plane.Write("local.txt", "local only\n")
	f.plane.Git("add", "-A")
	f.plane.Git("commit", "-q", "-m", "local")
	local := f.head()
	tip := f.peerCommits(t, "peer.txt", "from origin\n")
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if f.head() != local {
		t.Fatalf("a diverged local main moved to %s", f.head())
	}
	if origin := f.origin.Git("rev-parse", "main"); origin != tip {
		t.Fatalf("origin %s changed from %s: ship never force-pushes", origin, tip)
	}
	if !containsLog(*res, "diverged") {
		t.Errorf("logs=%v: a diverged main must be named", res.Logs)
	}
}

func TestShip_TheMainSyncRunsOnlyWhileTheShipWindowIsHeld(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		held     bool
		mutate   func(*Options)
		wantMove bool
		wantLog  string
	}{
		"window held":     {held: true, wantMove: true},
		"window not held": {wantLog: "sync skipped: ship window not held"},
		"manual class":    {held: true, mutate: func(o *Options) { o.Class = ClassManual }, wantLog: "sync skipped: not a worktree cycle lane"},
		"in-place cycle":  {held: true, mutate: func(o *Options) { o.ActiveWorktree = "" }, wantLog: "sync skipped: not a worktree cycle lane"},
		"dry run":         {held: true, mutate: func(o *Options) { o.DryRun = true }, wantLog: "sync skipped: not a worktree cycle lane"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newSyncFixture(t)
			before := f.head()
			tip := f.peerCommits(t, "peer.txt", "from origin\n")
			if tc.mutate != nil {
				tc.mutate(f.opts)
			}
			ctx := context.Background()
			if tc.held {
				ctx = heldWindow(t, f.plane.Dir)
			}
			res := &RunResult{}

			syncMainAfterMovedMain(ctx, f.opts, res, movedMain())

			want := before
			if tc.wantMove {
				want = tip
			}
			if f.head() != want {
				t.Fatalf("plane main %s, want %s", f.head(), want)
			}
			if tc.wantLog != "" && !containsLog(*res, tc.wantLog) {
				t.Errorf("logs=%v, want %q", res.Logs, tc.wantLog)
			}
		})
	}
}

func TestShip_ASyncOnlyFollowsAMovedMainFailure(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err  error
		move bool
	}{
		"no failure":         {},
		"fleet rebase":       {err: movedMain(), move: true},
		"head moved":         {err: shipErr(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, core.StageAtomicShip, "moved"), move: true},
		"other ship failure": {err: shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip, "drift")},
		"plain error":        {err: os.ErrClosed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newSyncFixture(t)
			before := f.head()
			tip := f.peerCommits(t, "peer.txt", "from origin\n")

			syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, &RunResult{}, tc.err)

			want := before
			if tc.move {
				want = tip
			}
			if f.head() != want {
				t.Fatalf("plane main %s, want %s", f.head(), want)
			}
		})
	}
}

func TestShip_ATrackedNonStampChangeBlocksTheSyncAndShipContinues(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	before := f.head()
	f.peerCommits(t, "base.txt", "origin edit\n")
	f.plane.Write("base.txt", "operator edit\n")
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if f.head() != before {
		t.Fatalf("a blocked fast-forward moved main to %s", f.head())
	}
	if body, _ := os.ReadFile(filepath.Join(f.plane.Dir, "base.txt")); string(body) != "operator edit\n" {
		t.Fatalf("the operator's tracked change was lost: %q", body)
	}
	if !containsLog(*res, "block the fast-forward") {
		t.Errorf("logs=%v", res.Logs)
	}
}

func TestShip_AStampTheLoopWroteIsKeptWhenOriginEditedTheSameItem(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	before := f.head()
	f.peerCommits(t, ".evolve/inbox/item.json", `{"id":"item","note":"origin"}`+"\n")
	stamp := `{"id":"item","failure_count":2}` + "\n"
	f.plane.Write(".evolve/inbox/item.json", stamp)

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, &RunResult{}, movedMain())

	body, _ := os.ReadFile(filepath.Join(f.plane.Dir, ".evolve/inbox/item.json"))
	if string(body) != stamp || f.head() != before {
		t.Fatalf("stamp %q, head %s: a mid-wave sync never rewrites a stamp the mover wrote; the boundary sync lands it", body, f.head())
	}
}

func TestShip_AStampOnAnItemOriginLeftAloneSurvivesTheFastForward(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	tip := f.peerCommits(t, "peer.txt", "from origin\n")
	stamp := `{"id":"item","failure_count":2}` + "\n"
	f.plane.Write(".evolve/inbox/item.json", stamp)

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, &RunResult{}, movedMain())

	body, _ := os.ReadFile(filepath.Join(f.plane.Dir, ".evolve/inbox/item.json"))
	if string(body) != stamp || f.head() != tip {
		t.Fatalf("stamp %q, head %s, want the stamp kept and main at %s", body, f.head(), tip)
	}
}

func TestShip_ANeverRefreshedOriginRefIsNeverTrusted(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	before := f.head()
	tip := f.peerCommits(t, "peer.txt", "from origin\n")
	f.plane.Git("fetch", "-q", "origin", "main")
	f.plane.Git("update-ref", "refs/remotes/origin/main", tip)
	f.plane.Git("remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if f.head() != before {
		t.Fatalf("main moved to %s on a stale remote-tracking ref after the fetch failed", f.head())
	}
	if !containsLog(*res, "fetch origin failed") {
		t.Errorf("logs=%v", res.Logs)
	}
}

func TestShip_ASyncOffMainIsSkipped(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.peerCommits(t, "peer.txt", "from origin\n")
	f.plane.Git("checkout", "-q", "-b", "side")
	before := f.head()
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if f.head() != before || !strings.Contains(strings.Join(res.Logs, "\n"), "plane is not on main") {
		t.Fatalf("head %s logs %v", f.head(), res.Logs)
	}
}

func TestShip_ARootCopyEqualToAClaimIsAbsorbedAfterTheFastForward(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.peerCommits(t, ".evolve/inbox/item2.json", `{"id":"item2"}`+"\n")
	claim := filepath.Join(f.plane.Dir, ".evolve/inbox/processing/cycle-9/item2.json")
	if err := os.MkdirAll(filepath.Dir(claim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claim, []byte(`{"id":"item2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if _, err := os.Stat(filepath.Join(f.plane.Dir, ".evolve/inbox/item2.json")); !os.IsNotExist(err) {
		t.Fatalf("the root copy of a claimed item stayed (stat err=%v), so the loop would claim it twice", err)
	}
	if !containsLog(*res, "item2") {
		t.Errorf("logs=%v", res.Logs)
	}
}

func TestShip_ASyncMovesNothingWhenMainIsCurrentOrAhead(t *testing.T) {
	t.Parallel()
	for name, ahead := range map[string]bool{"current": false, "ahead": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newSyncFixture(t)
			if ahead {
				f.plane.Write("local.txt", "unpublished landing\n")
				f.plane.Git("add", "-A")
				f.plane.Git("commit", "-q", "-m", "unpublished")
			}
			before := f.head()
			res := &RunResult{}

			syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

			if f.head() != before || containsLog(*res, "fast-forwarded") {
				t.Fatalf("head %s logs %v: only a strictly behind main moves", f.head(), res.Logs)
			}
		})
	}
}

func TestShip_ARelationThatCannotBeReadSkipsTheSync(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	before := f.head()
	f.peerCommits(t, "peer.txt", "from origin\n")
	f.opts.Runner = func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "rev-list" {
			return 128, nil
		}
		return sysexec.DefaultRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if f.head() != before || !containsLog(*res, "main sync skipped: rev-list") {
		t.Fatalf("head %s logs %v: an unreadable relation never fast-forwards", f.head(), res.Logs)
	}
}

func TestShip_AClaimListThatCannotBeReadIsWarnedAfterTheFastForward(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	tip := f.peerCommits(t, "peer.txt", "from origin\n")
	processing := filepath.Join(f.plane.Dir, ".evolve/inbox/processing/cycle-9")
	if err := os.MkdirAll(processing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(processing, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(processing, 0o755) })
	if _, err := os.ReadDir(processing); err == nil {
		t.Skip("the process reads a mode 000 directory (root)")
	}
	res := &RunResult{}

	syncMainAfterMovedMain(heldWindow(t, f.plane.Dir), f.opts, res, movedMain())

	if f.head() != tip || !containsLog(*res, "absorbing root copies failed") {
		t.Fatalf("head %s logs %v: the fast-forward stands and the failed absorb is named", f.head(), res.Logs)
	}
}

func TestShip_ALongSectionKeepsItsLeaseThroughBeatsAndStillFastForwardsMainInsideTheWindow(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	clock, beatsA, beatsB, probeB := shipwindowtest.NewClock(), shipwindowtest.NewBeats(), shipwindowtest.NewBeats(), shipwindowtest.NewProbe()
	f.opts.ShipWindow = windowOptions(clock, beatsA, nil, 101)
	laneB := &Options{Class: ClassCycle, ProjectRoot: f.plane.Dir, CycleID: 42, ShipWindow: windowOptions(clock, beatsB, probeB, 202)}
	ctx, releaseA := enterShipWindow(context.Background(), f.opts, &RunResult{})
	defer releaseA()
	tip := f.peerCommits(t, "peer.txt", "from origin\n")

	enteredB := make(chan func())
	probeB.Drain()
	go func() {
		_, releaseB := enterShipWindow(context.Background(), laneB, &RunResult{})
		enteredB <- releaseB
	}()
	lanes := shipwindowtest.Lanes{Clock: clock, Holder: beatsA, Waiter: probeB}
	for range 4 {
		lanes.Step(t, 5*time.Minute)
	}
	probeB.Drain()
	probeB.WaitPolls(t, 3)
	res := &RunResult{}
	syncMainAfterMovedMain(ctx, f.opts, res, movedMain())

	select {
	case <-enteredB:
		t.Fatal("lane B entered while lane A held a beating lease through a 20 minute section")
	default:
	}
	if err := shipwindow.HeldErr(ctx); err != nil {
		t.Fatalf("the holder lost its lease during the long section: %v", err)
	}
	if got := f.head(); got != tip {
		t.Fatalf("plane main %s, want origin tip %s: the fast-forward must run inside the window", got, tip)
	}
	releaseA()
	select {
	case releaseB := <-enteredB:
		releaseB()
	case <-time.After(5 * time.Second):
		t.Fatal("lane B never entered after lane A released")
	}
}

func TestShip_ALostLeaseNeverFastForwardsMain(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	clock, beats := shipwindowtest.NewClock(), shipwindowtest.NewBeats()
	lost := make(chan struct{})
	w := windowOptions(clock, beats, nil, 101)
	w.OnLost = func(*shipwindow.LostError) { close(lost) }
	lease, err := shipwindow.Acquire(context.Background(), filepath.Join(f.plane.Dir, ".evolve"), w)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Release() }()
	before := f.head()
	f.peerCommits(t, "peer.txt", "from origin\n")
	clock.Advance(shipwindow.DefaultMaxHold)
	beats.Tick(t, clock.Now())
	<-lost
	res := &RunResult{}

	syncMainAfterMovedMain(shipwindow.WithHeld(context.Background(), lease), f.opts, res, movedMain())

	if got := f.head(); got != before {
		t.Fatalf("plane main moved to %s without the lease", got)
	}
	if !containsLog(*res, "lease lost") {
		t.Errorf("logs=%v: the skip must name the lost lease", res.Logs)
	}
}
