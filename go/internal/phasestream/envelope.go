// Package phasestream is the single normalizer for evolve-loop phase output.
package phasestream

const SchemaVersion = "2.0"

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarn     Severity = "WARN"
	SeverityIncident Severity = "INCIDENT"
)

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
	KindStall         Kind = "stall"
	KindCorrelation   Kind = "correlation"
	KindUnknown       Kind = "unknown"
)

type Source struct {
	Producer string `json:"producer"`
	CLI      string `json:"cli,omitempty"`
	Cycle    int    `json:"cycle"`
	Phase    string `json:"phase"`
	Agent    string `json:"agent"`
}

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
