package config

type Stage int

const (
	StageOff Stage = iota
	StageShadow
	StageAdvisory
	StageEnforce
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

type Mode int

const (
	ModeDynamicLLM Mode = iota
	ModeStaticPreset
)

func (m Mode) String() string {
	if m == ModeStaticPreset {
		return "static"
	}
	return "llm"
}

type ModelRouting int

const (
	ModelRoutingStatic ModelRouting = iota
	ModelRoutingAdvisory
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

type Enable int

const (
	EnableContent Enable = iota
	EnableOn
	EnableOff
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

const (
	DeliverableKindCode     = "code"
	DeliverableKindDocument = "document"
)

const (
	SignalDeliverableKind = "deliverable_kind"
	SignalGoalType        = "scout.goal_type"
)

const (
	SandboxModeAuto = "auto"
	SandboxModeOn   = "on"
	SandboxModeOff  = "off"
)
