// Package convergence decides the next rung of any repeat-until-accepted loop: change the feedback,
// then the strategy, then the scope; land the best round; never run past the budget.
// See docs/architecture/packages/internal-convergence.md.
package convergence

import "github.com/mickeyyaya/evolve-loop/go/internal/policy"

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

type Status string

const (
	StatusOpen     Status = "OPEN"
	StatusFixed    Status = "FIXED"
	StatusDeferred Status = "DEFERRED"
	StatusDisputed Status = "DISPUTED"
	StatusFiled    Status = "FILED"
)

type Kind string

const (
	KindDefect     Kind = "defect"
	KindCapability Kind = "capability"
)

type Falsification string

const (
	FalsificationSurvived Falsification = "survived"
	FalsificationRefuted  Falsification = "refuted"
)

type Loop string

const (
	LoopAuditRepair         Loop = "audit-repair"
	LoopCodeReview          Loop = "code-review"
	LoopExplanationReauthor Loop = "explanation-reauthor"
	LoopConsoleLane         Loop = "console-lane"
	LoopCycle               Loop = "cycle"
	LoopInboxItem           Loop = "inbox-item"
	LoopShipRecovery        Loop = "ship-recovery"
)

type Action string

const (
	ActionContinue         Action = "Continue"
	ActionLand             Action = "Land"
	ActionSplit            Action = "Split"
	ActionAcceptWithLimits Action = "AcceptWithLimits"
	ActionAdjudicate       Action = "Adjudicate"
	ActionStop             Action = "Stop"
)

type Finding struct {
	ID            string        `json:"id"`
	Severity      Severity      `json:"severity"`
	Kind          Kind          `json:"kind,omitempty"`
	Class         string        `json:"class,omitempty"`
	Component     string        `json:"component,omitempty"`
	Location      string        `json:"location,omitempty"`
	Status        Status        `json:"status"`
	Late          bool          `json:"late,omitempty"`
	Certificate   string        `json:"certificate,omitempty"`
	Falsification Falsification `json:"falsification,omitempty"`
}

type Hunk struct {
	File string `json:"file"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

type Judgment struct {
	Index       int       `json:"index"`
	Findings    []Finding `json:"findings"`
	FixHunks    []Hunk    `json:"fix_hunks,omitempty"`
	Reentry     bool      `json:"reentry,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Edge        string    `json:"edge,omitempty"`
}

type ComponentFacts struct {
	Separable           bool   `json:"separable,omitempty"`
	FailSafeCertificate string `json:"fail_safe_certificate,omitempty"`
}

type TierEffort struct {
	Family string `json:"family,omitempty"`
	Tier   string `json:"tier,omitempty"`
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

type ModelEffort struct {
	Model  string
	Effort string
}

type HeadroomTable map[string]map[string]ModelEffort

type MassFunc func(round int, atBar []Finding) float64

type Input struct {
	Loop       Loop                      `json:"loop"`
	Round      int                       `json:"round"`
	Rounds     []Judgment                `json:"rounds"`
	Components map[string]ComponentFacts `json:"components,omitempty"`
	Fixer      TierEffort                `json:"fixer"`
	Judge      TierEffort                `json:"judge"`
	Mass       MassFunc                  `json:"-"`
	Headroom   HeadroomTable             `json:"-"`
	Config     policy.ConvergenceConfig  `json:"-"`
}

type Decision struct {
	Rung           int        `json:"rung"`
	Action         Action     `json:"action"`
	BlockingBar    Severity   `json:"blocking_bar"`
	VerifyOnly     bool       `json:"verify_only"`
	FreshContext   bool       `json:"fresh_context"`
	FixerRaise     TierEffort `json:"fixer_raise"`
	JudgeRaise     TierEffort `json:"judge_raise"`
	Defer          []string   `json:"defer,omitempty"`
	File           []string   `json:"file,omitempty"`
	SplitComponent string     `json:"split_component,omitempty"`
	Redesign       []string   `json:"redesign,omitempty"`
	LandRound      int        `json:"land_round"`
	Reasons        []string   `json:"reasons"`
}
