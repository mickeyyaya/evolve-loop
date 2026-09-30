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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// SSOT IPC-protocol-allowed: releasepipeline/rollback→ship subprocess
const envShipAutoConfirm = "EVOLVE_" + "SHIP_AUTO_CONFIRM"

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifySelfSHA(_ context.Context, opts *Options, res *RunResult) error {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		var err error
		binPath, err = os.Executable()
		if err != nil {
			return shipErr(core.CodeSelfSHAIO, core.ShipClassTransient, core.StageVerifySelfSHA,
				"ship: cannot resolve binary path: "+err.Error())
		}
	}

	actualSHA, err := sha256File(binPath)
	if err != nil {
		return shipErr(core.CodeSelfSHAIO, core.ShipClassTransient, core.StageVerifySelfSHA,
			"ship: cannot SHA ship binary: "+err.Error(), "bin_path", binPath)
	}
	pluginVer := pluginVersion(opts.PluginRoot)

	statePath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	// Holds the shared state.json lock across the whole TOFU read→decide→repin
	// so a concurrent allocator/UpdateState write cannot interleave.
	// See ADR-0049.
	release, lockErr := lockStateFile(statePath)
	if lockErr != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifySelfSHA,
			"ship: lock state.json: "+lockErr.Error(), "path", statePath)
	}
	defer release()
	stateMap, err := readStateMap(statePath)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifySelfSHA,
			"ship: read state.json: "+err.Error(), "path", statePath)
	}
	expectedSHA := stateString(stateMap, "expected_ship_sha")
	expectedVer := stateString(stateMap, "expected_ship_version")

	repin := func(reason string) error {
		stateMap["expected_ship_sha"] = actualSHA
		stateMap["expected_ship_version"] = pluginVer
		if err := writeStateMap(statePath, stateMap); err != nil {
			return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifySelfSHA,
				"ship: write state.json: "+err.Error(), "path", statePath)
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
		return shipErr(core.CodeSelfSHATampered, core.ShipClassIntegrity, core.StageVerifySelfSHA,
			fmt.Sprintf(
				"ship binary has been modified WITHIN plugin version %s (expected=%s actual=%s). "+
					"This indicates real local tampering or plugin install corruption. "+
					"To intentionally update: remove .evolve/state.json:expected_ship_sha and re-run.",
				pluginVer, expectedSHA, actualSHA,
			),
			"plugin_version", pluginVer, "expected_sha", expectedSHA, "actual_sha", actualSHA)
	}
}

// IntegrityError wraps a *core.ShipError for legacy exit-code-2 matching;
// construct only via shipErr.
type IntegrityError struct {
	Msg string          // human text (== wrapped ShipError.Message)
	se  *core.ShipError // the authoritative structured error
}

func (e *IntegrityError) Error() string {
	if e.se != nil {
		return e.se.Error()
	}
	return e.Msg
}

// Unwrap exposes the wrapped *core.ShipError so core.AsShipError (and any
// errors.As targeting *core.ShipError) recovers it through an IntegrityError.
func (e *IntegrityError) Unwrap() error {
	if e.se == nil {
		return nil
	}
	return e.se
}

// shipErr builds a *core.ShipError, wrapping it in *IntegrityError only when
// class is ShipClassIntegrity (see IntegrityError).
func shipErr(code core.ShipErrorCode, class core.ShipErrorClass, stage core.ShipStage, message string, debugKV ...string) error {
	se := core.NewShipError(code, class, stage, message, debugKV...)
	if class == core.ShipClassIntegrity {
		return &IntegrityError{Msg: message, se: se}
	}
	return se
}

// verifyClass may stage worktree changes: verifyManualConfirm runs `git add -A`.
func verifyClass(ctx context.Context, opts *Options, res *RunResult) error {
	switch opts.Class {
	case ClassCycle:
		res.Logs = append(res.Logs, "[ship] class: cycle (audit-bound)")
		res.Provenance = "cycle (audit-verified)"
		if err := verifyNoControlPlaneEdits(ctx, opts, res); err != nil {
			return err
		}
		if err := verifyAuditBinding(ctx, opts, res); err != nil {
			return err
		}
		return runPersonaLint(ctx, opts, res)

	case ClassRelease:
		res.Logs = append(res.Logs, "[ship] class: release (pipeline-internal)")
		res.Logs = append(res.Logs, "[ship]   → audit verification skipped: version-bump.sh mutates files post-audit")
		res.Logs = append(res.Logs, "[ship]   → this commit must be created by legacy/scripts/release-pipeline.sh only")
		res.Provenance = "release (pipeline-generated)"
		return nil

	case ClassManual:
		res.Logs = append(res.Logs, "[ship] class: manual (operator-driven)")
		if err := verifyManualConfirm(ctx, opts, res); err != nil {
			return err
		}
		// Runs after verifyManualConfirm's `git add -A` so the attestation's SHA
		// reflects the staged tree. Bypassed by Options.BypassCommitGate.
		if err := verifyCommitGateAttestation(ctx, opts, res); err != nil {
			return err
		}
		return runPersonaLint(ctx, opts, res)

	case ClassTrivial:
		res.Logs = append(res.Logs, "[ship] class: trivial (skip-audit eligible)")
		return verifyTrivial(ctx, opts, res)
	}
	return shipErr(core.CodeInvalidClass, core.ShipClassConfig, core.StageVerifyClass,
		"ship: invalid class "+string(opts.Class), "class", string(opts.Class))
}

