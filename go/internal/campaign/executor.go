package campaign

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

type WaveRunner func(ctx context.Context, wave []fleet.CycleSpec) []fleet.Result

type RunOptions struct {
	ProgressPath      string
	PlanSHA           string
	Resume            bool
	MaxRetries        int
	Cooldown          func() time.Duration
	Sleep             func(d time.Duration)
	BeforeWave        func(context.Context) error
	AfterWaveComplete func(wave int, prog *CampaignProgress)
}

func RunWaves(ctx context.Context, waves [][]fleet.CycleSpec, run WaveRunner, opts RunOptions) error {
	prog := &CampaignProgress{}
	if opts.Resume {
		loaded, err := LoadProgress(opts.ProgressPath)
		if err != nil {
			return err
		}
		if loaded.PlanSHA == opts.PlanSHA {
			prog = loaded
		}
	}
	prog.PlanSHA = opts.PlanSHA

	for i, w := range waves {
		if prog.IsWaveComplete(i) {
			continue
		}
		if opts.BeforeWave != nil {
			if err := opts.BeforeWave(ctx); err != nil {
				return fmt.Errorf("campaign: wave %d: pre-wave probes: %w", i+1, err)
			}
		}
		skipped, err := runWaveWithRetry(ctx, run, w, opts)
		if err != nil {
			return fmt.Errorf("campaign: wave %d: %w", i+1, err)
		}
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
		if opts.AfterWaveComplete != nil {
			opts.AfterWaveComplete(i, prog)
		}
	}
	return nil
}

func runWaveWithRetry(ctx context.Context, run WaveRunner, wave []fleet.CycleSpec, opts RunOptions) (skipped []fleet.CycleSpec, err error) {
	pending := wave
	for attempt := 0; ; attempt++ {
		failed := failedSpecs(pending, run(ctx, pending))
		if len(failed) == 0 {
			return nil, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
			return failed, nil
		}
		waitForCooldown(opts)
		pending = failed
	}
}

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

func failedSpecs(specs []fleet.CycleSpec, results []fleet.Result) []fleet.CycleSpec {
	var out []fleet.CycleSpec
	for i, r := range results {
		if i < len(specs) && r.Status() != fleet.LaneOK {
			out = append(out, specs[i])
		}
	}
	return out
}

func scopeIDs(specs []fleet.CycleSpec) []string {
	var ids []string
	for _, s := range specs {
		ids = append(ids, s.Scope...)
	}
	return ids
}

func waveCycleIDs(w []fleet.CycleSpec) []string {
	return scopeIDs(w)
}
