package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const LiveSmokeArtifact = "live-smoke-ok.txt"

const liveSmokeArtifactTimeoutS = 120

type LiveSmokeResult struct {
	RC          int
	Pattern     string
	Scrollback  string
	ServedModel string
}

func LiveSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) LiveSmokeResult {
	d, ok := LookupDriver(driverName)
	if !ok || !strings.HasSuffix(driverName, "-tmux") {
		return LiveSmokeResult{RC: ExitBadFlags}
	}
	deps = deps.withDefaults()
	cfg = smokeLaunchConfig(cfg, driverName, deps.Efforts)
	if cfg.Workspace == "" {
		tmp, err := os.MkdirTemp("", "evolve-livesmoke-*")
		if err != nil {
			return LiveSmokeResult{RC: ExitBadFlags}
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
			return LiveSmokeResult{RC: ExitBadFlags}
		}
		cfg.PromptFile = pf
	}
	if cfg.ArtifactTimeoutS == 0 {
		cfg.ArtifactTimeoutS = liveSmokeArtifactTimeoutS
	}
	start := deps.Now()
	rc, _ := launchPane(ctx, PurposeSmoke, d, cfg, deps)
	result := LiveSmokeResult{RC: rc, Pattern: EscalationPattern(cfg.Workspace)}
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")); err == nil {
		result.Scrollback = string(b)
	}
	result.ServedModel = servedModel(deps, tokenusage.Window{Worktree: cfg.Worktree, Driver: driverName, Scrollback: result.Scrollback, Start: start, End: deps.Now()})
	return result
}

func servedModel(deps Deps, w tokenusage.Window) string {
	if deps.TokenResolver == nil || w.Worktree == "" {
		return ""
	}
	for _, cwd := range cwdSpellings(w.Worktree) {
		w.Worktree = cwd
		r, err := deps.TokenResolver(w)
		if err != nil {
			fmt.Fprintf(deps.Stderr, "[bridge] live smoke: served model unreported: %v\n", err)
			return ""
		}
		if r.ServedModel != "" {
			return r.ServedModel
		}
	}
	return ""
}

func cwdSpellings(dir string) []string {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved == dir {
		return []string{dir}
	}
	return []string{dir, resolved}
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
