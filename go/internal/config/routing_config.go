package config

type DeliverableKindSpec struct {
	Root               string              `json:"root"`
	MinOptions         int                 `json:"min_options"`
	RequiredFiles      []string            `json:"required_files"`
	RequiredSections   map[string][]string `json:"required_sections"`
	ForbidPlaceholders []string            `json:"forbid_placeholders"`
	EvidenceFile       string              `json:"evidence_file"`
}

func (c RoutingConfig) DocumentSpec() (DeliverableKindSpec, bool) {
	spec, ok := c.DeliverableKinds[DeliverableKindDocument]
	return spec, ok
}

type Condition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

type RoutingBlock struct {
	InsertWhen []Condition `json:"insert_when"`
	SkipWhen   []Condition `json:"skip_when"`
	RubricHint []string    `json:"rubric_hint,omitempty"`
}

func (b RoutingBlock) TriggerIsTheWholeRule() bool {
	return len(b.InsertWhen) > 0 && len(b.RubricHint) == 0
}

type RoutingConfig struct {
	Stage        Stage
	Mode         Mode
	ModelRouting ModelRouting
	RolloutStages
	Mandatory                   []string
	Conditional                 map[string]CondRule
	DeliverableKinds            map[string]DeliverableKindSpec
	MaxInsertions               int
	ParallelEvaluateConcurrency int
	ScoutDecomposeConcurrency   int
	PhaseEnable                 map[string]Enable
	Triggers                    map[string]RoutingBlock
	Order                       []string
	SpineOrder                  []string
	LegalSuccessors             map[string][]string
	AuditFailRoutesTo           string
	GoalRecipes                 map[string][]string
	RoutingJudge                bool
	ReconDigest                 bool
	RePlanMaxDepth              int
	CompactPrompts              bool
}
