// Package phaseconfig is the unified descriptor of one pipeline phase: its spec, its dispatch config and its swarm worker count.
// See docs/architecture/packages/internal-phaseconfig.md.
package phaseconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Dispatch struct {
	CLI               string                      `json:"cli,omitempty"`
	CLIFallback       []string                    `json:"cli_fallback,omitempty"`
	CLIFallbackOnExit []int                       `json:"cli_fallback_on_exit,omitempty"`
	ModelTierDefault  string                      `json:"model_tier_default,omitempty"`
	ModelTierEnvelope *profiles.ModelTierEnvelope `json:"model_tier_envelope,omitempty"`
	AllowedCLIs       []string                    `json:"allowed_clis,omitempty"`
	AllowedTools      []string                    `json:"allowed_tools,omitempty"`
	DisallowedTools   []string                    `json:"disallowed_tools,omitempty"`
	PermissionMode    string                      `json:"permission_mode,omitempty"`
	Sandbox           *profiles.SandboxConfig     `json:"sandbox,omitempty"`
	SystemPrompt      string                      `json:"system_prompt,omitempty"`
}

type PhaseConfig struct {
	phasespec.PhaseSpec
	Dispatch     Dispatch `json:"dispatch,omitempty"`
	SwarmWorkers int      `json:"swarm_workers,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
}

func (c PhaseConfig) Spec() phasespec.PhaseSpec { return c.PhaseSpec }

func (c PhaseConfig) ProfileName() string {
	return strings.TrimPrefix(c.AgentName(), "evolve-")
}

func (c PhaseConfig) ToProfile() profiles.Profile {
	d := c.Dispatch
	return profiles.Profile{
		Name: c.ProfileName(),
		Role: c.ProfileName(),

		CLI:               d.CLI,
		AllowedCLIs:       d.AllowedCLIs,
		CLIFallback:       d.CLIFallback,
		CLIFallbackOnExit: d.CLIFallbackOnExit,
		ModelTierDefault:  d.ModelTierDefault,
		ModelTierEnvelope: d.ModelTierEnvelope,
		AllowedTools:      d.AllowedTools,
		DisallowedTools:   d.DisallowedTools,
		Sandbox:           d.Sandbox,
		PermissionMode:    d.PermissionMode,
		SystemPrompt:      d.SystemPrompt,
	}
}

func (c PhaseConfig) PromptBody() (string, bool) {
	return c.Prompt, c.Prompt != ""
}

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
