// Package bridge adapts the in-process bridge.Engine to the core.Bridge port
// and assembles the prompt each phase agent receives.
// See docs/architecture/packages/internal-adapters-bridge.md.
package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/skilloverlay"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// Interactive policies: how a phase agent resolves its own interactive prompts without blocking the loop.
const (
	PolicyRecommendedOrFirst = "recommended_or_first"
	PolicyEscalate           = "escalate"
	PolicyAutoYes            = "auto_yes"
)

// The policy blocks stay constant and under ~200 tokens so the cached prompt prefix stays stable.
const policyBlockRecommendedOrFirst = "## Subagent Interactive Policy (recommended_or_first)\n\n" +
	"If you would invoke AskUserQuestion or any equivalent interactive prompt, instead\n" +
	"auto-resolve as follows:\n" +
	"- Pick the option labeled \"(Recommended)\" if present.\n" +
	"- Otherwise pick the first listed option.\n" +
	"- Record the resolution in your output as: `Auto-picked: <choice> (policy: recommended-or-first)`.\n" +
	"- Never block on operator input; the loop is autonomous.\n\n---\n\n"

const policyBlockAutoYes = "## Subagent Interactive Policy (auto_yes)\n\n" +
	"For any binary yes/no prompt that would otherwise block, choose \"yes\" and note\n" +
	"the resolution in your output as: `Auto-picked: yes (policy: auto_yes)`.\n" +
	"For multi-option prompts, defer to recommended-or-first:\n" +
	"- Pick the option labeled \"(Recommended)\" if present.\n" +
	"- Otherwise pick the first listed option.\n" +
	"Never block on operator input; the loop is autonomous.\n\n---\n\n"

// Adapter is the core.Bridge implementation: it assembles the prompt, then delegates to the in-process Engine.
type Adapter struct {
	engineFactory func(env map[string]string) core.Bridge
	// onStopReview receives every stop-review decision (extend and pause), scoped to the request's Cycle.
	onStopReview func(cycle int, phase, action, reason string)
	resolver     phasecontract.Resolver
	// signals is the Center every engine this Adapter builds produces into; nil is the Null Object.
	signals *signalcenter.Center
	phaseIO config.Stage
	// recoveryStage is the phase-recovery program dial (channel, ask-broker, transient-dwell).
	recoveryStage string
	// fatalPaneStage is the fatal-pane fast-fail's own dial, independent of recoveryStage.
	fatalPaneStage string
	// bridgeConfig holds the policy.json timing overrides; zero values mean the engine's built-in defaults.
	bridgeConfig policy.BridgePolicy
	// contextFillWarnPct is the validated policy threshold; zero lets the engine apply its built-in default.
	contextFillWarnPct int
	// bootTimeoutStore records driver-scoped boot-timeout bench strikes.
	bootTimeoutStore *clihealth.Store
}

// New constructs an Adapter backed by the in-process bridge.Engine, resolving built-in contracts only.
func New() *Adapter {
	return &Adapter{
		engineFactory: func(env map[string]string) core.Bridge {
			return gobridge.NewEngine(gobridge.Deps{Env: env})
		},
		resolver: phasecontract.BuiltinResolver{},
	}
}

// NewDefault constructs the production Adapter, seeding it from <projectRoot>/.evolve/policy.json (fail-open to built-in defaults).
func NewDefault(projectRoot string, signals *signalcenter.Center) *Adapter {
	a := New()
	a.signals = signals
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err == nil {
		a.bridgeConfig = pol.BridgeConfig()
		// The validating resolver, never the raw field: an out-of-range value must arrive as the built-in.
		a.contextFillWarnPct = pol.ContextFillConfig().WarnThresholdPct
	}
	// Seeded for roots that never call the setters; a failed load leaves pol zero, so compiled defaults apply.
	// See ADR-0044.
	a.recoveryStage, a.fatalPaneStage = pol.BridgeRecoveryStages()
	a.bootTimeoutStore = clihealth.NewStore(projectRoot, nil)
	a.engineFactory = func(env map[string]string) core.Bridge {
		return gobridge.NewEngine(a.productionEngineDeps(env))
	}
	return a
}

