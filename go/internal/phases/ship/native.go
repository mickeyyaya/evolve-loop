package ship

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
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

// ExitCode bins ship.sh exit semantics (0 shipped, 1 runtime failure,
// 2 integrity failure, 127 required binary missing).
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
	Class Class

	// CommitMessage is the user-supplied commit body (footer appended later).
	CommitMessage string

	// DryRun skips all mutations but runs every read-only check.
	DryRun bool

	// Explicit emergency bypasses. The CLI exposes these as flags; environment
	// variables are intentionally not consulted.
	BypassCommitGate bool
	BypassPrefixGate bool
	// PushOnly pushes an already-committed, provenance-verified ahead set and
	// nothing else (pushonly.go); staged changes refuse.
	PushOnly bool

	// ProjectRoot is the writable side — where git lives, where .evolve/ writes go.
	ProjectRoot string

	// WorkspacePath is this run's per-cycle workspace
	// (<ProjectRoot>/.evolve/runs/cycle-<N>/); empty means a standalone
	// `evolve ship` uses the global cycle-state.json.
	// See ADR-0049.
	WorkspacePath string

	// Cycle explanation identity comes from the host-owned PhaseRequest, never
	// the Builder-writable workspace/run.json mirror; direct CLI calls leave it
	// zero and the native gate resolves it from cycle-state.json.
	CycleID                         int
	AuditRound                      int
	ActiveWorktree                  string
	WorktreeBaseSHA                 string
	ExplanationDocumentationVersion int
	BuildExplanation                *phaseio.ExplanationView
	RequireBuildExplanationHandoff  bool

	// ManifestGate is the ship-hygiene gate mode for the worktree ship path:
	// "" (default) is SHADOW (log out-of-manifest paths, never block);
	// "enforce" is FAIL-CLOSED (refuse to commit any path no phase report
	// declared). Config-sourced (policy.json); today only tests set enforce.
	ManifestGate string

	// RunID is this run's event-sourced identity. When set, the audit→ship
	// binding lookup (findLatestAudit) prefers the ledger entry stamped with
	// THIS RunID over a concurrent run's later entry.
	// See ADR-0049.
	RunID string

	// PluginRoot is the read-only side — where .claude-plugin/plugin.json lives.
	// May equal ProjectRoot when running from the evolve-loop repo itself.
	PluginRoot string

	// ShipBinaryPath is the path to the binary whose SHA is TOFU-pinned.
	// In the bash impl this is ship.sh itself; in native this is the
	// evolve binary (resolved via os.Executable when empty).
	ShipBinaryPath string

	// Env overrides for the operator-facing env vars. Empty values fall
	// through to os.Getenv. Key: EVOLVE_SHIP_AUTO_CONFIRM. Strict-audit is
	// sourced from .evolve/policy.json (workflow.strict_audit), not here.
	// EVOLVE_SHIP_RELEASE_NOTES is IPC-only (releasepipeline → evolve-ship
	// subprocess; split-const form).
	Env map[string]string

	// PhaseIO threads the EVOLVE_PHASE_IO stage into the audit-binding verdict
	// parse. At >= StageEnforce parseVerdicts is sentinel-first; StageOff (the
	// zero value) keeps the prose parse.
	// See ADR-0050.
	PhaseIO config.Stage

	// Stdin/Stdout/Stderr default to the real streams when nil.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// NowFn is a clock seam for tests.
	NowFn func() Now

	Sleep func(time.Duration)

	// CmdRunner is the git/gh/external-binary execution seam. Tests
	// inject a fake; production wiring uses execRunner.
	Runner CmdRunner

	internalAuditBoundTreeSHA string
	internalAuditArtifactSHA  string

	internalConsumedPaths []string

	// repairAttempted is the repair ladder's once-per-code-per-Run guard.
	repairAttempted map[core.ShipErrorCode]bool

	// shipLock is the test seam for the integrator lock; nil defaults to
	// flock.Lock on <ProjectRoot>/.evolve/ship.lock. Signature mirrors
	// flock.Lock.
	shipLock func(path string) (release func(), err error)

	// Signals is the root's Signal Center the landing's ship.warning events
	// reach (ADR-0103 unit 07): the orchestrator root threads it through
	// Config.Signals, the standalone `evolve ship` root builds its own; nil
	// (the `evolve phase ship` registry factory, direct-helper tests) is the
	// Null Object — the warnings are dropped, never a panic.
	Signals *signalcenter.Center

	// land is the landing, lazily built and cached by landing() (gitops_landing.go).
	land *landing.Landing
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
	Logs       []string
	DryRunPath string // non-empty when DryRun=1 and journal was written

	// RepairAttempted/RepairOutcome surface the repair ladder: the ShipError
	// code a typed repair was attempted for, and its outcome (e.g.
	// "repinned-verified-rebuild", "resume-pushed", "push-retried",
	// "colliders-healed:…", "needs-reaudit", "declined"); empty when no
	// repair fired. Multiple repairs in one Run record only the LAST
	// attempt — the full trail lives in Logs and the ShipError Debug map.
	// See ADR-0039.
	RepairAttempted string
	RepairOutcome   string
}

