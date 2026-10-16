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
	capture func(ctx context.Context, cli, command, await string) (string, error)
}

func NewController(cfg *Config, deps Deps) clicontrol.Controller {
	c := &cliController{cfg: cfg, deps: deps.withDefaults(), resolve: LoadManifest}
	c.capture = func(ctx context.Context, cli, command, _ string) (string, error) {
		return captureControl(ctx, c.perFamilyConfig(cli), c.deps, cli, command, helpCaptureSettleTicks)
	}
	return c
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
	cli := family + "-tmux"
	m, err := c.resolve(cli)
	if err != nil {
		return resp, fmt.Errorf("clicontrol: resolve %s: %w", cli, err)
	}
	spec, ok := m.Control(string(ev))
	if !ok {
		return resp, fmt.Errorf("%w: family=%s event=%s", clicontrol.ErrUnsupported, family, ev)
	}
	pane, err := c.capture(ctx, cli, spec.Send, spec.Await)
	if err != nil {
		return resp, fmt.Errorf("clicontrol: %s %s: %w", family, ev, err)
	}
	resp.Pane = pane
	return resp, nil
}
