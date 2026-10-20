// Package llmcalls owns the durable per-model-attempt ledger and its derived
// performance index.
package llmcalls

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	Filename             = "llm-calls.ndjson"
	SchemaVersion        = 2
	TimingBridgeDispatch = "bridge_dispatch"
	TimingLegacyUnknown  = "legacy_unspecified"
	SourceUnknown        = "unknown"

	UnknownModel             = "(unknown)"
	DispatchUnknown          = "unknown"
	DispatchArgv             = "argv"
	DispatchREPL             = "repl"
	DispatchPositional       = "positional"
	DispatchCLIDefault       = "cli_default"
	DispatchResumed          = "resumed_session"
	DispatchNotStarted       = "not_started"
	DispatchLegacyUnverified = "legacy_unverified"
)

type UsageStatus string

const (
	UsageMeasured      UsageStatus = "measured"
	UsagePartial       UsageStatus = "partial"
	UsageUnavailable   UsageStatus = "unavailable"
	UsageResolverError UsageStatus = "resolver_error"
)

const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
	OutcomeUnknown = "unknown"
)

type Record struct {
	SchemaVersion int    `json:"schema_version,omitempty"`
	CallID        string `json:"call_id,omitempty"`
	TS            string `json:"ts"`
	StartedAt     string `json:"started_at,omitempty"`
	EndedAt       string `json:"ended_at,omitempty"`
	TimingScope   string `json:"timing_scope,omitempty"`

	Agent           string `json:"agent"`
	Phase           string `json:"phase"`
	CLI             string `json:"cli"`
	Model           string `json:"model"`
	RequestedModel  string `json:"requested_model,omitempty"`
	DispatchedModel string `json:"dispatched_model,omitempty"`
	DispatchSource  string `json:"dispatch_source,omitempty"`
	Attempt         int    `json:"attempt"`

	Tokens             cyclestate.TokenUsage `json:"tokens"`
	Source             string                `json:"source"`
	UsageStatus        UsageStatus           `json:"usage_status,omitempty"`
	DurationMS         *int64                `json:"duration_ms,omitempty"`
	ExitCode           *int                  `json:"exit_code,omitempty"`
	Tripwire           bool                  `json:"tripwire"`
	FillPct            float64               `json:"fill_pct"`
	CauseCode          string                `json:"cause_code,omitempty"`
	FreshSessionRetry  bool                  `json:"fresh_session_retry,omitempty"`
	SessionRecoverable bool                  `json:"session_recoverable,omitempty"`

	FirstOutputMS     *int64 `json:"first_output_ms,omitempty"`
	FirstOutputSource string `json:"first_output_source,omitempty"`

	Rung         string `json:"rung,omitempty"`
	FailureClass string `json:"failure_class,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	Effort       string `json:"effort,omitempty"`
	ServedModel  string `json:"served_model,omitempty"`
	Context      string `json:"context,omitempty"`
	LadderStage  string `json:"ladder_stage,omitempty"`
	LadderWould  string `json:"ladder_would,omitempty"`
}

func (r Record) ModelIdentity() (model, source string) {
	if r.DispatchedModel != "" {
		return r.DispatchedModel, defaultString(r.DispatchSource, DispatchUnknown)
	}
	if r.SchemaVersion == 0 && r.CallID == "" {
		return UnknownModel, DispatchLegacyUnverified
	}
	return UnknownModel, defaultString(r.DispatchSource, DispatchUnknown)
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

var callSequence atomic.Uint64

func NewCallID(now time.Time) string {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err == nil {
		return fmt.Sprintf("%x-%s", uint64(now.UnixNano()), hex.EncodeToString(entropy[:]))
	}
	return fmt.Sprintf("%x-%x-%x", uint64(now.UnixNano()), uint64(os.Getpid()), callSequence.Add(1))
}
