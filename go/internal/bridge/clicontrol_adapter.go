package bridge

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

type cliController struct {
	cfg     *Config
	deps    Deps
	resolve func(cli string) (Manifest, error)
	capture func(ctx context.Context, cli string, spec ControlSpec) (string, error)
}

const controlAwaitWindows = "windows"

func NewController(cfg *Config, deps Deps) clicontrol.Controller {
	c := &cliController{cfg: cfg, deps: deps.withDefaults(), resolve: LoadManifest}
	c.capture = c.captureSpec
	return c
}

func (c *cliController) captureSpec(ctx context.Context, cli string, spec ControlSpec) (string, error) {
	cfg := c.perFamilyConfig(cli)
	if spec.Await == controlAwaitWindows {
		return captureUntil(ctx, cfg, c.deps, cli, spec.Send, usageCaptureSettleTicks, windowsSettled(*spec.Windows, c.deps.Now()))
	}
	return captureControl(ctx, cfg, c.deps, cli, spec.Send, helpCaptureSettleTicks)
}

func (c *cliController) perFamilyConfig(cli string) *Config {
	out := *c.cfg
	out.CLI = cli
	out.AllowedTools = append([]string(nil), c.cfg.AllowedTools...)
	out.ExtraFlags = append([]string(nil), c.cfg.ExtraFlags...)
	intent := LaunchIntent{Effort: LaunchEffort(cli, c.cfg.Model, c.deps.Efforts)}
	if c.cfg.AllowBypass {
		intent.Permission = "bypass"
	}
	out.Realization = RealizeFor(cli, intent)
	applyScratchCwd(&out)
	return &out
}

func (c *cliController) Do(ctx context.Context, family string, ev clicontrol.Event) (clicontrol.Response, error) {
	resp := clicontrol.Response{Family: family, Event: ev}
	r, err := Resolve(DefaultDriverSet().WithLoader(c.resolve), family)
	if err != nil {
		return resp, fmt.Errorf("clicontrol: %w", err)
	}
	cli := r.Driver
	spec, ok := r.Manifest.Control(string(ev))
	if !ok {
		return resp, fmt.Errorf("%w: family=%s event=%s", clicontrol.ErrUnsupported, family, ev)
	}
	pane, err := c.capture(ctx, cli, spec)
	if err != nil {
		return resp, fmt.Errorf("clicontrol: %s %s: %w", family, ev, err)
	}
	resp.Pane = pane
	return resp, nil
}
