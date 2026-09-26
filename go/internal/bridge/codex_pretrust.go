package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// tomlKeyEscaper escapes backslash, quote and the control characters that have a TOML short escape.
// An unescaped newline in a path would corrupt ~/.codex/config.toml and stop codex from starting.
var tomlKeyEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\b", `\b`,
	"\t", `\t`,
	"\n", `\n`,
	"\f", `\f`,
	"\r", `\r`,
)

// pretrustCodexProjects trusts cfg.Worktree and cfg.Workspace in the codex config so codex never renders its
// "Press enter to confirm" modal, and hides the rate-limit model-switch modal. The merge is append-only and
// idempotent. It runs under flock.WithPathLock because concurrent fleet cycles share this host-global file.
// Callers log an error and continue: a pretrust failure must not block a launch.
// See ADR-0049.
func pretrustCodexProjects(cfg *Config) error {
	paths := codexPretrustPaths(cfg)
	if len(paths) == 0 {
		return nil
	}
	configPath, err := resolveCodexConfigPath(cfg)
	if err != nil {
		return fmt.Errorf("resolve codex config path: %w", err)
	}
	// Create the dir at 0o700 before the lock, because PathLock's own MkdirAll would use 0o755.
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("ensure codex config dir: %w", err)
	}
	return flock.WithPathLock(configPath, func() error {
		existing, err := os.ReadFile(configPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read codex config: %w", err)
		}
		merged := appendCodexTrustEntries(string(existing), paths)
		// Also hide codex's rate-limit model-switch modal, which the auto-responder cannot dismiss.
		merged = appendCodexNotice(merged)
		if merged == string(existing) {
			return nil // every path already trusted + notice already present
		}
		// A unique temp file keeps lock-free readers from seeing a partial write.
		dir := filepath.Dir(configPath)
		base := filepath.Base(configPath)
		f, err := os.CreateTemp(dir, base+".tmp.*")
		if err != nil {
			return fmt.Errorf("create codex config tmp: %w", err)
		}
		tmp := f.Name()
		if _, err := f.Write([]byte(merged)); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("write codex config tmp: %w", err)
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("close codex config tmp: %w", err)
		}
		// CreateTemp creates the file 0o600, so no Chmod is needed.
		if err := os.Rename(tmp, configPath); err != nil {
			// Best-effort cleanup; a leftover .tmp.* file is harmless.
			_ = os.Remove(tmp)
			return fmt.Errorf("rename codex config: %w", err)
		}
		return nil
	})
}

// codexPretrustPaths returns cfg.Worktree then cfg.Workspace, skipping empty and duplicate paths.
func codexPretrustPaths(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, 0, 2)
	if cfg.Worktree != "" {
		out = append(out, cfg.Worktree)
	}
	if cfg.Workspace != "" && cfg.Workspace != cfg.Worktree {
		out = append(out, cfg.Workspace)
	}
	return out
}

// resolveCodexConfigPath returns cfg.codexConfigPath when set, else ~/.codex/config.toml.
func resolveCodexConfigPath(cfg *Config) (string, error) {
	if cfg != nil && cfg.codexConfigPath != "" {
		return cfg.codexConfigPath, nil
	}
	return defaultCodexConfigPath()
}

func defaultCodexConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

// codexVersionPathFn is the seam tests replace to redirect the codex version file.
var codexVersionPathFn func() (string, error) = defaultCodexVersionPath

func defaultCodexVersionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "version.json"), nil
}

func codexVersionPath() (string, error) {
	return codexVersionPathFn()
}

func dismissCodexUpdateNag() error {
	path, err := codexVersionPath()
	if err != nil {
		return fmt.Errorf("resolve codex version path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ensure codex version dir: %w", err)
	}
	var state map[string]any
	if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &state)
	}
	if state == nil {
		state = map[string]any{}
	}
	state["dismissed_version"] = "999.999.999"
	state["last_checked_at"] = time.Now().UTC().Format(time.RFC3339)
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode codex version state: %w", err)
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o600)
}

// appendCodexTrustEntries appends a trusted [projects."<path>"] block for each path whose header text is absent.
// Presence is a substring match; the result ends with a newline.
func appendCodexTrustEntries(existing string, paths []string) string {
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	for _, p := range paths {
		header := codexProjectHeader(p)
		if strings.Contains(out, header) {
			continue
		}
		if out != "" {
			out += "\n"
		}
		out += header + "\n" + `trust_level = "trusted"` + "\n"
	}
	return out
}

// codexRateLimitNudgeKey is the [notice] key that hides codex's "Switch to <mini>?" rate-limit modal.
const codexRateLimitNudgeKey = "hide_rate_limit_model_nudge"

// appendCodexNotice appends a [notice] table setting codexRateLimitNudgeKey unless the key text already appears.
func appendCodexNotice(existing string) string {
	if strings.Contains(existing, codexRateLimitNudgeKey) {
		return existing
	}
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	// [notice] is a top-level table, so it gets an extra blank line of separation.
	if out != "" {
		out += "\n"
	}
	return out + "[notice]\n" + codexRateLimitNudgeKey + " = true\n"
}

// codexProjectHeader builds the [projects."<path>"] header with the path TOML-escaped.
func codexProjectHeader(path string) string {
	return fmt.Sprintf(`[projects."%s"]`, tomlKeyEscaper.Replace(path))
}
