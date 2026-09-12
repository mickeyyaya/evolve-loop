package releasepipeline

import (
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
)

// release_run.go holds Run's execution shape: the per-run parameter object,
// the helper the six pre-publish steps share, and the three stages Run drives.
// Run itself stays in releasepipeline.go as the package's public facade.
//
// Before this, Run was one 206-line procedure in which the six pre-publish
// steps repeated the same six-line shape verbatim — log, call, on error
// journal+record+wrap-and-return, on success journal+record — with only the
// names and the wrap label varying.
//
// The helper below keeps BOTH names explicit, because they are not the same
// name and collapsing them would silently rename half the operator-facing
// surface: two of the six steps deliberately journal under one name and
// report under another ("full-dry-run-preflight" vs "full-dry-run preflight",
// "release-sh-check" vs "release.sh consistency"). Those strings are contract
// — the CLI prints them with %v and they are an operator's only clue to which
// step broke — so the divergence is pinned by exact equality in
// TestRun_PrePublishStepFailure_WrapsLabelAndJournals.
//
// Ship and post-publish stay their own methods rather than table entries:
// each carries a DIFFERENT sentinel (ErrShipFailed, ErrPostPublishFailed vs
// the pre-publish ErrPrePublishFailed), and the CLI maps those three to exit
// codes 2, 3 and 1. A single table would need a per-entry sentinel field,
// which is how a named stage list turns into a generic dispatcher.

// releaseRun is one Run's resolved inputs, its journal handle, and the Result
// it is populating.
type releaseRun struct {
	opts        Options
	steps       Steps
	now         func() time.Time
	logf        func(string, ...any)
	fromTag     string
	journal     *Journal
	journalPath string
	res         Result
}

// newReleaseRun performs Run's setup in its original order: validate the
// target, resolve the changelog range, resolve the clock, overlay step
// defaults, log the banner, and open the journal. The Result it carries is
// returned to the caller even on the two setup failures, which is why those
// return the object rather than nil.
func newReleaseRun(opts Options) (*releaseRun, error) {
	r := &releaseRun{opts: opts, res: Result{Target: opts.Target}}

	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}
	r.logf = func(format string, args ...any) {
		fmt.Fprintf(logw, "[release-pipeline] "+format+"\n", args...)
	}

	// Argument validation (semver target).
	if !semvercheck.IsSemver(opts.Target) {
		return r, fmt.Errorf("%w: target version not semver: %s", ErrPrePublishFailed, opts.Target)
	}

	// Resolve FromTag if not provided.
	r.fromTag = opts.FromTag
	if r.fromTag == "" {
		if t, err := resolvePrevTag(opts.RepoRoot); err == nil && t != "" {
			r.fromTag = t
		} else {
			r.logf("WARN: no previous tag found; changelog range will start from initial commit")
			if init, err := resolveInitCommit(opts.RepoRoot); err == nil {
				r.fromTag = init
			}
		}
	}

	// Resolve now seam.
	r.now = opts.Now
	if r.now == nil {
		r.now = time.Now
	}

	// Steps defaults: only overlay missing fields.
	r.steps = applyDefaultSteps(opts.Steps)
	// Thread StrictPass into the default preflight path. When the caller
	// provides their own Steps.Preflight (e.g. in tests), they own strictPass.
	if opts.Steps.Preflight == nil {
		sp := opts.StrictPass
		r.steps.Preflight = func(repoRoot, target string, dryRun, skipTests bool) error {
			return runPreflightLib(repoRoot, target, dryRun, skipTests, sp)
		}
	}

	r.logf("target: v%s", opts.Target)
	r.logf("changelog range: %s..HEAD", r.fromTag)
	r.logf("dry-run: %v | no-rollback: %v | skip-tests: %v",
		opts.DryRun, opts.NoRollback, opts.SkipTests)

	// Init journal.
	journal, journalPath, err := initJournal(opts, r.fromTag, r.now())
	if err != nil {
		return r, fmt.Errorf("%w: journal init: %v", ErrPrePublishFailed, err)
	}
	r.journal = journal
	r.journalPath = journalPath
	r.res.JournalPath = journalPath
	r.logf("journal: %s", journalPath)
	return r, nil
}