// productionEngineDeps is the one Deps builder for both production paths (NewDefault's factory and
// Launch's onStopReview branch), so they cannot drift apart.
func (a *Adapter) productionEngineDeps(env map[string]string) gobridge.Deps {
	return gobridge.Deps{
		Env:                   env,
		BootTimeoutStore:      a.bootTimeoutStore,
		BootTimeoutS:          a.bridgeConfig.BootTimeoutS,
		ArtifactTimeoutS:      a.bridgeConfig.ArtifactTimeoutS,
		ArtifactMaxExtends:    a.bridgeConfig.ArtifactMaxExtends,
		PhaseArtifactTimeoutS: a.bridgeConfig.PhaseArtifactTimeouts(),
		ScrollbackLines:       a.bridgeConfig.ScrollbackLines,
		TokenResolver:         tokenusage.DefaultResolver(configRoot(env)),
		ContextFillWarnPct:    a.contextFillWarnPct,
		RecoveryStage:         a.recoveryStage,
		FatalPaneStage:        a.fatalPaneStage,
		Signals:               a.signals,
		// A pane wall match escalates rc 85 only after a live probe confirms it; wired only at this
		// production root so engine tests keep the nil seam.
		CorroborateWall: gobridge.DefaultWallCorroborator(nil, os.Stderr),
	}
}

// configRoot is $HOME/.claude, reading HOME from the request env before the process env.
func configRoot(env map[string]string) string {
	home := env["HOME"]
	if home == "" {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".claude")
}

// BootTimeoutStoreWired reports whether a boot-timeout strike store is wired (true for NewDefault, false for New).
func (a *Adapter) BootTimeoutStoreWired() bool {
	return a.bootTimeoutStore != nil
}

// SetOnStopReview sets the callback for every stop-review decision made during Launch; nil clears it.
func (a *Adapter) SetOnStopReview(fn func(cycle int, phase, action, reason string)) {
	a.onStopReview = fn
}

// SetContractResolver installs a catalog-aware contract resolver for user and minted phases; nil restores built-ins only.
func (a *Adapter) SetContractResolver(r phasecontract.Resolver) {
	if r == nil {
		r = phasecontract.BuiltinResolver{}
	}
	a.resolver = r
}

// SetPhaseIOStage sets the phase-I/O stage; at StageAdvisory or above the contract block asks for structured failure sentinels.
func (a *Adapter) SetPhaseIOStage(stage config.Stage) {
	a.phaseIO = stage
}

// SetRecoveryStage sets the phase-recovery stage (channel, ask-broker, transient-dwell); "" normalizes to shadow.
func (a *Adapter) SetRecoveryStage(stage string) {
	a.recoveryStage = stage
}

// SetFatalPaneStage sets the fatal-pane fast-fail's own stage, independent of SetRecoveryStage; "" normalizes to shadow.
func (a *Adapter) SetFatalPaneStage(stage string) {
	a.fatalPaneStage = stage
}

// Launch assembles the prompt (contract, policy, rules, skills, directives, correction) and delegates to the Engine.
func (a *Adapter) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := validate(req); err != nil {
		return core.BridgeResponse{}, err
	}
	inproc := req
	// Each inject wraps the previous result, so Correction lands outermost. Nothing per-cycle may
	// precede the body: the prefix stays cacheable and the path lands in the tail.
	contractID := req.Agent
	if req.Contract != "" {
		contractID = req.Contract
		if _, ok := a.contractResolver().Resolve(contractID); !ok {
			return core.BridgeResponse{}, fmt.Errorf("bridge: deliverable contract %q not registered", contractID)
		}
	}
	body := a.injectContract(req.Prompt, contractID, req.ArtifactPath, req.Workspace)
	withPolicy := injectPolicyPrefix(body, resolvePolicy(req.ProjectRoot, req.Agent, req.InteractivePolicy))
	withRules := injectRulesPrefix(withPolicy, req.SystemPrompt)
	withSkills := injectSkillOverlays(withRules, req)
	withDirectives := injectOperatorDirectives(withSkills, req.OperatorDirectives)
	inproc.Prompt = injectCorrectionPrefix(withDirectives, req.CorrectionDirective)

	// The cycle-scoped OnStopReview needs this launch's own Deps, so this branch bypasses engineFactory.
	if a.onStopReview != nil {
		cycle := req.Cycle
		cb := a.onStopReview
		onSR := func(phase, action, reason string) { cb(cycle, phase, action, reason) }
		deps := a.productionEngineDeps(req.Env)
		deps.OnStopReview = onSR
		return gobridge.NewEngine(deps).Launch(ctx, inproc)
	}
	return a.engineFactory(req.Env).Launch(ctx, inproc)
}

// Probe delegates environment/CLI discovery to the Engine.
func (a *Adapter) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return a.engineFactory(nil).Probe(ctx)
}

