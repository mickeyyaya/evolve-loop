package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// driver_common.go — helpers shared by every Driver, factoring out the
// blocks that were copy-pasted across drivers/*.sh (prompt substitution,
// log-file setup, env construction). DRY by design: a new driver reuses
// these instead of re-implementing them.

// driverEnv returns the environment for the inner CLI: the process env
// plus the request-local Deps.Env overrides (later entries win, matching
// the adapter's env-merge).
func driverEnv(deps Deps) []string {
	env := os.Environ()
	for k, v := range deps.Env {
		env = append(env, k+"="+v)
	}
	return env
}

// preparePrompt reads the prompt file and applies the bridge's two
// substitutions — $CHALLENGE_TOKEN (minted via the Deps seam and
// persisted to workspace/challenge-token.txt) and $ARTIFACT_PATH —
// mirroring the identical block in each bash driver.
func preparePrompt(cfg *Config, deps Deps) (string, error) {
	raw, err := os.ReadFile(cfg.PromptFile)
	if err != nil {
		return "", fmt.Errorf("read prompt: %w", err)
	}
	content := string(raw)
	if strings.Contains(content, "$CHALLENGE_TOKEN") {
		tok, err := deps.NewChallengeToken()
		if err != nil {
			return "", fmt.Errorf("mint challenge token: %w", err)
		}
		if err := os.WriteFile(filepath.Join(cfg.Workspace, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("write challenge token: %w", err)
		}
		content = strings.ReplaceAll(content, "$CHALLENGE_TOKEN", tok)
	}
	content = strings.ReplaceAll(content, "$ARTIFACT_PATH", cfg.Artifact)
	return content, nil
}

// openDriverLogs ensures the workspace + log + artifact dirs exist and
// opens the stdout/stderr log files the inner CLI's output is redirected
// to. The returned closeFn must be deferred by the caller.
func openDriverLogs(cfg *Config) (stdoutF, stderrF *os.File, closeFn func(), err error) {
	noop := func() {}
	for _, d := range []string{cfg.Workspace, filepath.Dir(cfg.StdoutLog), filepath.Dir(cfg.StderrLog), filepath.Dir(cfg.Artifact)} {
		if d == "" {
			continue
		}
		if mkErr := os.MkdirAll(d, 0o755); mkErr != nil {
			return nil, nil, noop, fmt.Errorf("mkdir %s: %w", d, mkErr)
		}
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

// orDefault returns s, or def when s is empty — used only for diagnostic
// log strings.
func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// lookupEnv resolves key via the Deps seam, falling back to os.LookupEnv
// when no seam was injected (defensive — LaunchArgs always passes a
// defaulted Deps). Used by the credential-isolation guards.
func lookupEnv(deps Deps, key string) (string, bool) {
	if deps.LookupEnv != nil {
		return deps.LookupEnv(key)
	}
	return os.LookupEnv(key)
}

// fileNonEmpty reports whether path exists and has size > 0 (the bash
// `[[ -s "$f" ]]` artifact-presence test).
func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
