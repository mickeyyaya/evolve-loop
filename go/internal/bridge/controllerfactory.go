package bridge

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

type ControllerFactory struct {
	projectRoot   string
	baseWorkspace string
	agent         string
	deps          Deps
}

func NewControllerFactory(projectRoot, baseWorkspace, agent string, deps Deps) *ControllerFactory {
	if agent == "" {
		agent = "control"
	}
	return &ControllerFactory{projectRoot: projectRoot, baseWorkspace: baseWorkspace, agent: agent, deps: deps}
}

func (f *ControllerFactory) For(family string) clicontrol.Controller {
	resolved, _ := Resolve(DefaultDriverSet(), family)
	cfg := &Config{
		CLI:         resolved.Driver,
		ProjectRoot: f.projectRoot,
		Workspace:   filepath.Join(f.baseWorkspace, family),
		Agent:       f.agent,
	}
	launchPurposeConfig(PurposeProbe, cfg)
	return NewController(cfg, f.deps)
}
