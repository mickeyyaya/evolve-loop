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
		// Read-existing-or-mint: reuse the orchestrator's token written
		// at cycle start (one token per cycle invariant); mint only when
		// the file is absent or empty (e.g. standalone bridge invocation).
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
// doc-compliance variance, an ordered set of fallback locations that get
// relocated to the canonical path (the single source of truth downstream
// phases read). relocatedFrom returns the fallback the artifact was found at
// so the caller can log the normalization. Empty files never count (matching
// fileNonEmpty / the bash `[[ -s ]]` test).
//
// Fallback search order (first non-empty wins):
//  1. <workspace>/workspace/<base> — cycle-108: agents read the doc's
//     "workspace/" prefix as a literal subdir under their cwd.
//  2. <worktree>/<base> — cycle-141 ExitArtifactTimeout: the builder runs with
//     cwd=worktree (driver_tmux_repl.go), and the prompt names the artifact by
//     bare relative path ("Write build-report.md"), so the agent writes it into
//     the worktree root — which the driver did not poll.
//  3. <worktree>/workspace/<base> — the same "workspace/" literal-subdir
//     misread, but relative to the worktree cwd.
//
// Worktree candidates are only searched when cfg.Worktree is set (headless
// drivers / probes leave it empty), so that path is byte-identical to the
// pre-cycle-141 behavior.
//
// When a fallback artifact exists but the relocation fails (e.g. a read-only
// workspace), the error is RETURNED rather than swallowed: a silent (false, "")
// would make the driver spin the full artifact-wait window with no signal,
// hiding a "wrote to the wrong place AND could not be moved" condition from the
// operator. The caller logs it.
// See docs/architecture/adr/0024-conditional-ship-gate-floor-and-phase-advisor.md.
func artifactReady(cfg *Config) (ready bool, relocatedFrom string, err error) {
	if fileNonEmpty(cfg.Artifact) {
		return true, "", nil
	}
	base := filepath.Base(cfg.Artifact)
	candidates := []string{filepath.Join(cfg.Workspace, "workspace", base)}
	if cfg.Worktree != "" {
		candidates = append(candidates,
			filepath.Join(cfg.Worktree, base),
			filepath.Join(cfg.Worktree, "workspace", base),
		)
	}
	for _, fallback := range candidates {
		if fallback == cfg.Artifact || !fileNonEmpty(fallback) {
			continue
		}
		if rerr := relocateFile(fallback, cfg.Artifact); rerr != nil {
			return false, "", fmt.Errorf("relocate %s → %s: %w", fallback, cfg.Artifact, rerr)
		}
		return true, fallback, nil
	}
	return false, "", nil
}

// relocateFile moves src to dst, creating dst's parent directory. It tries an
// atomic rename first and falls back to copy+remove when rename fails (e.g. a
// cross-device move). The copy goes through a "<dst>.tmp.<pid>" temp file in
// dst's directory that is renamed into place, so a write that fails partway
// (ENOSPC, network I/O) never leaves a truncated non-empty file at the
// canonical path — which the poll loop would otherwise read as "ready". This
// mirrors the ${file}.tmp.$$ + mv discipline used across the codebase's
// atomic writers. Used by artifactReady to canonicalize a non-canonical write.
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
	tmp := fmt.Sprintf("%s.tmp.%d", dst, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("relocate: write dst tmp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("relocate: rename tmp into place: %w", err)
	}
	_ = os.Remove(src)
	return nil
}

// wsListMaxDepth/wsListMaxEntries bound the timeout diagnostic so a workspace
// that contains a git worktree (per-cycle worktrees live under the cycle dir)
// cannot flood stderr with thousands of lines. The diagnostic only needs the
// artifact plus one nesting level — the canonical <ws>/<file> and the
// non-canonical <ws>/workspace/<file> are both within depth 2.
const (
	wsListMaxDepth   = 2
	wsListMaxEntries = 200
)

// listWorkspaceFiles returns "relpath (N bytes)" lines for regular files under
// ws. Directories at depth >= wsListMaxDepth are pruned (their contents are
// not walked), and the total is capped at wsListMaxEntries; together these
// keep a per-cycle git worktree from flooding the diagnostic. Used to make an
// artifact-wait timeout self-diagnosing: instead of only reporting the path
// that did NOT appear, the driver lists what the agent actually wrote so an
// operator can see a misplaced artifact at a glance. Best-effort: a walk error
// yields a single diagnostic line rather than failing the caller.
func listWorkspaceFiles(ws string) []string {
	var out []string
	truncated := false
	err := filepath.Walk(ws, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries; keep walking
		}
		rel, relErr := filepath.Rel(ws, path)
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
