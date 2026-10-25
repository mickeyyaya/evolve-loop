package bridge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

type LaunchPurpose string

const (
	PurposeProbe      LaunchPurpose = "probe"
	PurposeUpdate     LaunchPurpose = "update"
	PurposeSmoke      LaunchPurpose = "smoke"
	PurposeInventory  LaunchPurpose = "inventory"
	PurposeModelQuery LaunchPurpose = "modelquery"
)

const noManifestReason = "no family manifest names this binary"

var runTmuxEnvNames = []string{"TMUX", "TMUX_PANE"}

type LLMExec struct {
	Purpose LaunchPurpose
	Binary  string
	Args    []string
	Dir     string
	Drop    []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

func LaunchLLM(ctx context.Context, deps Deps, x LLMExec) (int, error) {
	if deps.Runner == nil {
		deps.Runner = execRunner
	}
	if deps.Stderr == nil {
		deps.Stderr = os.Stderr
	}
	if err := deps.requireDispatchCredential(x.Purpose, x.Binary); err != nil {
		return ExitSafetyGate, err
	}
	deps.logUnsandboxed(x.Purpose, x.Binary)
	env := slices.DeleteFunc(probeEnv(deps, x.Binary), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(runTmuxEnvNames, name) || slices.Contains(x.Drop, name)
	})
	return deps.Runner(ctx, x.Binary, x.Dir, x.Args, env, x.Stdin, x.Stdout, x.Stderr)
}

func ModelQueryRunner(ctx context.Context, name string, args []string, stdin string) (string, error) {
	var out bytes.Buffer
	x := LLMExec{Purpose: PurposeModelQuery, Binary: name, Args: args, Stdout: &out, Stderr: &out}
	if stdin != "" {
		x.Stdin = strings.NewReader(stdin)
	}
	rc, err := LaunchLLM(ctx, Deps{}, x)
	if err == nil && rc != 0 {
		err = fmt.Errorf("%s: exit %d", name, rc)
	}
	return out.String(), err
}

func launchPane(ctx context.Context, purpose LaunchPurpose, driver Driver, cfg *Config, deps Deps) (int, error) {
	if err := deps.requireDispatchCredential(purpose, cfg.CLI); err != nil {
		return ExitSafetyGate, err
	}
	launchPurposeConfig(purpose, cfg)
	return driver.Launch(ctx, cfg, deps)
}

func launchPurposeConfig(purpose LaunchPurpose, cfg *Config) {
	cfg.AllowBypass = purpose == PurposeProbe || purpose == PurposeSmoke
}

func (deps Deps) logUnsandboxed(purpose LaunchPurpose, binary string) {
	reason := noManifestReason
	if r, err := Resolve(DefaultDriverSet(), binary); err == nil {
		reason = r.Manifest.ProbeUnsandboxedReason
	}
	fmt.Fprintf(deps.Stderr, "[bridge] %s %s runs unsandboxed: %s\n", purpose, binary, reason)
}
