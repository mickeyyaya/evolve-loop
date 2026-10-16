// Package routingtest is a configurable, composable ("Lego") test framework
// for the dynamic phase-routing kernel.
package routingtest

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type Surface int

const (
	PureKernel Surface = iota
	FullOrchestrator
)

const nonExpiredRecordedAt = "2099-01-01T00:00:00Z"

type ScenarioSpec struct {
	Name    string
	Surface Surface

	Stage         config.Stage
	Mode          config.Mode
	Mandatory     []string
	Conditional   map[string]config.CondRule
	Enable        map[string]config.Enable
	Triggers      map[string]config.RoutingBlock
	MaxInsertions int

	Signals SignalSpec
	Env     map[string]string
	Strict  bool

	Current   string
	Verdict   string
	Completed []string

	Verdicts  map[string]string
	FailedAt  []FailedRecordSpec
	LastCycle int

	Agent  AgentSpec
	Expect ExpectSpec
}

type SignalSpec struct {
	CycleSize             string
	TriageSize            string
	ACSRed                int
	ACSGreen              int
	ACSRegression         int
	FilesTouched          int
	DiffLOC               int
	SeverityMax           string
	BuildVerdict          string
	AuditVerdict          string
	AuditConf             float64
	AuditRedCount         int
	ScoutItemCount        int
	ScoutCarryover        int
	ScoutBacklog          int
	ArtifactBytes         int
	GoalType              string
	DeliverableKind       string
	TriageDeliverableKind string
}

type AgentSpec struct {
	Proposals map[string]*router.Proposal
	ErrorsOn  map[string]bool
	Plan      []router.PhasePlanEntry
	PlanError bool
}

func (a AgentSpec) active() bool {
	return len(a.Proposals) > 0 || len(a.ErrorsOn) > 0 || len(a.Plan) > 0 || a.PlanError
}

func (a AgentSpec) hasPlan() bool { return len(a.Plan) > 0 || a.PlanError }

type FailedRecordSpec struct {
	Classification string
	Verdict        string
}

type ExpectSpec struct {
	NextPhase     string
	Inserts       []string
	Skips         []string
	Clamps        []string
	Reason        string
	Justification string

	PhaseSequence    []core.Phase
	PhasesAbsent     []core.Phase
	DecisionInserts  []string
	DecisionClamps   []string
	RoutingLedgerMin int
	RetroPrefix      string
	ProposeAt        []string

	Invariants []string
}
