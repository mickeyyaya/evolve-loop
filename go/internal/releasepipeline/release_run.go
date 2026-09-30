package releasepipeline

import (
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/semvercheck"
)

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

func newReleaseRun(opts Options) (*releaseRun, error) {
	r := &releaseRun{opts: opts, res: Result{Target: opts.Target}, logf: releaseLogf(opts.Stderr)}

	if !semvercheck.IsSemver(opts.Target) {
		return r, fmt.Errorf("%w: target version not semver: %s", ErrPrePublishFailed, opts.Target)
	}

	r.resolveFromTag()

	r.now = opts.Now
	if r.now == nil {
		r.now = time.Now
	}

	r.steps = resolveSteps(opts)

	r.logf("target: v%s", opts.Target)
	r.logf("changelog range: %s..HEAD", r.fromTag)
	r.logf("dry-run: %v | no-rollback: %v | skip-tests: %v",
		opts.DryRun, opts.NoRollback, opts.SkipTests)

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

func releaseLogf(logw io.Writer) func(string, ...any) {
	if logw == nil {
		logw = io.Discard
	}
	return func(format string, args ...any) {
		fmt.Fprintf(logw, "[release-pipeline] "+format+"\n", args...)
	}
}

func (r *releaseRun) resolveFromTag() {
	r.fromTag = r.opts.FromTag
	if r.fromTag == "" {
		if t, err := resolvePrevTag(r.opts.RepoRoot); err == nil && t != "" {
			r.fromTag = t
		} else {
			r.logf("WARN: no previous tag found; changelog range will start from initial commit")
			if init, err := resolveInitCommit(r.opts.RepoRoot); err == nil {
				r.fromTag = init
			}
		}
	}
}

func resolveSteps(opts Options) Steps {
	steps := applyDefaultSteps(opts.Steps)
	if opts.Steps.Preflight == nil {
		strictPass := opts.StrictPass
		steps.Preflight = func(repoRoot, target string, dryRun, skipTests bool) error {
			return runPreflightLib(repoRoot, target, dryRun, skipTests, strictPass)
		}
	}
	return steps
}

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

func (r *releaseRun) skipPrePublishStep(journalName string) {
	appendStep(r.journal, r.journalPath, journalName, "skipped-dry-run", "", r.now())
}

func (r *releaseRun) prePublish() error {
	o := r.opts

	if o.RequirePreflight {
		r.logf("step: full-dry-run preflight (--require-preflight)")
		if err := r.runPrePublishStep("full-dry-run-preflight", "full-dry-run preflight", func() error {
			return r.steps.FullDryRunPreflight(o.RepoRoot, o.Target)
		}); err != nil {
			return err
		}
	}

	r.logf("step: preflight")
	if err := r.runPrePublishStep("preflight", "preflight", func() error {
		return r.steps.Preflight(o.RepoRoot, o.Target, o.DryRun, o.SkipTests)
	}); err != nil {
		return err
	}

	r.logf("step: changelog-gen")
	if err := r.runPrePublishStep("changelog-gen", "changelog-gen", func() error {
		return r.steps.ChangelogGen(o.RepoRoot, r.fromTag, "HEAD", o.Target, o.DryRun)
	}); err != nil {
		return err
	}

	r.logf("step: version-bump")
	if err := r.runPrePublishStep("version-bump", "version-bump", func() error {
		return r.steps.VersionBump(o.RepoRoot, o.Target, o.DryRun)
	}); err != nil {
		return err
	}

	if err := r.rebuildBinary(); err != nil {
		return err
	}

	return r.releaseShCheck()
}

func (r *releaseRun) rebuildBinary() error {
	o := r.opts
	if o.DryRun {
		r.logf("step: rebuild-binary (DRY-RUN — would run `go build -ldflags '-X …version=%s …'` -o go/evolve ./cmd/evolve from <RepoRoot>/go)", o.Target)
		r.skipPrePublishStep("rebuild-binary")
		return nil
	}
	r.logf("step: rebuild-binary")
	const dryRun = false
	return r.runPrePublishStep("rebuild-binary", "rebuild-binary", func() error {
		return r.steps.RebuildBinary(o.RepoRoot, o.Target, dryRun)
	})
}

func (r *releaseRun) releaseShCheck() error {
	o := r.opts
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
	// Classify before Ship commits the rebuilt go/evolve, or every release diffs as a binary-release.
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

func (r *releaseRun) postPublish() error {
	o := r.opts

	r.logf("step: marketplace-poll (max_wait=%s)", o.MaxPollWait)
	if err := r.steps.MarketplacePoll(o.RepoRoot, o.Target, o.MaxPollWait); err != nil {
		_, ferr := failPostPublish(&r.res, r.journal, r.journalPath, o, r.steps, r.logf, r.now,
			"marketplace-poll", "marketplace propagation failed", err)
		return ferr
	}
	appendStep(r.journal, r.journalPath, "marketplace-poll", "ok", "", r.now())
	r.res.StepsCompleted = append(r.res.StepsCompleted, "marketplace-poll")

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

func (r *releaseRun) complete() {
	setJournalField(r.journal, r.journalPath, "completed_at", r.now().UTC().Format(time.RFC3339))
	r.logf("DONE: v%s shipped, propagated, and verified", r.opts.Target)
	r.logf("journal: %s", r.journalPath)
	r.logf("NOTE: GitHub CI is NOT verified by this pipeline — confirm the `required CI` workflow (its `CI required` job) is green on the release commit (e.g. `gh run watch`), or publish via /publish (which watches CI).")
}