// runPrePublishStep is the shape all six pre-publish steps share. journalName
// is what the journal and Result ledgers record; errLabel is what the operator
// sees in the error. They are separate parameters because for two steps they
// genuinely differ. The journal's failure Note is the step's OWN error, not
// the wrapped one — the journal is the forensic record of what the step
// reported, and the wrap is for the caller.
func (r *releaseRun) runPrePublishStep(journalName, errLabel string, run func() error) error {
	if err := run(); err != nil {
		appendStep(r.journal, r.journalPath, journalName, "fail", err.Error(), r.now())
		r.res.StepsFailed = append(r.res.StepsFailed, journalName)
		return fmt.Errorf("%w: %s: %v", ErrPrePublishFailed, errLabel, err)
	}
	appendStep(r.journal, r.journalPath, journalName, "ok", "", r.now())
	r.res.StepsCompleted = append(r.res.StepsCompleted, journalName)
	return nil
}

// skipPrePublishStep records a step the dry run does not perform. It
// deliberately does NOT append to StepsCompleted — a skipped step is not a
// completed one, which TestRun_DryRun_StepLedgerExact pins.
func (r *releaseRun) skipPrePublishStep(journalName string) {
	appendStep(r.journal, r.journalPath, journalName, "skipped-dry-run", "", r.now())
}

// prePublish runs the six steps that precede the ship, in order. Each keeps
// its own log line and its own dry-run branch, because the branches differ:
// three steps pass DryRun THROUGH to the step (which decides for itself) and
// two skip entirely.
func (r *releaseRun) prePublish() error {
	o := r.opts

	// Step 0: full-dry-run preflight (opt-in).
	if o.RequirePreflight {
		r.logf("step: full-dry-run preflight (--require-preflight)")
		if err := r.runPrePublishStep("full-dry-run-preflight", "full-dry-run preflight", func() error {
			return r.steps.FullDryRunPreflight(o.RepoRoot, o.Target)
		}); err != nil {
			return err
		}
	}

	// Step 1: preflight.
	r.logf("step: preflight")
	if err := r.runPrePublishStep("preflight", "preflight", func() error {
		return r.steps.Preflight(o.RepoRoot, o.Target, o.DryRun, o.SkipTests)
	}); err != nil {
		return err
	}

	// Step 2: changelog-gen.
	r.logf("step: changelog-gen")
	if err := r.runPrePublishStep("changelog-gen", "changelog-gen", func() error {
		return r.steps.ChangelogGen(o.RepoRoot, r.fromTag, "HEAD", o.Target, o.DryRun)
	}); err != nil {
		return err
	}

	// Step 3: version-bump.
	r.logf("step: version-bump")
	if err := r.runPrePublishStep("version-bump", "version-bump", func() error {
		return r.steps.VersionBump(o.RepoRoot, o.Target, o.DryRun)
	}); err != nil {
		return err
	}

	// Step 3.5: rebuild-binary. Rebuilds go/evolve from the version-bumped
	// source so the marketplace binary is in sync with plugin.json:version
	// after this release. Without this step, operators install the new
	// plugin version but run the previous build. Best-effort in dry-run.
	if o.DryRun {
		r.logf("step: rebuild-binary (DRY-RUN — would run `go build -ldflags '-X …version=%s …'` -o go/evolve ./cmd/evolve from <RepoRoot>/go)", o.Target)
		r.skipPrePublishStep("rebuild-binary")
	} else {
		r.logf("step: rebuild-binary")
		if err := r.runPrePublishStep("rebuild-binary", "rebuild-binary", func() error {
			// The literal false is carried over verbatim: this branch only
			// runs when o.DryRun is already false, so the two are identical
			// here and swapping in o.DryRun would be an unobservable,
			// unmotivated edit inside a behavior-preserving change.
			return r.steps.RebuildBinary(o.RepoRoot, o.Target, false)
		}); err != nil {
			return err
		}
	}

	// Step 4: release.sh consistency check (skipped in dry-run).
	if o.DryRun {
		r.logf("step: release.sh-check (DRY-RUN — skipping; markers not actually bumped)")
		r.skipPrePublishStep("release-sh-check")
		return nil
	}
	r.logf("step: release.sh-check")
	return r.runPrePublishStep("release-sh-check", "release.sh consistency", func() error {
		return r.steps.ReleaseSh(o.RepoRoot, o.Target)
	})
}

