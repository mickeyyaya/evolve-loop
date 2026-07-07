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
	"os"
	"path/filepath"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// Interactive policy values for the typed profile policy and the per-agent
// request override EVOLVE_<AGENT>_INTERACTIVE_POLICY. The bridge
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
	// resolver resolves the deliverable contract injected into each phase's
	// prompt. Defaults to built-in-only; SetContractResolver upgrades it to a
	// catalog-aware resolver so user/minted phases get their spec-derived
	// contract block + exact-path footer (WS-A, ADR-0034).
	resolver phasecontract.Resolver
	// phaseIO is the EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.8b). At
	// >=StageAdvisory the injected contract block instructs build/scout/triage to
	// self-report failure via a structured sentinel; default StageOff keeps the
	// dispatched prompt byte-identical to pre-3.8b.
	phaseIO config.Stage
	// recoveryStage is the ADR-0044 Unified Phase Recovery stage, injected from
	// cfg.PhaseRecovery (policy-resolved). Passed to Deps.RecoveryStage on each
	// engine creation so fatalpane.go never reads the env var directly.
	recoveryStage string
	// bridgeConfig carries the timing overrides loaded from policy.json at
	// construction time. Zero values mean "use bridge built-in defaults".
	bridgeConfig policy.BridgePolicy
	// bootTimeoutStore records driver-scoped boot-timeout bench strikes.
	bootTimeoutStore *clihealth.Store
}

// New constructs an Adapter backed by the native-Go bridge.Engine. Tests
// override the engineFactory field directly to inject a fake.
func New() *Adapter {
	return &Adapter{
		engineFactory: func(env map[string]string) core.Bridge {
			return gobridge.NewEngine(gobridge.Deps{Env: env})
		},
		resolver: phasecontract.BuiltinResolver{},
	}
}

// NewDefault constructs the production Adapter, loading timing overrides
// from <projectRoot>/.evolve/policy.json when available (fail-open: a
// missing or unparseable policy.json falls back to bridge built-in defaults).
func NewDefault(projectRoot string) *Adapter {
	a := New()
	if pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); err == nil {
		a.bridgeConfig = pol.BridgeConfig()
	}
	a.bootTimeoutStore = clihealth.NewStore(projectRoot, nil)
	a.engineFactory = func(env map[string]string) core.Bridge {
		return gobridge.NewEngine(a.productionEngineDeps(env))
	}
	return a
}

// productionEngineDeps builds the gobridge.Deps shared by every production
// composition path in this Adapter (NewDefault's engineFactory and the
// onStopReview branch of Launch) so they cannot drift apart. Wires
// TokenResolver via tokenusage.DefaultResolver against the env's HOME (see
// configRoot) — the fix for the confirmed cycle-612+ bug where production
// launches got silent zero token telemetry.
func (a *Adapter) productionEngineDeps(env map[string]string) gobridge.Deps {
	return gobridge.Deps{
		Env:                env,
		BootTimeoutStore:   a.bootTimeoutStore,
		BootTimeoutS:       a.bridgeConfig.BootTimeoutS,
		ArtifactTimeoutS:   a.bridgeConfig.ArtifactTimeoutS,
		ArtifactMaxExtends: a.bridgeConfig.ArtifactMaxExtends,
		ScrollbackLines:    a.bridgeConfig.ScrollbackLines,
		TokenResolver:      tokenusage.DefaultResolver(configRoot(env)),
	}
}

// configRoot resolves the Claude config directory from a request-local env
// overlay, falling back to the process environment — same precedent as
// internal/bridge/doctor.go's doctorHome() + ".claude" (see
// internal/bridge/billing.go:47 for the exact join).
func configRoot(env map[string]string) string {
	home := env["HOME"]
	if home == "" {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".claude")
}

// BootTimeoutStoreWired reports whether the Adapter has a non-nil boot-timeout
// bench store. True for any Adapter built via NewDefault; false for bare New().
// Used by acceptance tests to confirm production deps inject the strike writer.
func (a *Adapter) BootTimeoutStoreWired() bool {
	return a.bootTimeoutStore != nil
}

// SetOnStopReview wires a callback invoked for every stop-review verdict the
// tmux driver makes during a Launch call. cycle is taken from BridgeRequest.Cycle.
// Passing nil clears the callback (no-op; default production state).
func (a *Adapter) SetOnStopReview(fn func(cycle int, phase, action, reason string)) {
	a.onStopReview = fn
}

// SetContractResolver upgrades the adapter to inject spec-derived contracts for
// user/minted phases. Pass a phasecontract.NewCatalogResolver(catalog.Get) built
// from the orchestrator's merged catalog. Passing nil restores built-in-only
// resolution (the default).
func (a *Adapter) SetContractResolver(r phasecontract.Resolver) {
	if r == nil {
		r = phasecontract.BuiltinResolver{}
	}
	a.resolver = r
}

// SetPhaseIOStage wires the EVOLVE_PHASE_IO rollout stage so the injected
// contract block activates the build/scout/triage self-report-failure
// instruction at >=StageAdvisory (ADR-0050 §3.8b). Default (unset) is StageOff:
// the dispatched prompt is byte-identical to pre-3.8b.
func (a *Adapter) SetPhaseIOStage(stage config.Stage) {
	a.phaseIO = stage
}

// SetRecoveryStage wires the ADR-0044 Unified Phase Recovery stage so
// the bridge engine's fatalpane detector reads the policy-resolved value
// instead of the retired EVOLVE_PHASE_RECOVERY env var. Default (unset) is
// "" which channel.ResolveStage normalizes to "shadow" (behavior-neutral).
func (a *Adapter) SetRecoveryStage(stage string) {
	a.recoveryStage = stage
}

