// native.go — entry point for the native Go ship implementation.
//
// This is the v11.3.0 replacement for shelling out to
// legacy/scripts/lifecycle/ship.sh. The bash script remains canonical until v12.0.0;
// callers that want the legacy path set EVOLVE_NATIVE_SHIP=0.
//
// The native Run() reproduces the full ship.sh state machine:
//  1. arg parse (--class, --dry-run, commit message) + EVOLVE_BYPASS_SHIP_VERIFY bridge
//  2. self-SHA TOFU (verify.go)
//  3. class-aware verification (audit.go for cycle, prompt for manual, skip for release/trivial)
//  4. atomic ship: stage + commit + ff-merge (if worktree) + push + optional gh release (gitops.go)
//  5. post-ship: lastCycleNumber bump, inbox lifecycle, post-cycle SHA repin
//
// The 23-case parity matrix in native_test.go pins behavior against the
// bash ship-integration-test.sh suite.
package ship

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Class enumerates the four commit lifecycles. Matches ship.sh --class.
type Class string

const (
	ClassCycle   Class = "cycle"
	ClassManual  Class = "manual"
	ClassRelease Class = "release"
	ClassTrivial Class = "trivial"
)

// IsValid reports whether c is one of the four supported classes.
func (c Class) IsValid() bool {
	switch c {
	case ClassCycle, ClassManual, ClassRelease, ClassTrivial:
		return true
	}
	return false
}

// ExitCode bins ship.sh exit semantics:
//
//	0  — shipped
//	1  — runtime failure (bad args, missing binary, git fail)
//	2  — integrity failure (audit binding, manual-confirm refused, SHA-pin tamper)
//	127 — required binary missing (git, jq, sha256sum/shasum)
type ExitCode int

const (
	ExitOK         ExitCode = 0
	ExitFailure    ExitCode = 1
	ExitIntegrity  ExitCode = 2
	ExitMissingBin ExitCode = 127
)

// Options captures every external knob ship.sh exposes. Tests construct
// these directly; CLI builds them from flags/env.
type Options struct {
	// Class is the commit lifecycle (cycle/manual/release/trivial).
	Class Class

	// CommitMessage is the user-supplied commit body (footer appended later).
	CommitMessage string

	// DryRun skips all mutations but runs every read-only check.
	DryRun bool

	// ProjectRoot is the writable side — where git lives, where .evolve/ writes go.
	ProjectRoot string

	// PluginRoot is the read-only side — where .claude-plugin/plugin.json lives.
	// May equal ProjectRoot when running from the evolve-loop repo itself.
	PluginRoot string

	// ShipBinaryPath is the path to the binary whose SHA is TOFU-pinned.
	// In the bash impl this is ship.sh itself; in native this is the
	// evolve binary (resolved via os.Executable when empty).
	ShipBinaryPath string

	// Env overrides for the operator-facing env vars. Empty values fall
	// through to os.Getenv. Keys: EVOLVE_BYPASS_SHIP_VERIFY,
	// EVOLVE_SHIP_AUTO_CONFIRM, EVOLVE_STRICT_AUDIT, EVOLVE_SHIP_RELEASE_NOTES,
	// EVOLVE_BYPASS_PREFIX_GATE.
	Env map[string]string

	// Stdin/Stdout/Stderr default to the real streams when nil.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// NowFn is a clock seam for tests.
	NowFn func() Now

	// CmdRunner is the git/gh/external-binary execution seam. Tests
	// inject a fake; production wiring uses execRunner.
	Runner CmdRunner

	// internalAuditBoundTreeSHA is set by audit.go after parsing
	// audit-report.md; gitops.go reads it to enforce the pre-merge
	// tree-SHA binding check. Not part of the public API.
	internalAuditBoundTreeSHA string
}

// Now is a minimal time interface (Unix seconds + RFC3339 formatter) so
// the ship package doesn't pull in time-zone behavior at the entry point.
type Now struct {
	Unix    int64
	RFC3339 string
}

// RunResult is the structured outcome. CLI wrappers translate this to
// process exit code; the PhaseRunner adapter translates to core.Verdict.
type RunResult struct {
	ExitCode   ExitCode
	CommitSHA  string
	ClassUsed  Class
	Provenance string
	Logs       []string // human-readable [ship] log lines
	DryRunPath string   // non-empty when DryRun=1 and journal was written
}

