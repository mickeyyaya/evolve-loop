package looppreflight

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	sandbox "github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/preflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
)

// Below minFreeDiskBytes, per-cycle worktrees and scrollback logs risk ENOSPC mid-cycle.
const minFreeDiskBytes uint64 = 500 << 20

// checkPipelineStructure halts on a spine phase without a factory or contract, an
// unloadable profile, or a profile CLI with no known driver, listing every gap.
func checkPipelineStructure(o resolved) CheckResult {
	const name = "pipeline-structure"
	var gaps []string

	for _, p := range o.spinePhases {
		if !o.factoryKnown(p) {
			gaps = append(gaps, fmt.Sprintf("phase %q: no registered factory (registry.For)", p))
		}
		if !o.contractKnown(p) {
			gaps = append(gaps, fmt.Sprintf("phase %q: no deliverable contract (phasecontract.For)", p))
		}
	}

	names, err := o.profileLister()
	if err != nil {
		gaps = append(gaps, fmt.Sprintf("profiles: cannot list %q: %v", o.profileDir, err))
	}
	for _, n := range names {
		prof, perr := o.profileGetter(n)
		if perr != nil {
			gaps = append(gaps, fmt.Sprintf("profile %q: cannot load: %v", n, perr))
			continue
		}
		for _, cli := range profileCLIs(prof) {
			if !o.driverKnown(cli) {
				gaps = append(gaps, fmt.Sprintf("profile %q: CLI %q resolves to no known driver", n, cli))
			}
		}
	}

	if len(gaps) > 0 {
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("%d pipeline-structure gap(s)", len(gaps)),
			Detail:  strings.Join(gaps, "\n"),
		}
	}
	return CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("%d spine phases wired; %d profile(s) resolve to known drivers", len(o.spinePhases), len(names)),
	}
}

// checkLLMCLIStatus halts when a CLI binary the profiles use is missing; each binary is probed once.
func checkLLMCLIStatus(o resolved) CheckResult {
	const name = "llm-cli-status"
	seen := map[string]struct{}{}
	var bins []string
	for _, d := range distinctDrivers(o.profileLister, o.profileGetter) {
		// Never "": distinctDrivers yields only the non-empty names profileCLIs collected.
		b := driverBinary(d)
		if _, dup := seen[b]; dup {
			continue
		}
		seen[b] = struct{}{}
		bins = append(bins, b)
	}

	var gaps []string
	for _, b := range bins {
		res, err := o.probeCLI(b)
		if err != nil {
			gaps = append(gaps, fmt.Sprintf("CLI %q: probe error: %v", b, err))
			continue
		}
		if !res.Found {
			gaps = append(gaps, fmt.Sprintf("CLI %q: not found [%s]", b, strings.Join(res.Checked, "; ")))
		}
	}

	if len(gaps) > 0 {
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("%d CLI binary/binaries missing", len(gaps)),
			Detail:  strings.Join(gaps, "\n"),
		}
	}
	return CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("%d CLI binary/binaries present", len(bins)),
	}
}

