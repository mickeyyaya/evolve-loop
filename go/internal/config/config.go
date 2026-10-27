// Package config is the composition-root loader of the routing configuration:
// the only reader of the routing env dials and the phase registry. Consumers
// receive the resolved RoutingConfig by injection.
package config

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Warning struct {
	Code    string
	Message string
	Fields  map[string]string
}

type Loader struct {
	readFile func(name string) ([]byte, error)
	signals  func() *signalcenter.Center
}

type Option func(*Loader)

func New(opts ...Option) *Loader {
	l := &Loader{readFile: os.ReadFile}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func WithReadFile(fn func(name string) ([]byte, error)) Option {
	return func(l *Loader) { l.readFile = fn }
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Loader) { l.signals = c }
}

func (l *Loader) SignalsWired() bool { return l.center() != nil }

func (l *Loader) Load(registryPath string) (RoutingConfig, []Warning) {
	var ws []Warning
	cfg := defaults()
	l.applyRegistryFile(&cfg, registryPath, &ws)
	from := len(ws)
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

func Load(registryPath string) (RoutingConfig, []Warning) {
	return New().Load(registryPath)
}

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

func defaultRollout() RolloutStages {
	return RolloutStages{
		CommitEvidence:   StageOff,
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

func defaultMandatory() []string { return []string{"scout", "build", "audit", "ship"} }

func defaultPhaseEnable() map[string]Enable {
	return map[string]Enable{
		"triage":        EnableOn,
		"tdd":           EnableOn,
		"build-planner": EnableOff,
		"swarm-plan":    EnableOff,
	}
}
