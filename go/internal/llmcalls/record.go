// Package llmcalls owns the durable per-model-attempt ledger and its derived
// performance index. It deliberately contains no provider or orchestration
// logic: producers supply finalized dispatch facts, and readers consume the
// same additive record schema.
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
	// Filename is the per-workspace model-attempt ledger.
	Filename = "llm-calls.ndjson"
	// SchemaVersion identifies records with verified lifecycle and dispatch
	// provenance fields. Legacy records have no version.
	SchemaVersion = 2
	// TimingBridgeDispatch means duration covers the shared Bridge launch
	// pipeline through driver completion, excluding token enrichment and the
	// ledger append itself.
	TimingBridgeDispatch = "bridge_dispatch"
	// TimingLegacyUnknown labels records that do not declare a timing scope.
	TimingLegacyUnknown = "legacy_unspecified"
	// SourceUnknown labels records that do not identify token evidence.
	SourceUnknown = "unknown"

	// UnknownModel is the display identity when no dispatched model was verified.
	UnknownModel = "(unknown)"
	// DispatchUnknown means the launch boundary could not establish the selector source.
	DispatchUnknown = "unknown"
	// DispatchArgv means a model selector was observed in process arguments.
	DispatchArgv = "argv"
	// DispatchREPL means a model selector was sent through an interactive command.
	DispatchREPL = "repl"
	// DispatchPositional means a model selector occupied the provider's positional argument.
	DispatchPositional = "positional"
	// DispatchCLIDefault means no explicit selector replaced the provider default.
	DispatchCLIDefault = "cli_default"
	// DispatchResumed means an existing session resumed without a new selector.
	DispatchResumed = "resumed_session"
	// DispatchNotStarted means no provider dispatch was observed.
	DispatchNotStarted = "not_started"
	// DispatchLegacyUnverified labels historical rows without dispatch provenance.
	DispatchLegacyUnverified = "legacy_unverified"
)

// UsageStatus distinguishes a measured zero from missing or partial token
// evidence. String values are part of the durable record contract.
type UsageStatus string

const (
	// UsageMeasured means complete token evidence was collected.
	UsageMeasured UsageStatus = "measured"
	// UsagePartial means token evidence is incomplete or not fully comparable.
	UsagePartial UsageStatus = "partial"
	// UsageUnavailable means no usable token evidence was collected.
	UsageUnavailable UsageStatus = "unavailable"
	// UsageResolverError means the token collector failed.
	UsageResolverError UsageStatus = "resolver_error"
)

const (
	// OutcomeSuccess means the attempt completed with exit code zero.
	OutcomeSuccess = "success"
	// OutcomeFailure means the attempt completed with a non-zero exit code.
	OutcomeFailure = "failure"
	// OutcomeUnknown means no terminal exit code was recorded.
	OutcomeUnknown = "unknown"
)

// Record is one completed orchestration-owned model launch attempt. Historical
// fields remain present so existing readers can decode new lines unchanged;
// the additive fields make measurement coverage and actual dispatch identity
// explicit. Pointer outcome fields preserve "missing" on legacy input.
type Record struct {
	SchemaVersion int    `json:"schema_version,omitempty"`
	CallID        string `json:"call_id,omitempty"`
	TS            string `json:"ts"`
	StartedAt     string `json:"started_at,omitempty"`
	EndedAt       string `json:"ended_at,omitempty"`
	TimingScope   string `json:"timing_scope,omitempty"`

	Agent string `json:"agent"`
	Phase string `json:"phase"`
	CLI   string `json:"cli"`
	// Model is the legacy requested-selector field. New consumers use the
	// explicit requested/dispatched pair below.
	Model           string `json:"model"`
	RequestedModel  string `json:"requested_model,omitempty"`
	DispatchedModel string `json:"dispatched_model,omitempty"`
	DispatchSource  string `json:"dispatch_source,omitempty"`
	Attempt         int    `json:"attempt"`

	Tokens      cyclestate.TokenUsage `json:"tokens"`
	Source      string                `json:"source"`
	UsageStatus UsageStatus           `json:"usage_status,omitempty"`
	DurationMS  *int64                `json:"duration_ms,omitempty"`
	ExitCode    *int                  `json:"exit_code,omitempty"`
	Tripwire    bool                  `json:"tripwire"`
	FillPct     float64               `json:"fill_pct"`
	CauseCode   string                `json:"cause_code,omitempty"`

	// First-output timing is intentionally optional. The current bridge has no
	// source that means the same thing across headless and REPL drivers.
	FirstOutputMS     *int64 `json:"first_output_ms,omitempty"`
	FirstOutputSource string `json:"first_output_source,omitempty"`
}

// ModelIdentity returns only a verified dispatched selector. A legacy model
// value is retained as requested context and never promoted to concrete model
// identity.
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

// NewCallID creates a process-unique, opaque identifier without introducing a
// third-party UUID dependency. Random bytes prevent collisions across
// processes; timestamp, pid, and a monotonic sequence preserve uniqueness if
// the operating-system random source ever fails.
func NewCallID(now time.Time) string {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err == nil {
		return fmt.Sprintf("%x-%s", uint64(now.UnixNano()), hex.EncodeToString(entropy[:]))
	}
	return fmt.Sprintf("%x-%x-%x", uint64(now.UnixNano()), uint64(os.Getpid()), callSequence.Add(1))
}
