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

// driverExists checks driver presence, not an adapter script's executable
// bit — the run path's RunOptions.AdapterExists default.
func driverExists(cli string) bool {
	_, ok := gobridge.LookupDriver(gobridge.DriverFor(cli))
	return ok
}

// execAdapterDeps builds the gobridge.Deps for the subagent composition
// root.
func execAdapterDeps(env map[string]string) gobridge.Deps {
	home := env["HOME"]
	if home == "" {
		home = os.Getenv("HOME")
	}
	configRoot := filepath.Join(home, ".claude")
	pol := projectPolicy(env)
	bridgeCfg := pol.BridgeConfig()
	for _, warning := range bridgeCfg.TierEffortWarnings() {
		fmt.Fprintf(os.Stderr, "[bridge] WARN policy %s\n", warning)
	}
	recoveryStage, fatalPaneStage := pol.BridgeRecoveryStages()
	return gobridge.Deps{
		Env:            env,
		TokenResolver:  tokenusage.DefaultResolver(configRoot),
		RecoveryStage:  recoveryStage,
		FatalPaneStage: fatalPaneStage,
		TierEffort:     bridgeCfg.TierEfforts(),
	}
}

// projectPolicy loads the dispatched project's policy.json, fail-open like
// adapters/bridge.NewDefault: no EVOLVE_PROJECT_ROOT or an unreadable file
// yields the zero Policy, whose accessors resolve the compiled defaults.
func projectPolicy(env map[string]string) policy.Policy {
	root := env["EVOLVE_PROJECT_ROOT"]
	if root == "" {
		return policy.Policy{}
	}
	pol, _ := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	return pol
}

// execAdapterDepsWith is execAdapterDeps carrying the root's Signal Center,
// so the engine's own bridge.warning, bridge.tripwire and pane.liveness
// producers report into it instead of the nil Null Object.
func execAdapterDepsWith(env map[string]string, signals *signalcenter.Center) gobridge.Deps {
	d := execAdapterDeps(env)
	d.Signals = signals
	return d
}

// defaultExecAdapter is the Center-less projection of execAdapter — the
// ValidateProfile default. The unused adapterPath parameter is kept for the
// injectable ExecAdapter seam; the default resolves RESOLVED_CLI from env
// onto a registered driver instead of reading a .sh file.
func defaultExecAdapter(ctx context.Context, _ string, env map[string]string) (int, error) {
	return execAdapter(ctx, env, nil)
}

// bridgeAdapter is the production Adapter: the gobridge engine over the
// rendered env, carrying the root's Signal Center.
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
	// The bridge's launch guard fails fast on an empty prompt, but
	// ValidateProfile sets PROMPT_FILE="" under VALIDATE_ONLY=1 and
	// validate-only never reads the prompt, so a placeholder satisfies the
	// guard without changing behavior. A real run always carries a
	// non-empty PROMPT_FILE, so this never masks a missing prompt.
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
		// Infra error from the bridge itself (guard failure, prompt write, …):
		// resp is the zero value (ExitCode 0), and "exit 0 + error" would
		// mislead VerifyArtifact, so this returns -1 on error instead.
		return -1, err
	}
	return resp.ExitCode, nil
}

// atoiOrZero parses a base-10 integer, returning 0 on any error or a
// negative value: the bridge treats Cycle<=0 as no cycle.
func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
