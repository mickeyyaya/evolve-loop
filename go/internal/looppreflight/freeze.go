package looppreflight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// pinnedListerTimeout sends a hung brew to the Warn-on-ambiguity path instead of stalling batch start.
const pinnedListerTimeout = 5 * time.Second

// defaultSelfUpdateEvidence is the known-updater registry, keyed by the updater-state file
// under the default home dir. A failed home lookup is ambiguity (error), not absence.
func defaultSelfUpdateEvidence(bin string) (bool, string, error) {
	var rel string
	var label string
	switch bin {
	case "codex":
		rel = filepath.Join(".codex", "version.json")
		label = "codex updater state"
	case "claude":
		rel = filepath.Join(".claude", "settings.json")
		label = "claude updater state"
	default:
		return false, "", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", fmt.Errorf("user home dir unresolvable (evidence for %q unverifiable): %w", bin, err)
	}
	p := filepath.Join(home, rel)
	if _, err := os.Stat(p); err != nil {
		return false, "", nil
	}
	// autoUpdates:false freezes claude at the source: the only freeze where claude is
	// not brew-installed, so without it the halt would be permanent there.
	if bin == "claude" {
		raw, err := os.ReadFile(p)
		if err != nil {
			return false, "", fmt.Errorf("claude settings unreadable (freeze state unverifiable): %w", err)
		}
		var s struct {
			AutoUpdates *bool `json:"autoUpdates"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return false, "", fmt.Errorf("claude settings unparsable (freeze state unverifiable): %w", err)
		}
		if s.AutoUpdates != nil && !*s.AutoUpdates {
			return false, "", nil
		}
	}
	return true, p + " present (" + label + ")", nil
}

// defaultPinnedLister lists brew-pinned formulae; the caller treats an error as ambiguity.
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

// checkCLIVersionFreeze is the Specification risky(bin) ∧ tmuxDriven(bin) ⇒ pinned(bin); ambiguity only warns.
// See ADR-0044.
func checkCLIVersionFreeze(o resolved) CheckResult {
	const name = "cli-version-freeze"

	// Only *-tmux drivers: a headless launch does not run the updater.
	seen := map[string]struct{}{}
	var bins []string
	for _, d := range distinctDrivers(o.profileLister, o.profileGetter) {
		if !bridge.IsTmuxDriver(d) {
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
		detail string
	}
	var risky []riskyEntry
	var evidenceErrs []string
	for _, b := range bins {
		ok, evidence, err := o.selfUpdateEvidence(b)
		if err != nil {
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

// withEvidenceWarnings appends unverifiable-evidence errors: a Pass demotes to Warn, and a Halt stays a Halt.
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
