package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Profile struct {
	Name            string
	Model           string
	AllowedTools    []string
	DisallowedTools []string
	PermissionMode  string
	StreamOutput    bool
	SessionName     string
	Sandbox         *ProfileSandbox
	ExtraFlagsByCLI map[string][]string
}

const untieredLaunchTier = "balanced"

func LaunchEffort(cli, model string, efforts policy.EffortTable, agents ...string) string {
	if model == "" || model == "auto" {
		model = manifestDefaultTier(cli)
	}
	effort, _ := efforts.Resolve(legacyTierAlias(model), agents...)
	return effort
}

func manifestDefaultTier(cli string) string {
	if m, err := LoadManifest(cli); err == nil && m.Params["model_tier"].Default != "" {
		return m.Params["model_tier"].Default
	}
	return untieredLaunchTier
}

type ProfileSandbox = profiles.SandboxConfig

var validPermissionModes = map[string]bool{
	"":                  true,
	"plan":              true,
	"default":           true,
	"acceptEdits":       true,
	"bypassPermissions": true,
	"auto":              true,
	"dontAsk":           true,
}

var sessionNameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type profileWire struct {
	Name            string              `json:"name"`
	Model           string              `json:"model"`
	AllowedTools    []string            `json:"allowed_tools"`
	DisallowedTools []string            `json:"disallowed_tools"`
	PermissionMode  string              `json:"permission_mode"`
	StreamOutput    *bool               `json:"stream_output"`
	SessionName     string              `json:"session_name"`
	Sandbox         *ProfileSandbox     `json:"sandbox"`
	EffortLevel     json.RawMessage     `json:"effort_level"`
	EffortOverrides json.RawMessage     `json:"effort_overrides"`
	ExtraFlagsByCLI map[string][]string `json:"extra_flags_by_cli"`
}

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

	if w.EffortLevel != nil || w.EffortOverrides != nil {
		return Profile{}, fmt.Errorf("bridge:profile: %s: %w", path, profiles.ErrRetiredEffortField)
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

	allowed, disallowed, err := expandToolPolicies(path, w.AllowedTools, w.DisallowedTools)
	if err != nil {
		return Profile{}, fmt.Errorf("bridge:profile: %w", err)
	}
	if i := slices.IndexFunc(allowed, func(t string) bool { return slices.Contains(disallowed, t) }); i >= 0 {
		return Profile{}, fmt.Errorf("bridge:profile: %q is in allowed_tools and disallowed_tools (in %s); Claude Code applies the deny, so remove one", allowed[i], path)
	}

	p := Profile{
		Name:            w.Name,
		Model:           w.Model,
		AllowedTools:    allowed,
		DisallowedTools: disallowed,
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

func expandToolPolicies(path string, allowed, disallowed []string) ([]string, []string, error) {
	if !hasToolPolicySentinel(allowed) && !hasToolPolicySentinel(disallowed) {
		return allowed, disallowed, nil
	}
	expanded, err := profiles.NewFromDir(filepath.Dir(path)).Get(strings.TrimSuffix(filepath.Base(path), ".json"))
	if err != nil {
		return nil, nil, err
	}
	return expanded.AllowedTools, expanded.DisallowedTools, nil
}

func hasToolPolicySentinel(tools []string) bool {
	return slices.ContainsFunc(tools, func(t string) bool { return strings.HasPrefix(t, "$include_policy:") })
}
