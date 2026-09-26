// Package interaction records every injection the loop fires into a phase agent with its
// resolved outcome, and decides the corrective interactions that repair a phase.
// See docs/architecture/packages/internal-interaction.md.
package interaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
)

// Event kinds: the closed vocabulary of injections the loop can fire.
const (
	KindNudge                = "nudge"
	KindAutoRespond          = "auto_respond"
	KindSalvage              = "salvage"
	KindKernelAnswer         = "kernel_answer"
	KindCorrectionRedispatch = "correction_redispatch"
	// KindSubmitVerify is one driver submission checked for delivery, recorded on every outcome.
	KindSubmitVerify = "submit_verify"
)

// Outcome results. Resolution uses only external evidence (artifact presence,
// pane pattern state, re-dispatch verdicts), never the agent's self-assessment.
const (
	// ResultArtifactAppeared: the contracted artifact appeared within the bounded wait window.
	ResultArtifactAppeared = "artifact_appeared"
	// ResultPromptCleared: the pattern that triggered the interaction no longer matched on the next capture.
	ResultPromptCleared = "prompt_cleared"
	// ResultNoEffect: the evidence shows the interaction did not help.
	ResultNoEffect = "no_effect"
	// ResultSuppressedLingering: a fire-once prompt's dismissed text still matched, so a re-fire was suppressed.
	ResultSuppressedLingering = "suppressed_lingering"
	// ResultRunEnded: the run ended before the next capture could resolve the interaction.
	ResultRunEnded = "run_ended"
	// ResultAccepted / ResultRejectedAgain: a correction re-dispatch's deliverable passed / failed its gate.
	ResultAccepted      = "accepted"
	ResultRejectedAgain = "rejected_again"
	// ResultDispatchFailed / ResultNonCanonicalVerdict: the correction re-dispatch errored / returned an unevaluable verdict.
	ResultDispatchFailed = "dispatch_failed"
	// ResultQuotaDeferred: the re-dispatch met a quota wall and the cycle deferred instead of failing.
	ResultQuotaDeferred       = "quota_deferred"
	ResultNonCanonicalVerdict = "non_canonical_verdict"

	// ResultSubmitVerified: the input line was already clear. It is not ResultPromptCleared,
	// which counts injections that worked; every dispatch's no-op would swamp that count.
	ResultSubmitVerified = "submit_verified"
	// ResultSubmittedAfterResend: the input line was still parked and a bounded re-send cleared it.
	ResultSubmittedAfterResend = "submitted_after_resend"
	// ResultSubmitWedged: the re-send budget ran out with the input line still parked.
	ResultSubmitWedged = "submit_wedged"
	// ResultNotVerified: delivery could not be checked, which must never read as clean.
	ResultNotVerified = "not_verified"
)

// Event is one injection fired at a phase agent.
type Event struct {
	// Kind is one of the Kind* constants.
	Kind string `json:"kind"`
	// Phase is the canonical phase name, or the driver name when the launch carries no agent.
	Phase string `json:"phase"`
	// Cycle is 0 outside a cycle.
	Cycle int `json:"cycle"`
	// Trigger names what provoked the interaction: "idle_no_artifact",
	// "idle_unrewritten_deliverable" (present but not rewritten since dispatch),
	// "contract_reject", "unknown_prompt".
	Trigger string `json:"trigger"`
	// Rung is the correction-ladder rung that produced the event, or "" outside the ladder.
	Rung string `json:"rung,omitempty"`
	// DecisionID correlates every rung of one correction decision.
	DecisionID string `json:"decision_id,omitempty"`
	// Payload is a digest of what was injected, neutralized and capped at 200 runes before write.
	Payload string `json:"payload,omitempty"`
	// RuleID is the auto-respond or promoted rule that fired, when one did.
	RuleID string `json:"rule_id,omitempty"`
}

// Outcome is an Event plus its resolved result; the ledger holds one Outcome per line.
type Outcome struct {
	Event
	// Result is one of the Result* constants; the vocabulary is open to later producers.
	Result string `json:"result"`
	// LatencyMS runs from injection to resolution.
	LatencyMS int64 `json:"latency_ms"`
	// CostUSD is advisor spend attributed to the interaction; 0 for deterministic rungs.
	CostUSD float64 `json:"cost_usd"`
}

const (
	payloadMaxChars = 200
	payloadMaxLines = 3
)

// Recorder is the single recording chokepoint, safe for concurrent use; a nil Recorder records nothing.
type Recorder struct {
	workspace string
	mu        sync.Mutex
	outcomes  []Outcome
}

// NewRecorder returns a Recorder appending to <workspace>/<phase>-interactions.ndjson.
// An empty workspace keeps records in memory only, so no file lands in the cwd.
func NewRecorder(workspace string) *Recorder {
	return &Recorder{workspace: workspace}
}

// Record neutralizes pane-derived fields, keeps the outcome in memory, and appends it to the
// per-phase ledger. File errors are swallowed: telemetry must never abort a phase.
func (r *Recorder) Record(out Outcome) {
	if r == nil {
		return
	}
	// Never trust the producer; Result is neutralized too, as defense in depth.
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

// neutralize ANSI-strips, marker-defangs and length-caps pane-derived text;
// closed-vocabulary strings pass through unchanged.
func neutralize(s string) string {
	if s == "" {
		return ""
	}
	// Digest caps each line, not the joined result; Payload's contract is a total cap.
	d := panetrust.Digest(s, payloadMaxLines, payloadMaxChars)
	if r := []rune(d); len(r) > payloadMaxChars {
		d = string(r[:payloadMaxChars])
	}
	return d
}

func ledgerPath(workspace, phase string) string {
	if phase == "" {
		phase = "unknown"
	}
	return filepath.Join(workspace, phase+"-interactions.ndjson")
}

// appendLedgerLine relies on O_APPEND only because the bridge and orchestrator never write one
// ledger at the same time; a concurrent cross-process producer needs a file lock.
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
