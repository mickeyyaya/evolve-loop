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

const (
	KindNudge                = "nudge"
	KindAutoRespond          = "auto_respond"
	KindSalvage              = "salvage"
	KindKernelAnswer         = "kernel_answer"
	KindCorrectionRedispatch = "correction_redispatch"
	KindSubmitVerify         = "submit_verify"
)

const (
	ResultArtifactAppeared    = "artifact_appeared"
	ResultPromptCleared       = "prompt_cleared"
	ResultNoEffect            = "no_effect"
	ResultSuppressedLingering = "suppressed_lingering"
	ResultRunEnded            = "run_ended"
	ResultAccepted            = "accepted"
	ResultRejectedAgain       = "rejected_again"
	ResultDispatchFailed      = "dispatch_failed"
	ResultQuotaDeferred       = "quota_deferred"
	ResultNonCanonicalVerdict = "non_canonical_verdict"

	ResultSubmitVerified       = "submit_verified"
	ResultSubmittedAfterResend = "submitted_after_resend"
	ResultSubmitWedged         = "submit_wedged"
	ResultNotVerified          = "not_verified"
)

type Event struct {
	Kind       string `json:"kind"`
	Phase      string `json:"phase"`
	Cycle      int    `json:"cycle"`
	Trigger    string `json:"trigger"`
	Rung       string `json:"rung,omitempty"`
	DecisionID string `json:"decision_id,omitempty"`
	Payload    string `json:"payload,omitempty"`
	RuleID     string `json:"rule_id,omitempty"`
}

type Outcome struct {
	Event
	Result    string  `json:"result"`
	LatencyMS int64   `json:"latency_ms"`
	CostUSD   float64 `json:"cost_usd"`
}

const (
	payloadMaxChars = 200
	payloadMaxLines = 3
)

type Recorder struct {
	workspace string
	mu        sync.Mutex
	outcomes  []Outcome
}

func NewRecorder(workspace string) *Recorder {
	return &Recorder{workspace: workspace}
}

func (r *Recorder) Record(out Outcome) {
	if r == nil {
		return
	}
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

func neutralize(s string) string {
	if s == "" {
		return ""
	}
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
