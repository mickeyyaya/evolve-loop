package bridge

import (
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// replWaitState owns state that persists across completion-wait observations.
// Keeping it per launch prevents state from leaking between concurrent REPLs
// and gives extracted transitions one explicit object to update.
type replWaitState struct {
	result replWaitResult

	intervalS       int
	maxExtends      int
	reviewer        StopReviewer
	livenessCenter  *panestream.SignalCenter
	paneProfile     panestream.PaneProfile
	livenessProfile panestream.PaneProfile
	recoveryStage   string
	fatalDetector   *recovery.FatalPaneDetector
	detector        completionDetector

	lastEvent             StopEvent
	lastVerdict           ReviewVerdict
	completed             bool
	transientShortcircuit bool
	nudgeSent             bool
	nudgeEvent            *interaction.Event
	nudgeAt               time.Time
	detectorErrorLogged   bool
	attempt               int
	intervalStartS        int
	waitedS               int

	checkpointExhaustion *exhaustionGate
	checkpointWall       *checkpointWallState
	checkpointFatal      *fatalPaneGate
}

type replWaitResult struct {
	lastGoodPane string
	peakTokens   int
}

func (r *replWaitResult) recordTokens(pane string) {
	if tokens := panestream.ExtractResponseTokens(pane); tokens > r.peakTokens {
		r.peakTokens = tokens
	}
}

func newReplWaitState(w replWaiter) *replWaitState {
	// A review interval replaces a hard wall-clock deadline so productive agents
	// can continue while an idle or hung agent is paused by the reviewer.
	interval := w.cfg.ArtifactTimeoutS
	if interval <= 0 {
		interval = defaultIfZero(w.deps.ArtifactTimeoutS, tmuxArtifactTimeoutS)
	}

	maxExtends := defaultIfZero(w.deps.ArtifactMaxExtends, defaultArtifactMaxExtends)
	reviewer := w.deps.Reviewer
	if reviewer == nil {
		reviewer = newDeterministicReviewer(maxExtends)
	}

	livenessCenter := w.deps.LivenessCenter
	if livenessCenter == nil {
		livenessCenter = panestream.NewSignalCenter()
	}
	paneProfile := w.channel.profile
	livenessProfile := paneProfile
	// Exhaustion is decided separately through its persisted and corroborated
	// gate. It must not override the liveness evidence given to the reviewer.
	livenessProfile.ExhaustedRegex = ""

	recoveryStage := recoveryStageFromEnv(w.deps)
	var fatalDetector *recovery.FatalPaneDetector
	if recoveryStage != "off" {
		fatalDetector = recovery.SeedDetectorWithPromotions(
			filepath.Join(w.cfg.ProjectRoot, ".evolve", "instincts", "fatal-signatures"))
	}

	return &replWaitState{
		intervalS:       interval,
		maxExtends:      maxExtends,
		reviewer:        reviewer,
		livenessCenter:  livenessCenter,
		paneProfile:     paneProfile,
		livenessProfile: livenessProfile,
		recoveryStage:   recoveryStage,
		fatalDetector:   fatalDetector,
		detector:        newCompletionDetector(w.cfg.Completion, w.cfg, w.deps, w.launch, w.artifactBase),
		// The fast-poll and checkpoint paths run at different cadences. This
		// state owns checkpoint-only gates so neither path can borrow a streak.
		checkpointExhaustion: newExhaustionGate(),
		checkpointWall:       &checkpointWallState{},
		checkpointFatal:      newFatalPaneGate(),
	}
}

func (s *replWaitState) recordNudgeOutcome(recorder *interaction.Recorder, now func() time.Time) {
	if s.nudgeEvent == nil {
		return
	}
	result := interaction.ResultNoEffect
	if s.completed {
		result = interaction.ResultArtifactAppeared
	}
	recorder.Record(interaction.Outcome{
		Event:     *s.nudgeEvent,
		Result:    result,
		LatencyMS: now().Sub(s.nudgeAt).Milliseconds(),
	})
}
