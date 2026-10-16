// Package phaseregistrar turns a unified phaseconfig.PhaseConfig into a
// registered, validated phase runner — the generalization of the
// build-time user-phase wiring in cmd_cycle.go.
package phaseregistrar

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type Registrar struct {
	Bridge       core.Bridge
	Prompts      *prompts.Loader
	ProfilesDir  string
	PhasesDir    string
	RegistryPath string
	NowFn        func() time.Time
}

type Result struct {
	Spec   phasespec.PhaseSpec
	Runner core.PhaseRunner
}

func (r Registrar) Register(cfg phaseconfig.PhaseConfig) (Result, error) {
	cfg.Optional = true
	if cfg.WritesSource {
		if cfg.Dispatch.Sandbox == nil {
			cfg.Dispatch.Sandbox = &profiles.SandboxConfig{}
		}
		cfg.Dispatch.Sandbox.Enabled = true
		cfg.Dispatch.Sandbox.ReadOnlyRepo = false
	}

	lacksSelectMetadata := cfg.Description == "" && cfg.WhenToUse == ""
	if lacksSelectMetadata {
		cfg.Description = fmt.Sprintf(mintedStubDescriptionFmt, cfg.Name)
		cfg.WhenToUse = mintedStubWhenToUse
	}

	if cfg.Catalog == "" {
		resolvable := false
		if r.Prompts != nil {
			_, aerr := r.Prompts.Agent(cfg.AgentName())
			resolvable = aerr == nil
		}
		if !resolvable {
			cfg.Catalog = phasespec.CatalogOnDemand
		}
	}

	spec := cfg.Spec()
	if violations := phasespec.ValidateUserSpec(spec); len(violations) > 0 {
		return Result{}, fmt.Errorf("phaseregistrar: invalid spec %q: %s", spec.Name, strings.Join(violations, "; "))
	}

	if cfg.Dispatch.CLI != "" {
		resolved := bridge.DriverFor(cfg.Dispatch.CLI)
		if _, ok := bridge.LookupDriver(resolved); !ok {
			return Result{}, fmt.Errorf("phaseregistrar: phase %q dispatch.cli %q does not resolve to a registered driver (resolved %q); refusing to mint an unresolvable profile", spec.Name, cfg.Dispatch.CLI, resolved)
		}
		cfg.Dispatch.CLI = resolved
	}

	prof := cfg.ToProfile()
	if !nameRE.MatchString(prof.Name) {
		return Result{}, fmt.Errorf("phaseregistrar: derived profile name %q must be lowercase kebab-case", prof.Name)
	}

	persistsProfile := r.ProfilesDir != ""
	if persistsProfile && cfg.Dispatch.CLI == "" {
		return Result{}, fmt.Errorf("phaseregistrar: phase %q has no dispatch.cli; refusing to mint a driverless profile stub", spec.Name)
	}

	tier := dispatchTier(cfg)
	if cfg.Dispatch.ModelTierEnvelope != nil && policy.TierRank(tier) == 0 {
		return Result{}, fmt.Errorf("phaseregistrar: phase %q tier %q is unclassifiable; must be fast|balanced|deep (or a known alias) when an envelope is set", spec.Name, tier)
	}
	if err := policy.ValidatePin(spec.Name, policy.Pin{CLI: cfg.Dispatch.CLI, Model: tier}, &prof); err != nil {
		return Result{}, fmt.Errorf("phaseregistrar: %w", err)
	}

	if r.RegistryPath != "" {
		now := time.Now()
		if r.NowFn != nil {
			now = r.NowFn()
		}
		if err := mintregistry.Append(r.RegistryPath, spec.Name, now); err != nil {
			return Result{}, fmt.Errorf("phaseregistrar: register mint: %w", err)
		}
	}

	if err := r.persist(spec, prof); err != nil {
		return Result{}, err
	}

	runner := specrunner.New(spec, specrunner.Config{
		Bridge:     r.Bridge,
		Prompts:    r.Prompts,
		NowFn:      r.NowFn,
		PromptBody: cfg.Prompt,
	})
	return Result{Spec: spec, Runner: runner}, nil
}

func dispatchTier(cfg phaseconfig.PhaseConfig) string {
	if cfg.Dispatch.ModelTierDefault != "" {
		return cfg.Dispatch.ModelTierDefault
	}
	return cfg.Model
}

func (r Registrar) persist(spec phasespec.PhaseSpec, prof profiles.Profile) error {
	if r.ProfilesDir != "" {
		if err := atomicwrite.JSON(filepath.Join(r.ProfilesDir, prof.Name+".json"), prof); err != nil {
			return fmt.Errorf("phaseregistrar: persist profile: %w", err)
		}
	}
	if r.PhasesDir != "" {
		path := filepath.Join(r.PhasesDir, spec.Name, "phase.json")
		if err := atomicwrite.JSON(path, spec); err != nil {
			return fmt.Errorf("phaseregistrar: persist phase spec: %w", err)
		}
	}
	return nil
}

const (
	mintedStubDescriptionFmt = "Runtime-minted stub for %q; its behavior is defined by the mint-time prompt, not by this catalog entry."
	mintedStubWhenToUse      = "Not selectable on merit: a runtime-minted stub with no advisor-supplied selection criteria. Give it real metadata before relying on it."
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
