package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// StopKind classifies the pipeline stop condition under review.
type StopKind string

const (
	// StopArtifactTimeout: a phase ran a full review interval without producing
	// its output artifact.
	StopArtifactTimeout StopKind = "artifact_timeout"
)

// StopEvent is the Observe-layer envelope: the evidence a reviewer needs to
// justify the next move.
type StopEvent struct {
	Kind       StopKind
	Phase      string // agent/role, e.g. "scout"
	Cycle      int
	ElapsedS   int    // total seconds waited so far
	IntervalS  int    // the review interval that just elapsed
	Attempt    int    // review index: 0 = first review, 1 = after one extension, …
	Progressed bool   // did the agent emit new output during the last interval?
	Busy       bool   // is the agent visibly mid-turn per the per-CLI busy affordance?
	StdoutTail string // recent pane/stdout — evidence for an LLM reviewer
	// InjectedPrompt: empty means fail-open — no echoes are stripped, never a
	// suppressed signal.
	InjectedPrompt string
	State          panestream.LivenessState
}

// ReviewAction is the Translate-layer verdict vocabulary.
type ReviewAction string

const (
	ReviewExtend ReviewAction = "extend" // still working — wait another interval
	ReviewPause  ReviewAction = "pause"  // stalled — surface for investigation, do not silently kill
	ReviewStop   ReviewAction = "stop"   // abandon the run
)

// ReviewVerdict is a reviewer's decision plus a human-readable justification,
// which the caller logs to the self-healing trail.
type ReviewVerdict struct {
	Action ReviewAction
	Reason string
	Cause  recovery.TerminalCause
}

// StopReviewer adjudicates a StopEvent into a verdict.
type StopReviewer interface {
	Review(ev StopEvent) ReviewVerdict
}

const artifactTimeoutMarker = launchoutcome.ArtifactTimeoutMarker

func reviewActionOrNone(a ReviewAction) string {
	if a == "" {
		return "none"
	}
	return string(a)
}

// livenessOrUnknown routes every LivenessState, including the zero value,
// through the same String() call, so a state can never be spelled twice.
func livenessOrUnknown(s panestream.LivenessState) string {
	return strings.ReplaceAll(s.String(), "-", "_")
}

// defaultArtifactMaxExtends bounds a busy-but-silent agent to ~30 min of
// extends (interval × 6) before the reviewer pauses for investigation.
const defaultArtifactMaxExtends = 6

type deterministicReviewer struct {
	maxExtends int
}

func newDeterministicReviewer(maxExtends int) deterministicReviewer {
	if maxExtends <= 0 {
		maxExtends = defaultArtifactMaxExtends
	}
	return deterministicReviewer{maxExtends: maxExtends}
}

// NewDeterministicReviewer constructs the Stage-0 deterministic reviewer.
func NewDeterministicReviewer(maxExtends int) StopReviewer {
	return newDeterministicReviewer(maxExtends)
}

func (ev StopEvent) livenessState() panestream.LivenessState {
	return ev.State
}

func (r deterministicReviewer) Review(ev StopEvent) ReviewVerdict {
	switch ev.livenessState() {
	case panestream.LivenessConverging:
		return ReviewVerdict{
			Action: ReviewExtend,
			Reason: fmt.Sprintf("agent converging: new output at interval %d — extend; real output is never capped", ev.Attempt+1),
		}
	case panestream.LivenessBusyButStagnant:
		if ev.Attempt < r.maxExtends {
			return ReviewVerdict{
				Action: ReviewExtend,
				Reason: fmt.Sprintf("agent busy mid-turn, no content delta (interval %d/%d) — extend", ev.Attempt+1, r.maxExtends),
			}
		}
		return ReviewVerdict{
			Action: ReviewPause,
			Reason: fmt.Sprintf("agent busy but produced no output — exhausted %d extensions, pause for investigation", r.maxExtends),
		}
	case panestream.LivenessHung:
		return ReviewVerdict{
			Action: ReviewPause,
			Reason: fmt.Sprintf("agent hung: stalled for %d consecutive busy intervals — fast-fail before %d-interval backstop", ev.Attempt+1, r.maxExtends),
		}
	default: // LivenessIdle or zero (fallback exhausted)
		return ReviewVerdict{
			Action: ReviewPause,
			Reason: fmt.Sprintf("no output during the last %ds interval — stalled; pause for investigation", ev.IntervalS),
		}
	}
}

func envInt(deps Deps, key string, def int) int {
	v, ok := lookupEnv(deps, key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return def
	}
	return n
}
