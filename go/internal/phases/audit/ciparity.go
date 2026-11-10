package audit

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var runCmd sysexec.RunFunc = sysexec.DefaultRunner

var integrationTierTimeout = ciparitygate.DefaultTimeouts().TierAttempt

type ciParity struct{ signals func() *signalcenter.Center }

var baseRedRunner = func() baseattr.Runner { return baseattr.GoTestRunner(ciparitygate.TierEnv()) }

func (c ciParity) wiredGates(req core.PhaseRequest) *ciparitygate.Gates {
	t := ciparitygate.DefaultTimeouts()
	t.TierAttempt = integrationTierTimeout
	opts := []ciparitygate.Option{ciparitygate.WithTimeouts(t), ciparitygate.WithSignals(c.signals)}
	if req.WorktreeBaseSHA != "" {
		opts = append(opts, ciparitygate.WithAttribution(baseRedAttribution(req)))
	}
	return ciparitygate.New(runCmd, changedPackagesForAudit, opts...)
}

func baseRedAttribution(req core.PhaseRequest) ciparitygate.Attribution {
	return func(ctx context.Context, r baseattr.Reds) (baseattr.Verdict, error) {
		root := cmp.Or(req.ProjectRoot, req.Worktree)
		return core.BaseRedAttributor(root, baseRedRunner(), os.Stderr).Attribute(ctx, req.WorktreeBaseSHA, r)
	}
}

func requestOf(req core.PhaseRequest) ciparitygate.Request {
	return ciparitygate.Request{Cycle: req.Cycle, ProjectRoot: req.ProjectRoot, Worktree: req.Worktree, Workspace: req.Workspace}
}

func (c ciParity) goVet(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates(req).GoVet(requestOf(req))
}

func (c ciParity) acsDurable(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates(req).ACSDurable(requestOf(req))
}

func (c ciParity) integrationTier(req core.PhaseRequest) ([]string, error) {
	offenders, err := c.wiredGates(req).IntegrationTier(requestOf(req))
	var notice ciparitygate.BaseRed
	if errors.As(err, &notice) {
		return nil, gateWarning(notice)
	}
	return offenders, err
}

func (c ciParity) apicoverEnforce(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates(req).ApicoverEnforce(requestOf(req))
}

func (c ciParity) apicoverGraduation(req core.PhaseRequest) ([]string, error) {
	return c.wiredGates(req).ApicoverGraduation(requestOf(req))
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
