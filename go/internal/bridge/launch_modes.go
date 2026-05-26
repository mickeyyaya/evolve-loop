package bridge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// launch_modes.go — the non-dispatch launch modes from bin/bridge
// cmd_launch: --validate-only (print resolved config), --dry-run (mock
// outputs, no LLM), and the --require-full tier precheck.

// printResolvedConfig writes the resolved launch configuration block to
// w, mirroring bin/bridge's `--validate-only` output. Returns ExitOK.
func printResolvedConfig(w io.Writer, cfg *Config, prof Profile) {
	fmt.Fprintln(w, "[bridge] validate-only — resolved config:")
	fmt.Fprintf(w, "  cli             = %s\n", cfg.CLI)
	fmt.Fprintf(w, "  profile         = %s\n", cfg.Profile)
	fmt.Fprintf(w, "  profile.name    = %s\n", prof.Name)
	fmt.Fprintf(w, "  profile.model   = %s\n", prof.Model)
	fmt.Fprintf(w, "  model           = %s\n", cfg.Model)
	fmt.Fprintf(w, "  prompt          = %s\n", cfg.PromptFile)
	fmt.Fprintf(w, "  workspace       = %s\n", cfg.Workspace)
	fmt.Fprintf(w, "  stdout-log      = %s\n", cfg.StdoutLog)
	fmt.Fprintf(w, "  stderr-log      = %s\n", cfg.StderrLog)
	fmt.Fprintf(w, "  artifact        = %s\n", cfg.Artifact)
	fmt.Fprintf(w, "  cycle           = %d\n", cfg.Cycle)
	fmt.Fprintf(w, "  worktree        = %s\n", cfg.Worktree)
	fmt.Fprintf(w, "  agent           = %s\n", cfg.Agent)
	fmt.Fprintf(w, "  allowed_tools   = %s\n", strings.Join(cfg.AllowedTools, ","))
	fmt.Fprintf(w, "  permission-mode = %s\n", orDefault(cfg.PermissionMode, "(driver default)"))
	fmt.Fprintf(w, "  stream-output   = %t\n", cfg.StreamOutput)
	fmt.Fprintf(w, "  session-name    = %s\n", orDefault(cfg.SessionName, "(auto-generated, ephemeral)"))
	fmt.Fprintf(w, "  require-full    = %t\n", cfg.RequireFull)
	fmt.Fprintf(w, "  allow-bypass    = %t\n", cfg.AllowBypass)
	fmt.Fprintf(w, "  human-input     = %t\n", cfg.HumanInput)
}

// runDryRun produces mock outputs (stdout-log, stderr-log, artifact) with
// the challenge token resolved, without invoking any LLM — the Go port of
// bin/bridge's _bridge_dry_run. Returns ExitOK.
func (e *Engine) runDryRun(cfg *Config, _ io.Writer, stderr io.Writer) int {
	if err := ensureDirs(cfg); err != nil {
		fmt.Fprintf(stderr, "[bridge] dry-run: %v\n", err)
		return ExitBadFlags
	}
	promptBytes, _ := os.ReadFile(cfg.PromptFile)
	challenge := "no-token"
	if strings.Contains(string(promptBytes), "$CHALLENGE_TOKEN") {
		if tok, err := e.deps.NewChallengeToken(); err == nil {
			challenge = tok
			_ = os.WriteFile(filepath.Join(cfg.Workspace, "challenge-token.txt"), []byte(tok+"\n"), 0o644)
		}
	}
	now := e.deps.Now().UTC().Format("2006-01-02T15:04:05Z")

	_ = os.WriteFile(cfg.StdoutLog, []byte(fmt.Sprintf(
		"[bridge dry-run] mock stdout — no LLM invoked\ncli=%s model=%s cycle=%d agent=%s\nartifact=%s\n",
		cfg.CLI, cfg.Model, cfg.Cycle, cfg.Agent, cfg.Artifact)), 0o644)
	_ = os.WriteFile(cfg.StderrLog, []byte(fmt.Sprintf(
		"[bridge dry-run] %s\n[bridge dry-run] would have invoked cli=%s model=%s\n[bridge dry-run] NO real call made; rc=0\n",
		now, cfg.CLI, cfg.Model)), 0o644)
	_ = os.WriteFile(cfg.Artifact, []byte(fmt.Sprintf(
		"<!-- challenge-token: %s -->\n<!-- bridge-dry-run: %s -->\n# bridge dry-run — synthetic phase agent output\n\n"+
			"This artifact was produced by `bridge launch --dry-run`. cli=%s model=%s\n\nDRY-RUN-OK — no LLM was called.\n",
		challenge, now, cfg.CLI, cfg.Model)), 0o644)

	fmt.Fprintf(stderr, "[bridge] dry-run complete: artifact=%s\n", cfg.Artifact)
	return ExitOK
}

// requireFullCheck enforces --require-full: the CLI's probed tier must be
// full or hybrid. Returns (exitCode, blocked). The Go port of the
// bin/bridge --require-full gate.
func (e *Engine) requireFullCheck(cfg *Config, stderr io.Writer) (int, bool) {
	tier := "none"
	if m, err := LoadManifest(cfg.CLI); err == nil {
		tier = resolveTier(m, func(b string) bool {
			if b == "" {
				return false
			}
			_, e2 := e.deps.LookPath(b)
			return e2 == nil
		})
	}
	if tier != "full" && tier != "hybrid" {
		fmt.Fprintf(stderr, "[bridge] --require-full set; cli=%s tier=%s (need full or hybrid)\n", cfg.CLI, tier)
		return ExitRequireFullUnmet, true
	}
	return ExitOK, false
}
