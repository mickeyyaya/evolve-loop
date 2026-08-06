// Package systemprompt resolves the launch-time system prompt / rules for an
// agent (facet B). It mirrors the resolvePolicy precedence chain so the
// per-agent and global env overrides behave identically to the interactive
// policy:
//
//	EVOLVE_<AGENT>_SYSTEM_PROMPT > EVOLVE_SYSTEM_PROMPT
//	  > profile.system_prompt > read(profile.digest_file) > read(profile.system_prompt_file) > ""
//
// digest_file (cycle-1391, tokenopt-role-scoped-instruction-digests Task 2)
// names a pre-generated role-scoped digest (go/internal/digest output). When
// set AND present on disk it wins over system_prompt_file; when unset, or
// set but the file is absent, the pre-cycle-1391 chain is unchanged.
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
	return envchain.ResolveNoOS(envchain.SystemPromptReqEnvKey, reqEnv, def, "")
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

// readRelativeFile reads path (resolved relative to dir when not absolute)
// and returns its content with trailing newlines trimmed. ok is false when
// the file cannot be read, so callers can fall through to the next tier in
// the precedence chain instead of returning empty.
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
