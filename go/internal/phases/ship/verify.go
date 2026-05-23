// verify.go — self-SHA TOFU + class-aware verification.
//
// Mirrors ship.sh sections 1 (lines 221-292) and 2 (lines 294-394):
//
//   - verifySelfSHA: version-aware TOFU pin of the ship binary's SHA.
//     5 paths: first-run pin, same-version-same-SHA pass,
//     same-version-different-SHA integrity-fail, no-version legacy
//     migration, plugin-version-change re-pin.
//
//   - verifyClass: cycle → audit-binding, manual → interactive y/N,
//     release → skip-with-log, trivial → cycle_size_estimate + critical-paths.
package ship

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// sha256File computes the SHA256 of a file's contents in hex.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifySelfSHA implements ship.sh's version-aware TOFU.
//
// Five branches:
//
//	1. no expected_ship_sha           → first run; pin both fields
//	2. expected matches actual:
//	   - no expected_ship_version     → schema migration; pin version
//	   - expected_ship_version set    → clean pass
//	3. expected != actual:
//	   - no expected_ship_version     → legacy SHA-only pin; migrate (re-pin)
//	   - expected_ship_version != current → plugin update; re-pin
//	   - same version, different SHA  → INTEGRITY-FAIL (real tampering)
//
// The state.json mutation preserves every other field (map-based).
func verifySelfSHA(_ context.Context, opts *Options, res *RunResult) error {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		var err error
		binPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("ship: cannot resolve binary path: %w", err)
		}
	}

	actualSHA, err := sha256File(binPath)
	if err != nil {
		return fmt.Errorf("ship: cannot SHA ship binary: %w", err)
	}
	pluginVer := pluginVersion(opts.PluginRoot)

	statePath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	stateMap, err := readStateMap(statePath)
	if err != nil {
		return fmt.Errorf("ship: read state.json: %w", err)
	}
	expectedSHA := stateString(stateMap, "expected_ship_sha")
	expectedVer := stateString(stateMap, "expected_ship_version")

	repin := func(reason string) error {
		stateMap["expected_ship_sha"] = actualSHA
		stateMap["expected_ship_version"] = pluginVer
		if err := writeStateMap(statePath, stateMap); err != nil {
			return fmt.Errorf("ship: write state.json: %w", err)
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] TOFU: %s — pinned ship binary SHA + plugin version='%s'", reason, pluginVer))
		return nil
	}

	switch {
	case expectedSHA == "":
		return repin("first run")
	case expectedSHA == actualSHA:
		if expectedVer == "" && pluginVer != "" {
			return repin("schema migration (no expected_ship_version recorded)")
		}
		// clean pass
		return nil
	case expectedVer == "":
		return repin("migrating legacy SHA-only pin to version-aware schema")
	case pluginVer != expectedVer:
		return repin(fmt.Sprintf("plugin version changed: '%s' → '%s'", expectedVer, pluginVer))
	default:
		return &IntegrityError{
			Msg: fmt.Sprintf(
				"ship binary has been modified WITHIN plugin version %s (expected=%s actual=%s). "+
					"This indicates real local tampering or plugin install corruption. "+
					"To intentionally update: remove .evolve/state.json:expected_ship_sha and re-run.",
				pluginVer, expectedSHA, actualSHA,
			),
		}
	}
}

// IntegrityError signals an exit-code-2 refusal. Distinct from runtime
// errors (which exit 1).
type IntegrityError struct {
	Msg string
}

func (e *IntegrityError) Error() string { return e.Msg }

