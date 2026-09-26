package config

// DeliverableKindSpec is the registry-declared shape of one deliverable kind; paths are relative to Root.
type DeliverableKindSpec struct {
	Root               string              `json:"root"`
	MinOptions         int                 `json:"min_options"`
	RequiredFiles      []string            `json:"required_files"`
	RequiredSections   map[string][]string `json:"required_sections"`
	ForbidPlaceholders []string            `json:"forbid_placeholders"`
	EvidenceFile       string              `json:"evidence_file"`
}

// DocumentSpec returns the document deliverable contract when the registry declares one.
func (c RoutingConfig) DocumentSpec() (DeliverableKindSpec, bool) {
	spec, ok := c.DeliverableKinds[DeliverableKindDocument]
	return spec, ok
}

// Condition is one declarative routing-trigger clause; package router evaluates it.
type Condition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// RoutingBlock is the per-phase declarative trigger set.
type RoutingBlock struct {
	InsertWhen []Condition `json:"insert_when"`
	SkipWhen   []Condition `json:"skip_when"`
	// RubricHint lines render into the advisor's decision rubric. A rubric-only block is walk-inert.
	RubricHint []string `json:"rubric_hint,omitempty"`
}

// RoutingConfig is the resolved routing configuration the composition root injects everywhere.
type RoutingConfig struct {
	Stage Stage
	Mode  Mode
	// ModelRouting is registry-only (config.model_routing); the zero value is static.
	ModelRouting ModelRouting
	RolloutStages
	Mandatory        []string            // ordered mandatory phase names
	Conditional      map[string]CondRule // phase -> conditional-mandatory rule
	DeliverableKinds map[string]DeliverableKindSpec
	MaxInsertions    int
	// ParallelEvaluateConcurrency bounds the evaluate phases the ParallelEvaluate dispatcher runs at once.
	ParallelEvaluateConcurrency int
	// ScoutDecomposeConcurrency bounds the parallel scout-scan workers.
	ScoutDecomposeConcurrency int
	PhaseEnable               map[string]Enable       // phase -> enablement source
	Triggers                  map[string]RoutingBlock // phase -> declarative triggers
	// Order is the registry phase sequence the router walks; empty falls back to the router's canonical order.
	Order []string
	// SpineOrder is the mandatory-default successor sequence; empty falls back to the kernel's literal.
	SpineOrder []string
	// LegalSuccessors maps a phase name, sentinels included, to its legal successors; empty falls
	// back to the kernel's literal graph. ValidateSafetyInvariants gates edits to it.
	LegalSuccessors map[string][]string
	// AuditFailRoutesTo is the audit-FAIL route ("retrospective" or "memo") from policy's failure_floor.
	AuditFailRoutesTo string
	// GoalRecipes maps a goal type to its ordered recipe tokens; registry-only.
	GoalRecipes map[string][]string
	// RoutingJudge enables LLM route-quality scoring off the build path; a bool because it never moves behavior.
	RoutingJudge bool
	// ReconDigest injects the deterministic pre-plan recon digest into the initial plan prompt.
	ReconDigest bool
	// RePlanMaxDepth caps the post-scout re-plans per cycle before escalating to the debugger.
	RePlanMaxDepth int
	// CompactPrompts strips the on-demand reference sections from disk-loaded agent docs.
	CompactPrompts bool
}
