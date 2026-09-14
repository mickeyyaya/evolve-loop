package subagentrun

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli"
)

// resolve is steps 2-7: the profile, the cli (router first, profile
// fallback), the driver check, the model tier and the capability manifest,
// over the complete identity admission returned. Every failure is ONE
// BRIDGE_SUBAGENT_RESOLUTION_FAILED whose step names the port; the profile
// step's reason carries the read error the returned text (the host's,
// verbatim) drops.
func (d *Dispatcher) resolve(req Request, id identity) (plan, error) {
	p := plan{profilePath: filepath.Join(req.ProfilesDir, id.role+".json")}
	profile, err := d.deps.Profile(p.profilePath)
	if err != nil {
		d.warn(id, req.Cycle, CodeResolutionFailed, "profile not found: "+p.profilePath+": "+err.Error(),
			map[string]string{"step": "profile", "path": p.profilePath})
		return plan{}, fmt.Errorf("subagent/run: profile not found: %s", p.profilePath)
	}
	p.profile = profile
	for _, step := range []func(Request, identity, *plan) error{d.resolveCLI, d.resolveTier, d.resolveCapability} {
		if err := step(req, id, &p); err != nil {
			return plan{}, err
		}
	}
	return p, nil
}

// resolveCLI is steps 3 and 5: the router's cli wins when it returned one
// (its tier rides along); otherwise the profile's cli field with
// source=profile — a router ERROR is the BRIDGE_SUBAGENT_LLM_RESOLVE_FALLBACK
// (a nil error with an empty cli is the designed "profile decides" and stays
// silent). Both paths canonicalise (antigravity → agy). The driver check
// receives the cli; the vestigial <AdaptersDir>/<cli>.sh path is composed
// only for the rejection's text and its signal's `path`.
func (d *Dispatcher) resolveCLI(req Request, id identity, p *plan) error {
	llm, llmErr := d.deps.ResolveLLM(id.role)
	if llmErr == nil && llm.CLI != "" {
		p.cli, p.source, p.model = llm.CLI, llm.Source, llm.ModelTier
	} else {
		p.cli, p.source = p.profile.CLI, "profile"
	}
	p.cli = detectcli.Canonical(p.cli)
	if llmErr != nil {
		d.warn(id, req.Cycle, CodeLLMResolveFallback, "llm resolver failed for "+id.role+"; cli taken from the profile: "+llmErr.Error(),
			map[string]string{"step": "cli", "source": p.source, "cli": p.cli})
	}
	if p.cli == "" {
		d.warn(id, req.Cycle, CodeResolutionFailed, "cli unresolved for agent "+req.Agent, map[string]string{"step": "cli", "path": p.profilePath})
		return fmt.Errorf("subagent/run: cli unresolved for agent %s", req.Agent)
	}
	p.adapterPath = legacyAdapterPath(req.AdaptersDir, p.cli)
	if !d.deps.AdapterExists(p.cli) {
		d.warn(id, req.Cycle, CodeResolutionFailed, "adapter not executable: "+p.adapterPath,
			map[string]string{"step": "driver", "path": p.adapterPath, "cli": p.cli})
		return fmt.Errorf("subagent/run: adapter not executable: %s", p.adapterPath)
	}
	return nil
}

// legacyAdapterPath is the <AdaptersDir>/<cli>.sh spelling the driver check
// and its error text kept from the bash adapter era — nobody opens the file.
func legacyAdapterPath(adaptersDir, cli string) string {
	return filepath.Join(adaptersDir, cli+".sh")
}

// resolveTier is step 6: a tier the router resolved wins; otherwise the
// adaptive resolver evaluates profile + mastery gate + diff complexity.
func (d *Dispatcher) resolveTier(req Request, id identity, p *plan) error {
	if p.model != "" {
		return nil
	}
	model, err := d.deps.ResolveTier(TierRequest{
		ProfilePath:            p.profilePath,
		Cycle:                  req.Cycle,
		ProjectRoot:            req.ProjectRoot,
		WorktreePath:           req.WorktreePath,
		ModelTierHint:          req.ModelTierHint,
		AuditorTierOverride:    req.AuditorTierOverride,
		DiffComplexityDisabled: req.DiffComplexityDisabled,
	})
	if err != nil {
		d.warn(id, req.Cycle, CodeResolutionFailed, "resolve tier: "+err.Error(),
			map[string]string{"step": "tier", "path": p.profilePath, "cli": p.cli})
		return fmt.Errorf("subagent/run: resolve tier: %w", err)
	}
	p.model = model
	return nil
}

// resolveCapability is step 7: the manifest under CapabilityDir (AdaptersDir
// when unset) — its warns ride the Warns channel, its flags the env and the
// ledger's quality_tier.
func (d *Dispatcher) resolveCapability(req Request, id identity, p *plan) error {
	capDir := req.CapabilityDir
	if capDir == "" {
		capDir = req.AdaptersDir
	}
	c, err := d.deps.Inspect(capDir, p.cli)
	if err != nil {
		d.warn(id, req.Cycle, CodeResolutionFailed, "capability inspect: "+err.Error(),
			map[string]string{"step": "capability", "path": capDir, "cli": p.cli})
		return fmt.Errorf("subagent/run: capability inspect: %w", err)
	}
	p.cap = c
	return nil
}
