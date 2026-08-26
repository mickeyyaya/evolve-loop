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

	// Mint-time SELECT-metadata default (docs/chronicle/2026-08-minted-stub-class.md).
	// A minted phase is forced Optional just above, so a config carrying neither
	// Description nor WhenToUse persists a phase.json that trips phasespec's
	// TestPhaseCatalog_OptionalPhasesHaveSelectMetadata the moment ship's
	// whole-tree bind sweeps the stub into git tracking — a repo-wide, CI-only
	// failure invisible to per-cycle changed-scope, hand-fixed four times (#399,
	// #404, #406, #407). Defaulting here, at the one seam every mint funnels
	// through, makes the artifact contract-safe by construction rather than by
	// advisor diligence. The condition mirrors the guard's own AND over both
	// fields: an advisor that supplied EITHER one keeps its text verbatim.
	if cfg.Description == "" && cfg.WhenToUse == "" {
		cfg.Description = fmt.Sprintf(mintedStubDescriptionFmt, cfg.Name)
		cfg.WhenToUse = mintedStubWhenToUse
	}

	// Mint-time persona-resolvability default (2026-08-26, instance #5 of the
	// persona-less-phase class — the cycle-1567/1568 halt): the mint's OWN
	// dispatch runs on the inline PromptBody, but the persisted phase.json
	// outlives the process, and a stub whose derived persona
	// (agents/evolve-<name>.md) exists nowhere both reds the tracked-menu
	// guard the moment ship's bind sweeps it into tracking (the lane-only
	// internal/core regression) and schedules a next-cycle dispatch that dies
	// at load-agent. Stamp catalog:"on-demand" at the same construction seam
	// as the SELECT-metadata default above — off the menu until someone writes
	// the persona, exactly the tracked-phase remedy (#493). A mint whose
	// persona DOES resolve, or that carries an explicit catalog word, keeps it
	// verbatim.
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

	// Bare CLI family name resolution (cycle 1325, mint-profile-driver-suffix):
	// bridge.DriverFor already projects a bare family name ("claude") onto its
	// concrete driver ("claude-tmux") for every other dispatch call site
	// (subagent, consensusdispatch) — Register was the one mint choke point
	// that skipped it, so an advisor-minted bare name reached disk unresolved
	// and preflight's exact-match LookupDriver rejected it hours later at the
	// next loop launch (the batch-15 incident this closes). Resolved BEFORE
	// ToProfile() so the persisted profile carries the resolved name, and
	// rejected outright (not silently persisted) when the resolved name is
	// not itself a registered driver — an unresolvable family name must fail
	// mint loudly, never at the next launch's preflight halt.
	if cfg.Dispatch.CLI != "" {
		resolved := bridge.DriverFor(cfg.Dispatch.CLI)
		if _, ok := bridge.LookupDriver(resolved); !ok {
			return Result{}, fmt.Errorf("phaseregistrar: phase %q dispatch.cli %q does not resolve to a registered driver (resolved %q); refusing to mint an unresolvable profile", spec.Name, cfg.Dispatch.CLI, resolved)
		}
		cfg.Dispatch.CLI = resolved
	}

	// The dispatch profile is persisted under (and the runner looks it up by)
	// the derived profile name. AgentName is advisor-controlled and NOT covered
	// by ValidateUserSpec's name check, so guard it here — a non-kebab name
	// would produce a bad filename and a silent runner lookup miss.
	prof := cfg.ToProfile()
	if !nameRE.MatchString(prof.Name) {
		return Result{}, fmt.Errorf("phaseregistrar: derived profile name %q must be lowercase kebab-case", prof.Name)
	}

	// A PERSISTED profile with no driver is the fourth instance of the same
	// class (#406): once tracked it fails the repo-contract suites
	// (profiles.TestSmoke_RealProfiles, TestRepoPersonaProfilePairing) and, at
	// dispatch preflight, resolves to no CLI at all. "No known driver" is not a
	// coherence question the auditor can weigh — it is a hard launch failure
	// discovered hours later, so reject the mint instead of writing the stub.
	//
	// Scoped to ProfilesDir != "" deliberately: with persistence off, Register
	// is a pure factory that cannot leak an artifact into the tree, and an
	// absent dispatch block is the legitimate, checked-in shape of every
	// .evolve/phases/*/phase.json — which the campaign study path registers
	// persistence-free (cmd/evolve/cmd_campaign.go:403). Guarding the artifact
	// where the artifact is created keeps that path working.
	if r.ProfilesDir != "" && cfg.Dispatch.CLI == "" {
		return Result{}, fmt.Errorf("phaseregistrar: phase %q has no dispatch.cli; refusing to mint a driverless profile stub", spec.Name)
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

// mintedStubDescriptionFmt / mintedStubWhenToUse are the honest reserved-intent
// SELECT metadata a runtime-minted phase carries when the advisor supplied
// none. They follow the #404 precedent — state what the phase actually is and
// steer the router AWAY from selecting an undefined stub — rather than padding
// phasespec's metadataAllowlist, whose stated remedy is "add metadata, do not
// pad the allowlist" and which is only allowed to shrink.
const (
	mintedStubDescriptionFmt = "Runtime-minted stub for %q; its behavior is defined by the mint-time prompt, not by this catalog entry."
	mintedStubWhenToUse      = "Not selectable on merit: a runtime-minted stub with no advisor-supplied selection criteria. Give it real metadata before relying on it."
)

// nameRE constrains a derived profile name to lowercase kebab-case — the same
// rule phasespec applies to phase names, enforced here on the (advisor-
// controlled) agent-derived profile name so it is a safe filename + lookup key.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
