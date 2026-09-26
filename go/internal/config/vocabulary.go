package config

// Stage is a rollout stage on the off → shadow → advisory → enforce ladder.
type Stage int

// The rollout stages, in ladder order.
const (
	StageOff      Stage = iota // legacy: static state machine drives, router off
	StageShadow                // router computes + logs, static still drives
	StageAdvisory              // router drives the optional surface; spine static
	StageEnforce               // router drives, clamped by the kernel
)

// String returns the stage word; StageOff renders as "0".
func (s Stage) String() string {
	switch s {
	case StageShadow:
		return "shadow"
	case StageAdvisory:
		return "advisory"
	case StageEnforce:
		return "enforce"
	default:
		return "0"
	}
}

// Mode selects the routing brain (Strategy).
type Mode int

// The routing brains.
const (
	ModeDynamicLLM   Mode = iota // LLM proposes, kernel clamps (default)
	ModeStaticPreset             // deterministic: triggers + spine only, no LLM
)

// String returns "static" or "llm".
func (m Mode) String() string {
	if m == ModeStaticPreset {
		return "static"
	}
	return "llm"
}

// ModelRouting decides who picks the CLI and model tier for a phase's dispatch. It is orthogonal
// to Stage and Mode: parsing or applying it never reads or writes either.
type ModelRouting int

const (
	// ModelRoutingStatic is the zero value: every phase's CLI and tier stay profile-pinned.
	ModelRoutingStatic ModelRouting = iota
	// ModelRoutingAdvisory logs the advisor's proposed {cli,tier} per phase but never applies it.
	ModelRoutingAdvisory
	// ModelRoutingAuto applies the advisor's proposal as a soft overlay, clamped by router.ClampPlanModelRouting.
	ModelRoutingAuto
)

// String returns "static", "advisory" or "auto".
func (m ModelRouting) String() string {
	switch m {
	case ModelRoutingAdvisory:
		return "advisory"
	case ModelRoutingAuto:
		return "auto"
	default:
		return "static"
	}
}

// Enable is the per-phase enablement decision source.
type Enable int

// The enablement sources.
const (
	EnableContent Enable = iota // decided by routing triggers (Specification)
	EnableOn                    // force-run
	EnableOff                   // force-skip
)

// String returns "on", "off" or "content".
func (e Enable) String() string {
	switch e {
	case EnableOn:
		return "on"
	case EnableOff:
		return "off"
	default:
		return "content"
	}
}

// DeliverableKindCode and DeliverableKindDocument are the deliverable kinds a cycle can declare.
const (
	DeliverableKindCode     = "code"
	DeliverableKindDocument = "document"
)

// SignalDeliverableKind and SignalGoalType are the routable field names of the deliverable-kind and goal-type signals.
const (
	SignalDeliverableKind = "deliverable_kind"
	SignalGoalType        = "scout.goal_type"
)

// The EVOLVE_SANDBOX modes.
const (
	SandboxModeAuto = "auto"
	SandboxModeOn   = "on"
	SandboxModeOff  = "off"
)