// Launch injects the resolved interactive policy into the prompt body and
// delegates to the Engine, which materializes the prompt, dispatches the
// driver, and reads the artifact into BridgeResponse.Stdout.
func (a *Adapter) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := validate(req); err != nil {
		return core.BridgeResponse{}, err
	}
	inproc := req
	// Prompt assembly order (top of string → bottom):
	//   Correction (outermost, re-dispatch only) > Operator Directives > Rules > Policy > Contract block > Body > path footer.
	// The contract's invariant block sits in the cacheable prefix; the volatile
	// per-cycle path lands in the footer (last line) — cache-safe AND recency-
	// optimal. See injectContract.
	body := a.injectContract(req.Prompt, req.Agent, req.ArtifactPath)
	withPolicy := injectPolicyPrefix(body, resolvePolicy(req.Agent, req.Env, req.InteractivePolicy))
	withRules := injectRulesPrefix(withPolicy, req.SystemPrompt)
	withDirectives := injectOperatorDirectives(withRules, req.OperatorDirectives)
	inproc.Prompt = injectCorrectionPrefix(withDirectives, req.CorrectionDirective)

	// When an onStopReview callback is wired (production path), build the engine
	// directly so we can inject the cycle-scoped OnStopReview into Deps.
	// Tests that override engineFactory leave onStopReview nil, so they continue
	// to use the engineFactory path unchanged.
	if a.onStopReview != nil {
		cycle := req.Cycle
		cb := a.onStopReview
		onSR := func(phase, action, reason string) { cb(cycle, phase, action, reason) }
		deps := a.productionEngineDeps(req.Env)
		deps.RecoveryStage = a.recoveryStage
		deps.OnStopReview = onSR
		return gobridge.NewEngine(deps).Launch(ctx, inproc)
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

// resolvePolicy returns the effective interactive policy for the given agent.
// The request-local env is the explicit override surface and profilePolicy is
// the typed profile value resolved by the runner. Process env is intentionally
// excluded so the profile remains the persistent SSOT.
//
// Effective precedence:
//
//  1. reqEnv[EVOLVE_<AGENT>_INTERACTIVE_POLICY]
//  2. profilePolicy
//  3. PolicyRecommendedOrFirst (default-on autonomy posture)
func resolvePolicy(agent string, reqEnv map[string]string, profilePolicy string) string {
	if agent != "" {
		if v := reqEnv[perAgentPolicyEnv(agent)]; v != "" {
			return v
		}
	}
	if profilePolicy != "" {
		return profilePolicy
	}
	return PolicyRecommendedOrFirst
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

// injectContract wraps the prompt body with the Deliverable Contract (ADR-0034)
// when the agent has a registered contract: the invariant instruction block is
// prepended (cacheable prefix) and the volatile exact-path footer is appended
// (last line). Agents with no contract (non-phase bridge callers) pass through
// unchanged. The path is surfaced in the prompt TEXT here, not just in the
// BridgeRequest.ArtifactPath flag the engine uses to poll — closing the gap that
// forced the agent to infer its own output path.
//
// Resolution runs through a.resolver: built-ins always, plus spec-derived
// contracts for user/minted phases when a catalog resolver is wired (WS-A). A
// nil resolver (zero-value Adapter in a test) degrades to built-in-only.
func (a *Adapter) injectContract(prompt, agent, artifactPath string) string {
	resolver := a.resolver
	if resolver == nil {
		resolver = phasecontract.BuiltinResolver{}
	}
	c, ok := resolver.Resolve(agent)
	if !ok {
		return prompt
	}
	// ADR-0050 §3.8b: at >=StageAdvisory, instruct build/scout/triage to
	// self-report failure via a structured sentinel. Gated >=advisory (NOT
	// enforce) so the advisory soak exercises the emitted sentinels before the
	// enforce flip; off/shadow keep the prompt byte-identical (the classifier's
	// always-on Pass 0 must not see new sentinels in production).
	includePhaseIO := a.phaseIO >= config.StageAdvisory
	return phasecontract.RenderContractBlockStage(c, includePhaseIO) + prompt + phasecontract.RenderContractFooter(c, artifactPath)
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

// injectCorrectionPrefix prepends a "## Correction" block carrying the
// orchestrator's contract-correction directive (the previous deliverable was
// rejected; fix it). Empty directive passes through unchanged. Applied at the
// same CLI-agnostic seam as injectRulesPrefix, OUTERMOST so it lands at the very
// top of the prompt where the agent sees the correction first.
func injectCorrectionPrefix(prompt, directive string) string {
	if directive == "" {
		return prompt
	}
	return "## Correction\n\n" + directive + "\n\n---\n\n" + prompt
}

// injectOperatorDirectives prepends the runtime operator-directives block (the
// pre-rendered global + per-loop guidance snapshotted at cycle start). Empty
// directives pass through unchanged (byte-identical to no directives). Applied at
// the same CLI-agnostic seam as injectRulesPrefix, sitting just below Correction
// so standing operator guidance is prominent without displacing an active
// contract-correction. The block is already a complete "## Operator Directives"
// section (rendered by internal/directives), so it is prepended verbatim.
func injectOperatorDirectives(prompt, directives string) string {
	if directives == "" {
		return prompt
	}
	return directives + "\n\n---\n\n" + prompt
}

// SetModelCatalogDirFn sets the model-catalog directory resolver in the inner bridge.
// Called by cmd/evolve to inject the active cycle's evolve directory without
// touching the process environment.
func SetModelCatalogDirFn(fn func() string) {
	gobridge.SetModelCatalogDirFn(fn)
}
