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

// resolveBinary returns the executable name for a driver's inner CLI,
// honoring the offline testing seam ported from the bash bridge: when
// BRIDGE_TESTING=1, a BRIDGE_<CLI>_BINARY override (e.g.
// BRIDGE_CLAUDE_BINARY) substitutes the real binary with a fake/stub so
// the e2e harness can drive the full cycle path without a live CLI.
// Outside testing the default name is always used, so a stray override in
// a production environment can never redirect a real launch.
//
// defaultName must be a base binary name (claude|codex|agy) — NOT a driver
// alias like "claude-tmux" — so the derived env key (BRIDGE_<UPPER>_BINARY)
// is a valid shell variable. All six drivers pass the base name.
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

// ensureDirs creates the workspace + log + artifact parent directories.
// Shared by openDriverLogs (headless drivers) and runTmuxREPL.
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

// openDriverLogs ensures the workspace + log + artifact dirs exist and
// opens the stdout/stderr log files the inner CLI's output is redirected
// to. The returned closeFn must be deferred by the caller.
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

// artifactReady reports whether the phase artifact is present and non-empty.
// It accepts the canonical cfg.Artifact path and, as a tolerance for agent
// doc-compliance variance, a single fallback: <workspace>/workspace/<basename>.
// Some agents read the "workspace/" prefix in the agent docs as a literal
// subdir under their cwd rather than the workspace directory, writing
// <workspace>/workspace/<file>; the driver only polls the canonical path, so
// that write caused the cycle-108 ExitArtifactTimeout. When the artifact is
// found in the fallback location it is relocated to the canonical path (the
// single source of truth that downstream phases read), and relocatedFrom
// returns the fallback path so the caller can log the normalization. Empty
// files never count (matching fileNonEmpty / the bash `[[ -s ]]` test).
// See docs/architecture/adr/0024-conditional-ship-gate-floor-and-phase-advisor.md.
func artifactReady(cfg *Config) (ready bool, relocatedFrom string) {
	if fileNonEmpty(cfg.Artifact) {
		return true, ""
	}
	fallback := filepath.Join(cfg.Workspace, "workspace", filepath.Base(cfg.Artifact))
	if fallback != cfg.Artifact && fileNonEmpty(fallback) {
		if err := relocateFile(fallback, cfg.Artifact); err == nil {
			return true, fallback
		}
	}
	return false, ""
}

// relocateFile moves src to dst, creating dst's parent directory. It tries an
// atomic rename first and falls back to copy+remove when rename fails (e.g. a
// cross-device move). Used by artifactReady to canonicalize a non-canonical
// artifact write.
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
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("relocate: write dst: %w", err)
	}
	_ = os.Remove(src)
	return nil
}

// listWorkspaceFiles returns "relpath (N bytes)" lines for every regular file
// under ws (recursing one workspace/ level deep). Used to make an
// artifact-wait timeout self-diagnosing: instead of only reporting the path
// that did NOT appear, the driver lists what the agent actually wrote so an
// operator can see a misplaced artifact at a glance. Best-effort: a walk error
// yields a single diagnostic line rather than failing the caller.
func listWorkspaceFiles(ws string) []string {
	var out []string
	err := filepath.Walk(ws, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries; keep walking
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(ws, path)
		if relErr != nil {
			rel = path
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
	return out
}
