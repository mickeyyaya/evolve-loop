package config

// vocabulary.go — the closed routing vocabularies every consumer reads: the
// rollout Stage ladder, the routing Mode, the model-authority axis, the
// per-phase Enable source, the deliverable kinds, the two ADR-0099 signal
// names and the sandbox modes. Pure data with String projections.

// Stage is the dynamic-routing rollout stage (shadow → advisory → enforce).
type Stage int

const (
	StageOff      Stage = iota // legacy: static state machine drives, router off
	StageShadow                // router computes + logs, static still drives
	StageAdvisory              // router drives the optional surface; spine static
	StageEnforce               // router drives, clamped by the kernel
)

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

// Mode selects the routing brain (Strategy). Default is DynamicLLM (locked decision).
type Mode int

const (
	ModeDynamicLLM   Mode = iota // LLM proposes, kernel clamps (default)
	ModeStaticPreset             // deterministic: triggers + spine only, no LLM
)

func (m Mode) String() string {
	if m == ModeStaticPreset {
		return "static"
	}
	return "llm"
}

// ModelRouting is the model-authority axis (cycle-436): who decides the LLM
// CLI + abstract model TIER for an EXISTING phase's dispatch. A THIRD axis,
// genuinely orthogonal to Stage (sequencing: which phases run) and Mode (the
// routing brain) — parsing/applying it must never read or write cfg.Stage or
// cfg.Mode, and vice versa (H3/TestC436_015).
type ModelRouting int

const (
	// ModelRoutingStatic is the zero value (safe default): every phase's CLI/
	// tier stays profile-pinned, exactly as today — an advisor {cli,tier}
	// proposal (if any) is never even generated as an authority signal.
	ModelRoutingStatic ModelRouting = iota
	// ModelRoutingAdvisory logs the advisor's proposed {cli,tier} per phase
	// (forensics/soak) but never applies it — dispatch stays profile-pinned.
	ModelRoutingAdvisory
	// ModelRoutingAuto applies the advisor's proposed {cli,tier} as a soft
	// overlay, clamped by router.ClampPlanModelRouting before dispatch.
	ModelRoutingAuto
)

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

const (
	EnableContent Enable = iota // decided by routing triggers (Specification)
	EnableOn                    // force-run
	EnableOff                   // force-skip
)

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

// DeliverableKindCode and DeliverableKindDocument are the two deliverable kinds
// a cycle can declare (ADR-0099). The vocabulary lives here because config is
// the leaf every consumer (router, core, the phases, the CLI) already imports.
const (
	DeliverableKindCode     = "code"
	DeliverableKindDocument = "document"
)

// SignalDeliverableKind and SignalGoalType are the routable field names of the
// two ADR-0099 signals — the ONE word a conditional_mandatory clause, an
// overlay `when` clause and core's dispatch projection all use for each
// (router.resolveField switches on them).
const (
	SignalDeliverableKind = "deliverable_kind"
	SignalGoalType        = "scout.goal_type"
)

// Sandbox mode string constants — exported so the bridge + tests can match
// without sprinkling magic strings.
const (
	SandboxModeAuto = "auto"
	SandboxModeOn   = "on"
	SandboxModeOff  = "off"
)
