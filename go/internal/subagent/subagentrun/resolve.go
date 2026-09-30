package subagentrun

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli"
)

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

func legacyAdapterPath(adaptersDir, cli string) string {
	return filepath.Join(adaptersDir, cli+".sh")
}

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
