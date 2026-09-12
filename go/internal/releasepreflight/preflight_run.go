package releasepreflight

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

// preflight_run.go holds Run's execution shape: the resolved seam snapshot,
// the parameter object each step mutates, and the ordered table those steps
// are driven from. Run itself stays in releasepreflight.go as the package's
// public facade; this file is what it delegates to.
//
// Before this, Run was one 252-line procedure carrying eight seams (four
// resolved eagerly at the top, four lazily mid-function), seven separate
// `opts.DryRun` reads, and seven scattered `res.StepsPassed++` sites. Step
// ORDER — which failure surfaces first when several would fail — was
// structural, expressible only by reading the whole body top to bottom.
// It is now a table a test asserts equality against (see
// TestRun_FirstFailingStepWins).
//
// Shape notes, so a later reader does not "tidy" away something load-bearing:
//   - The CI hard-gate and the advisory simulation are explicit calls AFTER
//     the table, not entries in it. Both are documented as not counting
//     toward StepsPassed/StepsTotal (the 5-step back-compat contract), and
//     folding them in would need a per-entry "counted" flag — a policy bit
//     for two cases, which is how a named stage list turns into a generic
//     middleware framework.
//   - Every step keeps its own log lines and its own dry-run branch inline.
//     Step 3 deliberately has NO dry-run branch (pinned by
//     TestRun_DryRun_SemverBumpStillEnforced).
//   - Error strings are reproduced verbatim: the existing suite asserts them
//     by substring, so their wording is a de-facto contract.

// resolved is Options with every path default and all eight seams filled in
// once, at the outer boundary. Resolution ASSIGNS func values and never
// invokes them, so a nil seam under DryRun/SkipTests is still never called —
// the contract TestRun_DryRunWithNilSeams pins.
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

// resolve fills in Options' defaults. The RepoRoot check is the one failure
// that can happen before any step runs.
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
	return r, nil
}

// preflightRun is one Run's resolved inputs, its log sink, and the Result it
// is populating. Step methods mutate res and return the ErrCheckFailed-wrapped
// error Run surfaces unchanged.
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

// preflightSteps is the COUNTED, ordered contract: StepsTotal is its length
// and StepsPassed advances once per step that returns nil. Order is the
// contract TestRun_FirstFailingStepWins asserts; reordering these entries
// changes which failure an operator sees first.
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
	if branch == "" {
		return fmt.Errorf("%w: detached HEAD — checkout a branch first", ErrCheckFailed)
	}
	p.logf("OK: on branch %s", branch)
	return nil
}

// stepSemverBump deliberately has NO dry-run branch: a dry run still reads
// plugin.json and still rejects an invalid bump, so an operator learns about
// a bad target before anything else runs. Pinned by
// TestRun_DryRun_SemverBumpStillEnforced.
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
	// Resolution failure is not fatal: an empty head simply keeps step 4's
	// conservative branch, which is the same posture as before this scoping.
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
		// An audit exists and did not pass, but it did not examine this
		// release commit's committed tree. Name the artifact and both
		// commits: reporting it as "no audit" would misdirect an operator
		// debugging a blocked release toward a missing-artifact hunt.
		p.logf("advisory: the most recent audit (%s) did not pass, but it did not audit this release commit's tree (audited %s, releasing %s) — CI-green on the release commit is the authoritative gate (/publish). Not treating it as a veto.",
			auditRes.artifact, shortSHA(auditRes.auditedHead), shortSHA(releaseHead))
	case auditVerdictNone:
		// Determinism: no on-disk audit available in this worktree (clean
		// checkout / CI / GC'd artifacts). The authoritative release gate is
		// CI-green on the release commit (enforced by /publish) — advisory only.
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

// stepGateSuites runs the gate-test suites and, on the real path only, the
// naming sub-check. The step counts as passed only after BOTH come back
// clean — the same semantics as the three separate StepsPassed++ sites it
// replaces (skip-tests, dry-run, and the end of the real branch).
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

	// Step 5 sub-check: no dead naming tokens survive in tracked files.
	// Shares the legacynames acs gate's scanner + SSOT (.evolve/naming.json),
	// so a release can't ship a rename that left a 404 slug / dead command
	// behind. No-ops when the repo has no manifest.
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

// gateReleaseCommitCI is the release-commit CI hard-gate (cycle-748,
// push-ci-watch-remote-parity): the remote go CI run for HEAD must be
// conclusion=success before tagging (v22.0.0 was cut on red CI). It does NOT
// count toward StepsPassed/StepsTotal (back-compat with the 5-step
// contract), which is why it is called after the table rather than listed in
// it. An UNAVAILABLE verdict (no repo, gh missing, no run visible) is
// advisory-skipped — same determinism rule as auditVerdictNone — but a
// present non-success verdict hard-fails unless AllowRedCI is explicitly set,
// and an override is always logged loudly and recorded in Result.CIOverridden.
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
	case ci.Conclusion == "":
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

// adviseSimulation is the advisory auto-respond simulation suite (v12.1.5+).
// It does NOT count toward StepsPassed/StepsTotal and never returns
// ErrCheckFailed — a failure is logged as WARN and recorded in the tri-state
// Result.SimulationAdvisoryOK (nil = not run). Promotes to a required step in
// v12.2.0, at which point it becomes a preflightSteps entry.
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
