package bridge

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// resolveBinary returns the executable name for a driver's inner CLI, honoring the offline testing seam:
// when BRIDGE_TESTING=1, a BRIDGE_<CLI>_BINARY override (e.g. BRIDGE_CLAUDE_BINARY) substitutes the real
// binary with a fake/stub so the e2e harness can drive the full cycle path without a live CLI. Outside
// testing the default name is always used, so a stray override in production can never redirect a real
// launch. defaultName must be a base binary name (claude|codex|agy), not a driver alias like
// "claude-tmux", so the derived env key is a valid shell variable.
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

// driverEnv returns the environment for the inner CLI: the process env, the CLI's own variables
// (manifest default_env, sorted), then the request-local Deps.Env overrides (later entries win,
// matching the adapter's env-merge).
func driverEnv(deps Deps, cliEnv map[string]string) []string {
	env := os.Environ()
	for _, k := range slices.Sorted(maps.Keys(cliEnv)) {
		env = append(env, k+"="+cliEnv[k])
	}
	for k, v := range deps.Env {
		env = append(env, k+"="+v)
	}
	return env
}

// exportLines renders env as the `export KEY=value` lines a tmux driver sends to the pane shell before
// the launch command, sorted so the pane transcript is stable.
func exportLines(env map[string]string) []string {
	var lines []string
	for _, k := range slices.Sorted(maps.Keys(env)) {
		lines = append(lines, "export "+k+"="+shellQuotePOSIX(env[k]))
	}
	return lines
}

// preparePrompt reads the prompt file and applies the bridge's two substitutions: $CHALLENGE_TOKEN (minted
// via the Deps seam and persisted to workspace/challenge-token.txt) and $ARTIFACT_PATH.
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

// lookupEnv resolves key against the same environment the inner CLI sees: the request-local Deps.Env
// overlay first, then the Deps.LookupEnv seam, then os.LookupEnv as a defensive fallback.
func lookupEnv(deps Deps, key string) (string, bool) {
	if v, ok := deps.Env[key]; ok {
		return v, true
	}
	if deps.LookupEnv != nil {
		return deps.LookupEnv(key)
	}
	return os.LookupEnv(key)
}

// fileNonEmpty reports whether path exists and has size > 0.
func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// regularFileNonEmpty is fileNonEmpty for paths about to be promoted into a committed deliverable: it
// Lstats rather than following links, and only a non-empty regular file qualifies, so a planted symlink
// (or device, FIFO or directory) can't redirect the promotion. fileNonEmpty's os.Stat follows links by
// design; its callers only ask whether the thing at the other end exists.
func regularFileNonEmpty(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Size() > 0
}

// IsDir reports whether path is an existing directory. Exported because the fleet worktree guard
// (driver_tmux_repl.go: `if !IsDir(workingDir)` → ExitBadFlags) is a launch-refusal predicate that callers
// outside this package must test against before dispatching, rather than re-deriving it and drifting from
// the guard: one predicate, one definition.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// artifactReady reports whether the phase artifact is present and non-empty. It accepts the canonical
// cfg.Artifact path and an ordered set of fallback locations (a tolerance for agent doc-compliance
// variance) that get relocated to the canonical path; relocatedFrom returns the fallback the artifact was
// found at so the caller can log the normalization. When a fallback exists but the relocation fails, the
// error is returned rather than swallowed, so the driver doesn't spin the full artifact-wait window with
// no signal.
// See ADR-0024.
func artifactReady(cfg *Config) (ready bool, relocatedFrom string, err error) {
	return artifactCanonicalize(cfg, relocateFile)
}

// artifactCanonicalize is artifactReady with the mover injected, so a caller that has not confirmed the
// artifact settled can restrict which relocation semantics are allowed to run; move is only ever consulted
// for a non-canonical artifact. relocateFile (rename, falling back to copy+remove) is correct only for a
// file already observed to have stopped changing; renameOnlyRelocate (rename or fail) is safe for a file
// that may still be growing, since rename preserves the inode an agent's open fd keeps appending to.
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

// artifactLocate reports where the phase artifact currently is — the canonical path when it holds a
// non-empty file, otherwise the first non-empty fallback in artifactReady's search order — without moving
// anything; it is the read-only half of artifactReady. The split exists because relocation is irreversible
// and, on relocateFile's copy+remove branch, destructive, so artifactDetector runs its stability window
// against this read-only answer and only relocates once the file has settled.
func artifactLocate(cfg *Config) (path string, found bool) {
	for _, c := range artifactCandidatePaths(cfg) {
		if regularFileNonEmpty(c) {
			return c, true
		}
	}
	return "", false
}

// artifactCandidatePaths is the full ordered set of locations artifactLocate can answer from: canonical
// first, then the fallbacks. Single-sourced so the pre-dispatch baseline (captureArtifactBaseline)
// snapshots exactly the locations completion can later certify, so a stray fallback shadowed by the
// canonical at capture time can't launder through if the canonical vanishes mid-session.
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

// relocateFile moves src to dst, creating dst's parent directory. It tries an atomic rename first and
// falls back to copy+remove when rename fails (e.g. a cross-device move), through a temp file in dst's
// directory that is renamed into place, so a write that fails partway never leaves a truncated non-empty
// file at the canonical path. The temp file is created with os.CreateTemp (an unpredictable suffix) rather
// than a predictable "<dst>.tmp.<pid>" name, so a planted symlink at a guessable name can't turn this copy
// into an arbitrary-write primitive. Used by artifactReady to canonicalize a write already observed to
// have stopped changing; callers that have not established that must use renameOnlyRelocate instead.
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
	tf, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp.*")
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
	if err := os.Chmod(tmp, 0o644); err != nil { // CreateTemp defaults to 0600; match the destination's 0644
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

// renameOnlyRelocate canonicalizes src → dst with rename semantics only: if the rename cannot be done (a
// cross-device src, an unwritable source directory) it reports the error instead of degrading to
// relocateFile's copy+remove. This is the mover for a caller that has not confirmed the artifact stopped
// changing, since rename is the one canonicalization safe for a file still being written — it relinks the
// same inode, so an agent's open fd keeps appending at the new path — whereas copy+remove would snapshot a
// half-written file and then delete the original.
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

// wsListMaxDepth/wsListMaxEntries bound the timeout diagnostic so a workspace containing a git worktree
// can't flood stderr with thousands of lines; the diagnostic only needs the artifact plus one nesting
// level, since the canonical <ws>/<file> and the non-canonical <ws>/workspace/<file> are both within depth 2.
const (
	wsListMaxDepth   = 2
	wsListMaxEntries = 200
)

// listWorkspaceFiles returns "relpath (N bytes)" lines for regular files under ws, pruning directories at
// depth >= wsListMaxDepth and capping the total at wsListMaxEntries. It makes an artifact-wait timeout
// self-diagnosing: instead of only reporting the path that did not appear, it lists what the agent actually
// wrote. Best-effort: a walk error yields a single diagnostic line rather than failing the caller.
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