// ship performs step 5. shipped is false when a dry run stopped here, which
// is the signal for Run to return successfully without touching the
// post-publish stages.
func (r *releaseRun) ship() (shipped bool, err error) {
	o := r.opts
	commitMsg := "release: v" + o.Target
	if o.DryRun {
		r.logf("step: ship.sh (DRY-RUN — would commit & push & gh release create)")
		r.logf("  commit msg: %s", commitMsg)
		r.skipPrePublishStep("ship")
		r.logf("")
		r.logf("DRY RUN COMPLETE — no mutations were made.")
		return false, nil
	}
	releaseNotes := r.releaseNotes()
	r.logf("step: ship.sh (--class release)")
	newSHA, err := r.steps.Ship(o.RepoRoot, commitMsg, releaseNotes)
	if err != nil {
		appendStep(r.journal, r.journalPath, "ship", "fail", err.Error(), r.now())
		r.res.StepsFailed = append(r.res.StepsFailed, "ship")
		return false, fmt.Errorf("%w: %v", ErrShipFailed, err)
	}
	appendStep(r.journal, r.journalPath, "ship", "ok", "", r.now())
	r.res.StepsCompleted = append(r.res.StepsCompleted, "ship")
	r.res.NewCommitSHA = newSHA
	setJournalField(r.journal, r.journalPath, "commit_sha", newSHA)
	return true, nil
}

// releaseNotes builds the notes body handed to the ship step.
//
// One-binary S5: stamp the fingerprint-impact class (binary- vs config-release)
// at the top of the notes so a corporate operator sees at a glance whether
// adopting this release needs a new approval. Skipped for empty notes to match
// the Fingerprints section's non-empty guard. MUST run BEFORE steps.Ship
// commits: rebuild-binary (step 3.5) rewrote go/evolve uncommitted, and
// `git diff prevTag..HEAD` compares COMMITTED trees — running after the commit
// would make every release diff as changed (always binary-release). On a git
// error the banner fails closed (assume approval needed) and we log it rather
// than silently drop the classification.
func (r *releaseRun) releaseNotes() string {
	notes := extractReleaseNotes(r.opts.RepoRoot, r.opts.Target)
	if notes == "" {
		return notes
	}
	banner, cerr := releaseClassBanner(r.opts.RepoRoot, r.opts.Target, r.fromTag)
	if cerr != nil {
		r.logf("WARN: release-class classification failed (%v) — stamping fail-closed 'unavailable' banner", cerr)
	}
	return banner + "\n\n" + notes
}

// postPublish runs steps 6 and 7. Both route a failure through
// failPostPublish, which owns the rollback decision — that is why they are
// not pre-publish steps: after the ship, a failure is recoverable only by
// rolling back, never by returning ErrPrePublishFailed.
func (r *releaseRun) postPublish() error {
	o := r.opts

	// Step 6: marketplace-poll (with auto-rollback).
	r.logf("step: marketplace-poll (max_wait=%s)", o.MaxPollWait)
	if err := r.steps.MarketplacePoll(o.RepoRoot, o.Target, o.MaxPollWait); err != nil {
		_, ferr := failPostPublish(&r.res, r.journal, r.journalPath, o, r.steps, r.logf, r.now,
			"marketplace-poll", "marketplace propagation failed", err)
		return ferr
	}
	appendStep(r.journal, r.journalPath, "marketplace-poll", "ok", "", r.now())
	r.res.StepsCompleted = append(r.res.StepsCompleted, "marketplace-poll")

	// Step 7: release-verify — the terminal self-consistency proof (binary
	// committed + pinned + version-stamped, local tag present). A release
	// that cannot prove itself must not stand: same post-publish rollback
	// semantics as a failed propagation.
	r.logf("step: release-verify")
	if err := r.steps.ReleaseVerify(o.RepoRoot, o.Target, r.res.NewCommitSHA); err != nil {
		_, ferr := failPostPublish(&r.res, r.journal, r.journalPath, o, r.steps, r.logf, r.now,
			"release-verify", "release self-consistency verification failed", err)
		return ferr
	}
	appendStep(r.journal, r.journalPath, "release-verify", "ok", "", r.now())
	r.res.StepsCompleted = append(r.res.StepsCompleted, "release-verify")
	return nil
}

// complete stamps the journal and logs the terminal disclosure.
func (r *releaseRun) complete() {
	setJournalField(r.journal, r.journalPath, "completed_at", r.now().UTC().Format(time.RFC3339))
	r.logf("DONE: v%s shipped, propagated, and verified", r.opts.Target)
	r.logf("journal: %s", r.journalPath)
	// GitHub CI is intentionally NOT checked here: this pipeline is self-contained
	// and gh-free (headless/cron-safe). A green pipeline therefore does NOT imply a
	// green `go`/`CI` workflow on the pushed commit (the v20.1.0 false-success: the
	// release exited 0 while the released commit's apicover gate was red). Disclose
	// the gap loudly; CI-gating lives in the /publish skill (pre-release CI-green
	// check + post-release CI watch).
	r.logf("NOTE: GitHub CI is NOT verified by this pipeline — confirm the `go` and `CI` workflows are green on the release commit (e.g. `gh run watch`), or publish via /publish (which watches CI).")
}
