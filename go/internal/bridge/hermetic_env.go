package bridge

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeIsolationI2Violation signalcenter.Code = "ISOLATION_I2_VIOLATION"

	cellEnvOrigin   = "Deps.cellEnv"
	envStageEnforce = "enforce"
	envStageOff     = "off"
)

var baseEnvNames = []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "TERM", "TMPDIR", "TMP", "TEMP", "TZ"}

var pipelineEnvMarkers = []string{
	"EVOLVE_PROJECT_ROOT", ipcenv.WorktreeRootKey, ipcenv.CycleStateFileKey, ipcenv.TmuxSocketKey,
	ipcenv.FleetKey, ipcenv.FleetScopeKey, ipcenv.FleetWidthKey, ipcenv.FleetReachHolderKey,
}

var envOptIns = map[string]string{
	"BRIDGE_ALLOW_ANTHROPIC_BASE_URL": "ANTHROPIC_BASE_URL",
	"BRIDGE_ALLOW_OPENAI_API_KEY":     "OPENAI_API_KEY",
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeIsolationI2Violation, "a variable of the launcher is outside the environment allowlist of an LLM child (I-2); in shadow the child still got it, in enforce the child did not; fields invariant, stage, action, cell, family, target (the name, never the value), rule, source")
}

type EnvDropLog struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (l *EnvDropLog) firstSeen(family string, names []string) []string {
	if l == nil {
		return names
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	return slices.DeleteFunc(slices.Clone(names), func(name string) bool {
		key := family + "/" + name
		if l.seen[key] {
			return true
		}
		l.seen[key] = true
		return false
	})
}

func allowedEnvName(passthrough []string) func(string) bool {
	return func(name string) bool {
		if !isShellIdentifier(name) {
			return false
		}
		return slices.Contains(baseEnvNames, name) || strings.HasPrefix(name, "LC_") || slices.Contains(pipelineEnvMarkers, name) || slices.Contains(passthrough, name)
	}
}

func hermeticEnv(parent, passthrough []string, cliEnv, pipeline map[string]string) []string {
	set := maps.Clone(cliEnv)
	if set == nil {
		set = map[string]string{}
	}
	for k, v := range pipeline {
		if slices.Contains(pipelineEnvMarkers, k) {
			set[k] = v
		}
	}
	allowed := allowedEnvName(passthrough)
	env := ipcenv.Keep(parent, func(name string) bool {
		_, overridden := set[name]
		return allowed(name) && !overridden
	})
	for _, k := range slices.Sorted(maps.Keys(set)) {
		if isShellIdentifier(k) {
			env = append(env, k+"="+set[k])
		}
	}
	return env
}

func droppedEnvNames(parent, passthrough []string) []string {
	allowed := allowedEnvName(passthrough)
	return sortedEnvNames(ipcenv.Keep(parent, func(name string) bool { return name != ipcenv.DispatchIDKey && !allowed(name) }))
}

func (deps Deps) envStage() string {
	if deps.EnvStage != "" {
		return channel.ResolveStage(deps.EnvStage)
	}
	cwd, _ := os.Getwd()
	layout := paths.Resolve(func(key string) string { v, _ := lookupEnv(deps, key); return v }, cwd)
	pol, err := policy.Load(filepath.Join(layout.EvolveDir, "policy.json"))
	if err != nil {
		w := deps.Stderr
		if w == nil {
			w = os.Stderr
		}
		fmt.Fprintf(w, "[bridge] WARN isolation stage: %v (the child environment stays at shadow)\n", err)
	}
	return channel.ResolveStage(pol.IsolationEnvStage())
}

func (deps Deps) envPassthrough(cfg *Config) []string {
	names := slices.Clone(cfg.Realization.EnvPassthrough)
	for _, optIn := range slices.Sorted(maps.Keys(envOptIns)) {
		if v, _ := lookupEnv(deps, optIn); v == "1" {
			names = append(names, envOptIns[optIn])
		}
	}
	return names
}

func sortedEnvNames(env []string) []string {
	names := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		names = append(names, name)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (deps Deps) cellEnv(cfg *Config) ([]string, bool) {
	stage := deps.envStage()
	if stage == envStageOff {
		return driverEnv(deps, cfg.Realization.Env), false
	}
	parent := os.Environ()
	passthrough := deps.envPassthrough(cfg)
	deps.reportDroppedEnv(cfg, stage, droppedEnvNames(parent, passthrough))
	if stage != envStageEnforce {
		return driverEnv(deps, cfg.Realization.Env), false
	}
	return hermeticEnv(parent, passthrough, cfg.Realization.Env, deps.Env), true
}

func (deps Deps) reportDroppedEnv(cfg *Config, stage string, names []string) {
	action := "logged"
	if stage == envStageEnforce {
		action = "refused"
	}
	id := configIdentity(cfg)
	for _, name := range deps.EnvDrops.firstSeen(cfg.CLI, names) {
		deps.Signals.Emit(signalcenter.Event{
			Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleIsolation, Origin: cellEnvOrigin,
			Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: CodeIsolationI2Violation,
			Reason: "launcher variable " + name + " is outside the environment allowlist of " + cfg.CLI,
			Fields: map[string]string{
				"invariant": "I2", "stage": stage, "action": action, "cell": cfg.Agent, "family": cfg.CLI,
				"target": name, "rule": "env.allowlist", "source": "launcher",
			},
		})
	}
}

func (deps Deps) paneEnv(cfg *Config) []string {
	env, hermetic := deps.cellEnv(cfg)
	if !hermetic {
		return nil
	}
	return env
}
