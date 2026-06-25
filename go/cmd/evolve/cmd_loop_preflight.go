package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// runLoopPreflightFn is the test seam for the pre-batch readiness gate; tests
// override it to force a halt/pass without a real environment probe.
var runLoopPreflightFn = defaultLoopPreflight

// defaultLoopPreflight runs the real deterministic readiness checks against the
// resolved project/profile layout. cfg.SkipPreflightBoot keeps the cheap
// checks but skips the (slow, tmux-touching) real REPL boot (--skip-preflight-boot).
func defaultLoopPreflight(cfg loopConfig, stderr io.Writer) looppreflight.Result {
	layout := paths.ResolveFromEnv()
	// Resolve the verified-fallback dial from policy (absent/malformed ⇒ off, the
	// dormant default — the canary never runs nor halts unless an operator opts in).
	pol, _ := policy.Load(filepath.Join(cfg.ProjectRoot, ".evolve", "policy.json"))
	res, err := looppreflight.Run(looppreflight.Options{
		ProjectRoot:         cfg.ProjectRoot,
		EvolveDir:           cfg.EvolveDir,
		ProfileDir:          layout.ProfilesDir,
		Stderr:              stderr,
		SkipBoot:            cfg.SkipPreflightBoot,
		NestedFallbackStage: parseGateStage(pol.SandboxConfig().NestedFallback),
	})
	if err != nil {
		// A harness fault (e.g. an unresolved project root) must fail LOUD as a
		// synthetic halt — never silently let a misconfigured gate pass a doomed
		// batch through.
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

// loopPreflightHalts runs the pre-batch readiness gate and reports whether the
// batch must abort. cfg.SkipPreflight (--skip-preflight) bypasses the gate
// entirely. Otherwise the result is always persisted to .evolve/loop-preflight.json
// and summarized to stderr.
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

// persistLoopPreflight writes the readiness result to
// .evolve/loop-preflight.json via atomic temp+rename (mirrors
// preflight.Profile.WriteToFile). Best-effort: a write failure WARNs but never
// changes the gate decision.
func persistLoopPreflight(evolveDir string, r looppreflight.Result, stderr io.Writer) {
	if evolveDir == "" {
		return
	}
	target := filepath.Join(evolveDir, "loop-preflight.json")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not create %s: %v\n", evolveDir, err)
		return
	}
	// PID-suffixed temp matches the project's atomic-write convention and avoids
	// a stale-.tmp collision between concurrent loop starts.
	tmp := fmt.Sprintf("%s.tmp.%d", target, os.Getpid())
	if err := os.WriteFile(tmp, append(r.PrettyJSON(), '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not write %s: %v\n", target, err)
		return
	}
	if err := os.Rename(tmp, target); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not finalize %s: %v\n", target, err)
	}
}
