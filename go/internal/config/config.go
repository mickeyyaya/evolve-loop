// Package config is the single composition-root loader for evolve-loop's
// dynamic-routing configuration: the ONLY place that reads the routing env
// vars and the central phase registry. Every downstream consumer receives the
// resolved RoutingConfig by injection and never calls os.Getenv.
//
// The Loader (ADR-0103 unit 08) resolves it in two steps. Load: the compiled
// defaults → docs/architecture/phase-registry.json → the contained env
// overrides → the spine and inert-enable validators (precedence env >
// registry > default). ApplyPolicyStages: the composition root's projection
// of .evolve/policy.json's gate, recovery, router and parallel-evaluate dials
// over that value. Both report every non-fatal diagnostic as a config.warning
// WARN under module config through the injected Signal Center; the
// package-level Load facade is the Center-less loader the per-phase callers
// keep (same value, same Warning slice, nothing emitted).
//
// Leaf package by design: stdlib plus internal/signalcenter and
// internal/paths (importgraph_test.go pins the allowlist). It must never
// import internal/core or internal/policy — both import it. Phase identifiers
// cross the boundary as plain strings; core converts to/from core.Phase at the
// call site. Design: docs/architecture/decomposition/08-config.md.
package config

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Warning is one non-fatal resolution diagnostic. Code is the closed set
// "unknown-value" | "weak-spine" | "spine-order" | "inert-phase-enable" |
// "registry-unreadable" | "registry-malformed" (signalCodes projects each onto
// its registered CONFIG_* code); Message is the operator sentence (the event's
// reason); Fields is what a triage reads — step (registry | env | spine |
// inert | policy), source, key, value, default, path, err, and the
// validators' own missing / audit_pos / ship_pos / phase / stage.
type Warning struct {
	Code    string
	Message string
	Fields  map[string]string
}

// Loader resolves the effective RoutingConfig. Its collaborators are explicit
// at construction: the file reader (os.ReadFile by default; tests inject bytes
// or errors — no disk) and the Signal Center through an accessor read at
// every use (a nil accessor, or one returning nil, is the Null Object).
type Loader struct {
	readFile func(name string) ([]byte, error)
	signals  func() *signalcenter.Center
}

// Option configures a Loader at construction (functional options).
type Option func(*Loader)

// New builds a Loader over the disk reader with no Center; options replace
// either collaborator.
func New(opts ...Option) *Loader {
	l := &Loader{readFile: os.ReadFile}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithReadFile installs the filesystem port the registry is read through.
func WithReadFile(fn func(name string) ([]byte, error)) Option {
	return func(l *Loader) { l.readFile = fn }
}

// WithSignals installs the accessor of the Signal Center the loader reports
// through — read at every use. A nil accessor, or one returning nil, is the
// Null Object; SignalsWired proves the production root wired one.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Loader) { l.signals = c }
}

// SignalsWired reports whether the loader currently reaches a Center.
func (l *Loader) SignalsWired() bool { return l.center() != nil }

// Load resolves the effective RoutingConfig: defaults → [EVOLVE_USE_PHASE_REGISTRY
// != "0": the registry file] → the env overrides → validateSpine →
// validateInertEnables. env is injected (never read from the process) so the
// loader stays testable and is the sole contained env site. After each step
// the warnings that step appended are stamped with the step's name (and the
// registry path or env var it read); then every Warning is emitted once, in
// order, as a config.warning WARN with Origin Loader.Load. Never fails.
func (l *Loader) Load(registryPath string, env map[string]string) (RoutingConfig, []Warning) {
	var ws []Warning
	cfg := defaults()
	if env["EVOLVE_USE_PHASE_REGISTRY"] != "0" {
		l.applyRegistryFile(&cfg, registryPath, &ws)
	}
	from := len(ws)
	applyEnv(&cfg, env, &ws)
	stamp(ws[from:], "step", "env", "source", "env")
	from = len(ws)
	validateSpine(cfg, &ws)
	stamp(ws[from:], "step", "spine")
	from = len(ws)
	validateInertEnables(cfg, &ws)
	stamp(ws[from:], "step", "inert")
	l.emit("Loader.Load", ws)
	return cfg, ws
}

// applyRegistryFile is the registry step: read through the injected port,
// overlay when it parsed, and stamp the step, source and path on whatever the
// read or the overlay warned.
func (l *Loader) applyRegistryFile(cfg *RoutingConfig, registryPath string, ws *[]Warning) {
	from := len(*ws)
	if doc, ok := l.readRegistry(registryPath, ws); ok {
		applyRegistry(cfg, doc, ws)
	}
	stamp((*ws)[from:], "step", "registry", "source", "registry", "path", registryPath)
}

// Load is the Strangler Fig facade over a Center-less Loader: the same value
// and Warning slice New().Load returns, nothing emitted. It is the loader the
// per-phase callers (router.PolicyForProject re-runs it every dispatch),
// `evolve phase verify`, `evolve solution` and the kernel test fixture keep;
// the cycle/loop composition root wires a Loader with the root's Center.
func Load(registryPath string, env map[string]string) (RoutingConfig, []Warning) {
	return New().Load(registryPath, env)
}

// RegistryPath is the ONE spelling of the phase registry's location under a
// project root: <root>/docs/architecture/phase-registry.json.
func RegistryPath(projectRoot string) string {
	return filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
}

