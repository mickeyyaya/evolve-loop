package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Profile is the parsed agent profile JSON, loaded once per launch and folded into the resolved Config.
type Profile struct {
	Name           string
	Model          string
	AllowedTools   []string
	PermissionMode string
	StreamOutput   bool
	SessionName    string
	Sandbox        *ProfileSandbox
	// EffortLevel is the abstract reasoning-effort dial (low | medium | high) carried from the profile JSON and realized per-CLI via LaunchIntent.Effort.
	EffortLevel string
	// ExtraFlagsByCLI is the per-CLI raw-flag escape hatch, realized only for
	// the matching CLI so a profile switched to another CLI carries none of
	// the original CLI's argv.
	// See ADR-0022.
	ExtraFlagsByCLI map[string][]string
	// EffortOverrides maps a resolved model tier to the effort rung to launch with; an absent tier key falls back to EffortLevel.
	EffortOverrides map[string]string
}

func (p Profile) effortForTier(tier string) string {
	if e, ok := p.EffortOverrides[tier]; ok && e != "" {
		return e
	}
	return p.EffortLevel
}

// ProfileSandbox shares the canonical profile schema so launch cannot silently
// drop filesystem restrictions parsed by the profile loader.
type ProfileSandbox = profiles.SandboxConfig

// validPermissionModes mirrors claude's --permission-mode set; "" means
// "let the driver/CLI decide" (v1 profile back-compat).
var validPermissionModes = map[string]bool{
	"":                  true,
	"plan":              true,
	"default":           true,
	"acceptEdits":       true,
	"bypassPermissions": true,
	"auto":              true,
	"dontAsk":           true,
}

// sessionNameRE matches the safe tmux session-name charset (no shell
// metachars), mirroring profile.sh + bin/bridge.
var sessionNameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// profileWire is the JSON-facing shape. stream_output is a *bool so a
// non-boolean JSON value surfaces as an unmarshal error (the bash side
// rejects non-bool stream_output too); absent → nil → default false.
type profileWire struct {
	Name            string              `json:"name"`
	Model           string              `json:"model"`
	AllowedTools    []string            `json:"allowed_tools"`
	PermissionMode  string              `json:"permission_mode"`
	StreamOutput    *bool               `json:"stream_output"`
	SessionName     string              `json:"session_name"`
	Sandbox         *ProfileSandbox     `json:"sandbox"`
	EffortLevel     string              `json:"effort_level"`
	EffortOverrides map[string]string   `json:"effort_overrides"`
	ExtraFlagsByCLI map[string][]string `json:"extra_flags_by_cli"`
}

// LoadProfile reads and validates an agent profile JSON, returning the parsed Profile.
func LoadProfile(path string) (Profile, error) {
	if path == "" {
		return Profile{}, fmt.Errorf("bridge:profile: empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, fmt.Errorf("bridge:profile: file not found: %s", path)
		}
		return Profile{}, fmt.Errorf("bridge:profile: read %s: %w", path, err)
	}
	var w profileWire
	if err := json.Unmarshal(data, &w); err != nil {
		return Profile{}, fmt.Errorf("bridge:profile: invalid JSON: %s", path)
	}

	if w.Name == "" {
		return Profile{}, fmt.Errorf("bridge:profile: missing required field: name (in %s)", path)
	}
	if !validPermissionModes[w.PermissionMode] {
		return Profile{}, fmt.Errorf(
			"bridge:profile: invalid permission_mode '%s' (in %s); valid: plan, default, acceptEdits, bypassPermissions, auto, dontAsk",
			w.PermissionMode, path)
	}
	if w.SessionName != "" {
		if len(w.SessionName) > 32 {
			return Profile{}, fmt.Errorf("bridge:profile: invalid session_name (in %s) — max 32 chars (got %d)", path, len(w.SessionName))
		}
		if !sessionNameRE.MatchString(w.SessionName) {
			return Profile{}, fmt.Errorf("bridge:profile: invalid session_name '%s' (in %s) — must match [a-zA-Z0-9._-]+", w.SessionName, path)
		}
	}

	p := Profile{
		Name:            w.Name,
		Model:           w.Model,
		AllowedTools:    w.AllowedTools,
		PermissionMode:  w.PermissionMode,
		SessionName:     w.SessionName,
		Sandbox:         w.Sandbox,
		EffortLevel:     w.EffortLevel,
		EffortOverrides: w.EffortOverrides,
		ExtraFlagsByCLI: w.ExtraFlagsByCLI,
	}
	if w.StreamOutput != nil {
		p.StreamOutput = *w.StreamOutput
	}
	return p, nil
}
