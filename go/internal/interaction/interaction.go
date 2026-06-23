// Package interaction owns interaction telemetry (ADR-0045 I1): every
// injection the loop fires into a phase agent — nudge, auto-respond
// keystrokes, salvage, kernel answer, correction re-dispatch — records a
// typed Event with its deterministically-resolved Outcome through one
// chokepoint, mirroring internal/recovery's C1 discipline ("an interaction
// that isn't recorded with its outcome doesn't exist").
//
// The validation batch (cycles 263–269) proved the cost of not having this:
// `nudgeSent=true` and nothing measures whether any nudge ever worked, so
// there is no tuning signal and no learning. I1 ships FIRST in the ADR-0045
// build order because every later component's effectiveness claim (salvage
// saved a re-dispatch; rule X fired N times, 0 false) must be measurable from
// day one — the soak for I2–I4 is this telemetry.
//
// Stage coupling: recording is side-effect-free observation, so the recorder
// runs at EVERY EVOLVE_PHASE_RECOVERY stage including `off` (the
// FatalPaneDetector precedent — classification always-on, only ACTING is
// staged). Only corrective actions gate on shadow/enforce.
//
// Threat S10 (stored-injection): pane-derived strings persist in the ledger
// and may later be read by an LLM (retro, advisor). Record therefore passes
// Payload and Result through panetrust neutralization BEFORE write — the
// ledger is safe-by-construction to feed back to any LLM.
//
// Leaf constraints (mirrors internal/recovery): importable by both core and
// bridge, so it imports neither — stdlib + the panetrust leaf only.
package interaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/mickeyyaya/evolveloop/go/internal/panetrust"
)

// Event kinds: the closed vocabulary of injections the loop can fire.
const (
	KindNudge                = "nudge"
	KindAutoRespond          = "auto_respond"
	KindSalvage              = "salvage"
	KindKernelAnswer         = "kernel_answer"
	KindCorrectionRedispatch = "correction_redispatch"
)

// Outcome results: the deterministic resolution vocabulary. Resolution never
// trusts agent self-assessment — only artifact presence, pane pattern state,
// and re-dispatch verdicts (external, unfakeable evidence).
const (
	// ResultArtifactAppeared: the contracted artifact appeared within the
	// bounded wait window after the interaction.
	ResultArtifactAppeared = "artifact_appeared"
	// ResultPromptCleared: the prompt pattern that triggered the interaction
	// no longer matched on the next capture.
	ResultPromptCleared = "prompt_cleared"
	// ResultNoEffect: the evidence shows the interaction did not help (the
	// artifact never appeared / the same pattern fired again). Recorded
	// honestly — a fired-and-forgotten interaction is the defect this
	// package exists to kill.
	ResultNoEffect = "no_effect"
	// ResultSuppressedLingering: a fire-once prompt's dismissed text still
	// matches in scrollback; the responder suppressed a re-fire. Genuinely
	// indistinguishable from an unanswered dialog at this layer, so it gets
	// its own honest bucket instead of a guess.
	ResultSuppressedLingering = "suppressed_lingering"
	// ResultRunEnded: the run concluded before the next capture could
	// resolve the interaction.
	ResultRunEnded = "run_ended"
	// ResultAccepted / ResultRejectedAgain: a correction re-dispatch's
	// deliverable passed / failed the review gate that triggered it.
	ResultAccepted      = "accepted"
	ResultRejectedAgain = "rejected_again"
	// ResultDispatchFailed / ResultNonCanonicalVerdict: the correction
	// re-dispatch itself errored / returned an unevaluable verdict.
	ResultDispatchFailed      = "dispatch_failed"
	ResultNonCanonicalVerdict = "non_canonical_verdict"
)

// Event is one injection fired at a phase agent (ADR-0045 I1).
type Event struct {
	// Kind is one of the Kind* constants.
	Kind string `json:"kind"`
	// Phase is the canonical phase name ("build"); falls back to the driver
	// name when the launch carries no agent.
	Phase string `json:"phase"`
	// Cycle is the loop cycle the interaction belongs to (0 outside a cycle).
	Cycle int `json:"cycle"`
	// Trigger names what provoked the interaction ("idle_no_artifact",
	// "contract_reject", "unknown_prompt", ...).
	Trigger string `json:"trigger"`
	// Rung is the correction-ladder rung that produced this event
	// ("salvage"|"live_fix"|"redispatch"|"") — load-bearing for the
	// rung-distribution acceptance metric (ADR-0045 §10(d)).
	Rung string `json:"rung,omitempty"`
	// DecisionID correlates all rungs of ONE correction decision, so a
	// salvage outcome can be linked to the re-dispatch it averted.
	DecisionID string `json:"decision_id,omitempty"`
	// Payload is a digest of what was injected (≤200 chars, neutralized
	// before write — never raw pane-derived text at full length).
	Payload string `json:"payload,omitempty"`
	// RuleID is the auto-respond rule (or promoted-rule id) that fired,
	// when applicable.
	RuleID string `json:"rule_id,omitempty"`
}

