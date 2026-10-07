package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type routingRun struct {
	bypass bool
	env    map[string]string
}

func overrideRefusals(r *cliroute.Router, projectRoot string, env map[string]string) ([]string, error) {
	agents, err := profiles.NewFromDir(routingProfilesDir(projectRoot)).List()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(agents)
	var refusals []string
	for _, agent := range agents {
		value, source := llmroute.EnvPrimary(agent, env)
		if value == "" {
			continue
		}
		d, err := r.Resolve(cliroute.Request{Agent: agent, ProjectRoot: projectRoot, DefaultModel: unsetDispatchTier})
		if err != nil {
			return nil, err
		}
		if !overrideAllowed(agent, value, d) {
			refusals = append(refusals, fmt.Sprintf("%s: %s=%s is outside its allowed set %v: %s", agent, strings.TrimSuffix(strings.TrimPrefix(source, "env("), ")"), value, d.Allowed, overrideRemedy(agent, source)))
		}
	}
	return refusals, nil
}

func overrideRemedy(agent, source string) string {
	key := envchain.PhaseEnvKey(agent, "CLI")
	if source == "env(EVOLVE_CLI)" {
		return "unset EVOLVE_CLI, or set " + key + " (or --cli " + agent + "=<driver>) for this agent"
	}
	return "unset " + key + ", or drop --cli " + agent
}

func overrideAllowed(agent, value string, d cliroute.Decision) bool {
	if profiles.IsClaudeFamilyFloor(agent) && llmroute.Family(llmroute.DefaultDriverForFamily(value)) != "claude" {
		return false
	}
	return d.Allows(value)
}

func wireCLIRouter(site routerSite, run routingRun, console io.Writer) (*cliroute.Router, error) {
	s, err := cliRouterSetup(site, routingHost(os.Stderr, time.Now))
	if err != nil {
		return nil, fmt.Errorf("the CLI routing table refuses to route: %w", err)
	}
	s.Bypass = run.bypass
	r, findings, err := cliroute.Build(s)
	reportRoutingFindings(console, findings)
	if err != nil {
		return nil, fmt.Errorf("the CLI routing table refuses to route: %w", err)
	}
	refusals, err := overrideRefusals(r, site.root, run.env)
	if err != nil {
		return nil, fmt.Errorf("checking the CLI overrides: %w", err)
	}
	if len(refusals) > 0 {
		return nil, fmt.Errorf("the CLI routing table refuses %d override(s): %s", len(refusals), strings.Join(refusals, "; "))
	}
	return r, nil
}
