package bridge

// clicontrol_adapter.go is the production clicontrol.Controller: it wires the
// abstract control vocabulary (clicontrol.Event) to the per-CLI mapping table
// (Manifest.Control) and the tmux executor (captureControl). The pipeline
// depends only on the clicontrol.Controller abstraction; this file is the one
// place the abstract→concrete translation happens.

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

// cliController is the production Controller. resolve and capture are seams
// (default to LoadManifest / captureControl) so the resolution + error logic is
// unit-testable without a real manifest or a tmux session. The struct holds no
// mutable state, so a single instance is safe for the concurrent fan-out the
// prober drives.
type cliController struct {
	cfg     *Config
	deps    Deps
	resolve func(cli string) (Manifest, error)
	capture func(ctx context.Context, cli, command, await string) (string, error)
}

// NewController builds the production clicontrol.Controller. cfg is a
// family-AGNOSTIC template (workspace, project root, bypass posture); each Do()
// derives the per-family driver + launch realization from it via
// perFamilyConfig, so one Controller probes every family without cross-CLI flag
// bleed. Each Do() boots-or-attaches the family's REPL via captureControl.
func NewController(cfg *Config, deps Deps) clicontrol.Controller {
	c := &cliController{cfg: cfg, deps: deps, resolve: LoadManifest}
	c.capture = func(ctx context.Context, cli, command, _ string) (string, error) {
		// Await is currently always the prompt marker (captureControl polls for
		// it); the field is carried for forward-compat with non-marker awaits.
		return captureControl(ctx, c.perFamilyConfig(cli), c.deps, cli, command, helpCaptureSettleTicks)
	}
	return c
}

// perFamilyConfig clones the template config for one driver: it sets the CLI and
// resolves that CLI's launch flags (bypass posture carried from the template),
// preserving the shared workspace/project-root. A shallow copy is safe — Do only
// reads the result to boot a transient probe REPL.
func (c *cliController) perFamilyConfig(cli string) *Config {
	out := *c.cfg
	out.CLI = cli
	// Defensively copy the slice fields so concurrent per-family configs never
	// share a backing array (the value copy above would alias them) — the
	// fan-out must be data-race-free even if a future template pre-populates them.
	out.AllowedTools = append([]string(nil), c.cfg.AllowedTools...)
	out.ExtraFlags = append([]string(nil), c.cfg.ExtraFlags...)
	intent := LaunchIntent{}
	if c.cfg.AllowBypass {
		intent.Permission = "bypass"
	}
	out.Realization = RealizeFor(cli, intent)
	return &out
}

// Do resolves family's interactive driver + the event's concrete command from
// the mapping table, then captures the response. A family with no mapping for
// the event yields clicontrol.ErrUnsupported WITHOUT booting a REPL.
func (c *cliController) Do(ctx context.Context, family string, ev clicontrol.Event) (clicontrol.Response, error) {
	resp := clicontrol.Response{Family: family, Event: ev}
	cli := family + "-tmux" // the interactive driver for the family
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
