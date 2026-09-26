// Package config is the composition-root loader of the routing configuration:
// the only reader of the routing env dials and the phase registry. Consumers
// receive the resolved RoutingConfig by injection.
package config

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Warning is one non-fatal resolution diagnostic: a legacy Code, the operator Message, and the triage Fields.
type Warning struct {
	Code    string
	Message string
	Fields  map[string]string
}

// Loader resolves the effective RoutingConfig through an injected file reader and Signal Center accessor.
type Loader struct {
	readFile func(name string) ([]byte, error)
	signals  func() *signalcenter.Center
}

// Option configures a Loader at construction.
type Option func(*Loader)

// New builds a Loader over the disk reader with no Center.
func New(opts ...Option) *Loader {
	l := &Loader{readFile: os.ReadFile}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithReadFile installs the reader the registry is read through.
func WithReadFile(fn func(name string) ([]byte, error)) Option {
	return func(l *Loader) { l.readFile = fn }
}

// WithSignals installs the Center accessor, read at every use; a nil accessor or a nil Center is the Null Object.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Loader) { l.signals = c }
}

// SignalsWired reports whether the loader currently reaches a Center.
func (l *Loader) SignalsWired() bool { return l.center() != nil }

// Load resolves defaults, the registry (unless EVOLVE_USE_PHASE_REGISTRY is "0"), the env dials and the
// validators, emits every warning once in order, and never fails. env is injected, never read from the process.
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

func (l *Loader) applyRegistryFile(cfg *RoutingConfig, registryPath string, ws *[]Warning) {
	from := len(*ws)
	if doc, ok := l.readRegistry(registryPath, ws); ok {
		applyRegistry(cfg, doc, ws)
	}
	stamp((*ws)[from:], "step", "registry", "source", "registry", "path", registryPath)
}

// Load is New().Load: the same value and warnings, with nothing emitted.
func Load(registryPath string, env map[string]string) (RoutingConfig, []Warning) {
	return New().Load(registryPath, env)
}

// RegistryPath is the one spelling of the phase registry's location under a project root.
func RegistryPath(projectRoot string) string {
	return filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
}

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

// defaultRollout's gate dials must equal policy's compiled defaults, because ApplyPolicyStages
// re-resolves them at the root (TestDefaults_GateDialsMatchPolicyCompiledDefaults).
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
		FatalPane:        StageEnforce,
		PhaseIO:          StageEnforce,
		RouterReplan:     StageShadow,
		MergeGate:        StageShadow,
		ParallelEvaluate: StageOff,
		ScoutDecompose:   StageOff,
	}
}

// defaultMandatory omits triage; the registry adds it. It deliberately
// diverges from phasecontract.RequiredRoles(): the router must always plan ship,
// which writes no report. It stays in config.go because acs/cycle1141 reads this file.
func defaultMandatory() []string { return []string{"scout", "build", "audit", "ship"} }

// defaultPhaseEnable is the enable floor the registry's `enabled` field overrides.
func defaultPhaseEnable() map[string]Enable {
	return map[string]Enable{
		"triage":        EnableOn,
		"tdd":           EnableOn,
		"build-planner": EnableOff,
		"swarm-plan":    EnableOff,
	}
}
