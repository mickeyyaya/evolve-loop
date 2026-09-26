package bridge

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

// ControllerFactory mints isolated, per-family clicontrol.Controllers. Each
// For(family) gets its own workspace subdir under baseWorkspace so concurrent
// probes never share bridge scratch (inject buffer, session record); tmux
// sessions stay unique via resolveSession's nonce and clihealth writes via its
// flock — together making the fan-out thread- and process-safe.
type ControllerFactory struct {
	projectRoot   string
	baseWorkspace string
	agent         string
	deps          Deps
}

// NewControllerFactory builds a factory rooted at projectRoot, isolating each
// family's session under baseWorkspace/<family>. agent labels the sessions
// (defaults to "control").
func NewControllerFactory(projectRoot, baseWorkspace, agent string, deps Deps) *ControllerFactory {
	if agent == "" {
		agent = "control"
	}
	return &ControllerFactory{projectRoot: projectRoot, baseWorkspace: baseWorkspace, agent: agent, deps: deps}
}

// For returns a Controller for family, bound to an isolated per-family
// workspace. The bypass posture is carried on the config; the per-family launch
// realization is derived inside Do (perFamilyConfig).
func (f *ControllerFactory) For(family string) clicontrol.Controller {
	cfg := &Config{
		CLI:         family + "-tmux",
		ProjectRoot: f.projectRoot,
		Workspace:   filepath.Join(f.baseWorkspace, family),
		Agent:       f.agent,
		AllowBypass: true,
	}
	return NewController(cfg, f.deps)
}
