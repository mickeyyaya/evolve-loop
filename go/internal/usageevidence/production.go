package usageevidence

import (
	"context"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

type Options struct {
	ProjectRoot string
	EvolveDir   string
	Config      policy.CLIHealthConfig
	Probe       func(ctx context.Context, family string) (string, error)
	Act         bool
	Log         io.Writer
	Now         func() time.Time
}

func New(o Options) Explain {
	log := o.Log
	if log == nil {
		log = io.Discard
	}
	src := &usageprobe.EvidenceSource{
		Prober: &usageprobe.Prober{
			Probe: o.Probe, Windows: ReadWindowsNow, Classify: bridge.ClassifyExhausted,
			Store: clihealth.NewStore(o.ProjectRoot, nil), Log: log,
		},
		EvolveDir: o.EvolveDir, Timeout: o.Config.UsageEvidenceTimeout(), TTL: o.Config.UsageEvidenceTTL(), Act: o.Act, Now: o.Now,
	}
	return func(ctx context.Context, driver string, since time.Time) usageprobe.Evidence {
		return src.Explain(ctx, usageprobe.Query{CLI: binaryOf(driver), Family: llmroute.Family(driver), Since: since})
	}
}

func binaryOf(driver string) string {
	if bin := llmroute.Binary(driver); bin != "" {
		return bin
	}
	return profiles.BaseCLI(driver)
}

func ReadWindowsNow(family, pane string) []quotastate.UsageWindow {
	return bridge.UsageWindows(family, pane, time.Now())
}

func UsagePane(factory *bridge.ControllerFactory) func(ctx context.Context, family string) (string, error) {
	return func(ctx context.Context, family string) (string, error) {
		resp, err := factory.For(family).Do(ctx, family, clicontrol.EventUsage)
		return resp.Pane, err
	}
}
