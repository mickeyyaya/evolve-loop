package bridge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// stopreview.go — Stage-0 of the self-healing review layer (the "vertical
// slice"; see docs/architecture/adr/0026-self-healing-review-layer.md).
//
// A pipeline stop condition (today: the *-tmux artifact wait elapsing a review
// interval without the artifact appearing) is no longer a silent kill. Instead
// it is an Observation that triggers a Review which justifies the next move:
//
//	Observe   → StopEvent (the evidence envelope)
//	Review    → StopReviewer.Review (deterministic now; LLM/orchestrator later)
//	Translate → ReviewVerdict {extend | pause | stop}
//	Execute   → the caller applies it and logs the justification
//
// Stage 0 ships the envelope + interface + a deterministic reviewer wired into
// the artifact wait. The loop extends this seam (Stage 1) to the other stop
// kinds (non-zero exit, launch error, audit block) and an LLM reviewer.

// StopKind classifies the pipeline stop condition under review. Stage 0 emits
// only StopArtifactTimeout; the enum is extension-ready so one review layer can
// cover every stop point without a parallel mechanism per kind.
type StopKind string

const (
	// StopArtifactTimeout: a phase ran a full review interval without producing
	// its output artifact.
	StopArtifactTimeout StopKind = "artifact_timeout"
)

// StopEvent is the Observe-layer envelope: the evidence a reviewer needs to
// justify the next move. Fields are additive — a new StopKind populates what it
// has and leaves the rest zero.
type StopEvent struct {
	Kind       StopKind
	Phase      string // agent/role, e.g. "scout"
	Cycle      int
	ElapsedS   int    // total seconds waited so far
	IntervalS  int    // the review interval that just elapsed
	Attempt    int    // review index: 0 = first review, 1 = after one extension, …
	Progressed bool   // did the agent emit new output during the last interval?
	Busy       bool   // is the agent visibly mid-turn per the per-CLI busy affordance?
	StdoutTail string // recent pane/stdout — evidence for an LLM reviewer (Stage 1)
	// State, when non-zero, carries the per-CLI liveness detector's structured
	// verdict; the reviewer uses it in preference to the coarse Progressed+Busy
	// booleans. Zero value (0) means "not set" — falls back to Progressed+Busy.
	State panestream.LivenessState
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
}

// StopReviewer adjudicates a StopEvent into a verdict. Stage 0 ships
// deterministicReviewer; Stage 1 adds an LLM/orchestrator reviewer behind this
// same interface, so the loop wiring never changes.
type StopReviewer interface {
	Review(ev StopEvent) ReviewVerdict
}

// defaultArtifactMaxExtends backstops a continuously-working-but-never-finishing
// agent: after this many review intervals the reviewer pauses for investigation
// rather than extending forever. With the 300s default interval this is ~30 min
// of wall-clock before a hung-yet-noisy agent is surfaced.
const defaultArtifactMaxExtends = 6

// deterministicReviewer is the Stage-0 reviewer: extend WITHOUT bound while the
// agent produces substantive output (converging work is never "stuck"), extend
// a busy-but-silent pane only up to maxExtends (a bare spinner proves liveness,
// not progress), else pause for investigation. No LLM call — a fast, cheap
// first-line decision whose key property is that it never kills an agent that is
// still doing work.
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
// Exported so external callers (ACS predicates, integration wiring) can use it
// without depending on the unexported newDeterministicReviewer.
func NewDeterministicReviewer(maxExtends int) StopReviewer {
	return newDeterministicReviewer(maxExtends)
}

// livenessState extracts the structured LivenessState for the reviewer decision.
// When ev.State is set (non-zero), it is returned directly; otherwise it is
// derived from the coarse Progressed+Busy booleans for backward compatibility
// with callers that do not yet populate State (legacy path).
func (ev StopEvent) livenessState() panestream.LivenessState {
	if ev.State != 0 {
		return ev.State
	}
	if ev.Progressed {
		return panestream.LivenessConverging
	}
	if ev.Busy {
		return panestream.LivenessBusyButStagnant
	}
	return panestream.LivenessIdle
}

func (r deterministicReviewer) Review(ev StopEvent) ReviewVerdict {
	// Reviewer decides from LivenessState → ReviewAction. The legacy coarse
	// Progressed+Busy booleans are converted to LivenessState by ev.livenessState()
	// when ev.State is not set, preserving backward compatibility with existing
	// test paths that do not populate State.
	//
	// Invariants preserved from the boolean era:
	//  Converging → Extend UNCONDITIONALLY (cycles 311/312: producing scout killed
	//    mid-work by the backstop — real output is never "stuck").
	//  BusyButStagnant → Extend BOUNDED by maxExtends (cycles 254/255: quiet-Opus
	//    extended-thinking paused at interval 0).
	//  Hung → fast-fail BEFORE maxExtends×interval backstop (new: the detector
	//    declares Hung after stallThreshold consecutive busy-stagnant intervals).
	//  Idle → Pause (no liveness signal at all).
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

// envInt resolves a positive integer from the launch environment via
// lookupEnv (the Deps.Env overlay, then the Deps.LookupEnv seam / os env),
// falling back to def when unset, empty, or non-positive.
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

// PaneHasSubstantiveChange split each pane string by newline, discard lines that
// match any volatile pattern, compare the stripped slices joined back as strings.
func PaneHasSubstantiveChange(prev, cur string) bool {
	cleanPrev := cleanPane(prev)
	cleanCur := cleanPane(cur)
	return cleanPrev != cleanCur
}

// cleanPane keeps only the agent CONTENT lines, dropping every chrome/affordance
// line per the single channel separator (panestream.ClassifyLine, ADR-0047).
// This is what makes a ticking spinner-stats line (claude `· Schlepping… (Ns ·
// ↑ Nk tokens)`) NOT count as progress — it is the live-turn affordance, the
// same line PaneBusy reads as busy. A genuinely-working agent still progresses
// via its real transcript (tool calls, output); a stalled one whose only delta
// is the clock no longer reads as progress (closes the ticking-clock hole).
func cleanPane(pane string) string {
	var lines []string
	for _, line := range strings.Split(pane, "\n") {
		if panestream.IsContentLine(line) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