// verifyClass runs the per-class pre-flight (audit-binding for cycle;
// interactive confirm for manual; kernel checks for trivial; log-only
// for release).
//
// Sets res.Provenance and may stage worktree changes (manual class).
func verifyClass(ctx context.Context, opts *Options, res *RunResult) error {
	switch opts.Class {
	case ClassCycle:
		res.Logs = append(res.Logs, "[ship] class: cycle (audit-bound)")
		res.Provenance = "cycle (audit-verified)"
		return verifyAuditBinding(ctx, opts, res)

	case ClassRelease:
		res.Logs = append(res.Logs, "[ship] class: release (pipeline-internal)")
		res.Logs = append(res.Logs, "[ship]   → audit verification skipped: version-bump.sh mutates files post-audit")
		res.Logs = append(res.Logs, "[ship]   → this commit must be created by legacy/scripts/release-pipeline.sh only")
		res.Provenance = "release (pipeline-generated)"
		return nil

	case ClassManual:
		res.Logs = append(res.Logs, "[ship] class: manual (operator-driven)")
		return verifyManualConfirm(ctx, opts, res)

	case ClassTrivial:
		res.Logs = append(res.Logs, "[ship] class: trivial (skip-audit eligible)")
		return verifyTrivial(ctx, opts, res)
	}
	return fmt.Errorf("ship: invalid class %q", opts.Class)
}