// Outcome is the Event plus its deterministically-resolved result. One ledger
// line per interaction: the Outcome embeds the Event it resolves.
type Outcome struct {
	Event
	// Result is one of the Result* constants (open vocabulary: later slices
	// add their own).
	Result string `json:"result"`
	// LatencyMS is injection→resolution latency.
	LatencyMS int64 `json:"latency_ms"`
	// CostUSD is advisor-consult spend attributed to this interaction
	// (0 for deterministic rungs).
	CostUSD float64 `json:"cost_usd"`
}

// Payload digest caps: 200 chars over at most 3 tail lines — compact,
// actionable feedback (the ACI principle), never raw injected text at length.
const (
	payloadMaxChars = 200
	payloadMaxLines = 3
)

// Recorder is the single recording chokepoint. Nil-receiver-safe (a nil
// recorder records nothing) so producers need no nil guards — the recovery
// detector idiom. Safe for concurrent use.
type Recorder struct {
	workspace string
	mu        sync.Mutex
	outcomes  []Outcome
}

// NewRecorder returns a Recorder appending to <workspace>/<phase>-interactions.ndjson.
// An empty workspace keeps in-memory records only (the C1 cwd-leak lesson:
// never invent a file location).
func NewRecorder(workspace string) *Recorder {
	return &Recorder{workspace: workspace}
}

// Record resolves one interaction: neutralizes pane-derived fields (S10),
// keeps the outcome in memory, and best-effort appends one ndjson line to the
// per-phase ledger. Telemetry must never abort a phase, so file errors are
// swallowed by design — the in-memory record still exists.
func (r *Recorder) Record(out Outcome) {
	if r == nil {
		return
	}
	// Neutralize at the chokepoint, never trust the producer: Payload (and,
	// defense-in-depth, Result) may carry pane-derived text.
	out.Payload = neutralize(out.Payload)
	out.Result = neutralize(out.Result)
	r.mu.Lock()
	r.outcomes = append(r.outcomes, out)
	r.mu.Unlock()
	if r.workspace == "" {
		return
	}
	appendLedgerLine(r.workspace, out)
}

// Outcomes returns a copy of every outcome recorded by this Recorder.
func (r *Recorder) Outcomes() []Outcome {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Outcome, len(r.outcomes))
	copy(out, r.outcomes)
	return out
}

// neutralize runs a string through the panetrust digest under the payload
// caps. Closed-vocabulary strings pass through unchanged; pane-derived text
// comes out ANSI-stripped, marker-defanged, and length-capped.
func neutralize(s string) string {
	if s == "" {
		return ""
	}
	// Digest caps each LINE at payloadMaxChars runes; the joined multi-line
	// result can exceed that. The Event.Payload contract is a TOTAL cap
	// (≤200 chars), so the joined string is capped again — both cuts are
	// intentional, not redundant.
	d := panetrust.Digest(s, payloadMaxLines, payloadMaxChars)
	if r := []rune(d); len(r) > payloadMaxChars {
		d = string(r[:payloadMaxChars])
	}
	return d
}

// ledgerPath names the per-phase ledger. An empty phase falls back to
// "unknown" rather than inventing an unnameable file.
func ledgerPath(workspace, phase string) string {
	if phase == "" {
		phase = "unknown"
	}
	return filepath.Join(workspace, phase+"-interactions.ndjson")
}

// appendLedgerLine best-effort appends one outcome. O_APPEND keeps the
// per-phase ledger safe under TODAY's producer timeline, which is temporally
// disjoint by construction: the bridge subprocess flushes its outcomes (defer
// in runTmuxREPL) before runner.Run returns, and the orchestrator records
// corrections only after that. POSIX guarantees append atomicity only up to
// PIPE_BUF, so if a later slice adds a producer that writes WHILE the phase
// runs (an I2 salvage rung, the I3 broker), this invariant must be revisited
// (file lock or single-writer funnel) — do not silently rely on it.
func appendLedgerLine(workspace string, out Outcome) {
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	f, err := os.OpenFile(ledgerPath(workspace, out.Phase), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(append(b, '\n'))
}
