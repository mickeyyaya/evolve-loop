// Package signalcenter is the one event stream that host components emit into and the orchestrator listens to.
package signalcenter

import (
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = "signal/1.0"

const MaxFields = 12

const MaxLineBytes = 4096

const MaxIdentRunes = 128

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarn     Severity = "WARN"
	SeverityIncident Severity = "INCIDENT"
)

const (
	infoLevel     = 10
	warnLevel     = 20
	incidentLevel = 30
)

var severityLevels = map[Severity]int{SeverityInfo: infoLevel, SeverityWarn: warnLevel, SeverityIncident: incidentLevel}

func (s Severity) Level() int { return severityLevels[s] }

func (s Severity) Valid() bool { return s.Level() != 0 }

func (s Severity) AtLeast(min Severity) bool {
	return s.Valid() && min.Valid() && s.Level() >= min.Level()
}

type Module string

const (
	ModuleOrchestrator    Module = "orchestrator"
	ModuleAdvisor         Module = "advisor"
	ModuleRunner          Module = "runner"
	ModuleBridge          Module = "bridge"
	ModuleLiveness        Module = "liveness"
	ModuleShip            Module = "ship"
	ModuleAudit           Module = "audit"
	ModuleTriage          Module = "triage"
	ModuleScout           Module = "scout"
	ModuleBuild           Module = "build"
	ModuleTDD             Module = "tdd"
	ModuleGateContract    Module = "gate.contract"
	ModuleGateEval        Module = "gate.eval"
	ModuleGateRepo        Module = "gate.repo"
	ModuleInbox           Module = "inbox"
	ModuleConfig          Module = "config"
	ModuleLoop            Module = "loop"
	ModuleWatchdog        Module = "watchdog"
	ModuleObserver        Module = "observer"
	ModuleDashboard       Module = "dashboard"
	ModuleLedger          Module = "ledger"
	ModuleOutcome         Module = "outcome"
	ModuleFailureDiag     Module = "failurediag"
	ModuleCarryover       Module = "carryover"
	ModuleFailureLearning Module = "failurelearning"
	ModuleSignalCenter    Module = "signalcenter"
	ModuleReview          Module = "review"
	ModuleQuality         Module = "quality"
	ModuleIsolation       Module = "isolation"
	ModuleCLI             Module = "cli"
	ModuleLadder          Module = "ladder"
	ModuleHealing         Module = "healing"
	ModuleControl         Module = "control"
	ModuleRepair          Module = "repair"
	ModuleCycle           Module = "cycle"
	ModuleTokens          Module = "tokens"
)

var knownModules = map[Module]bool{
	ModuleOrchestrator: true, ModuleAdvisor: true, ModuleRunner: true, ModuleBridge: true, ModuleLiveness: true,
	ModuleShip: true, ModuleAudit: true, ModuleTriage: true, ModuleScout: true, ModuleBuild: true, ModuleTDD: true,
	ModuleGateContract: true, ModuleGateEval: true, ModuleGateRepo: true, ModuleInbox: true, ModuleConfig: true,
	ModuleLoop: true, ModuleWatchdog: true, ModuleObserver: true, ModuleDashboard: true, ModuleLedger: true, ModuleOutcome: true, ModuleFailureDiag: true, ModuleCarryover: true, ModuleFailureLearning: true, ModuleSignalCenter: true, ModuleReview: true, ModuleQuality: true, ModuleIsolation: true, ModuleCLI: true, ModuleLadder: true, ModuleHealing: true, ModuleControl: true, ModuleRepair: true, ModuleCycle: true, ModuleTokens: true,
}

func (m Module) Known() bool { return knownModules[m] }

