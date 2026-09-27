package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// marshalIndent is json.MarshalIndent behind a var, so AddRule's otherwise-unreachable marshal-error path
// is testable.
var marshalIndent = json.MarshalIndent

// AppendInteractiveRule validates rule (name, regex, policy required; policy ∈ {auto_respond, escalate};
// auto_respond needs keys; duplicate name rejected) and returns a new slice with it appended.
func AppendInteractiveRule(prompts []ManifestPrompt, rule ManifestPrompt) ([]ManifestPrompt, error) {
	if rule.Name == "" || rule.Regex == "" || rule.Policy == "" {
		return nil, fmt.Errorf("bridge:add-rule: name, regex, policy are required")
	}
	switch rule.Policy {
	case "auto_respond", "escalate":
	default:
		return nil, fmt.Errorf("bridge:add-rule: invalid policy %q (want auto_respond|escalate)", rule.Policy)
	}
	if rule.Policy == "auto_respond" && rule.ResponseKeys == "" {
		return nil, fmt.Errorf("bridge:add-rule: policy=auto_respond requires non-empty response_keys")
	}
	for _, p := range prompts {
		if p.Name == rule.Name {
			return nil, fmt.Errorf("bridge:add-rule: rule named %q already exists", rule.Name)
		}
	}
	return append(append([]ManifestPrompt(nil), prompts...), rule), nil
}

// AddRule loads cli's current manifest (override or embedded), appends rule, and writes the result to the
// override dir, returning the written path.
func AddRule(cli string, rule ManifestPrompt) (string, error) {
	m, err := LoadManifest(cli)
	if err != nil {
		return "", err
	}
	updated, err := AppendInteractiveRule(m.InteractivePrompts, rule)
	if err != nil {
		return "", err
	}
	m.InteractivePrompts = updated

	dir := bridgeManifestDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("bridge:add-rule: mkdir override dir: %w", err)
	}
	b, err := marshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("bridge:add-rule: marshal manifest: %w", err)
	}
	path := filepath.Join(dir, cli+".json")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("bridge:add-rule: write %s: %w", path, err)
	}
	return path, nil
}
