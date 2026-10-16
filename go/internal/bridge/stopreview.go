package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

type StopKind string

const (
	StopArtifactTimeout StopKind = "artifact_timeout"
)

type StopEvent struct {
	Kind           StopKind
	Phase          string
	Cycle          int
	ElapsedS       int
	IntervalS      int
	Attempt        int
	Progressed     bool
	Busy           bool
	StdoutTail     string
	InjectedPrompt string
	State          panestream.LivenessState
}

type ReviewAction string

const (
	ReviewExtend ReviewAction = "extend"
	ReviewPause  ReviewAction = "pause"
	ReviewStop   ReviewAction = "stop"
)

type ReviewVerdict struct {
	Action ReviewAction
	Reason string
	Cause  recovery.TerminalCause
}

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

func livenessOrUnknown(s panestream.LivenessState) string {
	return strings.ReplaceAll(s.String(), "-", "_")
}

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
	default:
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
