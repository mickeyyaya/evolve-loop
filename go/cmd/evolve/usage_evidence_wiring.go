package main

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

var usageEvidenceFn = productionUsageEvidence

func productionUsageEvidence(projectRoot, evolveDir string, log io.Writer) usageevidence.Explain {
	factory := bridge.NewControllerFactory(projectRoot, filepath.Join(evolveDir, "usage-evidence"), "usage-evidence", bridge.Deps{})
	return usageevidence.New(usageevidence.Options{
		ProjectRoot: projectRoot, EvolveDir: evolveDir, Config: loadCLIHealthConfig(evolveDir),
		Probe: usageevidence.UsagePane(factory), Act: bridgechain.CLIHealthEnabled(nil), Log: log,
	})
}

func withUsageEvidence(inner core.Bridge, explain usageevidence.Explain, signals *signalcenter.Center) core.Bridge {
	if explain == nil {
		return inner
	}
	return usageevidence.Wrap(inner, explain, func() *signalcenter.Center { return signals })
}

func familyEvidence(explain usageevidence.Explain) func(ctx context.Context, family string) usageprobe.Evidence {
	if explain == nil {
		return nil
	}
	return func(ctx context.Context, family string) usageprobe.Evidence {
		return explain(ctx, family+"-tmux", time.Now())
	}
}

func evidenceSummary(explain usageevidence.Explain) func(driver string) string {
	if explain == nil {
		return nil
	}
	return func(driver string) string { return explain(context.Background(), driver, time.Now()).Summary() }
}

type observerDeps struct {
	signals *signalcenter.Center
	usage   usageevidence.Explain
}

func phaseObserverOptions(cfg policy.ObserverPolicy, recoveryStage string, deps observerDeps) []core.Option {
	if !*cfg.Autospawn {
		return nil
	}
	ca := observer.NewCoreAdapter(cfg)
	ca.RecoveryStage = recoveryStage
	ca.Signals = func() *signalcenter.Center { return deps.signals }
	ca.UsageEvidence = deps.usage
	return []core.Option{core.WithObserver(ca)}
}
