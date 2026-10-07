package convergence

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type splitExit int

const (
	splitNever splitExit = iota
	splitUnstages
	splitStopsWithContinuation
)

type loopRules struct {
	verifyOnly   bool
	fixerRaise   bool
	judgeRaise   bool
	freshContext bool
	raisesBar    bool
	defers       bool
	filesByRule  bool
	accepts      bool
	adjudicates  bool
	split        splitExit
	backwardEdge bool
}

var rulesByLoop = map[Loop]loopRules{
	LoopCodeReview: {verifyOnly: true, fixerRaise: true, judgeRaise: true, freshContext: true, raisesBar: true,
		defers: true, filesByRule: true, accepts: true, adjudicates: true, split: splitStopsWithContinuation},
	LoopConsoleLane: {verifyOnly: true, fixerRaise: true, judgeRaise: true, freshContext: true, raisesBar: true,
		defers: true, filesByRule: true, accepts: true, adjudicates: true, split: splitUnstages},
	LoopAuditRepair:         {judgeRaise: true, freshContext: true},
	LoopExplanationReauthor: {judgeRaise: true, freshContext: true},
	LoopCycle:               {fixerRaise: true, freshContext: true, backwardEdge: true},
	LoopInboxItem:           {fixerRaise: true, freshContext: true, split: splitUnstages},
	LoopShipRecovery:        {},
}

func (r loopRules) budget(cfg policy.ConvergenceConfig) int {
	if r.backwardEdge {
		return cfg.MaxBackwardEdges
	}
	return cfg.MaxFixRounds
}

var severityRank = map[Severity]int{SeverityInfo: 1, SeverityLow: 2, SeverityMedium: 3, SeverityHigh: 4, SeverityCritical: 5}

var defaultWeights = map[Severity]float64{SeverityCritical: 8, SeverityHigh: 4, SeverityMedium: 2, SeverityLow: 1, SeverityInfo: 0}

var (
	statuses         = []Status{StatusOpen, StatusFixed, StatusDeferred, StatusDisputed, StatusFiled}
	kinds            = []Kind{KindDefect, KindCapability}
	falsifications   = []Falsification{FalsificationSurvived, FalsificationRefuted}
	reasoningClasses = []string{"correctness", "concurrency", "architecture"}
	knownClasses     = []string{"correctness", "concurrency", "architecture", "format", "docs", "hygiene"}
)

func (s Severity) atLeast(bar Severity) bool { return severityRank[s] >= severityRank[bar] }

func hasSeverity(findings []Finding, sev Severity) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool { return f.Severity == sev })
}

func hasReasoningBlocker(findings []Finding) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool { return slices.Contains(reasoningClasses, f.Class) })
}

func idsOf(findings []Finding) []string {
	var ids []string
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	return ids
}
