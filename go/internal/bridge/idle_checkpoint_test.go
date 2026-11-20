package bridge

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const (
	idleCheckpointNudgeText = "Please write the deliverable"
	idleCheckpointBusyPane  = tmuxPromptMarkerDefault + "\n⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"
)

var stopReviewElapsedPattern = regexp.MustCompile(`stop-review\[[a-z_]+\] elapsed=(\d+)s`)

type idleCheckpointTmux struct {
	*nudgeRecordingTmux
	pasted atomic.Bool
	ticks  atomic.Int64
	paneAt func(tick int) string
	onTick func(tick int)
}

func (c *idleCheckpointTmux) PasteBuffer(ctx context.Context, buffer string) error {
	c.pasted.Store(true)
	return c.nudgeRecordingTmux.PasteBuffer(ctx, buffer)
}

func (c *idleCheckpointTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	pane, err := c.nudgeRecordingTmux.CapturePane(ctx, session, scrollback)
	if !c.pasted.Load() || c.paneAt == nil {
		return pane, err
	}
	return c.paneAt(int(c.ticks.Load())), nil
}

func (c *idleCheckpointTmux) sleep(d time.Duration) {
	if !c.pasted.Load() || d != artifactWaitInterval {
		return
	}
	tick := int(c.ticks.Add(1))
	if c.onTick != nil {
		c.onTick(tick)
	}
}

type idleCheckpointRun struct {
	code   int
	stderr string
	tmux   *idleCheckpointTmux
	fx     launchFixture
}

func (r idleCheckpointRun) firstStopReviewElapsed(t *testing.T) int {
	t.Helper()
	m := stopReviewElapsedPattern.FindStringSubmatch(r.stderr)
	if m == nil {
		t.Fatalf("no stop-review checkpoint was logged; stderr=%q", r.stderr)
	}
	elapsed, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("stop-review elapsed %q: %v", m[1], err)
	}
	return elapsed
}

func (r idleCheckpointRun) nudges() int {
	return r.tmux.deliveriesNaming(idleCheckpointNudgeText)
}

func writeIdleCheckpointDeliverable(t *testing.T, fx launchFixture) {
	t.Helper()
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nPROTOTYPE OK\n"), 0o644); err != nil {
		t.Errorf("write the deliverable: %v", err)
	}
}

func runIdleCheckpoint(t *testing.T, deps Deps, paneAt func(tick int) string, onTick func(fx launchFixture, tick int)) idleCheckpointRun {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	tm := &idleCheckpointTmux{
		nudgeRecordingTmux: &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}},
		paneAt:             paneAt,
	}
	if onTick != nil {
		tm.onTick = func(tick int) { onTick(fx, tick) }
	}
	deps.Tmux = tm
	deps.Sleep = tm.sleep
	deps.CaptureBaseline = zeroBaselineCapture
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := newTestEngine(deps).LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	return idleCheckpointRun{code: code, stderr: stderr.String(), tmux: tm, fx: fx}
}

func idlePane(int) string { return tmuxPromptMarkerDefault }

func busyPane(int) string { return idleCheckpointBusyPane }

func TestIdleCheckpoint_IdlePaneIsNudgedAfterTheGraceNotTheArtifactTimeout(t *testing.T) {
	t.Parallel()
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 300, IdleCheckpointGraceS: 10}, idlePane, nil)

	if first := r.firstStopReviewElapsed(t); first < 10 || first > 14 {
		t.Errorf("first checkpoint at %ds, want within one or two ticks of the 10s idle grace, not the 300s artifact timeout; stderr=%q", first, r.stderr)
	}
	if r.nudges() != 1 {
		t.Errorf("nudges delivered = %d, want exactly 1", r.nudges())
	}
	if r.code != ExitArtifactTimeout {
		t.Errorf("exit = %d, want ExitArtifactTimeout (%d): the nudge is one-shot", r.code, ExitArtifactTimeout)
	}
	nudgeOutcomes := 0
	for _, outcome := range readInteractionLedger(t, r.fx.ws, "claude-tmux") {
		if outcome.Kind == interaction.KindNudge {
			nudgeOutcomes++
		}
	}
	if nudgeOutcomes != 1 {
		t.Errorf("recorded nudge outcomes = %d, want 1", nudgeOutcomes)
	}
}

