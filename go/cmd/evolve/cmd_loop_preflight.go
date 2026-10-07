package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// runLoopPreflightFn is the test seam for the pre-batch readiness gate; tests
// override it to force a halt/pass without a real environment probe.
var runLoopPreflightFn = defaultLoopPreflight

func loopPreflightOptions(cfg loopConfig, stderr io.Writer) looppreflight.Options {
	layout := paths.ResolveFromEnv()
	// Absent or malformed policy resolves the fallback dial to off, the dormant
	// default: the canary never runs nor halts unless an operator opts in.
	pol, _ := policy.Load(filepath.Join(cfg.ProjectRoot, ".evolve", "policy.json"))
	return looppreflight.Options{
		ProjectRoot:         cfg.ProjectRoot,
		EvolveDir:           cfg.EvolveDir,
		ProfileDir:          layout.ProfilesDir,
		Stderr:              stderr,
		SkipBoot:            cfg.SkipPreflightBoot,
		NestedFallbackStage: parseGateStage(pol.SandboxConfig().NestedFallback),
		MinFreeBytes:        pol.PreflightConfig().MinFreeBytes(),
		Routing:             func() (looppreflight.Routing, error) { return preflightRouting(cfg.ProjectRoot) },
		UsageEvidence:       evidenceSummary(usageEvidenceFn(cfg.ProjectRoot, cfg.EvolveDir, stderr)),
	}
}

func defaultLoopPreflight(cfg loopConfig, stderr io.Writer) looppreflight.Result {
	res, err := looppreflight.Run(loopPreflightOptions(cfg, stderr))
	if err != nil {
		// A harness fault fails loud as a synthetic halt rather than silently
		// letting a misconfigured gate pass a doomed batch through.
		return looppreflight.Result{
			Checks: []looppreflight.CheckResult{{
				Name: "preflight", Level: looppreflight.LevelHalt,
				Message: "readiness gate could not run", Detail: err.Error(),
			}},
			ChecksTotal:  1,
			OverallLevel: looppreflight.LevelHalt,
		}
	}
	return res
}

func loopPreflightHalts(cfg loopConfig, stderr io.Writer) bool {
	if cfg.SkipPreflight {
		fmt.Fprintln(stderr, "[loop] readiness gate skipped (--skip-preflight)")
		return false
	}
	res := runLoopPreflightFn(cfg, stderr)
	persistLoopPreflight(cfg.EvolveDir, res, stderr)
	fmt.Fprint(stderr, res.Summary())
	return res.Halted()
}

// A write failure WARNs but never changes the gate decision.
func persistLoopPreflight(evolveDir string, r looppreflight.Result, stderr io.Writer) {
	if evolveDir == "" {
		return
	}
	target := filepath.Join(evolveDir, "loop-preflight.json")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not create %s: %v\n", evolveDir, err)
		return
	}
	// PID-suffixed to avoid a stale-.tmp collision between concurrent loop starts.
	tmp := fmt.Sprintf("%s.tmp.%d", target, os.Getpid())
	if err := os.WriteFile(tmp, append(r.PrettyJSON(), '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not write %s: %v\n", target, err)
		return
	}
	if err := os.Rename(tmp, target); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not finalize %s: %v\n", target, err)
	}
}

func runLoopPreflightOnly(cfg loopConfig, stdout, stderr io.Writer) int {
	probeDir := filepath.Join(cfg.ProjectRoot, ".evolve", "worktrees")
	_, statErr := os.Stat(probeDir)
	res := runLoopPreflightFn(cfg, stderr)
	if errors.Is(statErr, fs.ErrNotExist) {
		removeEmptyProbeDir(probeDir, stderr)
	}
	persistLoopPreflight(cfg.EvolveDir, res, stderr)
	fmt.Fprint(stdout, res.Summary())
	if !res.Halted() {
		fmt.Fprintf(stdout, "preflight-only: READY (%d/%d checks passed); no cycle dispatched\n", res.ChecksPassed, res.ChecksTotal)
		return 0
	}
	for _, c := range res.Checks {
		if c.Level == looppreflight.LevelHalt {
			fmt.Fprintf(stderr, "evolve loop: --preflight-only: blocking check %q halted: %s\n", c.Name, c.Message)
		}
	}
	return 1
}

func removeEmptyProbeDir(dir string, stderr io.Writer) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(entries) > 0) {
		return
	}
	if err == nil {
		err = os.Remove(dir)
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: --preflight-only: could not remove the gate's probe dir %s: %v\n", dir, err)
	}
}

func preflightRouting(projectRoot string) (looppreflight.Routing, error) {
	return looppreflight.CompileRouting(func() (*cliroute.Router, []cliroute.Finding, error) {
		router, findings, err := loadCLIRouter(projectRoot, cliroute.Host{})
		if err != nil {
			return nil, findings, fmt.Errorf("compile the CLI routing table of %s: %w", projectRoot, err)
		}
		return router, findings, nil
	}, profiles.NewFromDir(routingProfilesDir(projectRoot)).List)
}