// defaults is the compiled baseline every resolution starts from. Dynamic
// routing is DEFAULT-ON (Component #7): the advisor drives phase selection
// every cycle, with the integrity floor (ClampPlanToFloor + SpineSatisfiedUpTo)
// — not a flag — protecting the ship guarantee. EVOLVE_DYNAMIC_ROUTING still
// overrides (e.g. =off for the legacy static path). Flipped from StageOff
// after the advisory mode soaked since cycle-108. CompactPrompts defaults ON
// (strips ~23 KB/cycle of on-demand reference tails); the concurrency and
// re-plan bounds are the soak sweet spots their field docs record.
func defaults() RoutingConfig {
	return RoutingConfig{
		Stage:                       StageAdvisory,
		Mode:                        ModeDynamicLLM,
		RolloutStages:               defaultRollout(),
		CompactPrompts:              true,
		Mandatory:                   defaultMandatory(),
		Conditional:                 map[string]CondRule{"tdd": DefaultTddRule()},
		MaxInsertions:               4,
		ParallelEvaluateConcurrency: 3,
		ScoutDecomposeConcurrency:   4,
		RePlanMaxDepth:              1,
		PhaseEnable:                 defaultPhaseEnable(),
		Triggers:                    map[string]RoutingBlock{},
	}
}

// defaultRollout is the compiled rollout-dial baseline. SpineFloor=StageEnforce
// (the R8.5 flip, landed 2026-07-16 as its OWN dial): the artifact-backed
// spine floor ABORTS on a clean-absence handoff gap instead of
// WARN-and-proceed. Evidence: after the scout/audit digest fallbacks
// (router/digest.go), a 536-run-dir replay showed 0 would-block transitions
// on every cycle shape since ~cycle-480 (the only misses were pre-convention
// dirs from the 361-479 era). Degraded reads stay fail-open (the cleanAbsence
// guard in cyclerun_select.go), an enforce block records to failure-learning,
// and policy.json `recovery.spine_floor: "shadow"` is the no-recompile escape
// hatch. Deliberately DECOUPLED from PhaseRecovery: that dial is overloaded
// (ADR-0045 I6 folded the bidirectional channel into it, and the
// failure-adviser promotion path also keys on it), so arming the floor via
// PhaseRecovery would have armed two unsoaked subsystems. PhaseRecovery itself
// stays shadow. The gate dials (EvalGate, ContractGate, TriageCapGate,
// TopNGate, PhaseRecovery, SpineFloor, RouterReplan, ParallelEvaluate) are
// re-resolved from policy's compiled accessors by ApplyPolicyStages at the
// composition root — a two-source belief TestDefaults_GateDialsMatchPolicyCompiledDefaults
// pins equal (F6 in the unit doc unifies them).
func defaultRollout() RolloutStages {
	return RolloutStages{
		CommitEvidence:   StageOff,
		SandboxMode:      SandboxModeAuto,
		EvalGate:         StageEnforce,
		ContractGate:     StageEnforce,
		TriageCapGate:    StageEnforce,
		TopNGate:         StageEnforce,
		PhaseRecovery:    StageShadow,
		SpineFloor:       StageEnforce,
		PhaseIO:          StageEnforce,
		RouterReplan:     StageShadow,
		MergeGate:        StageShadow,
		ParallelEvaluate: StageOff,
		ScoutDecompose:   StageOff,
	}
}

// defaultMandatory is the compiled mandatory spine. NOTE: this built-in
// baseline intentionally omits triage; the real registry
// (docs/architecture/phase-registry.json) adds it via applyRegistry (cycles
// 263/264: the advisory router skipped the scope-clamp). Tests constructing
// RoutingConfig directly keep this 4-phase baseline.
//
// Mandatory DELIBERATELY diverges from phasecontract.RequiredRoles() —
// audited cycle-1141, kept separate on purpose. The two lists answer
// different questions and must not be merged:
//   - phasecontract.RequiredRoles()/RequiredArtifacts() = "what must a
//     COMPLETED cycle have PRODUCED" (report-bearing spine phases;
//     consumed by cyclehealth/redteamcheck/ledgerverify).
//   - this list = "what must the ROUTER always PLAN" — a superset that
//     includes "ship", a phase that writes no report and therefore
//     cannot appear in an artifact-derived registry vocabulary.
//
// Deriving this from the registry would silently drop "ship" from every
// plan; hardcoding the registry's half is the drift risk. The invariant
// that actually binds them — Mandatory ⊇ registry-required phases, plus
// "ship", and nothing the registry does not know — is asserted by the
// cycle-1141 ACS predicate rather than by shared code. That predicate reads
// THIS file by path (acs/cycle1141), so the defaults family stays in
// config.go.
func defaultMandatory() []string { return []string{"scout", "build", "audit", "ship"} }

// defaultPhaseEnable is the legacy phase-enable floor, so PhasePolicy
// reproduces pre-routing behavior even when the registry file is absent (e.g.
// tests): triage and tdd run by default; build-planner is opt-in (shadow).
// These are the floor the registry `enabled` field overrides.
func defaultPhaseEnable() map[string]Enable {
	return map[string]Enable{
		"triage":        EnableOn,
		"tdd":           EnableOn,
		"build-planner": EnableOff,
		"swarm-plan":    EnableOff,
	}
}