// verifyManualConfirm implements the --class manual interactive y/N
// prompt with EVOLVE_SHIP_AUTO_CONFIRM bypass.
func verifyManualConfirm(ctx context.Context, opts *Options, res *RunResult) error {
	// Stage everything so diff --cached reflects what will ship.
	exitCode, err := opts.Runner(ctx, "git", []string{"add", "-A"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, opts.Stderr)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("ship: git add -A failed (rc=%d): %w", exitCode, err)
	}
	// Check if there's anything staged.
	exitCode, err = opts.Runner(ctx, "git", []string{"diff", "--cached", "--quiet"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("ship: git diff --cached --quiet failed: %w", err)
	}
	if exitCode == 0 {
		// Nothing staged.
		res.Logs = append(res.Logs, "[ship] no staged changes; nothing to ship")
		return errEmptyDiff
	}

	if opts.envBool("EVOLVE_SHIP_AUTO_CONFIRM") {
		res.Logs = append(res.Logs, "[ship] EVOLVE_SHIP_AUTO_CONFIRM=1 — skipping interactive prompt (CI mode)")
		res.Provenance = "manual (auto-confirmed via env)"
		return nil
	}

	// Print the diff stat + first 80 lines of diff.
	fmt.Fprintln(opts.Stderr)
	fmt.Fprintln(opts.Stderr, "=== git diff --cached --stat ===")
	if _, err := opts.Runner(ctx, "git", []string{"diff", "--cached", "--stat"}, os.Environ(), opts.ProjectRoot, nil, opts.Stderr, opts.Stderr); err != nil {
		return fmt.Errorf("ship: diff stat: %w", err)
	}
	fmt.Fprintln(opts.Stderr)
	fmt.Fprintln(opts.Stderr, "=== git diff --cached (first 80 lines) ===")
	// Capture into a buffer, truncate to 80 lines.
	var diffBuf strings.Builder
	if _, err := opts.Runner(ctx, "git", []string{"diff", "--cached"}, os.Environ(), opts.ProjectRoot, nil, &diffBuf, io.Discard); err != nil {
		return fmt.Errorf("ship: diff: %w", err)
	}
	lines := strings.Split(diffBuf.String(), "\n")
	if len(lines) > 80 {
		lines = append(lines[:80], "  ... (diff truncated; see git diff --cached for full)")
	}
	fmt.Fprintln(opts.Stderr, strings.Join(lines, "\n"))
	fmt.Fprintln(opts.Stderr)

	// Refuse if stdin is not a tty (LLM agents cannot answer this).
	if !isTerminal(opts.Stdin) {
		return &IntegrityError{
			Msg: "--class manual requires interactive stdin (not a tty). Set EVOLVE_SHIP_AUTO_CONFIRM=1 for non-interactive use (CI), or run from a real terminal.",
		}
	}

	fmt.Fprint(opts.Stderr, `[ship] Confirm manual commit? Type EXACTLY "yes" to ship, anything else aborts: `)
	scanner := bufio.NewScanner(opts.Stdin)
	if !scanner.Scan() {
		return &IntegrityError{Msg: "manual confirmation read failed"}
	}
	if strings.TrimSpace(scanner.Text()) != "yes" {
		res.Logs = append(res.Logs, "[ship] manual confirmation declined — aborting")
		return &IntegrityError{Msg: "manual confirmation declined"}
	}
	res.Provenance = "manual (interactive-confirmed)"
	return nil
}

// errEmptyDiff is a sentinel for "no staged changes — exit 0 cleanly."
// Caller (Run) recognizes this and short-circuits to ExitOK.
var errEmptyDiff = &cleanExitError{}

type cleanExitError struct{}

func (*cleanExitError) Error() string { return "no staged changes (clean exit)" }

// isTerminal reports whether r is os.Stdin AND attached to a TTY.
// Conservative: anything else (test stdin, bytes.Buffer, /dev/null) is non-tty.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// verifyTrivial implements --class trivial:
//
//	1. cycle-state.json:cycle_size_estimate must equal "trivial"
//	2. No pipeline-critical paths in the staged/working diff
//
// Pipeline-critical paths (cannot bypass audit):
//
//	agents/, .agents/, skills/, legacy/scripts/lifecycle/,
//	legacy/scripts/guards/, legacy/scripts/dispatch/, .evolve/profiles/,
//	.claude-plugin/
func verifyTrivial(ctx context.Context, opts *Options, res *RunResult) error {
	csPath := filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json")
	csMap, err := readStateMap(csPath)
	if err != nil {
		return fmt.Errorf("ship: read cycle-state.json: %w", err)
	}
	est := stateString(csMap, "cycle_size_estimate")
	if est != "trivial" {
		return &IntegrityError{
			Msg: fmt.Sprintf("ship --class trivial requires cycle_size_estimate='trivial' in cycle-state.json (got: '%s')", est),
		}
	}

	// Gather staged + unstaged + untracked file lists.
	stagedOut, err := captureGitOutput(ctx, opts, "diff", "--cached", "--name-only")
	if err != nil {
		return err
	}
	unstagedOut, err := captureGitOutput(ctx, opts, "diff", "--name-only")
	if err != nil {
		return err
	}
	untrackedOut, err := captureGitOutput(ctx, opts, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return err
	}

	critical := []string{
		"agents/", ".agents/", "skills/",
		"legacy/scripts/lifecycle/", "legacy/scripts/guards/", "legacy/scripts/dispatch/",
		".evolve/profiles/", ".claude-plugin/",
	}
	allFiles := append(append(splitNonEmpty(stagedOut), splitNonEmpty(unstagedOut)...), splitNonEmpty(untrackedOut)...)
	dedup := map[string]struct{}{}
	for _, f := range allFiles {
		dedup[f] = struct{}{}
	}
	var hits []string
	for f := range dedup {
		for _, c := range critical {
			if strings.HasPrefix(f, c) {
				hits = append(hits, f)
				break
			}
		}
	}
	if len(hits) > 0 {
		sample := hits
		if len(sample) > 3 {
			sample = sample[:3]
		}
		return &IntegrityError{
			Msg: fmt.Sprintf(
				"ship --class trivial cannot touch pipeline-critical files (%d touched: %s). "+
					"Tier-1 strictness: agent personas, skills, kernel scripts, profiles, and plugin manifest require full audit. "+
					"Use --class cycle (full audit) or --class manual (operator-confirmed).",
				len(hits), strings.Join(sample, ","),
			),
		}
	}

	res.Logs = append(res.Logs,
		"[ship]   → audit verification skipped: cycle is classified as trivial",
		"[ship]   → kernel verified: 0 pipeline-critical paths touched",
	)
	res.Provenance = "trivial (skip-audit, kernel-verified)"
	return nil
}

// captureGitOutput runs git <args...> and returns stdout, ignoring rc.
// Used for the trivial-class critical-paths check; an empty repo is fine.
func captureGitOutput(ctx context.Context, opts *Options, args ...string) (string, error) {
	var buf strings.Builder
	exitCode, err := opts.Runner(ctx, "git", args, os.Environ(), opts.ProjectRoot, nil, &buf, io.Discard)
	if err != nil {
		return "", fmt.Errorf("ship: git %v: %w", args, err)
	}
	if exitCode > 1 {
		// rc=1 from git diff is "differences exist" — not an error.
		return "", fmt.Errorf("ship: git %v exited %d", args, exitCode)
	}
	return buf.String(), nil
}

// splitNonEmpty splits s on newlines, dropping empty entries.
func splitNonEmpty(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
