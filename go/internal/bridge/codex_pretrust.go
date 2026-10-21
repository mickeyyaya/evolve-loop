package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

var tomlKeyEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\b", `\b`,
	"\t", `\t`,
	"\n", `\n`,
	"\f", `\f`,
	"\r", `\r`,
)

type codexConfigTemp interface {
	io.Writer
	Close() error
	Name() string
}

var createCodexConfigTemp = func(dir, pattern string) (codexConfigTemp, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return f, nil
}

var renameCodexConfig = os.Rename

func pretrustCodexProjects(cfg *Config) error {
	paths := codexPretrustPaths(cfg)
	if len(paths) == 0 {
		return nil
	}
	configPath, err := resolveCodexConfigPath(cfg)
	if err != nil {
		return fmt.Errorf("resolve codex config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("ensure codex config dir: %w", err)
	}
	return flock.WithPathLock(configPath, func() error {
		existing, err := os.ReadFile(configPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read codex config: %w", err)
		}
		merged := appendCodexTrustEntries(pruneDeadCodexProjects(string(existing), paths), paths)
		merged = appendCodexNotice(merged)
		if merged == string(existing) {
			return nil
		}
		dir := filepath.Dir(configPath)
		base := filepath.Base(configPath)
		f, err := createCodexConfigTemp(dir, base+".tmp.*")
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
		if err := renameCodexConfig(tmp, configPath); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("rename codex config: %w", err)
		}
		return nil
	})
}

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

func resolveCodexConfigPath(cfg *Config) (string, error) {
	if cfg != nil && cfg.codexConfigPath != "" {
		return cfg.codexConfigPath, nil
	}
	return defaultCodexConfigPath()
}

func codexHome() (string, error) {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

func defaultCodexConfigPath() (string, error) {
	dir, err := codexHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

var codexVersionPathFn func() (string, error) = defaultCodexVersionPath

func defaultCodexVersionPath() (string, error) {
	dir, err := codexHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "version.json"), nil
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

const codexRateLimitNudgeKey = "hide_rate_limit_model_nudge"

func appendCodexNotice(existing string) string {
	if strings.Contains(existing, codexRateLimitNudgeKey) {
		return existing
	}
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if out != "" {
		out += "\n"
	}
	return out + "[notice]\n" + codexRateLimitNudgeKey + " = true\n"
}

func codexProjectHeader(path string) string {
	return fmt.Sprintf(`[projects."%s"]`, tomlKeyEscaper.Replace(path))
}

func pruneDeadCodexProjects(config string, keep []string) string {
	lines := strings.SplitAfter(config, "\n")
	var out strings.Builder
	dropping := false
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "[") {
			dropping = deadScratchProject(trimmed, keep)
		}
		if !dropping {
			out.WriteString(line)
		}
	}
	return out.String()
}

func deadScratchProject(header string, keep []string) bool {
	inner, ok := strings.CutPrefix(header, `[projects.`)
	if !ok || !strings.HasSuffix(inner, "]") {
		return false
	}
	path, err := strconv.Unquote(strings.TrimSuffix(inner, "]"))
	if err != nil || slices.Contains(keep, path) || !scratchProjectPath(path) {
		return false
	}
	_, err = os.Stat(path)
	return os.IsNotExist(err)
}

func scratchProjectPath(path string) bool {
	temp := filepath.Clean(os.TempDir()) + string(filepath.Separator)
	for _, prefix := range []string{temp, "/private" + temp} {
		if rel, ok := strings.CutPrefix(path, prefix); ok {
			return strings.HasPrefix(rel, "Test") || strings.HasPrefix(rel, "evolve-")
		}
	}
	return strings.Contains(path, "/.evolve/worktrees/")
}
