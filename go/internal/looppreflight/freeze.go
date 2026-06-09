package looppreflight

// freeze.go — ADR-0044 C5: the CLI-version-freeze readiness check
// (Specification pattern).
//
// cycle-262 D6: codex self-upgraded its own binary mid-phase — its updater
// ran `brew upgrade` on the TUI launch, printed "Update ran successfully!
// Please restart Codex.", and exited the REPL to a bare shell, which the
// bridge then nudged for ~20 minutes. The host fix was `brew pin codex`: a
// CONVERGENT STEADY STATE (survives reboots and crashed batches), not a
// per-cycle pin/unpin toggle (an unpin-on-exit leaks on any SIGKILL/OOM —
// see the ADR's alternatives-considered). This check verifies the steady
// state at batch start: any *-tmux CLI with self-update evidence on the host
// must be pinned, or the batch Halts with the exact convergent action.
//
// Scope: interactive *-tmux drivers only — the incident vector is the TUI
// launch path; headless `codex exec` does not run the updater. Probes are
// read-only (stat an evidence file, list brew pins) so the check is
// idempotent by construction. Ambiguity (pin listing failed: brew absent,
// exec error) WARNs with manual guidance — only CONFIRMED risk halts, the
// same fail-open posture as the eval gate.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// pinnedListerTimeout bounds the brew exec: a hung brew (lock contention, tap
// refresh) must degrade to the WARN-on-ambiguity path, never hang every batch
// start. Mirrors the package's BootBudget posture of deadlining real-host work.
const pinnedListerTimeout = 5 * time.Second

// defaultSelfUpdateEvidence reports whether bin is known to self-update on
// launch, based on host evidence. Registry-style: today the only entry is
// codex (its updater maintains ~/.codex/version.json — the file that recorded
// dismissed_version=0.137.0 < latest=0.138.0 right before the cycle-262
// mid-phase upgrade). A CLI without evidence is not freeze-checked; new
// self-updaters are added here as incidents (or release notes) reveal them.
// Assumption: codex keeps its updater state under ~/.codex (no CODEX_HOME-style
// override is documented today; revisit the registry entry if one appears).
// A failed home-dir lookup is AMBIGUITY (error), not absence of evidence —
// the caller WARNs instead of silently passing (fail loudly).
func defaultSelfUpdateEvidence(bin string) (bool, string, error) {
	if bin != "codex" {
		return false, "", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", fmt.Errorf("user home dir unresolvable (evidence for %q unverifiable): %w", bin, err)
	}
	p := filepath.Join(home, ".codex", "version.json")
	if _, err := os.Stat(p); err != nil {
		return false, "", nil
	}
	return true, p + " present (codex updater state)", nil
}

// defaultPinnedLister lists brew-pinned formulae. An error (brew absent, exec
// failure, timeout) flows to the caller, which treats it as ambiguity (WARN).
func defaultPinnedLister() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pinnedListerTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "brew", "list", "--pinned").Output()
	if err != nil {
		return nil, fmt.Errorf("brew list --pinned: %w", err)
	}
	var pins []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			pins = append(pins, s)
		}
	}
	return pins, nil
}

// checkCLIVersionFreeze (Halt on confirmed risk / Warn on ambiguity) is the
// Specification: risky(bin) ∧ tmuxDriven(bin) ⇒ pinned(bin). risky = host
// evidence of a self-updater; tmuxDriven = some profile routes the binary
// through a *-tmux driver.
func checkCLIVersionFreeze(o resolved) CheckResult {
	const name = "cli-version-freeze"

	// Distinct binaries reached via interactive *-tmux drivers.
	seen := map[string]struct{}{}
	var bins []string
	for _, d := range distinctDrivers(o.profileLister, o.profileGetter) {
		if !strings.HasSuffix(d, "-tmux") {
			continue
		}
		b := driverBinary(d)
		if _, dup := seen[b]; dup {
			continue
		}
		seen[b] = struct{}{}
		bins = append(bins, b)
	}

	type riskyEntry struct {
		bin    string
		detail string // "bin (evidence)" for the detail trail
	}
	var risky []riskyEntry
	var evidenceErrs []string
	for _, b := range bins {
		ok, evidence, err := o.selfUpdateEvidence(b)
		if err != nil {
			// Ambiguity, not absence: surface as WARN below (fail loudly),
			// never silently pass a binary whose evidence was unverifiable.
			evidenceErrs = append(evidenceErrs, fmt.Sprintf("%s: %v", b, err))
			continue
		}
		if ok {
			risky = append(risky, riskyEntry{bin: b, detail: fmt.Sprintf("%s (%s)", b, evidence)})
		}
	}
	if len(risky) == 0 {
		return withEvidenceWarnings(CheckResult{
			Name:    name,
			Level:   LevelPass,
			Message: fmt.Sprintf("no self-update evidence among %d tmux CLI(s)", len(bins)),
		}, evidenceErrs)
	}

	pins, err := o.pinnedLister()
	if err != nil {
		details := make([]string, len(risky))
		for i, e := range risky {
			details[i] = e.detail
		}
		return CheckResult{
			Name:    name,
			Level:   LevelWarn,
			Message: fmt.Sprintf("%d self-updating tmux CLI(s) found but pin state is unverifiable", len(risky)),
			Detail: fmt.Sprintf("%s\npin listing failed: %v\nverify manually that each is version-frozen before a long batch (cycle-262: codex self-upgraded mid-phase)",
				strings.Join(details, "\n"), err),
		}
	}
	pinned := map[string]struct{}{}
	for _, p := range pins {
		pinned[p] = struct{}{}
	}

	var unpinned []string
	var pinnedDetails []string
	for _, e := range risky {
		if _, ok := pinned[e.bin]; ok {
			pinnedDetails = append(pinnedDetails, e.detail)
			continue
		}
		unpinned = append(unpinned, fmt.Sprintf("%s — run: brew pin %s   (deliberate update later: brew unpin %s && brew upgrade %s && brew pin %s — never mid-batch)",
			e.detail, e.bin, e.bin, e.bin, e.bin))
	}
	if len(unpinned) > 0 {
		return withEvidenceWarnings(CheckResult{
			Name:    name,
			Level:   LevelHalt,
			Message: fmt.Sprintf("%d self-updating tmux CLI(s) not version-frozen", len(unpinned)),
			Detail: "a self-updating CLI can replace its own binary MID-PHASE and kill the REPL (cycle-262 D6)\n" +
				strings.Join(unpinned, "\n"),
		}, evidenceErrs)
	}
	return withEvidenceWarnings(CheckResult{
		Name:    name,
		Level:   LevelPass,
		Message: fmt.Sprintf("%d self-updating tmux CLI(s) version-frozen (convergent steady state)", len(risky)),
		Detail:  strings.Join(pinnedDetails, "\n"),
	}, evidenceErrs)
}

// withEvidenceWarnings folds unverifiable-evidence ambiguity into an already-
// decided result: the detail gains the error trail and a Pass demotes to Warn
// (fail loudly — an unverifiable binary must never silently pass). A Halt
// stays a Halt: confirmed risk outranks ambiguity.
func withEvidenceWarnings(res CheckResult, errs []string) CheckResult {
	if len(errs) == 0 {
		return res
	}
	if res.Level == LevelPass {
		res.Level = LevelWarn
		res.Message += fmt.Sprintf("; evidence unverifiable for %d CLI(s)", len(errs))
	}
	res.Detail = strings.TrimSpace(res.Detail +
		"\nevidence unverifiable (verify version-freeze manually; cycle-262: codex self-upgraded mid-phase):\n" +
		strings.Join(errs, "\n"))
	return res
}
