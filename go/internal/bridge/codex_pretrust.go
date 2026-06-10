package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
// Concurrency: a per-call PID-suffixed temp filename
// (config.toml.tmp.<pid>) ensures two simultaneous driver launches
// (different cycles, possibly different paths) cannot clobber each
// other's temp file mid-rename. The Rename ordering is still last-
// writer-wins (an inherent property of POSIX rename), but each writer's
// snapshot is internally consistent — codex's TOML parser tolerates
// duplicate sections (last-wins per its parser), and the next cycle's
// pretrust call re-adds any entry that lost the rename race. Best-
// effort semantics absorb the residual risk. Per cycle-122 review
// HIGH-1 finding.
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
	configPath, err := codexConfigPath()
	if err != nil {
		return fmt.Errorf("resolve codex config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("ensure codex config dir: %w", err)
	}
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
	// os.CreateTemp guarantees a unique filename — required for safe
	// concurrent driver launches (HIGH-1 + MEDIUM-1 from cycle-122
	// review): same-process goroutines share PID, so a PID-suffixed
	// name would collide between goroutines and one Rename would
	// clobber the other's WriteFile-in-progress.
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

// codexConfigPath resolves the TOML path. Honors EVOLVE_CODEX_CONFIG_PATH
// for tests; otherwise ~/.codex/config.toml.
func codexConfigPath() (string, error) {
	if v := os.Getenv("EVOLVE_CODEX_CONFIG_PATH"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func codexVersionPath() (string, error) {
	if v := os.Getenv("EVOLVE_CODEX_VERSION_PATH"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "version.json"), nil
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
