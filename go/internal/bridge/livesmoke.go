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

// LiveSmokeTest performs a REAL launch of the given *-tmux driver that
// SUBMITS one trivial contracted prompt and waits (briefly) for the artifact.
// It is the only probe shape that can see a quota wall: BootSmokeTest passes
// against a rate-limited CLI because provider walls appear only after work is
// submitted (cycle-283 — every codex phase re-discovered the wall the
// expensive way). Used by `evolve doctor live` and the loop's per-cycle bench
// canary.
//
// Returns the bridge exit code, the escalation pattern name when the launch
// died on a classified interactive wall ("rate_limit" — empty otherwise), and
// the captured pane scrollback (carries the wall text for reset-hint parsing).
func LiveSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) (rc int, pattern, scrollback string) {
	d, ok := LookupDriver(driverName)
	if !ok || !strings.HasSuffix(driverName, "-tmux") {
		return ExitBadFlags, "", ""
	}
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.CLI = driverName
	cfg.AllowBypass = true // the probe's task is inert; bypass-equivalent so the safety gate passes
	if cfg.Workspace == "" {
		tmp, err := os.MkdirTemp("", "evolve-livesmoke-*")
		if err != nil {
			return ExitBadFlags, "", ""
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cfg.Workspace = tmp
	}
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
	deps = deps.withDefaults()
	rc, _ = d.Launch(ctx, cfg, deps)
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")); err == nil {
		scrollback = string(b)
	}
	// The autoresponder persists its classification before an escalate exit;
	// surface the pattern name so callers can act on the CLASS (rate_limit)
	// rather than the bare exit code.
	if raw, err := os.ReadFile(filepath.Join(cfg.Workspace, "escalation-report.json")); err == nil {
		var rep struct {
			Pattern string `json:"pattern_name"`
		}
		if json.Unmarshal(raw, &rep) == nil {
			pattern = rep.Pattern
		}
	}
	return rc, pattern, scrollback
}
