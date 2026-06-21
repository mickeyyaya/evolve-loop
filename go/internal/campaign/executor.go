package campaign

// executor.go — the testable campaign wave-execution loop (Humble Object). The
// cmd shell (`evolve campaign run`) owns process/exec/signal wiring; this owns
// the orchestration policy: skip already-shipped waves on resume, run each wave
// through an injected WaveRunner, and checkpoint progress after every wave so a
// crash/interrupt resumes past completed work instead of re-burning it. Keeping
// it free of os/exec makes the large-scale failure modes (checkpoint/resume in
// Fix A, context+deadline in Fix B, failure classification in Fix C) unit-testable
// with a fake runner.

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// WaveRunner runs one wave's file-disjoint specs concurrently and returns the
// per-spec results in input order. Production injects fleet.Supervisor.Run; tests
// inject a fake.
type WaveRunner func(ctx context.Context, wave []fleet.CycleSpec) []fleet.Result

// RunOptions configures one campaign execution.
type RunOptions struct {
	// ProgressPath is where wave progress is checkpointed (campaign-progress-<goalHash>.json).
	ProgressPath string
	// PlanSHA binds progress to the exact plan; on Resume a mismatch discards stale progress.
	PlanSHA string
	// Resume loads prior progress and skips completed waves (when PlanSHA matches).
	Resume bool
	// MaxRetries is how many times a wave's FAILED specs are re-run (batched)
	// before the run aborts. 0 = no retry (stop at first failure).
	MaxRetries int
	// Cooldown returns how long to wait before retrying a wave's failed specs —
	// e.g. the time until a quota bench expires, so a walled wave backs off instead
	// of hammering the wall. Returns 0 (or nil hook) = retry immediately.
	Cooldown func() time.Duration
	// Sleep waits d before a retry (nil → time.Sleep); injectable for tests.
	Sleep func(d time.Duration)
	// BeforeWave runs before each non-skipped wave — e.g. a CLI-health canary that
	// clears expired benches so the wave doesn't re-hit a recovered wall. nil = no-op.
	BeforeWave func()
}

// RunWaves executes dependency-ordered waves with checkpointing. On Resume it
// skips waves already recorded complete for the SAME plan; after each wave fully
// succeeds it persists progress so a later crash resumes past it. A wave's failed
// specs are retried as a BATCH (only the failures re-run, recovering the
// parallelism of the passing cycles) up to MaxRetries times before the run aborts
// without marking the wave complete.
func RunWaves(ctx context.Context, waves [][]fleet.CycleSpec, run WaveRunner, opts RunOptions) error {
	prog := &CampaignProgress{}
	if opts.Resume {
		loaded, err := LoadProgress(opts.ProgressPath)
		if err != nil {
			return err
		}
		if loaded.PlanSHA == opts.PlanSHA { // matching plan ⇒ honor prior progress; else fresh
			prog = loaded
		}
	}
	prog.PlanSHA = opts.PlanSHA

	for i, w := range waves {
		if prog.IsWaveComplete(i) {
			continue
		}
		if opts.BeforeWave != nil {
			opts.BeforeWave()
		}
		skipped, err := runWaveWithRetry(ctx, run, w, opts)
		if err != nil {
			return fmt.Errorf("campaign: wave %d: %w", i+1, err)
		}
		// A wave whose only remaining failures were quarantined OPTIONAL cycles is
		// still marked complete (those ids go to FailedCycleIDs), so --resume does
		// NOT re-run it. A REQUIRED failure aborts above, leaving the wave
		// incomplete so a resumed run retries it.
		skippedIDs := scopeIDs(skipped)
		prog.MarkWaveComplete(i, subtractIDs(waveCycleIDs(w), skippedIDs))
		for _, id := range skippedIDs {
			if !containsStr(prog.FailedCycleIDs, id) {
				prog.FailedCycleIDs = append(prog.FailedCycleIDs, id)
			}
		}
		if err := prog.Save(opts.ProgressPath); err != nil {
			return err
		}
	}
	return nil
}

// runWaveWithRetry runs a wave, then re-runs ONLY its failed specs (batched) up
// to maxRetries times. Returns nil once every spec succeeds, or an error naming
// the still-failing cycles once retries are exhausted.
func runWaveWithRetry(ctx context.Context, run WaveRunner, wave []fleet.CycleSpec, opts RunOptions) (skipped []fleet.CycleSpec, err error) {
	pending := wave
	for attempt := 0; ; attempt++ {
		failed := failedSpecs(pending, run(ctx, pending))
		if len(failed) == 0 {
			return nil, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err() // operator interrupt / deadline: abort now, don't retry into it
		}
		if attempt >= opts.MaxRetries {
			var required []fleet.CycleSpec
			for _, s := range failed {
				if !s.Optional {
					required = append(required, s)
				}
			}
			if len(required) > 0 {
				return nil, fmt.Errorf("cycle(s) %v failed after %d attempt(s)", scopeIDs(required), attempt+1)
			}
			return failed, nil // all remaining failures are optional → quarantine + continue
		}
		waitForCooldown(opts) // back off (e.g. until a quota bench expires) before retrying
		pending = failed      // batched retry: re-run only the failures
	}
}

// waitForCooldown sleeps the Cooldown duration (if any) before a retry, so a
// quota-walled wave waits for the bench to expire instead of retrying into it.
func waitForCooldown(opts RunOptions) {
	if opts.Cooldown == nil {
		return
	}
	d := opts.Cooldown()
	if d <= 0 {
		return
	}
	if opts.Sleep != nil {
		opts.Sleep(d)
		return
	}
	time.Sleep(d)
}

// subtractIDs returns all minus the ids in remove (used to record only the
// SUCCEEDED cycles of a wave as completed, excluding quarantined optional ones).
func subtractIDs(all, remove []string) []string {
	if len(remove) == 0 {
		return all
	}
	out := make([]string, 0, len(all))
	for _, id := range all {
		if !containsStr(remove, id) {
			out = append(out, id)
		}
	}
	return out
}

// failedSpecs returns the specs whose result failed (non-zero exit or error).
// fleet.Supervisor returns results in input order, so results[i] pairs with specs[i].
func failedSpecs(specs []fleet.CycleSpec, results []fleet.Result) []fleet.CycleSpec {
	var out []fleet.CycleSpec
	for i, r := range results {
		if i < len(specs) && (r.Err != nil || r.ExitCode != 0) {
			out = append(out, specs[i])
		}
	}
	return out
}

// scopeIDs flattens the todo IDs across specs (for error messages).
func scopeIDs(specs []fleet.CycleSpec) []string {
	var ids []string
	for _, s := range specs {
		ids = append(ids, s.Scope...)
	}
	return ids
}

// waveCycleIDs flattens the todo IDs a wave's specs own (CycleSpec.Scope).
func waveCycleIDs(w []fleet.CycleSpec) []string {
	return scopeIDs(w)
}
