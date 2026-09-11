package runner

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
)

// phaseDispatchPlan contains the resolved, immutable inputs shared by every
// fallback attempt in one phase execution.
type phaseDispatchPlan struct {
	plan              llmroute.Plan
	overlayPolicy     policy.Policy
	modelSource       string
	permissionMode    string
	interactivePolicy string
	systemPrompt      string
}

func (b *BaseRunner) resolveDispatchPlan(req core.PhaseRequest, prep phasePreparation) (phaseDispatchPlan, *core.PhaseResponse, error) {
	resolved := phaseDispatchPlan{}
	var pin *policy.Pin
	if !req.BypassPolicy {
		loaded, err := policy.Load(filepath.Join(req.ProjectRoot, ".evolve", "policy.json"))
		if err != nil {
			resp := core.PhaseResponse{
				Phase: prep.phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
				Diagnostics: []core.Diagnostic{{Severity: "error", Message: err.Error()}},
			}
			return resolved, &resp, fmt.Errorf("%s: %w", prep.phase, err)
		}
		resolved.overlayPolicy = loaded
		if phasePin, ok := loaded.PinFor(prep.phase); ok {
			if err := policy.ValidatePin(prep.phase, phasePin, prep.profile); err != nil {
				resp := core.PhaseResponse{
					Phase: prep.phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
					Diagnostics: []core.Diagnostic{{Severity: "error", Message: err.Error()}},
				}
				return resolved, &resp, fmt.Errorf("%s: %w", prep.phase, err)
			}
			pin = &phasePin
			log.Diag().Infof("[runner] phase=%s policy pin: cli=%q model=%q\n", prep.phase, phasePin.CLI, phasePin.Model)
		}
	}

	autoExpand := func(role string) (string, bool) {
		result, err := b.resolveLLM(role, resolvellm.Options{})
		if err != nil || result.ModelTier == "" {
			return "", false
		}
		return result.ModelTier, true
	}
	resolved.plan = llmroute.Resolve(prep.profileName, prep.phase, b.hooks.DefaultModel(), req.Env, prep.profile, autoExpand, pin)

	overlayProposed := req.ModelRoutingCLI != "" || req.ModelRoutingTier != ""
	resolved.modelSource = "profile"
	switch {
	case pin != nil:
		resolved.modelSource = "pin"
	case overlayProposed:
		resolved.modelSource = "advisor"
	}
	if pin == nil && overlayProposed {
		resolved.plan = llmroute.ApplySoftOverlay(resolved.plan, llmroute.Overlay{CLI: req.ModelRoutingCLI, Tier: req.ModelRoutingTier}, prep.profile)
		b.diag.Infof("[runner] phase=%s advisor overlay cli=%s tier=%s\n", prep.phase, req.ModelRoutingCLI, req.ModelRoutingTier)
	} else if pin == nil {
		b.diag.Infof("[runner] phase=%s no advisor overlay (profile default)\n", prep.phase)
	}

	if pin == nil || pin.CLI == "" {
		before := resolved.plan.Candidates
		resolved.plan = llmroute.Probe(resolved.plan, nil)
		if !sameCandidates(before, resolved.plan.Candidates) {
			log.Diag().Infof("[runner] phase=%s capability probe reordered chain: %v -> %v\n",
				prep.phase, before, resolved.plan.Candidates)
		}
	}
	resolved.plan = b.applyBenchToPlan(req.ProjectRoot, prep.phase, resolved.plan, pin != nil && pin.CLI != "", req.Env)
	if (pin == nil || pin.CLI == "") && b.universalFallback && b.discoverCLIsFn != nil {
		discovered := allowedDiscovered(b.discoverCLIsFn(), prep.profile)
		before := resolved.plan.Candidates
		resolved.plan = llmroute.ApplyUniversalFallback(resolved.plan, discovered, nil)
		if !sameCandidates(before, resolved.plan.Candidates) {
			log.Diag().Infof("[runner] phase=%s UNIVERSAL-FALLBACK: configured chain %v all absent on this host — discovered+allowed CLIs appended -> %v\n",
				prep.phase, before, resolved.plan.Candidates)
		}
	}

	primaryCLI := resolved.plan.Candidates[0]
	if len(resolved.plan.Candidates) > 1 {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s fallback=%v triggers=%v\n",
			prep.phase, prep.profileName, primaryCLI, resolved.plan.PrimarySource, prep.profilePath,
			resolved.plan.Candidates[1:], resolved.plan.Triggers)
	} else {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s\n",
			prep.phase, prep.profileName, primaryCLI, resolved.plan.PrimarySource, prep.profilePath)
	}

	resolved.permissionMode = req.Env[envchain.PhaseEnvKey(prep.profileName, "PERMISSION_MODE")]
	if resolved.permissionMode == "" && prep.profile != nil {
		resolved.permissionMode = prep.profile.PermissionMode
	}
	if prep.profile != nil {
		resolved.interactivePolicy = prep.profile.InteractivePolicy
	}
	resolved.systemPrompt = systemprompt.Resolve(prep.profileName, prep.profileDir, req.Env)
	return resolved, nil, nil
}
