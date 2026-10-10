package subagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func driverExists(cli string) bool {
	_, ok := gobridge.LookupDriver(gobridge.DriverFor(cli))
	return ok
}

func execAdapterDeps(env map[string]string) gobridge.Deps {
	configRoot := tokenusage.ClaudeConfigRoot(env)
	pol := projectPolicy(env)
	recoveryStage, fatalPaneStage := pol.BridgeRecoveryStages()
	return gobridge.Deps{
		Env:            env,
		TokenResolver:  tokenusage.DefaultResolver(configRoot),
		RecoveryStage:  recoveryStage,
		FatalPaneStage: fatalPaneStage,
		Efforts:        pol.Efforts(),
	}
}

func projectPolicy(env map[string]string) policy.Policy {
	root := env["EVOLVE_PROJECT_ROOT"]
	if root == "" {
		return policy.Policy{}
	}
	pol, _ := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	return pol
}

func execAdapterDepsWith(env map[string]string, signals *signalcenter.Center) gobridge.Deps {
	d := execAdapterDeps(env)
	d.Signals = signals
	return d
}

func defaultExecAdapter(ctx context.Context, _ string, env map[string]string) (int, error) {
	return execAdapter(ctx, env, nil)
}

type bridgeAdapter struct {
	signals *signalcenter.Center
}

func (b bridgeAdapter) Exec(ctx context.Context, env subagentrun.AdapterEnv) (int, error) {
	return execAdapter(ctx, env.Map(), b.signals)
}

func execAdapter(ctx context.Context, env map[string]string, signals *signalcenter.Center) (int, error) {
	cli := gobridge.DriverFor(env["RESOLVED_CLI"])
	prompt := ""
	if pf := env["PROMPT_FILE"]; pf != "" {
		b, err := os.ReadFile(pf)
		if err != nil {
			return -1, fmt.Errorf("subagent: read prompt file %q: %w", pf, err)
		}
		prompt = string(b)
	}
	if prompt == "" {
		prompt = "(validate-only: no prompt)"
	}
	eng := gobridge.NewEngine(execAdapterDepsWith(env, signals))
	resp, err := eng.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      env["PROFILE_PATH"],
		Model:        env["RESOLVED_MODEL"],
		Prompt:       prompt,
		Workspace:    env["WORKSPACE_PATH"],
		Worktree:     env["WORKTREE_PATH"],
		ArtifactPath: env["ARTIFACT_PATH"],
		StdoutLog:    env["STDOUT_LOG"],
		StderrLog:    env["STDERR_LOG"],
		Cycle:        atoiOrZero(env["CYCLE"]),
		Env:          env,
	})
	if err != nil {
		return -1, err
	}
	return resp.ExitCode, nil
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