func verifyManualConfirm(ctx context.Context, opts *Options, res *RunResult) error {
	// Stage everything so diff --cached reflects what will ship.
	exitCode, err := opts.run(ctx, "git", []string{"add", "-A"}, io.Discard, opts.Stderr)
	if err != nil || exitCode != 0 {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
			fmt.Sprintf("ship: git add -A failed (rc=%d): %v", exitCode, err),
			"git_rc", fmt.Sprintf("%d", exitCode), "git_err", errStr(err))
	}
	exitCode, err = opts.run(ctx, "git", []string{"diff", "--cached", "--quiet"}, io.Discard, io.Discard)
	if err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: git diff --cached --quiet failed: "+err.Error(), "git_err", err.Error())
	}
	if exitCode == 0 {
		res.Logs = append(res.Logs, "[ship] no staged changes; nothing to ship")
		return errEmptyDiff
	}

	if opts.envBool(envShipAutoConfirm) {
		res.Logs = append(res.Logs, "[ship] "+envShipAutoConfirm+"=1 — skipping interactive prompt (CI mode)")
		res.Provenance = "manual (auto-confirmed via env)"
		return nil
	}

	fmt.Fprintln(opts.Stderr)
	fmt.Fprintln(opts.Stderr, "=== git diff --cached --stat ===")
	if _, err := opts.run(ctx, "git", []string{"diff", "--cached", "--stat"}, opts.Stderr, opts.Stderr); err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: diff stat: "+err.Error(), "git_err", err.Error())
	}
	fmt.Fprintln(opts.Stderr)
	fmt.Fprintln(opts.Stderr, "=== git diff --cached (first 80 lines) ===")
	var diffBuf strings.Builder
	if _, err := opts.run(ctx, "git", []string{"diff", "--cached"}, &diffBuf, io.Discard); err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: diff: "+err.Error(), "git_err", err.Error())
	}
	lines := strings.Split(diffBuf.String(), "\n")
	if len(lines) > 80 {
		lines = append(lines[:80], "  ... (diff truncated; see git diff --cached for full)")
	}
	fmt.Fprintln(opts.Stderr, strings.Join(lines, "\n"))
	fmt.Fprintln(opts.Stderr)

	// LLM agents cannot answer an interactive prompt; refuse when stdin isn't a tty.
	if !isTerminal(opts.Stdin) {
		return shipErr(core.CodeManualNotTTY, core.ShipClassConfig, core.StageVerifyClass,
			fmt.Sprintf("--class manual requires interactive stdin (not a tty). Set %s=1 for non-interactive use (CI), or run from a real terminal.", envShipAutoConfirm))
	}

	fmt.Fprint(opts.Stderr, `[ship] Confirm manual commit? Type EXACTLY "yes" to ship, anything else aborts: `)
	scanner := bufio.NewScanner(opts.Stdin)
	if !scanner.Scan() {
		return shipErr(core.CodeManualDeclined, core.ShipClassConfig, core.StageVerifyClass,
			"manual confirmation read failed")
	}
	if strings.TrimSpace(scanner.Text()) != "yes" {
		res.Logs = append(res.Logs, "[ship] manual confirmation declined — aborting")
		return shipErr(core.CodeManualDeclined, core.ShipClassConfig, core.StageVerifyClass,
			"manual confirmation declined")
	}
	res.Provenance = "manual (interactive-confirmed)"
	return nil
}

// errEmptyDiff is a sentinel for "no staged changes — exit 0 cleanly."
// Caller (Run) recognizes this and short-circuits to ExitOK.
var errEmptyDiff = &cleanExitError{}

type cleanExitError struct{}

func (*cleanExitError) Error() string { return "no staged changes (clean exit)" }

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

func verifyTrivial(ctx context.Context, opts *Options, res *RunResult) error {
	csPath := filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json")
	csMap, err := readStateMap(csPath)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageVerifyClass,
			"ship: read cycle-state.json: "+err.Error(), "path", csPath)
	}
	est := stateString(csMap, "cycle_size_estimate")
	if est != "trivial" {
		return shipErr(core.CodeTrivialNotTrivial, core.ShipClassConfig, core.StageVerifyClass,
			fmt.Sprintf("ship --class trivial requires cycle_size_estimate='trivial' in cycle-state.json (got: '%s')", est),
			"cycle_size_estimate", est)
	}

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
		return shipErr(core.CodeTrivialCriticalPaths, core.ShipClassConfig, core.StageVerifyClass,
			fmt.Sprintf(
				"ship --class trivial cannot touch pipeline-critical files (%d touched: %s). "+
					"Tier-1 strictness: agent personas, skills, kernel scripts, profiles, and plugin manifest require full audit. "+
					"Use --class cycle (full audit) or --class manual (operator-confirmed).",
				len(hits), strings.Join(sample, ","),
			),
			"critical_count", fmt.Sprintf("%d", len(hits)), "critical_sample", strings.Join(sample, ","))
	}

	res.Logs = append(res.Logs,
		"[ship]   → audit verification skipped: cycle is classified as trivial",
		"[ship]   → kernel verified: 0 pipeline-critical paths touched",
	)
	res.Provenance = "trivial (skip-audit, kernel-verified)"
	return nil
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func splitNonEmpty(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
