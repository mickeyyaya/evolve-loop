// Package acssuite is the deterministic, host-side EGPS predicate-suite
// runner. It restores the suite-execution mechanism that the deleted bash
// run-acs-suite.sh used to provide (v12 flag-day removed it without a Go
// port — see ADR-0025), and extends it with the standing red-team glob.
//
// Run globs three predicate roots, executes each bash predicate with a
// per-predicate timeout, and produces an acs-verdict.json conforming to the
// schema the audit + ship gates read (docs/architecture/egps-v10.md):
//
//   - <root>/acs/cycle-<N>/*.sh            this cycle's predicates
//   - <root>/acs/regression-suite/cycle-*/*.sh   accumulated prior predicates
//   - <root>/acs/red-team/rt-*.sh          standing adversarial predicates
//
// red_count == 0 ⇒ verdict PASS ⇒ ship_eligible. A non-zero exit is RED,
// EXCEPT exit 77 = SKIP (the TAP/automake convention): an evidence-absent
// predicate (e.g. a runtime-only regression predicate on a fresh clone where
// the gitignored .evolve/ artifact it inspects does not exist) is counted
// neither red nor green, so it cannot block the gate yet cannot fake a pass.
// CLI: `evolve acs suite --cycle N`.
package acssuite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// killGrace is how long runBash waits, after the timeout kills the predicate's
// process group, for I/O to drain before force-closing the pipes.
const killGrace = 2 * time.Second

// DefaultTimeout bounds a single predicate's execution. EGPS predicates are
// fast assertions; a predicate that hangs is treated as RED (timeout).
const DefaultTimeout = 60 * time.Second

// evidenceMax caps the captured output excerpt per predicate.
const evidenceMax = 600

// SkipExitCode is the TAP/automake SKIP convention: a predicate exiting 77
// declares its evidence absent / not-applicable on this clone. It is counted
// neither red nor green.
const SkipExitCode = 77

// Result is one predicate's outcome (egps-v10 schema).
type Result struct {
	ACID            string `json:"ac_id"`
	Predicate       string `json:"predicate"` // repo-relative path
	ExitCode        int    `json:"exit_code"`
	ResultStr       string `json:"result"` // "green" | "red" | "skip"
	DurationMS      int64  `json:"duration_ms"`
	IsRegression    bool   `json:"is_regression"`
	IsRedTeam       bool   `json:"is_red_team,omitempty"`
	EvidenceExcerpt string `json:"evidence_excerpt,omitempty"`
}

// PredicateSuite is the count breakdown.
type PredicateSuite struct {
	ThisCycleCount       int `json:"this_cycle_count"`
	RegressionSuiteCount int `json:"regression_suite_count"`
	RedTeamCount         int `json:"red_team_count"`
	SkippedCount         int `json:"skipped_count"`
	Total                int `json:"total"`
}

// Verdict is the acs-verdict.json schema read by audit + ship gates.
type Verdict struct {
	SchemaVersion  string         `json:"schema_version"`
	Cycle          int            `json:"cycle"`
	PredicateSuite PredicateSuite `json:"predicate_suite"`
	Results        []Result       `json:"results"`
	GreenCount     int            `json:"green_count"`
	RedCount       int            `json:"red_count"`
	SkipCount      int            `json:"skip_count"`
	RedIDs         []string       `json:"red_ids"`
	SkipIDs        []string       `json:"skip_ids,omitempty"`
	Verdict        string         `json:"verdict"` // PASS | FAIL
	ShipEligible   bool           `json:"ship_eligible"`
}

// Options configures Run. Root and Cycle are required.
type Options struct {
	Root    string        // repo root containing acs/
	Cycle   int           // current cycle number
	Timeout time.Duration // per-predicate; 0 → DefaultTimeout
	// Seams (default to production behavior when nil).
	Now  func() time.Time
	Exec func(ctx context.Context, path string) (exitCode int, output string)
}

type predFile struct {
	path         string // absolute
	isRegression bool
	isRedTeam    bool
}