func validate(req core.BridgeRequest) error { return gobridge.ValidateRequest(req) }

// resolvePolicy prefers a non-default policy.json value, then the profile's policy, then recommended_or_first.
func resolvePolicy(projectRoot string, agent string, profilePolicy string) string {
	pol := policy.InteractivePolicyFor(projectRoot, agent)
	if pol != "recommended_or_first" {
		return pol
	}
	if profilePolicy != "" {
		return profilePolicy
	}
	return PolicyRecommendedOrFirst
}

// injectPolicyPrefix prepends the policy block. escalate adds none; an unknown value gets the default
// block so a typo cannot break the loop's autonomy.
func injectPolicyPrefix(prompt, policy string) string {
	switch policy {
	case PolicyEscalate:
		return prompt
	case PolicyAutoYes:
		return policyBlockAutoYes + prompt
	default:
		return policyBlockRecommendedOrFirst + prompt
	}
}

func (a *Adapter) contractResolver() phasecontract.Resolver {
	resolver := a.resolver
	if resolver == nil {
		resolver = phasecontract.BuiltinResolver{}
	}
	return resolver
}

// injectContract puts the resolved contract's invariant block before the body and its exact-path tail after it.
// See ADR-0034.
func (a *Adapter) injectContract(prompt, contractID, artifactPath, workspace string) string {
	resolver := a.contractResolver()
	c, ok := resolver.Resolve(contractID)
	if !ok {
		if artifactPath == "" {
			return prompt
		}
		// The engine polls artifactPath, so the prompt must name it. Footer only: the full tail's
		// `evolve phase verify <agent>` self-check always exits 10 for an unregistered phase.
		c = phasecontract.Contract{Phase: contractID, AgentName: contractID, ArtifactName: filepath.Base(artifactPath)}
		return prompt + phasecontract.RenderContractFooter(c, artifactPath)
	}
	// Off and shadow keep the prompt byte-identical: the classifier's sentinel pass is not stage-gated.
	// See ADR-0050.
	includePhaseIO := a.phaseIO >= config.StageAdvisory
	return phasecontract.RenderContractBlockStage(c, includePhaseIO) + prompt + phasecontract.RenderContractTail(c, artifactPath, workspace)
}

// injectRulesPrefix prepends the agent's launch-time system prompt as a "## Rules" block.
// See ADR-0023.
func injectRulesPrefix(prompt, rules string) string {
	if rules == "" {
		return prompt
	}
	return "## Rules\n\n" + rules + "\n\n---\n\n" + prompt
}

// injectSkillOverlays prepends req.Skills, read from <ProjectRoot>/skills/<name>/SKILL.md. A skill
// that cannot be read is WARNed and skipped, never fatal.
func injectSkillOverlays(prompt string, req core.BridgeRequest) string {
	if req.ProjectRoot == "" || len(req.Skills) == 0 {
		return prompt
	}
	skillsDir := filepath.Join(req.ProjectRoot, "skills")
	overlay, missing := skilloverlay.Materialize(skillsDir, req.Skills)
	for _, m := range missing {
		log.Diag().Warnf("[bridge] WARN skill-overlay: configured skill %q for phase %q not found under %s — dispatched WITHOUT it (check policy.json overlays + skills registry)\n", m, req.Agent, skillsDir)
	}
	if overlay == "" {
		return prompt
	}
	return overlay + "---\n\n" + prompt
}

// injectCorrectionPrefix prepends the orchestrator's contract-correction directive as a "## Correction" block.
func injectCorrectionPrefix(prompt, directive string) string {
	if directive == "" {
		return prompt
	}
	return "## Correction\n\n" + directive + "\n\n---\n\n" + prompt
}

// injectOperatorDirectives prepends the already-rendered "## Operator Directives" block verbatim.
func injectOperatorDirectives(prompt, directives string) string {
	if directives == "" {
		return prompt
	}
	return directives + "\n\n---\n\n" + prompt
}

// SetModelCatalogDirFn sets the inner bridge's process-wide model-catalog directory resolver.
func SetModelCatalogDirFn(fn func() string) {
	gobridge.SetModelCatalogDirFn(fn)
}

// SignalsWired reports whether a Signal Center was injected at construction.
func (a *Adapter) SignalsWired() bool { return a.signals != nil }

// Signals returns the injected Signal Center for the phase runner to adopt; nil for the Null Object and a nil receiver.
func (a *Adapter) Signals() *signalcenter.Center {
	if a == nil {
		return nil
	}
	return a.signals
}
