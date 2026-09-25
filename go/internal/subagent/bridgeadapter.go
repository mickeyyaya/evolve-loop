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

// bridgeadapter.go is the host's bridge touch (ADR-0103 unit 16): the
// gobridge-backed defaults of the run path — the driver-presence check the
// dispatcher's AdapterExists port is bound to and the production Adapter that
// dispatches through the in-process engine carrying the root's Signal Center
// — plus the Center-less projection ValidateProfile's func-shaped seam
// defaults to. engine.go/autorespond.go are untouched; the one field this
// file sets on gobridge.Deps is Signals.

// driverExists reports whether cli has a registered bridge driver: the
// pre-flight "is this dispatchable?" check is driver presence, not an adapter
// script's executable bit, and it takes the cli itself (bridge.DriverFor +
// LookupDriver) — the run path's RunOptions.AdapterExists default.
func driverExists(cli string) bool {
	_, ok := gobridge.LookupDriver(gobridge.DriverFor(cli))
	return ok
}

// defaultExecAdapter dispatches the subagent through the in-process Go bridge
// instead of shelling `bash <cli>.sh`. The bridge owns the same contract the
// bash adapter had: it materializes the prompt, dispatches the driver, and
// writes the artifact at ArtifactPath. A VALIDATE_ONLY=1 entry in env is
// honored by the bridge's launch path (it prints the resolved config and
// returns ExitOK without invoking an LLM), so ValidateProfile's dry-validate
// keeps working (same ExitOK contract; no LLM invoked).
//
// adapterPath is retained for the injectable ExecAdapter seam (tests stub the
// whole function), but the default no longer reads the .sh file — it reads
// RESOLVED_CLI from env and projects it onto a registered driver via
// bridge.DriverFor.
// execAdapterDeps builds the gobridge.Deps for the subagent composition
// root, wiring TokenResolver via tokenusage.DefaultResolver against the
// env's HOME — the same configRoot-resolution convention as
// internal/adapters/bridge's productionEngineDeps (env["HOME"] falling back
// to os.Getenv("HOME"), joined with ".claude"). The two production
// composition roots (adapters/bridge, this package) share the single
// tokenusage.DefaultResolver helper, each resolving configRoot identically —
// and, since F27, the single policy.BridgeRecoveryStages accessor for the two
// ADR-0044 recovery dials, so a dead pane fast-fails on this root exactly as
// on the cycle root (it built Deps with neither dial, pinning both to shadow).
func execAdapterDeps(env map[string]string) gobridge.Deps {
	home := env["HOME"]
	if home == "" {
		home = os.Getenv("HOME")
	}
	configRoot := filepath.Join(home, ".claude")
	recoveryStage, fatalPaneStage := projectPolicy(env).BridgeRecoveryStages()
	return gobridge.Deps{
		Env:            env,
		TokenResolver:  tokenusage.DefaultResolver(configRoot),
		RecoveryStage:  recoveryStage,
		FatalPaneStage: fatalPaneStage,
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

// execAdapterDepsWith is execAdapterDeps carrying the root's Signal Center —
// the ONE bridge touch of unit 16 (ADR-0103): the engine's own bridge.warning,
// bridge.tripwire and pane.liveness producers report into the same Center the
// dispatcher does instead of into nil. nil stays the Null Object.
func execAdapterDepsWith(env map[string]string, signals *signalcenter.Center) gobridge.Deps {
	d := execAdapterDeps(env)
	d.Signals = signals
	return d
}

// defaultExecAdapter is the Center-less projection of execAdapter — the
// ValidateProfile default and the ACS-shaped func seam's spelling.
func defaultExecAdapter(ctx context.Context, _ string, env map[string]string) (int, error) {
	return execAdapter(ctx, env, nil)
}

// bridgeAdapter is the production Adapter of the unit-16 dispatcher: the
// gobridge engine over the rendered env, carrying the root's Center.
type bridgeAdapter struct {
	signals *signalcenter.Center
}

// Exec dispatches through the in-process bridge engine.
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
	// Contract bridge: the bash adapter accepted an empty prompt under
	// VALIDATE_ONLY=1 (ValidateProfile sets PROMPT_FILE=""), but the bridge's
	// launch guard fails fast on an empty prompt (an empty prompt would hang a
	// real launch at the artifact timeout). Validate-only never reads the prompt
	// — it prints the resolved config and returns ExitOK — so a placeholder
	// satisfies the guard without changing behavior. A real run (VALIDATE_ONLY=0)
	// always carries a non-empty PROMPT_FILE, so this never masks a missing prompt.
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
		// resp is the zero value (ExitCode 0), and "exit 0 + error" misleads
		// VerifyArtifact. Mirror the old bash defaultExecAdapter: -1 on error.
		return -1, err
	}
	return resp.ExitCode, nil
}

// atoiOrZero parses a base-10 integer, returning 0 on any error or for
// negative values (the bridge treats Cycle<=0 as "no cycle", matching the
// bash adapter's unset-CYCLE path).
func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