// checkHostCapabilities halts on missing tmux, an unwritable .evolve or runs dir, or a
// required sandbox the host lacks; low disk and a failed orphan reap only warn.
func checkHostCapabilities(o resolved) CheckResult {
	const name = "host-capabilities"
	var halts, warns []string

	if res, err := o.probeCLI("tmux"); err != nil {
		halts = append(halts, fmt.Sprintf("tmux: probe error: %v", err))
	} else if !res.Found {
		halts = append(halts, "tmux not found — the bridge cannot drive any *-tmux CLI without it")
	}

	for _, d := range []string{o.evolveDir, filepath.Join(o.evolveDir, "runs")} {
		if !o.dirWritable(d) {
			halts = append(halts, fmt.Sprintf("%s not writable", d))
		}
	}

	if sandboxWanted(o.profileLister, o.profileGetter) {
		if host := o.hostProbe(); !host.Sandbox.ExpectedToWork {
			if halt, warn := sandboxUnavailableIssue(host, o.sandboxMode()); halt != "" {
				halts = append(halts, halt)
			} else {
				warns = append(warns, warn)
			}
		}
	}

	if free, err := o.diskFreeBytes(o.evolveDir); err == nil && free < minFreeDiskBytes {
		warns = append(warns, fmt.Sprintf("low free disk: %d MiB (< %d MiB) under %s",
			free>>20, minFreeDiskBytes>>20, o.evolveDir))
	}

	// A wedged tmux must abandon the kill, not hang loop boot.
	reapCtx, cancel := context.WithTimeout(context.Background(), sessionreaper.DefaultReapTimeout)
	defer cancel()
	if _, err := sessionreaper.ReapOrphans(reapCtx, o.evolveDir, sessionreaper.Options{
		Now:      o.now,
		LeaseTTL: runlease.DefaultTTL,
		Kill:     o.orphanKill,
	}); err != nil {
		warns = append(warns, fmt.Sprintf("orphan session reap failed: %v", err))
	}

	switch {
	case len(halts) > 0:
		all := make([]string, 0, len(halts)+len(warns))
		all = append(all, halts...)
		all = append(all, warns...)
		return CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("%d host-capability gap(s)", len(halts)),
			Detail:  strings.Join(all, "\n"),
		}
	case len(warns) > 0:
		return CheckResult{
			Name:    name,
			Level:   LevelWarn,
			Message: fmt.Sprintf("%d host-capability warning(s)", len(warns)),
			Detail:  strings.Join(warns, "\n"),
		}
	default:
		return CheckResult{Name: name, Level: LevelPass, Message: "tmux present; .evolve writable; disk + orphan reaping healthy"}
	}
}

// checkCLIVersionDrift warns when a CLI's version differs from .evolve/cli-versions.json,
// then saves the current inventory there; a first batch only records the baseline.
func checkCLIVersionDrift(o resolved) CheckResult {
	const name = "cli-version-drift"

	current := o.versionInventory()
	if len(current) == 0 {
		return CheckResult{Name: name, Level: LevelPass, Message: "no CLI version inventory"}
	}

	cachePath := filepath.Join(o.evolveDir, "cli-versions.json")
	prev, _ := loadVersionCache(cachePath)

	var warns []string
	for bin, curVer := range current {
		prevVer, hadPrev := prev[bin]
		if !hadPrev || prevVer == curVer {
			continue
		}
		warns = append(warns, fmt.Sprintf("%s changed: %s → %s", bin, prevVer, curVer))
	}
	sort.Strings(warns)

	// Best-effort: a failed save only costs the next batch its comparison.
	_ = saveVersionCache(cachePath, current)

	if len(warns) > 0 {
		return CheckResult{
			Name:    name,
			Level:   LevelWarn,
			Message: fmt.Sprintf("%d CLI(s) changed version since last batch", len(warns)),
			Detail:  strings.Join(warns, "\n"),
		}
	}

	parts := make([]string, 0, len(current))
	for b, v := range current {
		parts = append(parts, b+"="+v)
	}
	sort.Strings(parts)
	return CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("CLI version inventory captured (%d bins)", len(current)),
		Detail:  strings.Join(parts, "\n"),
	}
}

// sandboxUnavailableIssue applies bridge dispatch's fail-closed decision: a nested session
// proves nothing, and only the EVOLVE_SANDBOX=off opt-out permits an unconfined launch.
func sandboxUnavailableIssue(host preflight.Profile, mode string) (halt, warn string) {
	ok, optOut, reason := sandbox.ConfinementSatisfied(host.ClaudeCode.Nested, mode)
	switch {
	case ok && optOut:
		return "", fmt.Sprintf(
			"EVOLVE_SANDBOX=off — host opt-out honoured; source-writing phases run UNCONFINED despite a sandbox-requiring profile (inner sandbox also unavailable: %s)",
			host.Sandbox.Reason)
	default:
		return fmt.Sprintf(
			"required Build sandbox unavailable (%s; %s); run on a supported non-nested host or explicitly opt out with EVOLVE_SANDBOX=off",
			host.Sandbox.Reason, reason), ""
	}
}
