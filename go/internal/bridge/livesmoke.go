package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LiveSmokeArtifact is the fixed artifact filename a live smoke-test asks the
// CLI to write into the probe workspace. Exported so callers (and tests) can
// locate it without threading the path around.
const LiveSmokeArtifact = "live-smoke-ok.txt"

// liveSmokeArtifactTimeoutS bounds the probe's artifact wait: long enough for
// a real model to answer a one-word task, far below the 300s phase default —
// a probe should fail fast, that is its job.
const liveSmokeArtifactTimeoutS = 120

// LiveSmokeTest performs a real launch of the given *-tmux driver, submits one trivial contracted prompt,
// and waits briefly for the artifact. It is the only probe shape that can see a quota wall — BootSmokeTest
// alone passes against a rate-limited CLI, since provider walls appear only after work is submitted.
// Returns the bridge exit code, the escalation pattern name when the launch died on a classified
// interactive wall ("rate_limit" — empty otherwise), and the captured pane scrollback.
func LiveSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) (rc int, pattern, scrollback string) {
	d, ok := LookupDriver(driverName)
	if !ok || !strings.HasSuffix(driverName, "-tmux") {
		return ExitBadFlags, "", ""
	}
	deps = deps.withDefaults()
	cfg = smokeLaunchConfig(cfg, driverName, deps.Efforts)
	if cfg.Workspace == "" {
		tmp, err := os.MkdirTemp("", "evolve-livesmoke-*")
		if err != nil {
			return ExitBadFlags, "", ""
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cfg.Workspace = tmp
	}
	// A health-canary probe has no worktree; it runs in a scratch dir under its Workspace, never the live
	// checkout, avoiding the os.Getwd() fallback.
	applyScratchCwd(cfg)
	cfg.Artifact = filepath.Join(cfg.Workspace, LiveSmokeArtifact)
	if cfg.PromptFile == "" {
		prompt := fmt.Sprintf("Health probe. Write the single word OK to %s and do nothing else.\n", cfg.Artifact)
		pf := filepath.Join(cfg.Workspace, "live-smoke-prompt.txt")
		if err := os.WriteFile(pf, []byte(prompt), 0o644); err != nil {
			return ExitBadFlags, "", ""
		}
		cfg.PromptFile = pf
	}
	if cfg.ArtifactTimeoutS == 0 {
		cfg.ArtifactTimeoutS = liveSmokeArtifactTimeoutS
	}
	// Dead-shell guard is armed by the real driver constructor (guardDeadShell), so smoke boots get the
	// same rejection a phase launch gets.
	rc, _ = d.Launch(ctx, cfg, deps)
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")); err == nil {
		scrollback = string(b)
	}
	return rc, EscalationPattern(cfg.Workspace), scrollback
}

func EscalationPattern(workspace string) string {
	raw, err := os.ReadFile(filepath.Join(workspace, "escalation-report.json"))
	if err != nil {
		return ""
	}
	var rep struct {
		Pattern string `json:"pattern_name"`
	}
	if json.Unmarshal(raw, &rep) != nil {
		return ""
	}
	return rep.Pattern
}
