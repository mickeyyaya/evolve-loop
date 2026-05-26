// Package systemprompt resolves the launch-time system prompt / rules for an
// agent (facet B). It mirrors the resolvePolicy precedence chain so the
// per-agent and global env overrides behave identically to the interactive
// policy:
//
//	EVOLVE_<AGENT>_SYSTEM_PROMPT > EVOLVE_SYSTEM_PROMPT
//	  > profile.system_prompt > read(profile.system_prompt_file) > ""
package systemprompt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Resolve returns the effective system prompt for agent. profileDir is the
// directory holding <agent>.json; reqEnv may be nil. A missing/unreadable
// profile contributes an empty default (never an error).
func Resolve(agent, profileDir string, reqEnv map[string]string) string {
	def := profileDefault(agent, profileDir)
	if agent != "" {
		if v := envchain.Resolve(envchain.PhaseEnvKey(agent, "SYSTEM_PROMPT"), reqEnv, "", ""); v != "" {
			return v
		}
	}
	return envchain.Resolve("EVOLVE_SYSTEM_PROMPT", reqEnv, def, "")
}

// profileDefault reads the profile's system_prompt (or system_prompt_file,
// resolved relative to profileDir when not absolute). Inline wins over file.
func profileDefault(agent, profileDir string) string {
	loader := profiles.NewFromDir(profileDir)
	if loader == nil {
		return ""
	}
	prof, err := loader.Get(agent)
	if err != nil {
		return ""
	}
	if prof.SystemPrompt != "" {
		return prof.SystemPrompt
	}
	if prof.SystemPromptFile != "" {
		p := prof.SystemPromptFile
		if !filepath.IsAbs(p) {
			p = filepath.Join(profileDir, p)
		}
		if b, err := os.ReadFile(p); err == nil {
			return strings.TrimRight(string(b), "\n")
		}
	}
	return ""
}