func TestIdleCheckpoint_DefaultGraceStillBeatsTheArtifactTimeout(t *testing.T) {
	t.Parallel()
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 300}, idlePane, nil)

	if first := r.firstStopReviewElapsed(t); first >= 300 {
		t.Errorf("first checkpoint at %ds with no configured grace, want the default grace to fire before the 300s artifact timeout", first)
	}
	if r.nudges() != 1 {
		t.Errorf("nudges delivered = %d, want exactly 1", r.nudges())
	}
}

func TestIdleCheckpoint_GraceBeyondTheIntervalChangesNothing(t *testing.T) {
	t.Parallel()
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 20, IdleCheckpointGraceS: 600}, idlePane, nil)

	if first := r.firstStopReviewElapsed(t); first < 20 || first > 24 {
		t.Errorf("first checkpoint at %ds, want the 20s artifact timeout: a grace longer than the interval never delays it", first)
	}
}

func TestIdleCheckpoint_BusyPaneWaitsTheFullInterval(t *testing.T) {
	t.Parallel()
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 40, IdleCheckpointGraceS: 10}, busyPane, nil)

	if first := r.firstStopReviewElapsed(t); first < 40 {
		t.Errorf("first checkpoint at %ds on a busy pane, want no earlier than the 40s artifact timeout", first)
	}
	if r.nudges() != 0 {
		t.Errorf("a busy pane was nudged %d time(s), want 0", r.nudges())
	}
}

func TestIdleCheckpoint_IdleClockRestartsAfterBusyWork(t *testing.T) {
	t.Parallel()
	const busyTicks = 20
	paneAt := func(tick int) string {
		if tick <= busyTicks {
			return idleCheckpointBusyPane
		}
		return tmuxPromptMarkerDefault
	}
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 300, IdleCheckpointGraceS: 10}, paneAt, nil)

	if first := r.firstStopReviewElapsed(t); first < 48 || first > 60 {
		t.Errorf("first checkpoint at %ds, want about 10s after the pane went idle at 40s: the idle clock must restart when the pane was busy", first)
	}
}

func TestIdleCheckpoint_ArtifactInsideTheGraceCompletesWithoutANudge(t *testing.T) {
	t.Parallel()
	writeAt := func(fx launchFixture, tick int) {
		if tick == 3 {
			writeIdleCheckpointDeliverable(t, fx)
		}
	}
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 300, IdleCheckpointGraceS: 10}, idlePane, writeAt)

	if r.code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK: the deliverable landed inside the grace; stderr=%q", r.code, r.stderr)
	}
	if r.nudges() != 0 {
		t.Errorf("nudges delivered = %d, want 0 when the deliverable lands inside the grace", r.nudges())
	}
	if stopReviewElapsedPattern.MatchString(r.stderr) {
		t.Errorf("a checkpoint fired although the deliverable landed inside the grace; stderr=%q", r.stderr)
	}
}

func TestIdleCheckpoint_ArtifactAfterTheEarlyNudgeStillCompletes(t *testing.T) {
	t.Parallel()
	writeAt := func(fx launchFixture, tick int) {
		if tick == 40 {
			writeIdleCheckpointDeliverable(t, fx)
		}
	}
	r := runIdleCheckpoint(t, Deps{ArtifactTimeoutS: 300, IdleCheckpointGraceS: 10}, idlePane, writeAt)

	if r.nudges() != 1 {
		t.Errorf("nudges delivered = %d, want exactly 1 at the idle grace", r.nudges())
	}
	if r.code != ExitOK {
		t.Errorf("exit = %d, want ExitOK: a deliverable written well after the early nudge must still complete the phase; stderr=%q", r.code, r.stderr)
	}
}
