package bridge

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func BootSmokeTest(ctx context.Context, driverName string, cfg *Config, deps Deps) (rc int, scrollback string) {
	d, ok := LookupDriver(driverName)
	if !ok || !strings.HasSuffix(driverName, "-tmux") {
		return ExitBadFlags, ""
	}
	deps = deps.withDefaults()
	cfg = smokeLaunchConfig(cfg, driverName, deps.Efforts)
	cfg.BootOnly = true
	if cfg.Workspace == "" {
		tmp, err := os.MkdirTemp("", "evolve-bootsmoke-*")
		if err != nil {
			return ExitBadFlags, ""
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cfg.Workspace = tmp
	}
	applyScratchCwd(cfg)
	rc, _ = d.Launch(ctx, cfg, deps)
	if b, err := os.ReadFile(filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")); err == nil {
		scrollback = string(b)
	}
	return rc, scrollback
}

func ScrollbackTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append([]string{lines[i]}, out...)
		}
	}
	return strings.Join(out, "\n")
}

func smokeLaunchConfig(cfg *Config, driverName string, efforts policy.EffortTable) *Config {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.CLI = driverName
	cfg.AllowBypass = true
	if reflect.ValueOf(cfg.Realization).IsZero() {
		cfg.Realization = RealizeFor(driverName, LaunchIntent{Permission: "bypass", ModelTier: cfg.Model, Effort: LaunchEffort(driverName, cfg.Model, efforts)})
	}
	return cfg
}
