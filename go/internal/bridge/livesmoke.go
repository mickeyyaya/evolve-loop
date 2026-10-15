package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const LiveSmokeArtifact = "live-smoke-ok.txt"

const liveSmokeArtifactTimeoutS = 120

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
