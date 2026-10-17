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

type Class string

const (
	ClassCycle   Class = "cycle"
	ClassManual  Class = "manual"
	ClassRelease Class = "release"
	ClassTrivial Class = "trivial"
)

func (c Class) IsValid() bool {
	switch c {
	case ClassCycle, ClassManual, ClassRelease, ClassTrivial:
		return true
	}
	return false
}

type ExitCode int

const (
	ExitOK         ExitCode = 0
	ExitFailure    ExitCode = 1
	ExitIntegrity  ExitCode = 2
	ExitMissingBin ExitCode = 127
)

type Options struct {
	Class Class

	CommitMessage string

	DryRun bool

	BypassCommitGate bool
	BypassPrefixGate bool
	PushOnly         bool

	ProjectRoot string

	WorkspacePath string

	CycleID                         int
	AuditRound                      int
	ActiveWorktree                  string
	WorktreeBaseSHA                 string
	ExplanationDocumentationVersion int
	BuildExplanation                *phaseio.ExplanationView
	RequireBuildExplanationHandoff  bool

	ManifestGate string

	RunID string

	PluginRoot string

	ShipBinaryPath string

	Env map[string]string

	PhaseIO config.Stage

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	NowFn func() Now

	Sleep func(time.Duration)

	Runner CmdRunner

	internalAuditBoundTreeSHA string
	internalAuditArtifactSHA  string

	internalConsumedPaths []string

	repairAttempted map[core.ShipErrorCode]bool

	shipLock func(path string) (release func(), err error)

	Signals *signalcenter.Center

	land *landing.Landing

	relPath func(base, target string) (string, error)
}

type Now struct {
	Unix    int64
	RFC3339 string
}

type RunResult struct {
	ExitCode   ExitCode
	CommitSHA  string
	ClassUsed  Class
	Provenance string
	Logs       []string
	DryRunPath string

	RepairAttempted string
	RepairOutcome   string
}

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

func finalize(ctx context.Context, opts *Options, res *RunResult, err error, exitReason string) (RunResult, error) {
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
	if !hasCycle || bindingCycle != cid || bindingCommitSHA == "" || committedTree == "" || auditTree == "" {
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
	if ok, _ := auditBindingSatisfied(ctx, resumedAuditView(opts, auditTree), "", committedTree); !ok {
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
