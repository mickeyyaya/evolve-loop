package recovery

// stallpolicy.go — ADR-0044 C4: the Strategy that maps an observer stall
// incident to a recovery action.
//
// cycle-262 D5: the observer detects stalls but its only action is an inline
// "Enforce → SIGTERM" branch welded into the detector. Extracting the
// decision into this Strategy keeps the observer pure detection (SRP) and
// gives recovery one owner for "what do we DO about a stall": the policy is
// injected at the observer's two INCIDENT emit sites; a nil policy preserves
// the legacy branch byte-for-byte, so this seam ships behavior-neutral and
// the C3 composition slice wires a real, stage-gated implementation.

// StallAction is the typed verdict a StallPolicy returns for one stall
// incident. String values land verbatim in the INCIDENT envelope's "action"
// key — the justification trail (every recovery decision is recorded).
type StallAction string

const (
	// StallExtend: the agent is plausibly still working — keep waiting.
	// Outranks the legacy Enforce kill when a policy is injected.
	StallExtend StallAction = "extend"
	// StallKillRetry: terminate the agent's process group; the
	// orchestrator's bounded retry ladder dispatches a FRESH attempt (never
	// a reused REPL/context — phase isolation is sacred).
	StallKillRetry StallAction = "kill_retry"
	// StallEscalate: surface for the operator/advisor, take no action —
	// the posture for integrity-adjacent states, which are never
	// auto-recovered (ADR-0044 locked decision).
	StallEscalate StallAction = "escalate"
)

// StallEvent describes one observer stall incident — the evidence a policy
// decides on. String/int-only leaf envelope (mirrors PhaseOutcome).
type StallEvent struct {
	// Kind is the incident type: "stuck_no_output" (idle clock tripped) or
	// "stuck_no_progress" (babbling-but-livelocked backstop tripped).
	Kind  string
	Phase string
	// IdleS is the seconds the tripped clock observed (idle seconds for
	// stuck_no_output; no-progress seconds for stuck_no_progress).
	IdleS      int
	ThresholdS int
	// ToolCalls / ToolResults carry the progress counters (zero for
	// stuck_no_output, which trips on the idle clock alone).
	ToolCalls   int
	ToolResults int
}

// StallPolicy maps a stall incident to a recovery action plus a
// human-readable justification. Implementations must be side-effect-free —
// ACTING on the verdict (the kill, the log) is the observer's job, so a
// policy stays trivially testable.
//
// Call contract: Decide fires on EVERY observer poll tick for as long as the
// stall condition holds (legacy semantics — there is no once-at-onset
// guard), from a single goroutine per observer instance. An implementation
// shared across observers must be concurrency-safe; one that accumulates
// state (back-off counters) must expect rapid repeated calls per incident.
type StallPolicy interface {
	Decide(ev StallEvent) (StallAction, string)
}
