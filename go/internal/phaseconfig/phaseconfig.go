// Package phaseconfig is the single, self-describing unit for one pipeline
// phase: its identity + I/O + classify + routing (from phasespec.PhaseSpec),
// its DISPATCH config (the cli/model/tools/sandbox/persona that determine how
// the phase's agent launches), and its per-phase swarm worker count.
//
// This is the unified "phase configuration" the routing advisor emits when it
// mints a phase, and the object the Registrar (Stage 2) decomposes into the
// spec + profile + prompt that the existing loaders already consume. Per the
// approved design (option B): the unified type lives HERE as the API/emission
// shape; the on-disk PhaseSpec + profiles.Profile + agent prompt loaders are
// left untouched and reconstructed via the Spec()/ToProfile()/PromptBody()
// decomposition methods. No churn to the runner, cli_chain, resolvellm, or the
// 16 existing profiles.
//
// Layering: imports phasespec + profiles + stdlib only (phasespec already
// imports config; profiles is stdlib-only). It MUST NOT import core, runner, or
// llmroute — those CONSUME a PhaseConfig (or its decomposed parts), never the
// reverse.
package phaseconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Dispatch is the launch-time configuration for a phase's agent — the subset of
// the legacy profiles.Profile that the dispatch path (llmroute + runner)
// consumes, EXCLUDING fields phasespec.PhaseSpec already carries (Name, Agent,
// Model, Optional, IO, Classify, Routing, After). Reuses the profiles.* value
// types so ToProfile is a direct field copy.
type Dispatch struct {
	CLI               string   `json:"cli,omitempty"`
	CLIFallback       []string `json:"cli_fallback,omitempty"`
	CLIFallbackOnExit []int    `json:"cli_fallback_on_exit,omitempty"`
	// ModelTierDefault is the tier when PhaseSpec.Model is "auto"/unset. Leave
	// empty to let resolvellm derive the tier from the spec model hint.
	ModelTierDefault  string                      `json:"model_tier_default,omitempty"`
	ModelTierEnvelope *profiles.ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
	AllowedCLIs       []string                    `json:"allowed_clis,omitempty"`
	AllowedTools      []string                    `json:"allowed_tools,omitempty"`
	DisallowedTools   []string                    `json:"disallowed_tools,omitempty"`
	PermissionMode    string                      `json:"permission_mode,omitempty"`
	Sandbox           *profiles.SandboxConfig     `json:"sandbox,omitempty"`
	EffortLevel       string                      `json:"effort_level,omitempty"`
	// SystemPrompt is the per-agent launch-time rules block ("persona"). Inline
	// only here — a minted phase carries its persona in-band, so it needs no
	// system_prompt_file on disk.
	SystemPrompt string `json:"system_prompt,omitempty"`
}

// PhaseConfig is the unified phase descriptor (option B): the embedded
// phasespec.PhaseSpec supplies identity + sequence position (After) + IO +
// classify + routing + the model hint; Dispatch supplies the launch config;
// SwarmWorkers is the per-phase parallel worker count; Prompt optionally
// carries the agent prompt body in-band.
type PhaseConfig struct {
	phasespec.PhaseSpec
	Dispatch Dispatch `json:"dispatch,omitempty"`
	// SwarmWorkers is the per-phase parallel worker count for the swarm harness.
	// 0 = use the global default (EVOLVE_SWARM_CONCURRENCY); >1 fans the phase
	// out to that many workers (writers require disjoint file ownership — see
	// the swarm harness). This is the per-phase knob the global flag lacked.
	SwarmWorkers int `json:"swarm_workers,omitempty"`
	// Prompt is the agent prompt body, in-band. Empty means "load agents/<agent>.md
	// by name" (the existing prompts.Loader path); non-empty lets a minted phase
	// ship its prompt without a file on disk.
	Prompt string `json:"prompt,omitempty"`
}

// Spec returns the embedded phasespec.PhaseSpec — the routing/IO/classify view
// the catalog + orchestrator already understand.
func (c PhaseConfig) Spec() phasespec.PhaseSpec { return c.PhaseSpec }

// ProfileName is the canonical profile/agent key for this phase (the
// "evolve-" prefix stripped from the agent name), matching the runner's
// profileName derivation.
func (c PhaseConfig) ProfileName() string {
	return strings.TrimPrefix(c.AgentName(), "evolve-") // PhaseSpec default: "evolve-<name>"
}

// ToProfile maps the DISPATCH-RELEVANT subset of this config into a
// profiles.Profile (cli/fallback/tier/envelope/allowed_clis/tools/permission/
// sandbox/persona/budget/effort) so a minted PhaseConfig dispatches through the
// unchanged runner/llmroute path. It is intentionally PARTIAL: Profile fields
// not modeled in Dispatch (MaxTurns, ParallelEligible, OutputArtifact,
// ResearchQuota, ModelTierOverrides, AddDir, …) take their zero
// value, which is a safe conservative default for a minted phase (e.g.
// ParallelEligible=false honors the single-writer invariant; OutputArtifact is
// derived from spec.Outputs by specrunner). The Stage-2 Registrar overlays this
// onto a template profile when a minted phase needs a non-default for one of
// those fields — do NOT assume ToProfile alone is a complete profile.
func (c PhaseConfig) ToProfile() profiles.Profile {
	d := c.Dispatch
	return profiles.Profile{
		Name: c.ProfileName(),
		Role: c.ProfileName(), // mirrors Name; Dispatch carries no separate role field

		CLI:               d.CLI,
		AllowedCLIs:       d.AllowedCLIs,
		CLIFallback:       d.CLIFallback,
		CLIFallbackOnExit: d.CLIFallbackOnExit,
		ModelTierDefault:  d.ModelTierDefault,
		ModelTierEnvelope: d.ModelTierEnvelope,
		AllowedTools:      d.AllowedTools,
		DisallowedTools:   d.DisallowedTools,
		Sandbox:           d.Sandbox,
		EffortLevel:       d.EffortLevel,
		PermissionMode:    d.PermissionMode,
		SystemPrompt:      d.SystemPrompt,
	}
}

// PromptBody returns the in-band agent prompt and whether one was supplied
// (false → the caller loads agents/<agent>.md by name, the legacy path).
func (c PhaseConfig) PromptBody() (string, bool) {
	return c.Prompt, c.Prompt != ""
}

// Load reads a unified phase-config JSON at path into a PhaseConfig. A missing
// or empty Name is an error (a phase with no identity can't be routed or
// dispatched).
func Load(path string) (PhaseConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PhaseConfig{}, fmt.Errorf("phaseconfig: read %s: %w", path, err)
	}
	var pc PhaseConfig
	if err := json.Unmarshal(raw, &pc); err != nil {
		return PhaseConfig{}, fmt.Errorf("phaseconfig: parse %s: %w", path, err)
	}
	if pc.Name == "" {
		return PhaseConfig{}, fmt.Errorf("phaseconfig: %s has empty name", path)
	}
	return pc, nil
}
