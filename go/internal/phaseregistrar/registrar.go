// Package phaseregistrar turns a unified phaseconfig.PhaseConfig into a
// registered, validated phase runner — the generalization of the build-time
// user-phase wiring in cmd_cycle.go. It is the seam advisor-minted phases
// (Steps 11/12) go through: validate against the user-phase floor, clamp the
// dispatch against the envelope/allowed-CLIs guardrails, persist the spec +
// dispatch profile so the unchanged runner resolves them from disk, and
// construct a specrunner-backed core.PhaseRunner.
//
// SRP: Register is a pure factory — it returns the runner + normalized spec;
// splicing the result into the runners map, routing config, and phase catalog
// stays the caller's job because those structures live in different
// composition roots (cmd_cycle at build time, the orchestrator at mint time).
package phaseregistrar

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// Registrar builds phase runners from unified configs. Bridge + Prompts are
// the runner dependencies; ProfilesDir/PhasesDir are where the minted phase's
// dispatch profile and spec are persisted (empty → skip that persistence).
// RegistryPath is the shared active-mints registry (mintregistry.Path) every
// lane's tree-diff guard consults; empty skips registration (single-lane
// harnesses without the guard).
type Registrar struct {
	Bridge       core.Bridge
	Prompts      *prompts.Loader
	ProfilesDir  string
	PhasesDir    string
	RegistryPath string
	NowFn        func() time.Time
}

// Result is a registrable phase: the normalized spec (for the caller to splice
// into the catalog + routing) and the constructed runner.
type Result struct {
	Spec   phasespec.PhaseSpec
	Runner core.PhaseRunner
}

// Register validates, clamps, normalizes, persists, and constructs a phase
// from cfg. It rejects (non-nil error) when the spec violates the user-phase
// floor or the dispatch breaches the envelope/allowed-CLIs guardrails — so a
// minted phase can never be a trust-kernel escape hatch.
func (r Registrar) Register(cfg phaseconfig.PhaseConfig) (Result, error) {
	// Normalize BEFORE validation: a user/minted phase is always optional (it
	// cannot satisfy the build→audit→ship floor), and a source-writer always
	// runs sandboxed. These are forced, not rejected.
	cfg.Optional = true
	if cfg.WritesSource {
		if cfg.Dispatch.Sandbox == nil {
			cfg.Dispatch.Sandbox = &profiles.SandboxConfig{}
		}
		cfg.Dispatch.Sandbox.Enabled = true
		cfg.Dispatch.Sandbox.ReadOnlyRepo = false // a writer needs to write
	}

	spec := cfg.Spec()
	if violations := phasespec.ValidateUserSpec(spec); len(violations) > 0 {
		return Result{}, fmt.Errorf("phaseregistrar: invalid spec %q: %s", spec.Name, strings.Join(violations, "; "))
	}

	// The dispatch profile is persisted under (and the runner looks it up by)
	// the derived profile name. AgentName is advisor-controlled and NOT covered
	// by ValidateUserSpec's name check, so guard it here — a non-kebab name
	// would produce a bad filename and a silent runner lookup miss.
	prof := cfg.ToProfile()
	if !nameRE.MatchString(prof.Name) {
		return Result{}, fmt.Errorf("phaseregistrar: derived profile name %q must be lowercase kebab-case", prof.Name)
	}

	// Clamp dispatch against the config's own guardrails (envelope +
	// allowed_clis). Reuses the policy pin clamp — the dispatch is a "pin" the
	// advisor proposed, validated against the profile ToProfile() yields. An
	// envelope with an UNCLASSIFIABLE tier (rank 0) must REJECT, not exempt:
	// ValidatePin skips rank 0 (correct for an exact-model pin), but a minted
	// phase must never escape its envelope via a novel/typo tier string.
	tier := dispatchTier(cfg)
	if cfg.Dispatch.ModelTierEnvelope != nil && policy.TierRank(tier) == 0 {
		return Result{}, fmt.Errorf("phaseregistrar: phase %q tier %q is unclassifiable; must be fast|balanced|deep (or a known alias) when an envelope is set", spec.Name, tier)
	}
	if err := policy.ValidatePin(spec.Name, policy.Pin{CLI: cfg.Dispatch.CLI, Model: tier}, &prof); err != nil {
		return Result{}, fmt.Errorf("phaseregistrar: %w", err)
	}

	// Register-before-persist (cycle-967 Variant A2): the minted name must be
	// discoverable by every lane's tree-diff guard BEFORE its files can appear
	// in a shared-tree diff — the reverse order recreates the cross-lane
	// false-abort race. An unregistered mint is an abort landmine for every
	// concurrent lane, so a registry failure rejects the mint loudly.
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

// dispatchTier is the tier the phase dispatches at: the explicit dispatch
// default, else the spec model hint (which may be a raw model id — TierRank
// classifies both, so the clamp still applies).
func dispatchTier(cfg phaseconfig.PhaseConfig) string {
	if cfg.Dispatch.ModelTierDefault != "" {
		return cfg.Dispatch.ModelTierDefault
	}
	return cfg.Model
}

// persist writes the dispatch profile and spec atomically; either write is
// skipped when its dir is empty.
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

// nameRE constrains a derived profile name to lowercase kebab-case — the same
// rule phasespec applies to phase names, enforced here on the (advisor-
// controlled) agent-derived profile name so it is a safe filename + lookup key.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
