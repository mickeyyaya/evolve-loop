// Package bridge wraps the tools/agent-bridge/bin/bridge subprocess as
// a core.Bridge implementation. The plan §5 contract is: Go shells to
// `bridge launch / probe / validate / report` and parses output. No
// reimplementation of the CLI dispatch logic — that lives in bash and
// gets ported only when the v2 bridge effort lands.
//
// Production wiring goes through NewDefault (or New with binary="bridge"
// resolved on PATH). Tests inject a CmdRunner to drive subprocess
// behavior without actually exec()ing bridge.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Interactive policy values for EVOLVE_INTERACTIVE_POLICY and the
// per-agent override EVOLVE_<AGENT>_INTERACTIVE_POLICY. The bridge
// prepends a deterministic policy block to the prompt body so phase
// agents self-resolve interactive prompts (AskUserQuestion, y/N) without
// blocking the autonomous loop. See docs/architecture/plan-mode-dispatch.md
// (v12.1) for the design rationale.
const (
	PolicyRecommendedOrFirst = "recommended_or_first"
	PolicyEscalate           = "escalate"
	PolicyAutoYes            = "auto_yes"
)

// policyBlockRecommendedOrFirst is the prompt prefix injected when the
// effective policy is recommended_or_first. Kept short to stay well
// under the 200-token cache-prefix budget called out in the v12.1 plan.
const policyBlockRecommendedOrFirst = "## Subagent Interactive Policy (recommended_or_first)\n\n" +
	"If you would invoke AskUserQuestion or any equivalent interactive prompt, instead\n" +
	"auto-resolve as follows:\n" +
	"- Pick the option labeled \"(Recommended)\" if present.\n" +
	"- Otherwise pick the first listed option.\n" +
	"- Record the resolution in your output as: `Auto-picked: <choice> (policy: recommended-or-first)`.\n" +
	"- Never block on operator input; the loop is autonomous.\n\n---\n\n"

// policyBlockAutoYes is the prompt prefix injected when the effective
// policy is auto_yes. For multi-option prompts the agent falls back to
// the recommended-or-first rule.
const policyBlockAutoYes = "## Subagent Interactive Policy (auto_yes)\n\n" +
	"For any binary yes/no prompt that would otherwise block, choose \"yes\" and note\n" +
	"the resolution in your output as: `Auto-picked: yes (policy: auto_yes)`.\n" +
	"For multi-option prompts, defer to recommended-or-first:\n" +
	"- Pick the option labeled \"(Recommended)\" if present.\n" +
	"- Otherwise pick the first listed option.\n" +
	"Never block on operator input; the loop is autonomous.\n\n---\n\n"

// CmdRunner is the seam for injecting subprocess behavior. The default
// impl (execRunner) calls exec.CommandContext; tests provide a fake.
//
// Return value is the exit code; err is non-nil only on truly
// unrecoverable failures (binary not found, context cancellation).
// A non-zero exit code with err==nil is the normal "process ran but
// failed" path.
type CmdRunner func(ctx context.Context, name string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

// Adapter is the core.Bridge implementation.
type Adapter struct {
	binary string
	runner CmdRunner
}

// New constructs an Adapter using the given bridge binary path and
// command runner. Pass nil runner to use the default exec.Command
// runner — appropriate for production. Empty binary defaults to
// looking up "bridge" on PATH.
func New(binary string, runner CmdRunner) *Adapter {
	if binary == "" {
		binary = "bridge"
	}
	if runner == nil {
		runner = execRunner
	}
	return &Adapter{binary: binary, runner: runner}
}

// NewDefault constructs an Adapter with the conventional binary path
// (tools/agent-bridge/bin/bridge relative to the project root) and the
// default exec runner.
func NewDefault(projectRoot string) *Adapter {
	return New(filepath.Join(projectRoot, "tools", "agent-bridge", "bin", "bridge"), nil)
}

// Launch invokes `bridge launch ...` with flags derived from req.
// On exit code 0 the artifact file is read into BridgeResponse.Stdout
// (mirroring the cli_adapters convention where the artifact IS the
// output the orchestrator cares about). On non-zero exit the response
// carries the exit code but the call returns an error.
func (a *Adapter) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := validate(req); err != nil {
		return core.BridgeResponse{}, err
	}
	// 1. Materialize prompt to a file under Workspace.
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return core.BridgeResponse{}, fmt.Errorf("bridge: ensure workspace: %w", err)
	}
	promptFile := filepath.Join(req.Workspace, fmt.Sprintf("%s-prompt.txt", nonEmpty(req.Agent, "agent")))
	prompt := injectPolicyPrefix(req.Prompt, resolvePolicy(req.Agent, req.Env))
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		return core.BridgeResponse{}, fmt.Errorf("bridge: write prompt: %w", err)
	}
	// 2. Derive missing log paths.
	stdoutLog := req.StdoutLog
	if stdoutLog == "" {
		stdoutLog = filepath.Join(req.Workspace, fmt.Sprintf("%s-stdout.log", nonEmpty(req.Agent, "agent")))
	}
	stderrLog := req.StderrLog
	if stderrLog == "" {
		stderrLog = filepath.Join(req.Workspace, fmt.Sprintf("%s-stderr.log", nonEmpty(req.Agent, "agent")))
	}
	// 3. Build argv.
	args := []string{
		"launch",
		"--cli=" + req.CLI,
		"--profile=" + req.Profile,
		"--model=" + req.Model,
		"--prompt-file=" + promptFile,
		"--workspace=" + req.Workspace,
		"--stdout-log=" + stdoutLog,
		"--stderr-log=" + stderrLog,
		"--artifact=" + req.ArtifactPath,
	}
	if req.Cycle > 0 {
		args = append(args, "--cycle="+strconv.Itoa(req.Cycle))
	}
	if req.Agent != "" {
		args = append(args, "--agent="+req.Agent)
	}
	if req.Worktree != "" {
		args = append(args, "--worktree="+req.Worktree)
	}
	args = append(args, req.ExtraFlags...)

	// 4. Build env (KEY=VALUE; inherit parent env + override with req.Env).
	env := os.Environ()
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}

	// 5. Run.
	var stdoutBuf, stderrBuf bytes.Buffer
	exitCode, err := a.runner(ctx, a.binary, args, env, nil, &stdoutBuf, &stderrBuf)
	resp := core.BridgeResponse{
		ExitCode: exitCode,
		Stderr:   stderrBuf.String(),
	}
	if err != nil {
		return resp, fmt.Errorf("bridge: launch: %w", err)
	}
	if exitCode != 0 {
		return resp, fmt.Errorf("bridge: launch exit=%d: %s", exitCode, truncate(resp.Stderr, 200))
	}
	// 6. Read artifact (best-effort — missing artifact on exit 0 is
	// not fatal; some agent profiles legitimately produce no artifact).
	if b, err := os.ReadFile(req.ArtifactPath); err == nil {
		resp.Stdout = string(b)
	}
	return resp, nil
}

