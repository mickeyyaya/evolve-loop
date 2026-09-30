package releasepreflight

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

type resolved struct {
	target         string
	repoRoot       string
	pluginJSONPath string
	ledgerPath     string

	dryRun     bool
	skipTests  bool
	strictPass bool
	allowRedCI bool

	now              func() time.Time
	gitClean         func(string) (bool, error)
	currentBranch    func(string) (string, error)
	gateRunner       func(string, string) error
	nameGuard        func(string) ([]naminguard.Violation, error)
	simulationRunner func(string) error
	ciConclusion     func(string) (CIRunStatus, error)
	headSHA          func(string) (string, error)
}

func resolve(opts Options) (resolved, error) {
	if opts.RepoRoot == "" {
		return resolved{}, fmt.Errorf("%w: RepoRoot required", ErrCheckFailed)
	}
	r := resolved{
		target:           opts.Target,
		repoRoot:         opts.RepoRoot,
		pluginJSONPath:   opts.PluginJSONPath,
		ledgerPath:       opts.LedgerPath,
		dryRun:           opts.DryRun,
		skipTests:        opts.SkipTests,
		strictPass:       opts.StrictPass,
		allowRedCI:       opts.AllowRedCI,
		now:              opts.Now,
		gitClean:         opts.GitClean,
		currentBranch:    opts.CurrentBranch,
		gateRunner:       opts.GateTestRunner,
		nameGuard:        opts.NameGuard,
		simulationRunner: opts.SimulationRunner,
		ciConclusion:     opts.CIConclusion,
		headSHA:          opts.HeadSHA,
	}
	r.applyDefaults()
	return r, nil
}

func (r *resolved) applyDefaults() {
	if r.pluginJSONPath == "" {
		r.pluginJSONPath = filepath.Join(r.repoRoot, ".claude-plugin", "plugin.json")
	}
	if r.ledgerPath == "" {
		r.ledgerPath = filepath.Join(r.repoRoot, ".evolve", "ledger.jsonl")
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.gitClean == nil {
		r.gitClean = defaultGitClean
	}
	if r.currentBranch == nil {
		r.currentBranch = defaultCurrentBranch
	}
	if r.gateRunner == nil {
		r.gateRunner = defaultGateTestRunner
	}
	if r.nameGuard == nil {
		r.nameGuard = defaultNameGuard
	}
	if r.simulationRunner == nil {
		r.simulationRunner = defaultSimulationRunner
	}
	if r.ciConclusion == nil {
		r.ciConclusion = defaultCIConclusion
	}
	if r.headSHA == nil {
		r.headSHA = defaultHeadSHA
	}
}

type preflightRun struct {
	o    resolved
	res  Result
	logf func(string, ...any)
}

func newPreflightRun(o resolved, stderr io.Writer) *preflightRun {
	if stderr == nil {
		stderr = io.Discard
	}
	return &preflightRun{
		o:   o,
		res: Result{StepsTotal: len(preflightSteps)},
		logf: func(format string, args ...any) {
			fmt.Fprintf(stderr, "[preflight] "+format+"\n", args...)
		},
	}
}

var preflightSteps = []func(*preflightRun) error{
	(*preflightRun).stepTreeClean,
	(*preflightRun).stepBranchAttached,
	(*preflightRun).stepSemverBump,
	(*preflightRun).stepRecentAudit,
	(*preflightRun).stepGateSuites,
}

func (p *preflightRun) stepTreeClean() error {
	p.logf("step 1: working tree clean?")
	if p.o.dryRun {
		p.logf("DRY-RUN: would check git diff --quiet HEAD")
		return nil
	}
	clean, err := p.o.gitClean(p.o.repoRoot)
	if err != nil {
		return fmt.Errorf("%w: step 1 git error: %v", ErrCheckFailed, err)
	}
	if !clean {
		return fmt.Errorf("%w: working tree has uncommitted changes — commit or stash first", ErrCheckFailed)
	}
	p.logf("OK: working tree clean")
	return nil
}

func (p *preflightRun) stepBranchAttached() error {
	p.logf("step 2: branch attached?")
	if p.o.dryRun {
		p.logf("DRY-RUN: would check git symbolic-ref --short HEAD")
		return nil
	}
	branch, err := p.o.currentBranch(p.o.repoRoot)
	if err != nil {
		return fmt.Errorf("%w: step 2 git error: %v", ErrCheckFailed, err)
	}
	if branch == detachedHEADBranch {
		return fmt.Errorf("%w: detached HEAD — checkout a branch first", ErrCheckFailed)
	}
	p.logf("OK: on branch %s", branch)
	return nil
}

func (p *preflightRun) stepSemverBump() error {
	p.logf("step 3: target version %s > current?", p.o.target)
	if _, _, _, ok := ParseSemver(p.o.target); !ok {
		return fmt.Errorf("%w: target version not semver: %s", ErrCheckFailed, p.o.target)
	}
	if _, err := os.Stat(p.o.pluginJSONPath); err != nil {
		return fmt.Errorf("%w: plugin.json missing at %s", ErrCheckFailed, p.o.pluginJSONPath)
	}
	current, err := ExtractJSONVersion(p.o.pluginJSONPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCheckFailed, err)
	}
	p.res.CurrentVersion = current
	if _, _, _, ok := ParseSemver(current); !ok {
		return fmt.Errorf("%w: current plugin.json version not semver: %s", ErrCheckFailed, current)
	}
	if p.o.target == current {
		return fmt.Errorf("%w: target %s equals current %s — nothing to bump", ErrCheckFailed, p.o.target, current)
	}
	if !SemverGT(p.o.target, current) {
		return fmt.Errorf("%w: target %s is not greater than current %s", ErrCheckFailed, p.o.target, current)
	}
	p.logf("OK: %s → %s (valid bump)", current, p.o.target)
	return nil
}

