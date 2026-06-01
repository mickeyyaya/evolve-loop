package bridge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
	StdoutTail string // recent pane/stdout — evidence for an LLM reviewer (Stage 1)
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

// deterministicReviewer is the Stage-0 reviewer: extend while the agent is
// producing output (up to maxExtends), else pause for investigation. No LLM
// call — a fast, cheap first-line decision whose key property is that it never
// kills an agent that is still doing work.
type deterministicReviewer struct {
	maxExtends int
}

func newDeterministicReviewer(maxExtends int) deterministicReviewer {
	if maxExtends <= 0 {
		maxExtends = defaultArtifactMaxExtends
	}
	return deterministicReviewer{maxExtends: maxExtends}
}

func (r deterministicReviewer) Review(ev StopEvent) ReviewVerdict {
	if ev.Progressed {
		if ev.Attempt < r.maxExtends {
			return ReviewVerdict{
				Action: ReviewExtend,
				Reason: fmt.Sprintf("agent still producing output (interval %d/%d) — extend", ev.Attempt+1, r.maxExtends),
			}
		}
		return ReviewVerdict{
			Action: ReviewPause,
			Reason: fmt.Sprintf("agent still producing output but exhausted %d extensions — pause for investigation", r.maxExtends),
		}
	}
	return ReviewVerdict{
		Action: ReviewPause,
		Reason: fmt.Sprintf("no output during the last %ds interval — stalled; pause for investigation", ev.IntervalS),
	}
}

// envInt resolves a positive integer from the Deps env overlay (then os.Getenv),
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

var (
	rxBraille      = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]`)
	rxAsciiSpinner = regexp.MustCompile(`^[-/|\\]\s`)
	rxDeliberating = regexp.MustCompile(`Deliberating.*[0-9]+[ms]`)
	rxTokens       = regexp.MustCompile(`↓\s*[0-9]+(\.[0-9]+)?k\s+tokens`)
)

// PaneHasSubstantiveChange split each pane string by newline, discard lines that
// match any volatile pattern, compare the stripped slices joined back as strings.
func PaneHasSubstantiveChange(prev, cur string) bool {
	cleanPrev := cleanPane(prev)
	cleanCur := cleanPane(cur)
	return cleanPrev != cleanCur
}

func cleanPane(pane string) string {
	var lines []string
	for _, line := range strings.Split(pane, "\n") {
		if rxBraille.MatchString(line) {
			continue
		}
		if rxAsciiSpinner.MatchString(line) {
			continue
		}
		if rxDeliberating.MatchString(line) {
			continue
		}
		if rxTokens.MatchString(line) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
