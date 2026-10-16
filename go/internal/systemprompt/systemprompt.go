// Package systemprompt resolves the launch-time system prompt for an agent.
// Per-agent and global environment overrides win over the profile.
package systemprompt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func Resolve(agent, profileDir string, reqEnv map[string]string) string {
	def := profileDefault(agent, profileDir)
	if agent != "" {
		if v := envchain.Resolve(envchain.PhaseEnvKey(agent, "SYSTEM_PROMPT"), reqEnv, "", ""); v != "" {
			return v
		}
	}
	return envchain.ResolveNoOS(envchain.SystemPromptReqEnvKey, reqEnv, def, "")
}

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
	if prof.DigestFile != "" {
		if content, ok := readRelativeFile(profileDir, prof.DigestFile); ok {
			return content
		}
	}
	if prof.SystemPromptFile != "" {
		if content, ok := readRelativeFile(profileDir, prof.SystemPromptFile); ok {
			return content
		}
	}
	return ""
}

func readRelativeFile(dir, path string) (content string, ok bool) {
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(b), "\n"), true
}