func (p *preflightRun) stepRecentAudit() error {
	p.logf("step 4: recent auditor PASS verdict?")
	if p.o.dryRun {
		p.logf("DRY-RUN: would check %s for recent auditor PASS", p.o.ledgerPath)
		return nil
	}
	releaseHead, headErr := p.o.headSHA(p.o.repoRoot)
	if headErr != nil {
		p.logf("advisory: could not resolve the release commit (%v) — a failing audit will be treated as blocking", headErr)
		releaseHead = ""
	}
	auditRes, err := checkRecentAudit(p.o.ledgerPath, releaseHead, p.o.strictPass, p.o.now())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCheckFailed, err)
	}
	p.res.AuditArtifact = auditRes.artifact
	p.res.AuditVerdict = auditRes.verdict
	p.res.AuditAge = auditRes.age
	p.res.PhantomEntries = auditRes.phantomCount
	switch auditRes.verdict {
	case auditVerdictScopedOut:
		p.logf("advisory: the most recent audit (%s) did not pass, but it did not audit this release commit's tree (audited %s, releasing %s) — CI-green on the release commit is the authoritative gate (/publish). Not treating it as a veto.",
			auditRes.artifact, shortSHA(auditRes.auditedHead), shortSHA(releaseHead))
	case auditVerdictNone:
		p.logf("advisory: no on-disk audit in this worktree — CI-green on the release commit is the authoritative gate (/publish). Skipping the audit-PASS check.")
	default:
		if auditRes.phantomCount > 0 {
			p.logf("WARN: skipped %d phantom auditor entry/entries (artifact missing on disk). Using most-recent VALID entry.", auditRes.phantomCount)
		}
		if auditRes.verdict == "WARN" {
			p.logf("INFO: most recent audit is WARN (fluent posture; ships by default). Set EVOLVE_RELEASE_STRICT_PASS=1 for strict-PASS gate.")
		}
		p.logf("OK: latest audit %s, artifact=%s", auditRes.verdict, auditRes.artifact)
	}
	return nil
}

