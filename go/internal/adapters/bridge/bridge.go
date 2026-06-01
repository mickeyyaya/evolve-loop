// Package bridge adapts the in-process native-Go bridge.Engine to the
// core.Bridge port, adding the one concern the Engine deliberately does
// not own: interactive-policy injection into the prompt body. The bash
// tools/agent-bridge subprocess and the EVOLVE_BRIDGE_GO toggle that
// selected it were removed in the v12 flag-day cutover — the Go bridge is
// now the only implementation, so this adapter has a single path.
//
// Production wiring goes through NewDefault; tests override engineFactory
// to inject a fake core.Bridge and assert delegation + policy injection.
package bridge

import (
	"context"
	"errors"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
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

// Adapter is the core.Bridge implementation: it injects the interactive
// policy prefix, then delegates to the in-process Go bridge.Engine built
// by engineFactory. A single Adapter instance is safe for sequential reuse.
type Adapter struct {
	// engineFactory builds the in-process core.Bridge for a given
	// request-local env overlay. Defaulted in New; overridable in tests.
	engineFactory func(env map[string]string) core.Bridge
	// onStopReview, when non-nil, is invoked for every stop-review decision
	// the tmux driver makes (extend AND pause). The cycle number is taken from
	// BridgeRequest.Cycle at the time of the Launch call, so the callback is
	// cycle-scoped. Set via SetOnStopReview after construction.
	onStopReview func(cycle int, phase, action, reason string)
}

// New constructs an Adapter backed by the native-Go bridge.Engine. Tests
// override the engineFactory field directly to inject a fake.
func New() *Adapter {
	return &Adapter{
		engineFactory: func(env map[string]string) core.Bridge {
			return gobridge.NewEngine(gobridge.Deps{Env: env})
		},
	}
}

// NewDefault constructs the production Adapter. projectRoot is accepted
// for call-site stability (every phase passes req.ProjectRoot) and is
// reserved for future project-root-relative manifest resolution; the
// Engine currently resolves paths from the request, so it is unused here.
func NewDefault(projectRoot string) *Adapter { //nolint:unparam // projectRoot reserved for call-site stability
	return New()
}

// SetOnStopReview wires a callback invoked for every stop-review verdict the
// tmux driver makes during a Launch call. cycle is taken from BridgeRequest.Cycle.
// Passing nil clears the callback (no-op; default production state).
func (a *Adapter) SetOnStopReview(fn func(cycle int, phase, action, reason string)) {
	a.onStopReview = fn
}

// Launch injects the resolved interactive policy into the prompt body and
// delegates to the Engine, which materializes the prompt, dispatches the
// driver, and reads the artifact into BridgeResponse.Stdout.
func (a *Adapter) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := validate(req); err != nil {
		return core.BridgeResponse{}, err
	}
	inproc := req
	inproc.Prompt = injectRulesPrefix(injectPolicyPrefix(req.Prompt, resolvePolicy(req.Agent, req.Env)), req.SystemPrompt)

	// When an onStopReview callback is wired (production path), build the engine
	// directly so we can inject the cycle-scoped OnStopReview into Deps.
	// Tests that override engineFactory leave onStopReview nil, so they continue
	// to use the engineFactory path unchanged.
	if a.onStopReview != nil {
		cycle := req.Cycle
		cb := a.onStopReview
		onSR := func(phase, action, reason string) { cb(cycle, phase, action, reason) }
		return gobridge.NewEngine(gobridge.Deps{Env: req.Env, OnStopReview: onSR}).Launch(ctx, inproc)
	}
	return a.engineFactory(req.Env).Launch(ctx, inproc)
}

// Probe delegates environment/CLI discovery to the Engine.
func (a *Adapter) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return a.engineFactory(nil).Probe(ctx)
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

// resolvePolicy returns the effective interactive policy for the given
// agent. The lookup chain is two layered envchain.Resolve calls — the
// per-agent override layer first, then the global EVOLVE_INTERACTIVE_POLICY
// layer — so the precedence semantics live in envchain and stay aligned
// with the runner's per-phase env resolution and any future config-knob site.
//
// Effective precedence:
//
//  1. reqEnv[EVOLVE_<AGENT>_INTERACTIVE_POLICY]
//  2. os.Getenv(EVOLVE_<AGENT>_INTERACTIVE_POLICY)
//  3. reqEnv[EVOLVE_INTERACTIVE_POLICY]
//  4. os.Getenv(EVOLVE_INTERACTIVE_POLICY)
//  5. PolicyRecommendedOrFirst (default-on autonomy posture)
func resolvePolicy(agent string, reqEnv map[string]string) string {
	if agent != "" {
		if v := envchain.Resolve(perAgentPolicyEnv(agent), reqEnv, "", ""); v != "" {
			return v
		}
	}
	return envchain.Resolve("EVOLVE_INTERACTIVE_POLICY", reqEnv, "", PolicyRecommendedOrFirst)
}

// perAgentPolicyEnv maps an agent name to the per-agent override env
// key: "scout" → "EVOLVE_SCOUT_INTERACTIVE_POLICY"; hyphens become
// underscores so "tdd-engineer" → "EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY".
// Delegates to envchain.PhaseEnvKey so the naming rule lives in one place.
func perAgentPolicyEnv(agent string) string {
	return envchain.PhaseEnvKey(agent, "INTERACTIVE_POLICY")
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
	default: // PolicyRecommendedOrFirst and unknown values both inject the default block
		return policyBlockRecommendedOrFirst + prompt
	}
}

// injectRulesPrefix prepends a "## Rules" block carrying the per-agent
// launch-time system prompt (facet B). Empty rules pass through unchanged.
// Applied at the same seam as injectPolicyPrefix so it is CLI-agnostic —
// headless and tmux drivers alike — and sidesteps launchCmdLine's lack of
// shell-quoting (a multi-line system prompt never touches the launch argv).
func injectRulesPrefix(prompt, rules string) string {
	if rules == "" {
		return prompt
	}
	return "## Rules\n\n" + rules + "\n\n---\n\n" + prompt
}
