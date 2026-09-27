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

// runCmd is the subprocess runner the CI-parity gates and the probe
// quarantine use. A package var so tests can inject a fake runner and
// exercise the gates without forking the real go toolchain.
var runCmd sysexec.RunFunc = sysexec.DefaultRunner

// integrationTierTimeout bounds EACH integration-tier attempt (first run and
// serialized retake separately) — a PROJECTION of the leaf's default, kept as
// a var so the orchestration tests can shrink it through the production
// Classify path without a 15-minute wait.
var integrationTierTimeout = ciparitygate.DefaultTimeouts().TierAttempt

// ciParity is the per-Phase adapter onto the gates; the Center accessor is
// its only state (nil = the registry path's Null Object).
type ciParity struct{ signals func() *signalcenter.Center }

// wiredGates is the one construction of the gates
// (TestCIParityGates_OneConstructionSite), built per call so runCmd and
// integrationTierTimeout are read live rather than cached at entry.
func (c ciParity) wiredGates() *ciparitygate.Gates {
	t := ciparitygate.DefaultTimeouts()
	t.TierAttempt = integrationTierTimeout
	return ciparitygate.New(runCmd, changedPackagesForAudit, ciparitygate.WithTimeouts(t), ciparitygate.WithSignals(c.signals))
}

// requestOf is the ONE projection from the phase request onto the gates'
// input: the cycle, the runtime root, the shipped tree, the workspace.
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

// wire fills the five CI-parity hooks with the adapter's methods, only the
// hooks no Option already set, so a fake hook injected through the production
// constructor is never post-clobbered
// (TestNewDefaultWithStageCompactSpec_AnOptionSettingAHookIsHonoured).
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

// The five Strangler facades are the spellings the host tests and the by-name
// ACS predicates keep. Center-less (Null Object) and guarded to have no
// non-test caller (TestCIParityGates_OneConstructionSite).
//
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

// changedPackagesForAudit locates this cycle's changed-package set and
// reports whether it is derivable. It prefers the build handoff when present,
// then falls back to a deterministic git derivation. The derivable flag tells
// a git-clean tree (nothing changed) apart from an underivable set (git
// failed), so an underivable cycle cannot ship a silent PASS.
func changedPackagesForAudit(projectRoot string, cycle int) ([]string, bool) {
	if projectRoot == "" {
		return nil, false
	}
	dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		if pkgs := changedpkgs.ChangedPackages(filepath.Join(dir, name)); len(pkgs) > 0 {
			return pkgs, true // handoff present and non-empty → derivable
		}
	}
	return changedpkgs.FromGitChecked(projectRoot, "HEAD")
}
