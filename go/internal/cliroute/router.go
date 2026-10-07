package cliroute

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

type Setup struct {
	Policy   policy.Policy
	Catalog  Catalog
	Profiles ProfileSource
	Host     Host
	Options  []CompileOption
	Bypass   bool
}

var ErrRefused = errors.New("cliroute: route refused")

var errNoSetup = errors.New("cliroute: this router was built from one table, not a Setup, so it has nothing to recompile from")

func Build(s Setup) (*Router, []Finding, error) {
	pair, findings, err := s.compile(s.Catalog)
	if err != nil {
		return nil, findings, err
	}
	r := &Router{host: s.Host, setup: &s}
	r.tables.Store(&pair)
	return r, findings, nil
}

func (s Setup) compile(cat Catalog) (tablePair, []Finding, error) {
	declared, findings := Compile(s.Policy, cat, s.Profiles, s.Options...)
	bypass, bypassFindings := Compile(policy.Policy{Workflow: s.Policy.Workflow}, cat, s.Profiles, s.Options...)
	if err := refusal(append(slices.Clone(findings), bypassFindings...)); err != nil {
		return tablePair{}, findings, err
	}
	if s.Bypass {
		declared = bypass
	}
	return tablePair{declared: declared, bypass: bypass}, findings, nil
}

func (r *Router) Recompile(cat Catalog) error {
	if r.setup == nil {
		return errNoSetup
	}
	r.recompiling.Lock()
	defer r.recompiling.Unlock()
	pair, _, err := r.setup.compile(cat)
	if err != nil {
		return fmt.Errorf("cliroute: recompiling after a catalog change: %w", err)
	}
	r.tables.Store(&pair)
	return nil
}

func (r *Router) Policy() policy.Policy {
	return r.tables.Load().declared.pol
}

func (r *Router) Findings() []Finding {
	return r.tables.Load().declared.Findings()
}

func (r *Router) ResolveRole(role string, opts resolvellm.Options) (resolvellm.Result, error) {
	if !r.tables.Load().declared.declared {
		return resolvellm.Resolve(role, opts)
	}
	d, err := r.Resolve(Request{Agent: role, ProjectRoot: opts.ProjectRoot, DefaultModel: unsetTierDispatchesAs})
	if err != nil {
		return resolvellm.Result{}, err
	}
	walk, err := d.Walk()
	if err != nil {
		return resolvellm.Result{}, err
	}
	return resolvellm.Result{CLI: walk.Candidates[0], ModelTier: d.Plan.Model, Source: d.Rule}, nil
}

func NewSingleProfileRouter(pol policy.Policy, sp SingleProfile, h Host) (*Router, error) {
	table, _ := Compile(pol, nil, sp)
	return New(table, h)
}

type SingleProfile struct {
	Agent   string
	Profile *profiles.Profile
}

func (s SingleProfile) List() ([]string, error) {
	if s.Profile == nil {
		return nil, nil
	}
	return []string{s.Agent}, nil
}

func (s SingleProfile) Get(name string) (profiles.Profile, error) {
	if s.Profile == nil || name != s.Agent {
		return profiles.Profile{}, fmt.Errorf("cliroute: profile %s: %w", name, fs.ErrNotExist)
	}
	return *s.Profile, nil
}

var ErrNoRootRouter = errors.New("cliroute: this launch has no compiled router")

func RefuseLaunchRouter(pol policy.Policy) error {
	if pol.CLIRouting == nil {
		return nil
	}
	return fmt.Errorf("%w: %w, but the project declares a cli_routing table; the composition root must inject the router it compiled at start", ErrRefused, ErrNoRootRouter)
}

func NewLegacyLaunchRouter(projectRoot string, sp SingleProfile) (*Router, error) {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return nil, err
	}
	if err := RefuseLaunchRouter(pol); err != nil {
		return nil, err
	}
	return NewSingleProfileRouter(policy.Policy{}, sp, Host{})
}
