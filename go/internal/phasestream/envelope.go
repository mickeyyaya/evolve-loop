// Package phasestream is the single normalizer for evolve-loop phase
// output. It classifies raw CLI stdout/stderr (stream-json from
// claude-p/codex/agy, or plaintext scrollback from claude-tmux) into a
// unified envelope stream — one <agent>-events.ndjson per phase that
// every downstream consumer (cyclecost, the observer rules, and
// cycleclassify) reads instead of re-parsing raw logs.
//
// Design: ADR-0020. The raw .log is never touched; this package only
// produces the clean event stream.
package phasestream

// SchemaVersion is the envelope schema. Extends phaseobserver's 1.0
// envelope with seq (gap-free per-file ordering) and kind.
const SchemaVersion = "2.0"

// Severity is the system-wide vocabulary, shared with the observer.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarn     Severity = "WARN"
	SeverityIncident Severity = "INCIDENT"
)

// Kind enumerates the normalized event taxonomy (ADR-0020).
type Kind string

const (
	KindResult        Kind = "result"
	KindProgress      Kind = "progress"
	KindToolUse       Kind = "tool_use"
	KindToolResult    Kind = "tool_result"
	KindInteraction   Kind = "interaction"
	KindAssistantText Kind = "assistant_text"
	KindThinking      Kind = "thinking"
	KindRateLimit     Kind = "rate_limit"
	KindInfraFailure  Kind = "infra_failure"
	KindSystemHook    Kind = "system_hook"
	KindError         Kind = "error"
	KindUnknown       Kind = "unknown"
)

// Source identifies the producer + phase context. Mirrors
// phaseobserver's envelope source block.
type Source struct {
	Producer string `json:"producer"` // normalizer | observer
	CLI      string `json:"cli,omitempty"`
	Cycle    int    `json:"cycle"`
	Phase    string `json:"phase"`
	Agent    string `json:"agent"`
}

// Envelope is one normalized event. Data is an untyped map to match the
// existing phaseobserver wire shape and stay forward-compatible; typed
// accessors live in consumer packages.
type Envelope struct {
	SchemaVersion string         `json:"schema_version"`
	Seq           int64          `json:"seq"`
	TS            string         `json:"ts"`
	TraceID       string         `json:"trace_id"`
	Source        Source         `json:"source"`
	Kind          Kind           `json:"kind"`
	Severity      Severity       `json:"severity"`
	Data          map[string]any `json:"data,omitempty"`
}
