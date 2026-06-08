package looppreflight

import (
	"fmt"
	"path/filepath"
	"strings"
)

// minFreeDiskBytes is the low-disk warning threshold (500 MiB). Below this the
// bridge's per-cycle worktrees + scrollback logs risk an ENOSPC mid-cycle.
const minFreeDiskBytes uint64 = 500 << 20

// stalePrefix is the tmux session-name prefix the bridge uses; leftover
// sessions from a SIGKILL'd cycle cause the resource contention that helped
// trigger the cycle-258 ExitREPLBootTimeout.
const stalePrefix = "evolve-bridge-"

// checkPipelineStructure (Halt) verifies the loop's static wiring is intact:
//   - every spine phase has BOTH a registered factory and a deliverable contract
//   - the profiles directory lists and each profile loads
//   - every profile's CLI and cli_fallback entries resolve to a known driver
//
// It accumulates ALL gaps into one CheckResult so the operator sees every
// problem at once, then halts if any were found.
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

// checkLLMCLIStatus (Halt) confirms each distinct CLI binary the profiles use is
// actually installed. Driver names are mapped to their binary (claude-tmux and
// claude-p both → claude) and probed once each; a missing binary halts with the
// probe's checked-paths trail so the operator sees where it looked.
func checkLLMCLIStatus(o resolved) CheckResult {
	const name = "llm-cli-status"
	seen := map[string]struct{}{}
	var bins []string
	for _, d := range distinctDrivers(o.profileLister, o.profileGetter) {
		// driverBinary never returns "" here: distinctDrivers only yields the
		// non-empty CLI/fallback names profileCLIs collected.
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

// checkHostCapabilities verifies the host can host the bridge. Halts: tmux
// absent, or .evolve/ (and .evolve/runs/) not writable — the bridge cannot run
// at all without these. Warns (degraded but runnable): profiles request
// sandboxing the host won't provide, free disk below the threshold, or stale
// evolve-bridge-* tmux sessions linger (the contention that contributed to
// cycle-258). All accumulate; a halt outranks warnings in the verdict.
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
			warns = append(warns, fmt.Sprintf(
				"profiles request sandboxing but host sandbox is not expected to work (%s) — the bridge degrades gracefully",
				host.Sandbox.Reason))
		}
	}

	if free, err := o.diskFreeBytes(o.evolveDir); err == nil && free < minFreeDiskBytes {
		warns = append(warns, fmt.Sprintf("low free disk: %d MiB (< %d MiB) under %s",
			free>>20, minFreeDiskBytes>>20, o.evolveDir))
	}

	if sessions, err := o.tmuxSessions(); err == nil {
		var stale []string
		for _, s := range sessions {
			if strings.HasPrefix(s, stalePrefix) {
				stale = append(stale, s)
			}
		}
		if len(stale) > 0 {
			warns = append(warns, fmt.Sprintf("%d stale bridge tmux session(s) (contention risk): %s",
				len(stale), strings.Join(stale, ", ")))
		}
	}

	switch {
	case len(halts) > 0:
		// On a halt, surface the warnings too — the operator fixes everything at once.
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
		return CheckResult{Name: name, Level: LevelPass, Message: "tmux present; .evolve writable; disk + sessions healthy"}
	}
}
