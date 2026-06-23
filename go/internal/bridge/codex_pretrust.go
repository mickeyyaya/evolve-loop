package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/flock"
)

// tomlKeyEscaper escapes the characters TOML basic strings prohibit
// inside table-key quotes. Per TOML spec §2.4, ALL control characters
// (U+0000–U+001F except U+0009 tab, which is technically allowed) must
// be escaped or the file is unparseable. A path that smuggled a literal
// \n through codexProjectHeader would corrupt ~/.codex/config.toml and
// codex would refuse to start — far worse than the modal-stall this
// helper exists to prevent. Per cycle-122 review HIGH-2 finding.
var tomlKeyEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\b", `\b`,
	"\t", `\t`,
	"\n", `\n`,
	"\f", `\f`,
	"\r", `\r`,
)

// pretrustCodexProjects writes per-cycle trust entries into
// ~/.codex/config.toml for cfg.Worktree and cfg.Workspace so codex's own
// permission layer (separate from the bridge's sandbox-exec/bwrap host
// sandbox) treats writes to those paths as allowed and does NOT render
// the "Press enter to confirm" runtime modal that hung cycle-122 tdd.
//
// Cycle-121's research dossier flagged this as "Fix A" deferred at the
// time; cycle-122 made the deferral cost concrete. See
// docs/incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md
// and codex Fix A in
// knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md.
//
// The merge is APPEND-ONLY and idempotent: if a `[projects."<path>"]`
// section is already present, the path is skipped (no duplicate).
//
// Concurrency (ADR-0049 N10): under `evolve fleet` the whole-cycle project
// lock is skipped, so several cycles' codex Preflights pre-trust DISTINCT
// paths into this one host-global file at once. A unique CreateTemp keeps each
// writer's temp private, but the read-merge-write-RENAME was last-writer-wins:
// two cycles that each read a config WITHOUT the other's entry each rename
// their own snapshot, so the final file keeps only the last writer's trust
// entries. The cycle whose entry was dropped then hits the "Press enter to
// confirm" modal that hung cycle-122. The whole RMW now runs under
// flock.WithPathLock(configPath) so the append-only merges serialize and
// compose losslessly — every path stays trusted. (The lock pairs with, it does
// not replace, the atomic CreateTemp+Rename: the lock prevents lost updates,
// the rename keeps lock-free readers tear-free.)
//
// No-ops when cfg.Worktree AND cfg.Workspace are both empty. Returns nil
// (best-effort: a pretrust failure must NOT block phase launch — the
// modal-stall path still defends via Fix 2's extended fallback trigger
// list, and the operator sees the warning on stderr).
//
// Test seam: EVOLVE_CODEX_CONFIG_PATH overrides the resolved path.
func pretrustCodexProjects(cfg *Config) error {
	paths := codexPretrustPaths(cfg)
	if len(paths) == 0 {
		return nil
	}
	configPath, err := resolveCodexConfigPath(cfg)
	if err != nil {
		return fmt.Errorf("resolve codex config path: %w", err)
	}
	// MkdirAll at 0o700 BEFORE the lock: PathLock's own MkdirAll uses 0o755, so
	// creating the dir here first preserves codex's stricter permission.
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("ensure codex config dir: %w", err)
	}
	return flock.WithPathLock(configPath, func() error {
		existing, err := os.ReadFile(configPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read codex config: %w", err)
		}
		merged := appendCodexTrustEntries(string(existing), paths)
		// cycle-142: also suppress codex's "Approaching rate limits / Switch to
		// <mini>?" model-switch modal, which is undismissable by the auto-responder
		// and stalls the phase until the artifact-wait deadline.
		merged = appendCodexNotice(merged)
		if merged == string(existing) {
			return nil // every path already trusted + notice already present
		}
		// os.CreateTemp guarantees a unique filename so lock-free readers never
		// observe a partial write (the lock above serializes our own writers).
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
		// CreateTemp defaults to 0o600 on POSIX, matching the original
		// WriteFile perm choice — no Chmod needed.
		if err := os.Rename(tmp, configPath); err != nil {
			// Best-effort cleanup; the .tmp.* file is harmless if Remove
			// also fails (user-owned dir, dropped on next pretrust attempt).
			_ = os.Remove(tmp)
			return fmt.Errorf("rename codex config: %w", err)
		}
		return nil
	})
}

// codexPretrustPaths returns the non-empty subset of cfg.Worktree +
// cfg.Workspace. Order is deterministic (worktree then workspace) so
// tests can assert exact merge output.
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

// resolveCodexConfigPath returns cfg.codexConfigPath when set, otherwise
// falls back to the default ~/.codex/config.toml.
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

// codexVersionPathFn is the DI seam for resolving the codex version file path.
// Tests replace this var (with t.Cleanup restore) to inject a temporary path.
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

// appendCodexTrustEntries returns existing with one new
// `[projects."<path>"]\ntrust_level = "trusted"\n` block appended per
// path that doesn't already have a section header in existing. The
// check is substring-based — codex's TOML parser is permissive and
// duplicate sections last-win, so a false-negative skip (rare: a
// section header inside a string literal) only causes a benign
// duplicate, never a missing trust entry.
//
// The returned content is guaranteed to end with a newline so future
// appends start on a fresh line.
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

// codexRateLimitNudgeKey is the config.toml key that suppresses codex's
// "Approaching rate limits / Switch to <mini>?" model-switch modal. Without it,
// codex can render an undismissable modal mid-run that stalls the phase until
// the artifact-wait deadline (cycle-142). Per the codex config reference
// ([notice] hide_rate_limit_model_nudge).
const codexRateLimitNudgeKey = "hide_rate_limit_model_nudge"

// appendCodexNotice appends a `[notice]` table setting
// hide_rate_limit_model_nudge = true when existing does not already set that
// key. Append-only + idempotent, mirroring appendCodexTrustEntries. The
// returned content ends with a newline. The key-presence check is substring-
// based (consistent with appendCodexTrustEntries): codex's TOML parser
// last-wins on duplicates, so a rare false-negative only yields a benign
// duplicate, never a missing suppression.
func appendCodexNotice(existing string) string {
	if strings.Contains(existing, codexRateLimitNudgeKey) {
		return existing
	}
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	// Extra blank line before the block: [notice] is a top-level TOML table, so
	// it gets clearer separation from the preceding [projects."…"] sections than
	// the single-line gap appendCodexTrustEntries uses between same-kind entries.
	if out != "" {
		out += "\n"
	}
	return out + "[notice]\n" + codexRateLimitNudgeKey + " = true\n"
}

// codexProjectHeader builds the `[projects."<escaped-path>"]` line for
// path. Uses tomlKeyEscaper to handle the full set of TOML basic-string
// prohibitions (backslash, double-quote, and all control characters per
// TOML §2.4). Typical filesystem paths contain none of these, but a
// path with an embedded newline that smuggled past upstream validation
// would otherwise corrupt config.toml and prevent codex from starting.
func codexProjectHeader(path string) string {
	return fmt.Sprintf(`[projects."%s"]`, tomlKeyEscaper.Replace(path))
}
