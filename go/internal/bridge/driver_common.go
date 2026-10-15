package bridge

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func resolveBinary(deps Deps, defaultName string) string {
	if v, _ := lookupEnv(deps, "BRIDGE_TESTING"); v != "1" {
		return defaultName
	}
	key := "BRIDGE_" + strings.ToUpper(defaultName) + "_BINARY"
	if v, ok := lookupEnv(deps, key); ok && v != "" {
		return v
	}
	return defaultName
}

func driverEnv(deps Deps, cliEnv map[string]string) []string {
	env := slices.DeleteFunc(os.Environ(), isDispatchTag)
	for _, k := range slices.Sorted(maps.Keys(cliEnv)) {
		env = append(env, k+"="+cliEnv[k])
	}
	for k, v := range deps.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func exportLines(env map[string]string) []string {
	var lines []string
	for _, k := range slices.Sorted(maps.Keys(env)) {
		lines = append(lines, "export "+k+"="+shellQuotePOSIX(env[k]))
	}
	return lines
}

func preparePrompt(cfg *Config, deps Deps) (string, error) {
	raw, err := os.ReadFile(cfg.PromptFile)
	if err != nil {
		return "", fmt.Errorf("read prompt: %w", err)
	}
	content := string(raw)
	if strings.Contains(content, "$CHALLENGE_TOKEN") {
		var tok string
		if existing, err := os.ReadFile(filepath.Join(cfg.Workspace, "challenge-token.txt")); err == nil {
			if v := strings.TrimSpace(string(existing)); v != "" {
				tok = v
			}
		}
		if tok == "" {
			minted, err := deps.NewChallengeToken()
			if err != nil {
				return "", fmt.Errorf("mint challenge token: %w", err)
			}
			if err := os.WriteFile(filepath.Join(cfg.Workspace, "challenge-token.txt"), []byte(minted+"\n"), 0o644); err != nil {
				return "", fmt.Errorf("write challenge token: %w", err)
			}
			tok = minted
		}
		content = strings.ReplaceAll(content, "$CHALLENGE_TOKEN", tok)
	}
	content = strings.ReplaceAll(content, "$ARTIFACT_PATH", cfg.Artifact)
	return content, nil
}

func ensureDirs(cfg *Config) error {
	for _, d := range []string{cfg.Workspace, filepath.Dir(cfg.StdoutLog), filepath.Dir(cfg.StderrLog), filepath.Dir(cfg.Artifact)} {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func openDriverLogs(cfg *Config) (stdoutF, stderrF *os.File, closeFn func(), err error) {
	noop := func() {}
	if mkErr := ensureDirs(cfg); mkErr != nil {
		return nil, nil, noop, mkErr
	}
	stdoutF, err = os.Create(cfg.StdoutLog)
	if err != nil {
		return nil, nil, noop, fmt.Errorf("create stdout log: %w", err)
	}
	stderrF, err = os.Create(cfg.StderrLog)
	if err != nil {
		_ = stdoutF.Close()
		return nil, nil, noop, fmt.Errorf("create stderr log: %w", err)
	}
	return stdoutF, stderrF, func() { _ = stdoutF.Close(); _ = stderrF.Close() }, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func lookupEnv(deps Deps, key string) (string, bool) {
	if v, ok := deps.Env[key]; ok {
		return v, true
	}
	if deps.LookupEnv != nil {
		return deps.LookupEnv(key)
	}
	return os.LookupEnv(key)
}

func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

func regularFileNonEmpty(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Size() > 0
}

func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func artifactReady(cfg *Config) (ready bool, relocatedFrom string, err error) {
	return artifactCanonicalize(cfg, relocateFile)
}

func artifactCanonicalize(cfg *Config, move func(src, dst string) error) (ready bool, relocatedFrom string, err error) {
	path, found := artifactLocate(cfg)
	if !found {
		return false, "", nil
	}
	if path == cfg.Artifact {
		return true, "", nil
	}
	if rerr := move(path, cfg.Artifact); rerr != nil {
		return false, "", fmt.Errorf("relocate %s → %s: %w", path, cfg.Artifact, rerr)
	}
	return true, path, nil
}

func artifactLocate(cfg *Config) (path string, found bool) {
	for _, c := range artifactCandidatePaths(cfg) {
		if regularFileNonEmpty(c) {
			return c, true
		}
	}
	return "", false
}

func artifactCandidatePaths(cfg *Config) []string {
	out := []string{cfg.Artifact}
	base := filepath.Base(cfg.Artifact)
	candidates := []string{filepath.Join(cfg.Workspace, "workspace", base)}
	if cfg.Worktree != "" {
		candidates = append(candidates,
			filepath.Join(cfg.Worktree, base),
			filepath.Join(cfg.Worktree, "workspace", base),
		)
	}
	for _, c := range candidates {
		if c != cfg.Artifact {
			out = append(out, c)
		}
	}
	return out
}

type relocateTempFile interface {
	Write([]byte) (int, error)
	Close() error
	Name() string
}

var relocateCreateTemp = func(dir, pattern string) (relocateTempFile, error) { return os.CreateTemp(dir, pattern) }

var relocateChmod = os.Chmod

func relocateFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("relocate: mkdir dst dir: %w", err)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("relocate: read src: %w", err)
	}
	tf, err := relocateCreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp.*")
	if err != nil {
		return fmt.Errorf("relocate: create dst tmp: %w", err)
	}
	tmp := tf.Name()
	if _, werr := tf.Write(data); werr != nil {
		_ = tf.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("relocate: write dst tmp: %w", werr)
	}
	if cerr := tf.Close(); cerr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("relocate: close dst tmp: %w", cerr)
	}
	if err := relocateChmod(tmp, 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("relocate: chmod dst tmp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("relocate: rename tmp into place: %w", err)
	}
	_ = os.Remove(src)
	return nil
}

func renameOnlyRelocate(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("relocate (rename-only): mkdir dst dir: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("relocate (rename-only): %w — refusing the copy+remove "+
			"fallback because this artifact was never observed to settle", err)
	}
	return nil
}

const (
	wsListMaxDepth   = 2
	wsListMaxEntries = 200
)

var (
	walkWorkspace         = filepath.Walk
	relativeWorkspacePath = filepath.Rel
)

func listWorkspaceFiles(ws string) []string {
	var out []string
	truncated := false
	err := walkWorkspace(ws, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := relativeWorkspacePath(ws, path)
		if relErr != nil {
			rel = path
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator))
		}
		if info.IsDir() {
			if depth >= wsListMaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= wsListMaxEntries {
			truncated = true
			return filepath.SkipAll
		}
		out = append(out, fmt.Sprintf("%s (%d bytes)", rel, info.Size()))
		return nil
	})
	if err != nil {
		return []string{fmt.Sprintf("(walk error: %v)", err)}
	}
	if len(out) == 0 {
		return []string{"(workspace is empty)"}
	}
	if truncated {
		out = append(out, fmt.Sprintf("(… truncated at %d entries / depth %d)", wsListMaxEntries, wsListMaxDepth))
	}
	return out
}