func (p *preflightRun) stepGateSuites() error {
	p.logf("step 5: gate-test suites green?")
	if p.o.skipTests {
		p.logf("WARN: --skip-tests set; skipping gate-test execution")
		return nil
	}
	if p.o.dryRun {
		p.logf("DRY-RUN: would run %d gate-test suites", len(DefaultGateTestSuites))
		return nil
	}
	for _, suite := range DefaultGateTestSuites {
		p.logf("  running %s...", suite)
		if err := p.o.gateRunner(p.o.repoRoot, suite); err != nil {
			return fmt.Errorf("%w: gate-test suite failed: %s — re-run interactively to inspect (%v)",
				ErrCheckFailed, suite, err)
		}
		p.res.GateTestsPassed++
	}
	p.logf("OK: all %d gate-test suites green", len(DefaultGateTestSuites))

	p.logf("  scanning for dead naming tokens (.evolve/naming.json)...")
	vs, err := p.o.nameGuard(p.o.repoRoot)
	if err != nil {
		return fmt.Errorf("%w: naming guard error: %v", ErrCheckFailed, err)
	}
	if len(vs) > 0 {
		return fmt.Errorf("%w: %d dead naming token(s) in tracked files — run `evolve names fix`: %s",
			ErrCheckFailed, len(vs), vs[0])
	}
	p.logf("OK: no dead naming tokens")
	return nil
}

func (p *preflightRun) gateReleaseCommitCI() error {
	p.logf("release-commit CI conclusion green?")
	if p.o.dryRun {
		p.logf("DRY-RUN: would check the remote CI conclusion for HEAD")
		return nil
	}
	ci, err := p.o.ciConclusion(p.o.repoRoot)
	if err != nil {
		return fmt.Errorf("%w: release-commit CI check error: %v", ErrCheckFailed, err)
	}
	p.res.CIConclusion = ci.Conclusion
	switch {
	case ci.Conclusion == ciConclusionUnavailable:
		p.logf("advisory: remote CI conclusion unavailable (no run visible / gh unavailable) — /publish's CI-green check remains authoritative")
	case ci.Conclusion == "success":
		p.logf("OK: release-commit CI conclusion=success %s", ci.RunURL)
	case p.o.allowRedCI:
		p.res.CIOverridden = true
		p.logf("OVERRIDE: release-commit CI conclusion is %q (run %s) — NOT green", ci.Conclusion, ci.RunURL)
		p.logf("OVERRIDE: proceeding ONLY because --allow-red-ci was explicitly passed; this release ships without a green remote CI verdict")
	default:
		return fmt.Errorf("%w: release-commit CI conclusion is %q, not success (run %s) — fix CI or pass --allow-red-ci to override",
			ErrCheckFailed, ci.Conclusion, ci.RunURL)
	}
	return nil
}

func (p *preflightRun) adviseSimulation() {
	if p.o.skipTests {
		p.logf("advisory: auto-respond simulation suite — skipped (--skip-tests)")
		return
	}
	if p.o.dryRun {
		p.logf("advisory: auto-respond simulation suite — skipped (dry-run)")
		return
	}
	p.logf("advisory: running auto-respond simulation suite...")
	if err := p.o.simulationRunner(p.o.repoRoot); err != nil {
		f := false
		p.res.SimulationAdvisoryOK = &f
		p.logf("WARN: auto-respond simulation suite failed (advisory in v12.1.5; required in v12.2.0): %v", err)
		return
	}
	t := true
	p.res.SimulationAdvisoryOK = &t
	p.logf("OK: auto-respond simulation suite passed")
}

func (p *preflightRun) logDone() {
	dryRunSuffix := ""
	if p.o.dryRun {
		dryRunSuffix = " (dry-run)"
	}
	p.logf("DONE: preflight passed for %s%s", p.o.target, dryRunSuffix)
}