// Run executes the ship lifecycle end-to-end; missing Options fields are
// resolved from env/defaults.
func Run(ctx context.Context, opts Options) (RunResult, error) {
	res, resumed, err := resumeFirst(ctx, &opts)
	if resumed || err != nil {
		return res, err
	}
	return runStages(ctx, &opts, res)
}

func resumeFirst(ctx context.Context, opts *Options) (RunResult, bool, error) {
	res := RunResult{ClassUsed: opts.Class}
	if err := opts.validate(); err != nil {
		return res, false, err
	}
	opts.applyDefaults()
	resumed, err := resumeLanding(ctx, opts, &res)
	if err != nil {
		out, ferr := finalize(ctx, opts, &res, err, "landing-resume")
		return out, true, ferr
	}
	if !resumed {
		return res, false, nil
	}
	if err := postShip(ctx, opts, &res); err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: post-ship hook error: "+err.Error())
	}
	out, ferr := finalize(ctx, opts, &res, nil, landingResumed)
	return out, true, ferr
}

func (o *Options) validate() error {
	if o.CommitMessage == "" && !o.PushOnly {
		return shipErr(core.CodeArgs, core.ShipClassConfig, core.StageArgs,
			"ship: commit message required")
	}
	if !o.Class.IsValid() && !o.PushOnly {
		return shipErr(core.CodeInvalidClass, core.ShipClassConfig, core.StageArgs,
			"ship: invalid --class "+string(o.Class)+" (must be: cycle|manual|release|trivial)",
			"class", string(o.Class))
	}
	if o.ProjectRoot == "" {
		return shipErr(core.CodeArgs, core.ShipClassConfig, core.StageArgs,
			"ship: ProjectRoot required")
	}
	return nil
}

func (o *Options) applyDefaults() {
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.PluginRoot == "" {
		o.PluginRoot = o.ProjectRoot
	}
	if o.Runner == nil {
		o.Runner = sysexec.DefaultRunner
	}
	if o.NowFn == nil {
		o.NowFn = defaultNow
	}
}

func runStages(ctx context.Context, opts *Options, res RunResult) (RunResult, error) {
	if err := verifyNativeExplanation(ctx, opts); err != nil {
		return finalize(ctx, opts, &res, shipErr(
			core.CodeExplanationDocumentation,
			core.ShipClassPrecondition,
			core.StageVerifyExplanation,
			"ship explanation documentation gate: "+err.Error(),
		), "verify-explanation-documentation")
	}
	if _, err := runStageWithRepair(ctx, opts, &res, func() error {
		return verifySelfSHA(ctx, opts, &res)
	}); err != nil {
		return finalize(ctx, opts, &res, err, "verify-self-sha")
	}
	if opts.PushOnly {
		return finalize(ctx, opts, &res, runPushOnly(ctx, opts, &res), "push-only")
	}
	if opts.Class == ClassCycle {
		commitSHA, idempotent, err := checkPostPushIdempotency(ctx, opts)
		if err == nil && idempotent {
			return reportPostPush(ctx, opts, &res, commitSHA)
		}
	}
	return shipVerified(ctx, opts, &res)
}

func reportPostPush(ctx context.Context, opts *Options, res *RunResult, commitSHA string) (RunResult, error) {
	if err := verifyPostPushPredicateEvidence(ctx, opts, res, commitSHA); err != nil {
		return finalize(ctx, opts, res, err, "verify-post-push-predicate-evidence")
	}
	res.CommitSHA = commitSHA
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] post-push correction detected (HEAD is already the ship commit %s); succeeding report-only", res.CommitSHA))
	return finalize(ctx, opts, res, nil, "post-push-idempotency")
}

