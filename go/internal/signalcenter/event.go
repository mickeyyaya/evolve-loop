// Package signalcenter is the ONE event stream every host-side component
// produces into and the orchestrator listens to (ADR-0101, docs/architecture/
// signal-center-design.md). It is a leaf: stdlib plus internal/log for the one
// bounded-field sanitizer. core, bridge and cmd import it; it imports none of
// them.
//
// Vocabulary: Module and Kind are closed sets; Code is MODULE_SNAKE_CASE and
// registered by its owning module; Severity is the schema-1.0 contract of
// docs/architecture/observer-severity.md (INFO/WARN/INCIDENT), adopted
// unchanged. An event that breaks a rule is never dropped — Normalize stamps
// it with a SIGNALCENTER_* code, keeps the raw values in Fields and raises it
// to at least WARN, so drift is visible in the same file the operator reads.
package signalcenter

import (
	"regexp"
	"sort"
	"strings"
)

// SchemaVersion is the wire version of Event ("signal/…" so it can never be
// confused with the phase-observer envelope's "1.0").
const SchemaVersion = "signal/1.0"

// MaxFields bounds Event.Fields; over-cap keys are dropped and counted in
// fields.truncated.
const MaxFields = 12

// MaxLineBytes bounds one rendered JSON line, so an O_APPEND single write stays
// atomic; over-cap events drop fields largest-first (fields.truncated), then
// cut Reason to fit (trailing …). Identifiers are bounded to MaxIdentRunes, so
// the bound holds for any event.
const MaxLineBytes = 4096

// MaxIdentRunes bounds Origin, Phase and RunID — identifiers, never prose.
const MaxIdentRunes = 128

// Severity is the response tier of an event — the schema-1.0 contract:
// INFO (10) log only · WARN (20) note, continue, flag for review · INCIDENT
// (30) act. There is no fourth tier; the triage identity is Kind + Code.
type Severity string

// The three tiers of the contract.
const (
	SeverityInfo     Severity = "INFO"
	SeverityWarn     Severity = "WARN"
	SeverityIncident Severity = "INCIDENT"
)

var severityLevels = map[Severity]int{SeverityInfo: 10, SeverityWarn: 20, SeverityIncident: 30}

// Level is the contract's numeric value (10/20/30); 0 for anything else.
func (s Severity) Level() int { return severityLevels[s] }

// Valid reports whether s is one of the three tiers.
func (s Severity) Valid() bool { return s.Level() != 0 }

// AtLeast reports whether s is min or a higher tier; false for invalid input.
func (s Severity) AtLeast(min Severity) bool {
	return s.Valid() && min.Valid() && s.Level() >= min.Level()
}

// Module names the component that emitted an event — a closed set. Adding one
// is a reviewed edit of this list and its test.
type Module string

// The closed module set (design §5.1).
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
)

var knownModules = map[Module]bool{
	ModuleOrchestrator: true, ModuleAdvisor: true, ModuleRunner: true, ModuleBridge: true, ModuleLiveness: true,
	ModuleShip: true, ModuleAudit: true, ModuleTriage: true, ModuleScout: true, ModuleBuild: true, ModuleTDD: true,
	ModuleGateContract: true, ModuleGateEval: true, ModuleGateRepo: true, ModuleInbox: true, ModuleConfig: true,
	ModuleLoop: true, ModuleWatchdog: true, ModuleObserver: true, ModuleDashboard: true, ModuleLedger: true, ModuleOutcome: true, ModuleFailureDiag: true, ModuleCarryover: true, ModuleFailureLearning: true, ModuleSignalCenter: true,
}

// Known reports whether m is in the closed set.
func (m Module) Known() bool { return knownModules[m] }

// Modules lists the closed set, sorted.
func Modules() []Module {
	out := make([]Module, 0, len(knownModules))
	for m := range knownModules {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// codePrefix is the MODULE_ prefix a module's codes must carry: the module
// name upper-cased with dots as underscores ("gate.contract" → "GATE_CONTRACT_").
func (m Module) codePrefix() string {
	return strings.ToUpper(strings.ReplaceAll(string(m), ".", "_")) + "_"
}

// Kind names what happened — a closed, dotted <subject>.<event> vocabulary.
type Kind string

// The closed kind set (design §5.2).
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
	KindCycleSealed            Kind = "cycle.sealed"
	KindLoopWave               Kind = "loop.wave"
	KindLoopHalt               Kind = "loop.halt"
	KindLoopEscalation         Kind = "loop.escalation"
	KindListenerPanicked       Kind = "signalcenter.listener_panicked"
	KindRegistryDrift          Kind = "signalcenter.registry_drift"
	KindSinkDropped            Kind = "signalcenter.sink_dropped"
)

var knownKinds = map[Kind]bool{
	KindPhaseDispatched: true, KindPhaseOutcome: true, KindPhaseAborted: true, KindGateRejected: true,
	KindGatePassed: true, KindGateCorrected: true, KindShipLanded: true, KindShipError: true, KindShipWarning: true, KindSystemFailure: true,
	KindQuotaPaused: true, KindBridgeWarning: true, KindBridgeTripwire: true, KindPaneLiveness: true,
	KindLedgerAppended: true, KindOutcomeWarning: true, KindFailureDiagWarning: true, KindCarryoverWarning: true, KindFailureLearningWarning: true, KindConfigWarning: true, KindRunnerWarning: true, KindInboxWarning: true, KindAuditWarning: true, KindObserverWarning: true, KindCycleSealed: true, KindLoopWave: true, KindLoopHalt: true,
	KindLoopEscalation: true, KindListenerPanicked: true, KindRegistryDrift: true, KindSinkDropped: true,
}

// terminalKinds end or change a cycle's course — the kind-first triage entry
// point (design §9).
var terminalKinds = map[Kind]bool{
	KindPhaseAborted: true, KindGateRejected: true, KindShipError: true,
	KindSystemFailure: true, KindQuotaPaused: true, KindLoopHalt: true,
}

// Known reports whether k is in the closed set.
func (k Kind) Known() bool { return knownKinds[k] }

// Terminal reports whether k ends or changes a cycle's course.
func (k Kind) Terminal() bool { return terminalKinds[k] }

// Kinds lists the closed set, sorted.
func Kinds() []Kind {
	out := make([]Kind, 0, len(knownKinds))
	for k := range knownKinds {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Code is a registered MODULE_SNAKE_CASE reason code — required on every WARN
// and INCIDENT, optional on INFO.
type Code string

var codeRE = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)

// Valid reports whether c is well-formed (not whether it is registered).
func (c Code) Valid() bool { return codeRE.MatchString(string(c)) }

// BelongsTo reports whether c carries m's prefix.
func (c Code) BelongsTo(m Module) bool {
	return c.Valid() && strings.HasPrefix(string(c), m.codePrefix())
}

var originRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// ValidOrigin reports whether s is a Func or Type.Method name as Go spells it.
func ValidOrigin(s string) bool { return originRE.MatchString(s) }

// Event is the ONE schema (design §4). Emit stamps SchemaVersion, Seq, PID and
// TS; producers fill the rest. No nested structs: one event is one greppable
// line; structured detail belongs in the artifact named by fields.path.
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
