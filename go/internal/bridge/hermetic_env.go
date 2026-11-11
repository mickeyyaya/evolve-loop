package bridge

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	cellEnvOrigin     = "Deps.cellEnv"
	envStageEnforce   = "enforce"
	envStageOff       = "off"
	ghConfigDirName   = "gh-config"
	cellGitConfigName = "gitconfig"
	hostGitTimeout    = 5 * time.Second
)

var baseEnvNames = []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "TERM", "TMPDIR", "TMP", "TEMP", "TZ"}

type credentialOptIn struct {
	key     string
	allowed func(Deps) bool
}

var credentialOptIns = []credentialOptIn{
	{"ANTHROPIC_BASE_URL", func(d Deps) bool { return d.AllowAnthropicBaseURL }},
	{"OPENAI_API_KEY", func(d Deps) bool { return d.AllowOpenAIAPIKey }},
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
	cellMarkers := ipcenv.CellMarkers()
	return func(name string) bool {
		if !isShellIdentifier(name) {
			return false
		}
		return slices.Contains(baseEnvNames, name) || strings.HasPrefix(name, "LC_") || slices.Contains(cellMarkers, name) || slices.Contains(passthrough, name)
	}
}

func hermeticEnv(parent, passthrough []string, cliEnv, pipeline map[string]string, creds ipcenv.CredentialSet) []string {
	set := maps.Clone(cliEnv)
	if set == nil {
		set = map[string]string{}
	}
	cellMarkers := ipcenv.CellMarkers()
	for k, v := range pipeline {
		if slices.Contains(cellMarkers, k) {
			set[k] = v
		}
	}
	for _, k := range creds.Unset {
		delete(set, k)
	}
	maps.Copy(set, creds.Set)
	allowed := allowedEnvName(passthrough)
	env := ipcenv.Keep(parent, func(name string) bool {
		_, overridden := set[name]
		return allowed(name) && !overridden && !slices.Contains(creds.Unset, name)
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
	return deps.isolationStage(deps.EnvStage, policy.Policy.IsolationEnvStage)
}

func (deps Deps) execStage() string {
	return deps.isolationStage(deps.ExecStage, policy.Policy.IsolationExecStage)
}

func (deps Deps) isolationStage(explicit string, read func(policy.Policy) string) string {
	if explicit != "" {
		return channel.ResolveStage(explicit)
	}
	cwd, _ := os.Getwd()
	layout := paths.Resolve(func(key string) string { v, _ := lookupEnv(deps, key); return v }, cwd)
	pol, err := policy.Load(filepath.Join(layout.EvolveDir, "policy.json"))
	if err != nil {
		w := deps.Stderr
		if w == nil {
			w = os.Stderr
		}
		fmt.Fprintf(w, "[bridge] WARN isolation stage: %v (the isolation stage stays at shadow)\n", err)
	}
	return channel.ResolveStage(read(pol))
}

func (deps Deps) envPassthrough(cfg *Config) []string {
	names := slices.Clone(cfg.Realization.EnvPassthrough)
	for _, o := range credentialOptIns {
		if !o.allowed(deps) {
			continue
		}
		names = append(names, o.key)
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
	deps = deps.withCellMarkers(cfg)
	seat := cfg.Decision.SeatToken != ""
	stage := envStageEnforce
	if !seat {
		stage = deps.envStage()
	}
	creds := deps.credentialSet(cfg)
	if stage == envStageOff {
		return isolatedDriverEnv(deps, cfg.Realization.Env, creds), false
	}
	parent := os.Environ()
	passthrough := deps.envPassthrough(cfg)
	deps.reportDroppedEnv(cfg, stage, droppedEnvNames(parent, passthrough))
	if stage != envStageEnforce {
		return isolatedDriverEnv(deps, cfg.Realization.Env, creds), false
	}
	cliEnv := cfg.Realization.Env
	if seat {
		cliEnv = seatEnv(cliEnv, cfg.Decision)
	}
	return hermeticEnv(parent, passthrough, cliEnv, deps.Env, creds), true
}

func (deps Deps) credentialSet(cfg *Config) ipcenv.CredentialSet {
	if cfg.credentials != nil {
		return *cfg.credentials
	}
	ghDir, gitConfig := os.DevNull, os.DevNull
	if cfg.Workspace != "" {
		ghDir = filepath.Join(cfg.Workspace, ghConfigDirName)
		if err := os.MkdirAll(ghDir, 0o700); err != nil {
			deps.warn("gh config dir %s: %v", ghDir, err)
		}
		gitConfig = deps.writeCellGitConfig(filepath.Join(cfg.Workspace, cellGitConfigName))
	}
	creds := ipcenv.CredentialIsolation(ghDir, gitConfig)
	cfg.credentials = &creds
	return creds
}

func (deps Deps) writeCellGitConfig(path string) string {
	body := ipcenv.GitIdentityConfig(deps.hostGitConfig("user.name"), deps.hostGitConfig("user.email"))
	if body == "" {
		return os.DevNull
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		deps.warn("git config %s: %v", path, err)
		return os.DevNull
	}
	return path
}

func (deps Deps) warn(format string, args ...any) {
	if deps.Stderr != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN "+format+"\n", args...)
	}
}

func (deps Deps) hostGitConfig(key string) string {
	if deps.HostGitConfig == nil {
		return readHostGitConfig(key)
	}
	return deps.HostGitConfig(key)
}

func readHostGitConfig(key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), hostGitTimeout)
	defer cancel()
	cmd := sysexec.Command(ctx, "git", "config", "--get", key)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
			Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: isolation.ViolationCode(2),
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

func (deps Deps) checkReusedSessionEnv(ctx context.Context, cfg *Config, session string) error {
	env := deps.paneEnv(cfg)
	if env == nil {
		return nil
	}
	starter, ok := deps.Tmux.(envSessionStarter)
	if !ok {
		return errNoEnvSessionStart
	}
	got, err := starter.SessionEnvFingerprint(ctx, session)
	if err != nil {
		return fmt.Errorf("read the env fingerprint of session %s: %w", session, err)
	}
	if got != envFingerprint(ipcenv.Keep(env, isShellIdentifier)) {
		return fmt.Errorf("existing session %s was not born with this cell environment (env fingerprint mismatch); launch a new session", session)
	}
	return nil
}

func (deps Deps) cellMarkers(cfg *Config) map[string]string {
	depth := ipcenv.DispatchDepth(func(key string) string { v, _ := lookupEnv(deps, key); return v })
	markers := map[string]string{ipcenv.DispatchDepthKey: strconv.Itoa(depth + 1)}
	if cfg.Agent != "" {
		markers[ipcenv.PhaseKey] = cfg.Agent
	}
	return markers
}

func (deps Deps) withCellMarkers(cfg *Config) Deps {
	env := maps.Clone(deps.Env)
	if env == nil {
		env = map[string]string{}
	}
	maps.Copy(env, deps.cellMarkers(cfg))
	deps.Env = env
	return deps
}
