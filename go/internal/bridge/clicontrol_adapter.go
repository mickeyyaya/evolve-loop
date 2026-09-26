package bridge

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

// cliController is the production clicontrol.Controller; resolve and capture are test seams.
// It holds no mutable state, so one instance is safe for the prober's concurrent fan-out.
type cliController struct {
	cfg     *Config
	deps    Deps
	resolve func(cli string) (Manifest, error)
	capture func(ctx context.Context, cli, command, await string) (string, error)
}

// NewController builds the production clicontrol.Controller; each Do derives its family's config from the template cfg, so flags never bleed across CLIs.
func NewController(cfg *Config, deps Deps) clicontrol.Controller {
	c := &cliController{cfg: cfg, deps: deps, resolve: LoadManifest}
	c.capture = func(ctx context.Context, cli, command, _ string) (string, error) {
		// The await is always the prompt marker today, which captureControl polls for.
		return captureControl(ctx, c.perFamilyConfig(cli), c.deps, cli, command, helpCaptureSettleTicks)
	}
	return c
}

// perFamilyConfig clones the template for one CLI and realizes its launch flags, carrying the bypass posture.
func (c *cliController) perFamilyConfig(cli string) *Config {
	out := *c.cfg
	out.CLI = cli
	// Copy the slices so concurrent per-family configs never share a backing array.
	out.AllowedTools = append([]string(nil), c.cfg.AllowedTools...)
	out.ExtraFlags = append([]string(nil), c.cfg.ExtraFlags...)
	intent := LaunchIntent{}
	if c.cfg.AllowBypass {
		intent.Permission = "bypass"
	}
	out.Realization = RealizeFor(cli, intent)
	return &out
}

// Do runs one control event on the family's interactive driver. An unmapped event returns
// clicontrol.ErrUnsupported without booting a REPL.
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