// Probe shells `bridge probe` and parses the {os, results: [...]} JSON
// into a core.BridgeProbe.
func (a *Adapter) Probe(ctx context.Context) (core.BridgeProbe, error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	exitCode, err := a.runner(ctx, a.binary, []string{"probe"}, os.Environ(), nil, &stdoutBuf, &stderrBuf)
	if err != nil {
		return core.BridgeProbe{}, fmt.Errorf("bridge: probe: %w", err)
	}
	if exitCode != 0 {
		return core.BridgeProbe{}, fmt.Errorf("bridge: probe exit=%d: %s", exitCode, truncate(stderrBuf.String(), 200))
	}
	var raw struct {
		OS      string `json:"os"`
		Results []struct {
			CLI    string `json:"cli"`
			Tier   string `json:"tier"`
			Binary string `json:"binary"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdoutBuf.Bytes(), &raw); err != nil {
		return core.BridgeProbe{}, fmt.Errorf("bridge: parse probe json: %w", err)
	}
	out := core.BridgeProbe{
		Version: raw.OS,
		CLIs:    make(map[string]string, len(raw.Results)),
	}
	for _, r := range raw.Results {
		out.CLIs[r.CLI] = r.Tier
	}
	return out, nil
}

func validate(req core.BridgeRequest) error {
	switch "" {
	case req.CLI:
		return errors.New("bridge: CLI required")
	case req.Profile:
		return errors.New("bridge: Profile required")
	case req.Workspace:
		return errors.New("bridge: Workspace required")
	case req.ArtifactPath:
		return errors.New("bridge: ArtifactPath required")
	}
	return nil
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// resolvePolicy returns the effective interactive policy for the given
// agent. Precedence: per-agent override (reqEnv > process env) > global
// env > default (PolicyRecommendedOrFirst).
//
// reqEnv is the per-request env map (BridgeRequest.Env). It takes
// precedence over the parent process env so the orchestrator can pin a
// policy for a specific phase without mutating its own environment.
func resolvePolicy(agent string, reqEnv map[string]string) string {
	if agent != "" {
		perAgentKey := perAgentPolicyEnv(agent)
		if v, ok := reqEnv[perAgentKey]; ok && v != "" {
			return v
		}
		if v := os.Getenv(perAgentKey); v != "" {
			return v
		}
	}
	if v, ok := reqEnv["EVOLVE_INTERACTIVE_POLICY"]; ok && v != "" {
		return v
	}
	if v := os.Getenv("EVOLVE_INTERACTIVE_POLICY"); v != "" {
		return v
	}
	return PolicyRecommendedOrFirst
}

// perAgentPolicyEnv maps an agent name to the per-agent override env
// key: "scout" → "EVOLVE_SCOUT_INTERACTIVE_POLICY"; hyphens become
// underscores so "tdd-engineer" → "EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY".
func perAgentPolicyEnv(agent string) string {
	upper := strings.ReplaceAll(strings.ToUpper(agent), "-", "_")
	return "EVOLVE_" + upper + "_INTERACTIVE_POLICY"
}

// injectPolicyPrefix prepends the policy block to the prompt body based
// on the resolved policy. Returns the original prompt unchanged when
// policy is "escalate" (operator opted out of auto-resolution).
// Unknown values fall through to recommended_or_first so a typo in env
// configuration cannot break the autonomy posture.
func injectPolicyPrefix(prompt, policy string) string {
	switch policy {
	case PolicyEscalate:
		return prompt
	case PolicyAutoYes:
		return policyBlockAutoYes + prompt
	case PolicyRecommendedOrFirst:
		return policyBlockRecommendedOrFirst + prompt
	default:
		return policyBlockRecommendedOrFirst + prompt
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// execRunner is the production CmdRunner — wraps exec.CommandContext.
func execRunner(ctx context.Context, name string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
