package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// Profile is the parsed agent profile JSON — the Go port of
// lib/profile.sh's bridge_profile_* exports. Loaded once per launch and
// folded into the resolved Config.
type Profile struct {
	Name           string
	Model          string
	AllowedTools   []string
	PermissionMode string
	StreamOutput   bool
	SessionName    string
	Sandbox        *ProfileSandbox
	// ExtraFlagsByCLI is the per-CLI raw-flag escape hatch (ADR-0022). Flags
	// are keyed by the CLI they belong to ("claude-tmux": [...]) and realized
	// ONLY for the matching CLI, so a claude-origin profile switched to
	// agy/codex realizes none of claude's argv. Replaces the flat extra_flags
	// that forwarded one CLI's vocabulary verbatim to every CLI.
	ExtraFlagsByCLI map[string][]string
}

// ProfileSandbox is the bridge's minimal view of profile.sandbox.
type ProfileSandbox struct {
	AllowNetwork bool
}

// validPermissionModes mirrors the claude --permission-mode choice set
// that bin/bridge and profile.sh both validate against. "" means
// "let the driver/CLI decide" (back-compat with v1 profiles).
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
	ExtraFlagsByCLI map[string][]string `json:"extra_flags_by_cli"`
}

// LoadProfile reads and validates an agent profile JSON, returning the
// parsed Profile. Error messages mirror lib/profile.sh so operator-facing
// diagnostics stay identical across the bash→Go cutover. Validation:
// name required; permission_mode in the allowed set; session_name (when
// set) ≤32 chars and matching [a-zA-Z0-9._-]+.
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
		ExtraFlagsByCLI: w.ExtraFlagsByCLI,
	}
	if w.StreamOutput != nil {
		p.StreamOutput = *w.StreamOutput
	}
	return p, nil
}
