package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

type bootRecoveryResult struct {
	Quarantined bool
	Sealed      bool
	SHAMismatch bool
	Healed      bool
	HaltSelfSHA bool
}

var bootRecoverFn = defaultBootRecovery

var shipRepinProvenanceFn = defaultShipRepinProvenance

func defaultShipRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return version.Commit(), func(c string) bool {
		if c == "" {
			return false
		}
		return sysexec.Command(context.Background(), "git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}
}

func defaultBootRecovery(ctx context.Context, cfg loopConfig, ledger core.Ledger, stderr io.Writer) bootRecoveryResult {
	var res bootRecoveryResult

	if mismatch, _ := detectShipSHAMismatch(cfg, stderr); mismatch {
		res.SHAMismatch = true
		if withinVersionShipSHAMismatch(cfg) {
			res.HaltSelfSHA = true
			printSelfSHAHaltRecipe(cfg, stderr)
			return res
		}
		if attemptBootRepin(cfg, stderr) {
			res.Healed = true
			res.SHAMismatch = false
		}
	}

	if _, sealed, err := core.AutosealStaleMarker(ctx, ledger, core.SealOptions{
		EvolveDir:   cfg.EvolveDir,
		ProjectRoot: cfg.ProjectRoot,
		Reason:      "boot auto-seal: stranded cycle-state marker, owner PID dead",
	}, pidAlive); err != nil {
		if !errors.Is(err, core.ErrNothingToReset) {
			fmt.Fprintf(stderr, "[loop] boot-recovery: autoseal: %v\n", err)
		}
	} else if sealed {
		res.Sealed = true
		fmt.Fprintf(stderr, "[loop] boot-recovery: auto-sealed a stranded dead-owner cycle marker\n")
	}

	quarantineLabel := fmt.Sprintf("boot-quarantine-%s", time.Now().UTC().Format(time.RFC3339))
	if stashed, err := core.QuarantineDirtyTree(ctx, cfg.ProjectRoot, quarantineLabel); err != nil {
		fmt.Fprintf(stderr, "[loop] boot-recovery: quarantine: %v\n", err)
	} else if stashed {
		res.Quarantined = true
		fmt.Fprintf(stderr, "[loop] boot-recovery: quarantined leaked tracked-source dirt into a git stash (recover with: git stash pop)\n")
	}

	return res
}

func detectShipSHAMismatch(cfg loopConfig, stderr io.Writer) (bool, string) {
	raw, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "state.json"))
	if err != nil {
		return false, ""
	}
	var st map[string]any
	if json.Unmarshal(raw, &st) != nil {
		return false, ""
	}
	expected, _ := st["expected_ship_sha"].(string)
	if expected == "" {
		return false, ""
	}
	binPath := filepath.Join(cfg.ProjectRoot, "go", "bin", "evolve")
	mismatch, actual, err := core.ShipSHAMismatch(binPath, expected)
	if err != nil {
		return false, ""
	}
	if mismatch {
		fmt.Fprintf(stderr, "[loop] boot-recovery: ship binary SHA mismatch (expected %s, on-disk %s) — attempting provenance-gated auto-repin\n", expected, actual)
	}
	return mismatch, actual
}

func withinVersionShipSHAMismatch(cfg loopConfig) bool {
	raw, err := os.ReadFile(filepath.Join(cfg.EvolveDir, "state.json"))
	if err != nil {
		return false
	}
	var st map[string]any
	if json.Unmarshal(raw, &st) != nil {
		return false
	}
	expectedVer, _ := st["expected_ship_version"].(string)
	if expectedVer == "" {
		return false
	}
	return expectedVer == ship.PluginVersion(cfg.ProjectRoot)
}

func printSelfSHAHaltRecipe(cfg loopConfig, stderr io.Writer) {
	fmt.Fprintf(stderr, "[loop] boot-recovery: ship binary was modified WITHIN plugin version %q "+
		"(expected_ship_sha != on-disk go/bin/evolve) — SELF_SHA_TAMPERED. HALTING pre-scout; "+
		"no cycle will run on a ship doomed from boot.\n", ship.PluginVersion(cfg.ProjectRoot))
	fmt.Fprintln(stderr, "[loop]   To unblock (rebuild from committed source, then re-authorize the pin):")
	fmt.Fprintln(stderr, "[loop]     1. make -C go build")
	fmt.Fprintln(stderr, "[loop]     2. evolve reset-sha -operator")
	fmt.Fprintln(stderr, "[loop]     3. relaunch the loop")
	fmt.Fprintln(stderr, "[loop]   (If this is NOT expected, investigate local tampering / plugin install corruption before re-pinning.)")
}

func attemptBootRepin(cfg loopConfig, stderr io.Writer) bool {
	commit, prov := shipRepinProvenanceFn(cfg.ProjectRoot)
	statePath := filepath.Join(cfg.EvolveDir, "state.json")
	binPath := filepath.Join(cfg.ProjectRoot, "go", "bin", "evolve")
	res, err := phaseintegrity.RepinIfDrifted(statePath, binPath, commit, "", prov)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] boot-recovery: ship-SHA auto-repin declined (%v) — rebuild from committed source then `evolve reset-sha` to authorize, or investigate tampering\n", err)
		return false
	}
	if !res.Repinned {
		return false
	}
	fmt.Fprintf(stderr, "[loop] boot-recovery: auto-repinned expected_ship_sha %.12s -> %.12s (authorized: %s) — legitimate rebuild self-healed at boot\n", res.OldSHA, res.NewSHA, res.Authorized)
	return true
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