func shipVerified(ctx context.Context, opts *Options, res *RunResult) (RunResult, error) {
	resumed, err := runStageWithRepair(ctx, opts, res, func() error {
		return verifyClass(ctx, opts, res)
	})
	if err != nil {
		if _, isClean := err.(*cleanExitError); isClean {
			res.ExitCode = ExitOK
			writeDryRunJournal(ctx, opts, res, "no-staged-changes")
			return *res, nil
		}
		return finalize(ctx, opts, res, err, "verify-class")
	}
	res.Logs = append(res.Logs, "[ship] provenance: "+res.Provenance)
	if !resumed {
		if _, err := runStageWithRepair(ctx, opts, res, func() error {
			return atomicShip(ctx, opts, res)
		}); err != nil {
			return finalize(ctx, opts, res, err, "atomic-ship")
		}
	}
	if err := postShip(ctx, opts, res); err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: post-ship hook error: "+err.Error())
	}
	return finalize(ctx, opts, res, nil, "normal")
}

// finalize classifies an error into the right ExitCode and writes the
// dry-run journal if applicable. Returns the result + a (possibly nil)
// error suitable for the caller.
func finalize(ctx context.Context, opts *Options, res *RunResult, err error, exitReason string) (RunResult, error) {
	// Every MINTED commit is journaled, on success or after a rejected push
	// (pushonly.go); push-only itself mints nothing so it is exempt.
	journalMinted(opts, res)
	if err == nil {
		res.ExitCode = ExitOK
		writeDryRunJournal(ctx, opts, res, exitReason)
		return *res, err
	}
	if se, ok := core.AsShipError(err); ok && se.Class == core.ShipClassIntegrity {
		res.ExitCode = ExitIntegrity
		res.Logs = append(res.Logs, "[ship] INTEGRITY-FAIL: "+err.Error())
	} else {
		res.ExitCode = ExitFailure
		res.Logs = append(res.Logs, "[ship] FAIL: "+err.Error())
	}
	writeDryRunJournal(ctx, opts, res, exitReason)
	return *res, err
}

func (o *Options) envBool(key string) bool {
	if v, ok := o.Env[key]; ok {
		return v == "1"
	}
	return os.Getenv(key) == "1"
}

func (o *Options) envStr(key string) string {
	if v, ok := o.Env[key]; ok {
		return v
	}
	return os.Getenv(key)
}

func checkPostPushIdempotency(ctx context.Context, opts *Options) (string, bool, error) {
	cid, ok, err := cycleIDForShip(opts)
	if err != nil || !ok {
		return "", false, err
	}
	bindingPath := filepath.Join(core.RunWorkspacePath(opts.ProjectRoot, cid), dossier.ShipBindingFile)
	bindingMap, err := readStateMap(bindingPath)
	if err != nil {
		return "", false, err
	}
	bindingCycle, hasCycle := stateInt(bindingMap, "cycle")
	bindingCommitSHA := stateString(bindingMap, "commit_sha")
	committedTree := stateString(bindingMap, "tree_sha_committed")
	auditTree := stateString(bindingMap, "audit_bound_tree_sha")
	if !hasCycle || bindingCycle != cid || bindingCommitSHA == "" || committedTree == "" || auditTree == "" || auditTree != committedTree {
		return "", false, nil
	}
	head, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	if err != nil {
		return "", false, err
	}
	head = strings.TrimSpace(head)
	if head != bindingCommitSHA {
		return "", false, nil
	}
	headTree, err := captureGitOutput(ctx, opts, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return "", false, err
	}
	headTree = strings.TrimSpace(headTree)
	if headTree == "" || headTree != committedTree {
		return "", false, nil
	}
	return head, true, nil
}

func journalMinted(opts *Options, res *RunResult) {
	if res.CommitSHA == "" || opts.DryRun || opts.PushOnly || journalHasSHA(opts.ProjectRoot, res.CommitSHA) {
		return
	}
	cid, _, cerr := cycleIDForShip(opts)
	err := errors.Join(cerr, appendShipJournal(opts.ProjectRoot, shipJournalEntry{SHA: res.CommitSHA, Class: string(opts.Class), Cycle: cid}))
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: the ship journal entry of %s is not complete: %v", res.CommitSHA, err))
	}
}

func runIDForShip(opts *Options) (string, error) {
	if opts.RunID != "" {
		return opts.RunID, nil
	}
	state, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return "", err
	}
	return stateString(state, "run_id"), nil
}

func cycleIDForShip(opts *Options) (int, bool, error) {
	if opts.CycleID > 0 {
		return opts.CycleID, true, nil
	}
	state, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return 0, false, err
	}
	cycle, ok := stateInt(state, "cycle_id")
	return cycle, ok && cycle > 0, nil
}
