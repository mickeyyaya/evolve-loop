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

const (
	PolicyRecommendedOrFirst = "recommended_or_first"
	PolicyEscalate           = "escalate"
	PolicyAutoYes            = "auto_yes"
)

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

type Adapter struct {
	engineFactory       func(env map[string]string) core.Bridge
	onStopReview        func(cycle int, phase, action, reason string)
	resolver            phasecontract.Resolver
	signals             *signalcenter.Center
	phaseIO             config.Stage
	recoveryStage       string
	fatalPaneStage      string
	bridgeConfig        policy.BridgePolicy
	efforts             policy.EffortTable
	floors              policy.Floors
	contextFillWarnPct  int
	paneCaptureMaxBytes int
	bootTimeoutStore    *clihealth.Store
}

func New() *Adapter {
	return &Adapter{
		engineFactory: func(env map[string]string) core.Bridge {
			return gobridge.NewEngine(gobridge.Deps{Env: env, TokenResolver: tokenusage.DefaultResolver(tokenusage.ClaudeConfigRoot(env))})
		},
		resolver: phasecontract.BuiltinResolver{},
	}
}

func NewDefault(projectRoot string, signals *signalcenter.Center) *Adapter {
	a := New()
	a.signals = signals
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err == nil {
		a.bridgeConfig = pol.BridgeConfig()
		a.efforts = pol.Efforts()
		a.floors = pol.Floors()
		a.contextFillWarnPct = pol.ContextFillConfig().WarnThresholdPct
	}
	a.recoveryStage, a.fatalPaneStage = pol.BridgeRecoveryStages()
	a.paneCaptureMaxBytes = pol.AttemptPostmortemConfig().MaxPaneCaptureBytes
	a.bootTimeoutStore = clihealth.NewStore(projectRoot, nil)
	a.engineFactory = func(env map[string]string) core.Bridge {
		return gobridge.NewEngine(a.productionEngineDeps(env))
	}
	return a
}

func (a *Adapter) productionEngineDeps(env map[string]string) gobridge.Deps {
	return gobridge.Deps{
		Env:                   env,
		BootTimeoutStore:      a.bootTimeoutStore,
		BootTimeoutS:          a.bridgeConfig.BootTimeoutS,
		ArtifactTimeoutS:      a.bridgeConfig.ArtifactTimeoutS,
		ArtifactMaxExtends:    a.bridgeConfig.ArtifactMaxExtends,
		PhaseArtifactTimeoutS: a.bridgeConfig.PhaseArtifactTimeouts(),
		Efforts:               a.efforts,
		ScrollbackLines:       a.bridgeConfig.ScrollbackLines,
		TokenResolver:         tokenusage.Resolver(tokenusage.Roots{Claude: tokenusage.ClaudeConfigRoot(env), Agy: tokenusage.AgyDataRoot(env)}, tokenusage.SQLiteCLI),
		ContextFillWarnPct:    a.contextFillWarnPct,
		PaneCaptureMaxBytes:   a.paneCaptureMaxBytes,
		RecoveryStage:         a.recoveryStage,
		FatalPaneStage:        a.fatalPaneStage,
		Signals:               a.signals,
		CorroborateWall:       gobridge.DefaultWallCorroborator(nil, os.Stderr),
		ModelChain:            gobridge.ModelChainFor(a.bridgeConfig.ModelFailoverOn()),
		Floors:                a.floors,
	}
}

func (a *Adapter) BootTimeoutStoreWired() bool {
	return a.bootTimeoutStore != nil
}

func (a *Adapter) SetOnStopReview(fn func(cycle int, phase, action, reason string)) {
	a.onStopReview = fn
}

func (a *Adapter) SetContractResolver(r phasecontract.Resolver) {
	if r == nil {
		r = phasecontract.BuiltinResolver{}
	}
	a.resolver = r
}

func (a *Adapter) SetPhaseIOStage(stage config.Stage) {
	a.phaseIO = stage
}

func (a *Adapter) SetRecoveryStage(stage string) {
	a.recoveryStage = stage
}

func (a *Adapter) SetFatalPaneStage(stage string) {
	a.fatalPaneStage = stage
}

func (a *Adapter) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := validate(req); err != nil {
		return core.BridgeResponse{}, err
	}
	inproc := req
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

func (a *Adapter) Probe(ctx context.Context) (core.BridgeProbe, error) {
	return a.engineFactory(nil).Probe(ctx)
}

func validate(req core.BridgeRequest) error { return gobridge.ValidateRequest(req) }

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

func (a *Adapter) injectContract(prompt, contractID, artifactPath, workspace string) string {
	resolver := a.contractResolver()
	c, ok := resolver.Resolve(contractID)
	if !ok {
		if artifactPath == "" {
			return prompt
		}
		c = phasecontract.Contract{Phase: contractID, AgentName: contractID, ArtifactName: filepath.Base(artifactPath)}
		return prompt + phasecontract.RenderContractFooter(c, artifactPath)
	}
	includePhaseIO := a.phaseIO >= config.StageAdvisory
	return phasecontract.RenderContractBlockStage(c, includePhaseIO) + prompt + phasecontract.RenderContractTail(c, artifactPath, workspace)
}

func injectRulesPrefix(prompt, rules string) string {
	if rules == "" {
		return prompt
	}
	return "## Rules\n\n" + rules + "\n\n---\n\n" + prompt
}

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

func injectCorrectionPrefix(prompt, directive string) string {
	if directive == "" {
		return prompt
	}
	return "## Correction\n\n" + directive + "\n\n---\n\n" + prompt
}

func injectOperatorDirectives(prompt, directives string) string {
	if directives == "" {
		return prompt
	}
	return directives + "\n\n---\n\n" + prompt
}

func SetModelCatalogDirFn(fn func() string) {
	gobridge.SetModelCatalogDirFn(fn)
}

func (a *Adapter) SignalsWired() bool { return a.signals != nil }

func (a *Adapter) Signals() *signalcenter.Center {
	if a == nil {
		return nil
	}
	return a.signals
}