func Modules() []Module {
	out := make([]Module, 0, len(knownModules))
	for m := range knownModules {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (m Module) codePrefix() string {
	return strings.ToUpper(strings.ReplaceAll(string(m), ".", "_")) + "_"
}

type Kind string

const (
	KindPhaseDispatched        Kind = "phase.dispatched"
	KindPhaseOutcome           Kind = "phase.outcome"
	KindPhaseAborted           Kind = "phase.aborted"
	KindGateRejected           Kind = "gate.rejected"
	KindGateCorrected          Kind = "gate.corrected"
	KindGatePassed             Kind = "gate.passed"
	KindShipLanded             Kind = "ship.landed"
	KindShipError              Kind = "ship.error"
	KindShipWarning            Kind = "ship.warning"
	KindSystemFailure          Kind = "system.failure"
	KindQuotaPaused            Kind = "quota.paused"
	KindBridgeWarning          Kind = "bridge.warning"
	KindBridgeTripwire         Kind = "bridge.tripwire"
	KindPaneLiveness           Kind = "pane.liveness"
	KindLedgerAppended         Kind = "ledger.appended"
	KindOutcomeWarning         Kind = "outcome.warning"
	KindFailureDiagWarning     Kind = "failurediag.warning"
	KindCarryoverWarning       Kind = "carryover.warning"
	KindFailureLearningWarning Kind = "failurelearning.warning"
	KindConfigWarning          Kind = "config.warning"
	KindObserverWarning        Kind = "observer.warning"
	KindAuditWarning           Kind = "audit.warning"
	KindInboxWarning           Kind = "inbox.warning"
	KindRunnerWarning          Kind = "runner.warning"
	KindAdvisorWarning         Kind = "advisor.warning"
	KindCycleSealed            Kind = "cycle.sealed"
	KindLoopWave               Kind = "loop.wave"
	KindLoopWarning            Kind = "loop.warning"
	KindLoopHalt               Kind = "loop.halt"
	KindLoopEscalation         Kind = "loop.escalation"
	KindLoopStarted            Kind = "loop.started"
	KindLoopExit               Kind = "loop.exit"
	KindLoopLost               Kind = "loop.lost"
	KindListenerPanicked       Kind = "signalcenter.listener_panicked"
	KindRegistryDrift          Kind = "signalcenter.registry_drift"
	KindSinkDropped            Kind = "signalcenter.sink_dropped"
	KindQualityReading         Kind = "quality.reading"
	KindQualityBreach          Kind = "quality.breach"
	KindQualityWarning         Kind = "quality.warning"
	KindIsolationViolation     Kind = "isolation.violation"
	KindIsolationContext       Kind = "isolation.context"
	KindCLIQueueApplied        Kind = "cli.queue.applied"
	KindLadderDecision         Kind = "ladder.decision"
	KindSelfhealIncident       Kind = "selfheal.incident"
	KindSelfhealDecision       Kind = "selfheal.decision"
	KindControlBypass          Kind = "control.bypass"
)

const (
	KindIsolationRequest         Kind = "isolation.request"
	KindIsolationRequestDecision Kind = "isolation.request.decision"
)

var knownKinds = map[Kind]bool{
	KindPhaseDispatched: true, KindPhaseOutcome: true, KindPhaseAborted: true, KindGateRejected: true,
	KindGatePassed: true, KindGateCorrected: true, KindShipLanded: true, KindShipError: true, KindShipWarning: true, KindSystemFailure: true,
	KindQuotaPaused: true, KindBridgeWarning: true, KindBridgeTripwire: true, KindPaneLiveness: true,
	KindLedgerAppended: true, KindOutcomeWarning: true, KindFailureDiagWarning: true, KindCarryoverWarning: true, KindFailureLearningWarning: true, KindConfigWarning: true, KindAdvisorWarning: true, KindRunnerWarning: true, KindInboxWarning: true, KindAuditWarning: true, KindObserverWarning: true, KindCycleSealed: true, KindLoopWave: true, KindLoopHalt: true, KindLoopWarning: true,
	KindLoopEscalation: true, KindLoopStarted: true, KindLoopExit: true, KindLoopLost: true, KindListenerPanicked: true, KindRegistryDrift: true, KindSinkDropped: true,
	KindQualityReading: true, KindQualityBreach: true, KindQualityWarning: true, KindIsolationViolation: true, KindIsolationContext: true, KindIsolationRequest: true, KindIsolationRequestDecision: true, KindCLIQueueApplied: true, KindLadderDecision: true, KindSelfhealIncident: true, KindSelfhealDecision: true,
	KindControlBypass: true,
}

var terminalKinds = map[Kind]bool{
	KindPhaseAborted: true, KindGateRejected: true, KindShipError: true,
	KindSystemFailure: true, KindQuotaPaused: true, KindLoopHalt: true,
}

func (k Kind) Known() bool { return knownKinds[k] }

func (k Kind) Terminal() bool { return terminalKinds[k] }

func Kinds() []Kind {
	out := make([]Kind, 0, len(knownKinds))
	for k := range knownKinds {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

type Code string

const CodeLoopLost Code = "LOOP_LOST"

var codeRE = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)

func (c Code) Valid() bool { return codeRE.MatchString(string(c)) }

func (c Code) BelongsTo(m Module) bool {
	return c.Valid() && strings.HasPrefix(string(c), m.codePrefix())
}

var originRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

func ValidOrigin(s string) bool { return originRE.MatchString(s) }

const StreamFileName = "signals.ndjson"

type Event struct {
	SchemaVersion string            `json:"schema_version"`
	Seq           uint64            `json:"seq"`
	PID           int               `json:"pid"`
	TS            string            `json:"ts"`
	Cycle         int               `json:"cycle,omitempty"`
	RunID         string            `json:"run_id,omitempty"`
	Phase         string            `json:"phase,omitempty"`
	Attempt       int               `json:"attempt,omitempty"`
	Module        Module            `json:"module"`
	Origin        string            `json:"origin"`
	Kind          Kind              `json:"kind"`
	Code          Code              `json:"code,omitempty"`
	Severity      Severity          `json:"severity"`
	Reason        string            `json:"reason"`
	Fields        map[string]string `json:"fields,omitempty"`
}