// Run discovers and executes the predicate suite, returning the Verdict.
func Run(opts Options) (Verdict, error) {
	if opts.Root == "" {
		return Verdict{}, fmt.Errorf("acssuite: Root required")
	}
	if opts.Cycle <= 0 {
		return Verdict{}, fmt.Errorf("acssuite: Cycle must be > 0")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	execFn := opts.Exec
	if execFn == nil {
		execFn = func(ctx context.Context, path string) (int, string) {
			return runBash(ctx, path)
		}
	}

	files, err := discover(opts.Root, opts.Cycle)
	if err != nil {
		return Verdict{}, err
	}

	v := Verdict{SchemaVersion: "1.0", Cycle: opts.Cycle}
	for _, pf := range files {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := now()
		exitCode, output := execFn(ctx, pf.path)
		dur := now().Sub(start).Milliseconds()
		cancel()

		rel := relPath(opts.Root, pf.path)
		r := Result{
			ACID:         acIDFromRel(rel),
			Predicate:    rel,
			ExitCode:     exitCode,
			DurationMS:   dur,
			IsRegression: pf.isRegression,
			IsRedTeam:    pf.isRedTeam,
		}
		switch exitCode {
		case 0:
			r.ResultStr = "green"
			v.GreenCount++
		case SkipExitCode:
			// TAP/automake SKIP: evidence absent / not applicable on this
			// clone. Counted neither red nor green; capture the SKIP reason.
			r.ResultStr = "skip"
			v.SkipCount++
			v.SkipIDs = append(v.SkipIDs, r.ACID)
			r.EvidenceExcerpt = excerpt(output)
		default:
			r.ResultStr = "red"
			v.RedCount++
			v.RedIDs = append(v.RedIDs, r.ACID)
			r.EvidenceExcerpt = excerpt(output)
		}
		switch {
		case pf.isRedTeam:
			v.PredicateSuite.RedTeamCount++
		case pf.isRegression:
			v.PredicateSuite.RegressionSuiteCount++
		default:
			v.PredicateSuite.ThisCycleCount++
		}
		v.Results = append(v.Results, r)
	}
	v.PredicateSuite.SkippedCount = v.SkipCount
	v.PredicateSuite.Total = len(v.Results) // skips included
	// Invariant: a skip increments neither GreenCount nor RedCount, so
	// PASS ⟺ red_count==0 is preserved exactly as before SKIP existed.
	if v.RedCount == 0 {
		v.Verdict = "PASS"
		v.ShipEligible = true
	} else {
		v.Verdict = "FAIL"
	}
	return v, nil
}

// discover globs the three predicate roots in a deterministic order.
func discover(root string, cycle int) ([]predFile, error) {
	var out []predFile

	cycleGlob := filepath.Join(root, "acs", fmt.Sprintf("cycle-%d", cycle), "*.sh")
	cyc, err := filepath.Glob(cycleGlob)
	if err != nil {
		return nil, fmt.Errorf("acssuite: glob cycle: %w", err)
	}
	sort.Strings(cyc)
	for _, p := range cyc {
		out = append(out, predFile{path: p})
	}

	regGlob := filepath.Join(root, "acs", "regression-suite", "cycle-*", "*.sh")
	reg, err := filepath.Glob(regGlob)
	if err != nil {
		return nil, fmt.Errorf("acssuite: glob regression: %w", err)
	}
	sort.Strings(reg)
	for _, p := range reg {
		out = append(out, predFile{path: p, isRegression: true})
	}

	rtGlob := filepath.Join(root, "acs", "red-team", "rt-*.sh")
	rt, err := filepath.Glob(rtGlob)
	if err != nil {
		return nil, fmt.Errorf("acssuite: glob red-team: %w", err)
	}
	sort.Strings(rt)
	for _, p := range rt {
		out = append(out, predFile{path: p, isRedTeam: true})
	}
	return out, nil
}

// runBash executes `bash <path>` and returns its exit code + combined output.
// A non-exec failure (e.g. bash missing) or timeout maps to a non-zero exit so
// the predicate counts as RED — the suite never silently swallows a failure.
//
// The predicate runs in its own process group so a timeout can kill the whole
// tree (a predicate that spawns `sleep` would otherwise hold the output pipe
// open and defeat ctx cancellation). The Cancel hook signals the group on
// timeout; in the narrow window where the deadline fires before Start sets
// cmd.Process the signal is skipped, so WaitDelay is the guaranteed backstop —
// it force-closes the pipes killGrace after cancellation, ensuring Run always
// returns (a timed-out predicate counts RED) rather than blocking on a child.
func runBash(ctx context.Context, path string) (int, string) {
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// Negative pid → signal the whole process group.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = killGrace
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if code := ee.ExitCode(); code >= 0 {
			return code, string(out)
		}
		// Killed by signal (timeout) → no exit code; treat as RED timeout.
		return 124, string(out) + "\n[acssuite] predicate killed (timeout)"
	}
	// Could not run at all (bash missing, etc.) — RED with diagnostic.
	return 126, string(out) + "\n[acssuite] exec error: " + err.Error()
}

// acIDFromRel derives a stable id from a repo-relative path: "<parent-dir>/<base-without-ext>".
func acIDFromRel(rel string) string {
	rel = strings.TrimSuffix(rel, ".sh")
	return strings.TrimPrefix(rel, "acs/")
}

func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= evidenceMax {
		return s
	}
	return s[:evidenceMax] + "…"
}

// WriteVerdict marshals v to <evolveDir>/runs/cycle-<N>/acs-verdict.json
// atomically (tmp + rename) and returns the path written.
func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dir := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", v.Cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("acssuite: mkdir %s: %w", dir, err)
	}
	dst := filepath.Join(dir, "acs-verdict.json")
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("acssuite: marshal: %w", err)
	}
	// Random tmp suffix (not PID) so concurrent same-process writers to the
	// same cycle dir cannot collide — matches acsrunner.WriteVerdict.
	tmpf, err := os.CreateTemp(dir, "acs-verdict.*.tmp")
	if err != nil {
		return "", fmt.Errorf("acssuite: create tmp: %w", err)
	}
	tmp := tmpf.Name()
	tmpf.Close()
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return "", fmt.Errorf("acssuite: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("acssuite: rename: %w", err)
	}
	return dst, nil
}
