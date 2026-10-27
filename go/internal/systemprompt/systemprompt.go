// Package systemprompt resolves the launch-time system prompt for an agent.
package systemprompt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func Resolve(agent, profileDir string) string {
	return profileDefault(agent, profileDir)
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
