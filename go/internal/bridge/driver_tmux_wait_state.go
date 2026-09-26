package bridge

import (
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// replWaitState is allocated per launch so state cannot leak between
// concurrent REPLs.
type replWaitState struct {
	result replWaitResult

	intervalS       int
	maxExtends      int
	reviewer        StopReviewer
	livenessCenter  *panestream.LivenessCenter
	paneProfile     panestream.PaneProfile
	livenessProfile panestream.PaneProfile
	fatalPaneStage  string
	fatalDetector   *recovery.FatalPaneDetector
	detector        completionDetector

	lastEvent               StopEvent
	lastVerdict             ReviewVerdict
	completed               bool
	transientShortcircuit   bool
	nudgeSent               bool
	submitWedged            bool
	nudgeEvent              *interaction.Event
	nudgeAt                 time.Time
	detectorErrorLogged     bool
	terminalDetectorErrored bool
	lastDetectorErr         error
	cancellationErr         error
	attempt                 int
	intervalStartS          int
	waitedS                 int

	checkpointExhaustion *exhaustionGate
	checkpointWall       *checkpointWallState
	checkpointFatal      *fatalPaneGate
}

func (s *replWaitState) observeDetector(err error) bool {
	s.terminalDetectorErrored = err != nil
	if err == nil {
		return false
	}
	s.lastDetectorErr = err
	if s.detectorErrorLogged {
		return false
	}
	s.detectorErrorLogged = true
	return true
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
		livenessCenter = panestream.NewLivenessCenter()
	}
	livenessCenter.RegisterLivenessHandler(paneLivenessHandler(w.deps.Signals, configIdentity(w.cfg)))
	paneProfile := w.channel.profile
	livenessProfile := paneProfile
	// Exhaustion decides separately through its own gate; it must not
	// override the liveness evidence handed to the reviewer.
	livenessProfile.ExhaustedRegex = ""

	fatalPaneStage := fatalPaneStageOf(w.deps)
	var fatalDetector *recovery.FatalPaneDetector
	if fatalPaneStage != "off" {
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
		fatalPaneStage:  fatalPaneStage,
		fatalDetector:   fatalDetector,
		detector:        newCompletionDetector(w.cfg.Completion, w.cfg, w.deps, w.launch, w.artifactBase),
		// Checkpoint-only gates live on this state so the fast-poll and
		// checkpoint paths can't share a streak.
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
