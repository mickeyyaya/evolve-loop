package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

const routingDiscoveryTimeout = 30 * time.Second

var routingDoctor = func(ctx context.Context) gobridge.DoctorReport {
	rep, _ := gobridge.NewEngine(gobridge.Deps{}).Doctor(ctx, "", false)
	return rep
}

func discoveredTail() []string {
	ctx, cancel := context.WithTimeout(context.Background(), routingDiscoveryTimeout)
	defer cancel()
	return universalFallbackTail(routingDoctor(ctx).Results)
}

func routingProfilesDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "profiles")
}

func buildCLIRouter(projectRoot string, cat cliroute.Catalog, host cliroute.Host) (*cliroute.Router, []cliroute.Finding, error) {
	s, err := cliRouterSetup(routerSite{root: projectRoot, catalog: cat}, host)
	if err != nil {
		return nil, nil, err
	}
	return cliroute.Build(s)
}

type routerSite struct {
	root    string
	catalog cliroute.Catalog
}

func cliRouterSetup(site routerSite, host cliroute.Host) (cliroute.Setup, error) {
	pol, err := policy.Load(filepath.Join(site.root, ".evolve", "policy.json"))
	if err != nil {
		return cliroute.Setup{}, err
	}
	return routerSetupOf(pol, site, host), nil
}

func routerSetupOf(pol policy.Policy, site routerSite, host cliroute.Host) cliroute.Setup {
	return cliroute.Setup{
		Policy:   pol,
		Catalog:  site.catalog,
		Profiles: profiles.NewFromDir(routingProfilesDir(site.root)),
		Host:     host,
		Options:  []cliroute.CompileOption{cliroute.WithToolCapable(gobridge.HasToolUse)},
	}
}

func loadCLIRouter(projectRoot string, host cliroute.Host) (*cliroute.Router, []cliroute.Finding, error) {
	return buildCLIRouter(projectRoot, routingCatalog(projectRoot), host)
}

func routingCatalog(projectRoot string) phasespec.Catalog {
	builtin, err := phasespec.Load(config.RegistryPath(projectRoot))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[phases] WARN builtin registry load failed (%v); the routing table compiles without the builtin phases\n", err)
	}
	userSpecs, _ := discoverUserSpecsClamped(projectRoot, cmdutil.NewPromptsLoader(projectRoot))
	merged, _ := builtin.Merge(userSpecs)
	return merged
}

func routingHost(logw io.Writer, now func() time.Time) cliroute.Host {
	logf := func(format string, args ...any) { fmt.Fprintf(logw, format, args...) }
	return cliroute.Host{
		Discover: memoized(discoveredTail),
		Bench: func(projectRoot, label string, plan llmroute.Plan, env map[string]string) llmroute.Plan {
			return bridgechain.ApplyCLIHealthBench(projectRoot, label, plan, env, now, func(format string, args ...any) {
				logf("[cliroute] "+format, args...)
			})
		},
		Logf: logf,
	}
}

func memoized(discover func() []string) func() []string {
	var once sync.Once
	var discovered []string
	return func() []string {
		once.Do(func() { discovered = discover() })
		return discovered
	}
}

func reportRoutingFindings(w io.Writer, findings []cliroute.Finding) {
	for _, f := range findings {
		fmt.Fprintf(w, "[cli-routing] %s %s: %s\n", f.Severity, f.Key, f.Message)
	}
}

const exitRoutingRefused = 2

func roleResolver(r *cliroute.Router) func(string) (resolvellm.Result, error) {
	return func(role string) (resolvellm.Result, error) {
		return r.ResolveRole(role, resolvellm.Options{})
	}
}

func rootRouter(layout paths.Layout, verb string, logw, stderr io.Writer) (*cliroute.Router, bool) {
	router, findings, err := loadCLIRouter(layout.ProjectRoot, routingHost(logw, time.Now))
	if err != nil {
		reportRoutingFindings(stderr, findings)
		fmt.Fprintf(stderr, "%s: %v\n", verb, err)
		return nil, false
	}
	return router, true
}

func detectRouter(projectRoot string, stderr io.Writer) *cliroute.Router {
	router, _, err := loadCLIRouter(projectRoot, cliroute.Host{})
	if err != nil {
		fmt.Fprintf(stderr, "evolve setup detect: WARN the CLI routing table refuses to route (%v); phases render unresolved\n", err)
		return nil
	}
	return router
}

func installRootRouter(projectRoot string, stderr io.Writer) error {
	router, findings, err := loadCLIRouter(projectRoot, routingHost(stderr, time.Now))
	if err != nil {
		reportRoutingFindings(stderr, findings)
		return fmt.Errorf("the CLI routing table refuses to route: %w", err)
	}
	runner.DefaultRouter = router
	return nil
}

func rootRouterInstaller(projectRoot string) error {
	return installRootRouter(projectRoot, os.Stderr)
}
