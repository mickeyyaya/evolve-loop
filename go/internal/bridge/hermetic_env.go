package bridge

import (
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeIsolationI2Violation signalcenter.Code = "ISOLATION_I2_VIOLATION"

	cellEnvOrigin   = "Deps.cellEnv"
	envStageEnforce = "enforce"
	envStageOff     = "off"
)

var baseEnvNames = []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "TERM", "TMPDIR", "TMP", "TEMP", "TZ"}

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
		if !isShellIdentifier(name) || name == ipcenv.DispatchIDKey {
			return false
		}
		return slices.Contains(baseEnvNames, name) || strings.HasPrefix(name, "LC_") || strings.HasPrefix(name, "EVOLVE_") || slices.Contains(passthrough, name)
	}
}

func hermeticEnv(parent, passthrough []string, cliEnv, pipeline map[string]string) []string {
	set := maps.Clone(cliEnv)
	if set == nil {
		set = map[string]string{}
	}
	maps.Copy(set, pipeline)
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
	stage := channel.ResolveStage(deps.EnvStage)
	if stage == envStageOff {
		return driverEnv(deps, cfg.Realization.Env), false
	}
	parent := os.Environ()
	deps.reportDroppedEnv(cfg, stage, droppedEnvNames(parent, cfg.Realization.EnvPassthrough))
	if stage != envStageEnforce {
		return driverEnv(deps, cfg.Realization.Env), false
	}
	return hermeticEnv(parent, cfg.Realization.EnvPassthrough, cfg.Realization.Env, deps.Env), true
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
