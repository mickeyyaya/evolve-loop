package recovery

// StallAction is a StallPolicy verdict; its value lands verbatim in the INCIDENT envelope's "action" key.
type StallAction string

const (
	// StallExtend keeps waiting; it outranks the legacy enforce kill.
	StallExtend StallAction = "extend"
	// StallKillRetry kills the agent's process group; the retry ladder dispatches a fresh attempt.
	StallKillRetry StallAction = "kill_retry"
	// StallEscalate surfaces the stall to the operator and takes no action.
	StallEscalate StallAction = "escalate"
)

// StallEvent is the observer-detected stall a StallPolicy decides on.
type StallEvent struct {
	// Kind is "stuck_no_output", "stuck_no_progress" or "process_dead".
	Kind  string
	Phase string
	// IdleS is the seconds the tripped clock observed: idle time, or time without progress.
	IdleS       int
	ThresholdS  int
	ToolCalls   int
	ToolResults int
}

// StallPolicy maps a stall to an action and a justification, without side effects.
type StallPolicy interface {
	// Decide runs on every observer tick while the stall holds, from one goroutine
	// per observer; an implementation shared across observers must be concurrency-safe.
	Decide(ev StallEvent) (StallAction, string)
}
