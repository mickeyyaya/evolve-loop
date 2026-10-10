package audit

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var runCmd sysexec.RunFunc = sysexec.DefaultRunner

var integrationTierTimeout = ciparitygate.DefaultTimeouts().TierAttempt

type ciParity struct{ signals func() *signalcenter.Center }

func (c ciParity) wiredGates() *ciparitygate.Gates {
	t := ciparitygate.DefaultTimeouts()
	t.TierAttempt = integrationTierTimeout
	return ciparitygate.New(runCmd, changedPackagesForAudit, ciparitygate.WithTimeouts(t), ciparitygate.WithSignals(c.signals))
}

func requestOf(req core.PhaseRequest) ciparitygate.Request {
	return ciparitygate.Request{Cycle: req.Cycle, ProjectRoot: req.ProjectRoot, Worktree: req.Worktree, Workspace: req.Workspace}
}

func (c ciParity) goVet(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates().GoVet(requestOf(req))
}

func (c ciParity) acsDurable(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates().ACSDurable(requestOf(req))
}

func (c ciParity) integrationTier(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates().IntegrationTier(requestOf(req))
}

func (c ciParity) apicoverEnforce(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates().ApicoverEnforce(requestOf(req))
}

func (c ciParity) apicoverGraduation(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates().ApicoverGraduation(requestOf(req))
}

func (c ciParity) wire(cfg *Config) {
	fill := func(hook *func(core.PhaseRequest) ([]string, error), gate func(core.PhaseRequest) ([]string, error)) {
		if *hook == nil {
			*hook = gate
		}
	}
	fill(&cfg.CheckGoVet, c.goVet)
	fill(&cfg.CheckACSDurable, c.acsDurable)
	fill(&cfg.CheckIntegrationTier, c.integrationTier)
	fill(&cfg.CheckApicoverEnforce, c.apicoverEnforce)
	fill(&cfg.CheckApicoverNewPkgGraduation, c.apicoverGraduation)
}

// Deprecated: removed when the ACS predicates are re-pointed at the leaf
// (follow-up 14-3); production wires ciParity's methods.
func goVetCheckDefault(req core.PhaseRequest) ([]string, error) { return ciParity{}.goVet(req) }

func acsDurableCheckDefault(req core.PhaseRequest) ([]string, error) {
	return ciParity{}.acsDurable(req)
}

func integrationTierCheckDefault(req core.PhaseRequest) ([]string, error) {
	return ciParity{}.integrationTier(req)
}

func apicoverEnforceChangedDefault(req core.PhaseRequest) ([]string, error) {
	return ciParity{}.apicoverEnforce(req)
}

func apicoverNewPackageGraduationDefault(req core.PhaseRequest) ([]string, error) {
	return ciParity{}.apicoverGraduation(req)
}

func changedPackagesForAudit(projectRoot string, cycle int) ([]string, bool) {
	if projectRoot == "" {
		return nil, false
	}
	dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		if pkgs := changedpkgs.ChangedPackages(filepath.Join(dir, name)); len(pkgs) > 0 {
			return pkgs, true
		}
	}
	return changedpkgs.FromGitChecked(projectRoot, "HEAD")
}
