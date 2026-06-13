package bridge

import (
	"fmt"
	"regexp"
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

func (r deterministicReviewer) Review(ev StopEvent) ReviewVerdict {
	// Two liveness signals, two budgets — conflating them killed working agents:
	//
	//  Progressed = the agent emitted substantive new output this interval. Real
	//  output is proof of work that is CONVERGING, so it is never "stuck": a
	//  scout reading a large failure backlog produces output for 30+ min and
	//  must extend UNCONDITIONALLY. Capping progress at maxExtends killed
	//  cycle-311/312's producing scout mid-work with a bogus "artifact timeout".
	//
	//  Busy (no Progressed) = the per-CLI busy affordance is up but the pane has
	//  no content delta. Load-bearing for quiet extended-thinking models (Opus):
	//  the only delta is the "Deliberating Ns"/token-counter lines that
	//  PaneHasSubstantiveChange strips as volatile, so Progressed reads false
	//  while the agent is demonstrably alive. Pausing such an agent at interval 0
	//  recorded a PASS audit report as FAIL and halted the batch (cycles
	//  254/255). But a bare spinner only proves the process is up, not that it is
	//  converging — so this signal stays BOUNDED by maxExtends to surface a
	//  genuinely-hung busy-spinner agent. A busy pane buys extensions, not
	//  immortality.
	if ev.Progressed {
		return ReviewVerdict{
			Action: ReviewExtend,
			Reason: fmt.Sprintf("agent still producing output (interval %d) — extend; real output is never stuck", ev.Attempt+1),
		}
	}
	if ev.Busy {
		if ev.Attempt < r.maxExtends {
			return ReviewVerdict{
				Action: ReviewExtend,
				Reason: fmt.Sprintf("agent busy mid-turn (no pane delta — likely extended thinking) (interval %d/%d) — extend", ev.Attempt+1, r.maxExtends),
			}
		}
		return ReviewVerdict{
			Action: ReviewPause,
			Reason: fmt.Sprintf("agent busy but produced no output — exhausted %d extensions, pause for investigation", r.maxExtends),
		}
	}
	return ReviewVerdict{
		Action: ReviewPause,
		Reason: fmt.Sprintf("no output during the last %ds interval — stalled; pause for investigation", ev.IntervalS),
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

// rxTokens parses a ↓ response-token count for extractTokenCount (a VALUE
// extractor, distinct from panestream.ClassifyLine's chrome classification).
var rxTokens = regexp.MustCompile(`↓\s*([0-9]+(?:\.[0-9]+)?)k\s+tokens`)

func extractTokenCount(pane string) int {
	peak := 0
	for _, match := range rxTokens.FindAllStringSubmatch(pane, -1) {
		if len(match) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		tokens := int(v*1000 + 0.5)
		if tokens > peak {
			peak = tokens
		}
	}
	return peak
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