// Run executes the ship lifecycle end-to-end. Caller-supplied opts drive
// behavior; missing fields are resolved from env/defaults.
//
// This is the public entry point for both the cmd_ship.go CLI surface
// and the PhaseRunner dispatcher in ship.go.
//
// On success: ExitCode=0, CommitSHA populated for non-dry-run cycle/manual/release classes.
// On integrity failure: ExitCode=2 with structured Logs explaining the refusal.
// On runtime failure: ExitCode=1 with the underlying err returned.
func Run(ctx context.Context, opts Options) (RunResult, error) {
	res := RunResult{ClassUsed: opts.Class}

	// 0. Validate inputs.
	if opts.CommitMessage == "" {
		return res, fmt.Errorf("ship: commit message required")
	}
	if !opts.Class.IsValid() {
		return res, fmt.Errorf("ship: invalid --class %q (must be: cycle|manual|release|trivial)", opts.Class)
	}
	if opts.ProjectRoot == "" {
		return res, fmt.Errorf("ship: ProjectRoot required")
	}

	// Defaults.
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.PluginRoot == "" {
		// When unset, fall back to ProjectRoot — the typical case when
		// running inside the evolve-loop source repo itself.
		opts.PluginRoot = opts.ProjectRoot
	}
	if opts.Runner == nil {
		opts.Runner = execRunner
	}
	if opts.NowFn == nil {
		opts.NowFn = defaultNow
	}

	// Handle the EVOLVE_BYPASS_SHIP_VERIFY=1 legacy bridge: translates to
	// --class manual + auto-confirm with a deprecation log. ship.sh does
	// this in section 1; replicating here preserves the audit trail and
	// the "Test K" deprecation-warning behavior.
	if opts.envBool("EVOLVE_BYPASS_SHIP_VERIFY") && opts.Class == ClassCycle {
		res.Logs = append(res.Logs,
			"DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1 is deprecated in v8.25.0+",
			"  → Migrate to: evolve ship --class manual \"<msg>\"",
			"  → Or for CI:  EVOLVE_SHIP_AUTO_CONFIRM=1 evolve ship --class manual \"<msg>\"",
			"  → Treating this invocation as: --class manual + EVOLVE_SHIP_AUTO_CONFIRM=1",
		)
		opts.Class = ClassManual
		if opts.Env == nil {
			opts.Env = map[string]string{}
		}
		opts.Env["EVOLVE_SHIP_AUTO_CONFIRM"] = "1"
		res.ClassUsed = ClassManual
	}

	// 1. Self-SHA TOFU verification. Writes state.json on first-run /
	// version-bump / legacy migration. INTEGRITY-FAILs on same-version
	// SHA mismatch.
	if err := verifySelfSHA(ctx, &opts, &res); err != nil {
		return finalize(ctx, &opts, &res, err, "verify-self-sha")
	}

	// 2. Class-aware pre-flight (audit-binding, kernel checks, or interactive confirm).
	if err := verifyClass(ctx, &opts, &res); err != nil {
		if _, isClean := err.(*cleanExitError); isClean {
			res.ExitCode = ExitOK
			writeDryRunJournal(ctx, &opts, &res, "no-staged-changes")
			return res, nil
		}
		return finalize(ctx, &opts, &res, err, "verify-class")
	}
	res.Logs = append(res.Logs, "[ship] provenance: "+res.Provenance)

	// 3. Atomic ship (commit + push + optional gh release).
	if err := atomicShip(ctx, &opts, &res); err != nil {
		return finalize(ctx, &opts, &res, err, "atomic-ship")
	}

	// 4. Post-ship hooks (lastCycleNumber, inbox lifecycle, post-cycle repin).
	if err := postShip(ctx, &opts, &res); err != nil {
		// Post-ship errors are non-fatal: the commit is already on remote.
		res.Logs = append(res.Logs, "[ship] WARN: post-ship hook error: "+err.Error())
	}

	res.ExitCode = ExitOK
	writeDryRunJournal(ctx, &opts, &res, "normal")
	return res, nil
}

// finalize classifies an error into the right ExitCode and writes the
// dry-run journal if applicable. Returns the result + a (possibly nil)
// error suitable for the caller.
func finalize(ctx context.Context, opts *Options, res *RunResult, err error, exitReason string) (RunResult, error) {
	if err == nil {
		res.ExitCode = ExitOK
	} else if _, isIntegrity := err.(*IntegrityError); isIntegrity {
		res.ExitCode = ExitIntegrity
		res.Logs = append(res.Logs, "[ship] INTEGRITY-FAIL: "+err.Error())
	} else {
		res.ExitCode = ExitFailure
		res.Logs = append(res.Logs, "[ship] FAIL: "+err.Error())
	}
	writeDryRunJournal(ctx, opts, res, exitReason)
	return *res, err
}

// envBool reads an env var from Opts.Env (with os.Getenv fallback) and
// reports whether it equals "1".
func (o *Options) envBool(key string) bool {
	if v, ok := o.Env[key]; ok {
		return v == "1"
	}
	return os.Getenv(key) == "1"
}

// envStr reads an env var from Opts.Env (with os.Getenv fallback).
func (o *Options) envStr(key string) string {
	if v, ok := o.Env[key]; ok {
		return v
	}
	return os.Getenv(key)
}
