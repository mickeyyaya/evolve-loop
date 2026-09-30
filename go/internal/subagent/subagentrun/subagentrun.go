// Package subagentrun is the `evolve subagent run` execution path, from request
// admission through the bridge exec to artifact verification and the ledger record.
// See docs/architecture/packages/internal-subagent-subagentrun.md.
package subagentrun

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeRequestRejected       signalcenter.Code = "BRIDGE_SUBAGENT_REQUEST_REJECTED"
	CodeResolutionFailed      signalcenter.Code = "BRIDGE_SUBAGENT_RESOLUTION_FAILED"
	CodeLLMResolveFallback    signalcenter.Code = "BRIDGE_SUBAGENT_LLM_RESOLVE_FALLBACK"
	CodeWorktreeFallback      signalcenter.Code = "BRIDGE_SUBAGENT_WORKTREE_FALLBACK"
	CodeGitStateUnknown       signalcenter.Code = "BRIDGE_SUBAGENT_GIT_STATE_UNKNOWN"
	CodePrepareFailed         signalcenter.Code = "BRIDGE_SUBAGENT_PREPARE_FAILED"
	CodeAdapterExecFailed     signalcenter.Code = "BRIDGE_SUBAGENT_ADAPTER_EXEC_FAILED"
	CodeArtifactIntegrityFail signalcenter.Code = "BRIDGE_SUBAGENT_ARTIFACT_INTEGRITY_FAIL"
	CodeVerdictFail           signalcenter.Code = "BRIDGE_SUBAGENT_VERDICT_FAIL"
	CodeArtifactHashFailed    signalcenter.Code = "BRIDGE_SUBAGENT_ARTIFACT_HASH_FAILED"
	CodeLedgerWriteFailed     signalcenter.Code = "BRIDGE_SUBAGENT_LEDGER_WRITE_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeRequestRejected, "an `evolve subagent run` request was rejected at admission (no prompt reader, an unknown agent role, a negative cycle, a missing workspace, the retired in-process escape hatch, or the recursion depth cap); no port was consulted; fields.step=validate, reason_class")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeResolutionFailed, "the dispatch could not resolve its profile, cli, driver, model tier or capability manifest (fields.step names which: profile | cli | driver | tier | capability; the profile step's reason carries the read error the returned text drops)")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeLLMResolveFallback, "the LLM router returned an error, so the cli was taken from the profile's cli field with source=profile — silent before unit 16; fields.step=cli, source, cli")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeWorktreeFallback, "WorktreePath was not propagated, so WORKTREE_PATH falls back to the project root and the agent runs against the main tree (the Warns entry callers print is kept); fields.step=env, worktree, project_root")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeGitStateUnknown, "git HEAD or the tree-diff sha could not be captured (an error or an empty value), so the ledger line stamps \"unknown\" — silent before unit 16; fields.step=provenance, head, tree_state, project_root")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodePrepareFailed, "the artifact directory, the challenge token, the prompt read or the prompt temp file failed before the adapter ran (fields.step = artifact_dir | token | prompt | stage; fields.op = create | write on stage)")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeAdapterExecFailed, "the bridge adapter returned an infrastructure error (exit -1 on a launch fault); the ledger line is still written and the verdict fields carry what the artifact ladder found; the ONE outcome signal of such a run; fields.step=exec, exit_code, verdict, integrity")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeArtifactIntegrityFail, "the adapter returned but the artifact failed the integrity ladder (fields.rung = missing | stale | unreadable | empty | token_missing); the reason is the ladder's diagnostic the run's result used to drop; fields.step=verify, artifact, exit_code")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeVerdictFail, "the artifact is sound but the adapter exited non-zero (the bridge's exit codes reach here as fields.exit_code); fields.step=verify, cli, artifact")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeArtifactHashFailed, "the artifact stood the ladder (PASS or FAIL) but could not be hashed, so the ledger line stamps artifact_sha256=\"\" — silent before unit 16; fields.step=hash, artifact")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeLedgerWriteFailed, "the agent_subprocess ledger line or its tip could not be written (fields.op = mkdir | chain_link | open | write | close | tip_tmp | tip_rename); the returned error keeps its bare text and masks any adapter error, exactly as before; fields.step=ledger, path")
}

const (
	VerdictPASS          = "PASS"
	VerdictFAIL          = "FAIL"
	VerdictIntegrityFail = "INTEGRITY_FAIL"
	ArtifactMaxAge       = 5 * time.Minute
	ChallengeTokenBytes  = 8
)

var ErrInProcessDispatchBanned = errors.New(
	"subagent/run: in-process dispatch (LEGACY_AGENT_DISPATCH) is retired — all agent dispatch must go through the bridge (`evolve subagent run`); unset LEGACY_AGENT_DISPATCH",
)

type Deps struct {
	Profile       func(path string) (Profile, error)
	ResolveLLM    func(role string) (LLM, error)
	Inspect       func(capDir, cli string) (Capability, error)
	ResolveTier   func(TierRequest) (string, error)
	KnownRole     func(role string) bool
	GuardDepth    func(depth int) error
	AdapterExists func(cli string) bool
	Adapter       Adapter
	GitState      func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	RunID         func(workspace string) string
}

type Dispatcher struct {
	deps       Deps
	now        func() time.Time
	rand       func([]byte) (int, error)
	stat       func(string) (time.Time, error)
	read       func(string) ([]byte, error)
	hash       func(string) (string, error)
	stager     PromptStager
	openLedger func(string) (io.WriteCloser, error)
	signals    func() *signalcenter.Center
}

type Option func(*Dispatcher)

func New(deps Deps, opts ...Option) *Dispatcher {
	d := &Dispatcher{
		deps: deps, now: time.Now, rand: rand.Read,
		stat: StatMTime, read: os.ReadFile, hash: HashFile,
		stager: tempFileStager{create: os.CreateTemp}, openLedger: openAppend,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func WithClock(now func() time.Time) Option { return func(d *Dispatcher) { d.now = now } }

func WithRand(rng func([]byte) (int, error)) Option { return func(d *Dispatcher) { d.rand = rng } }

func WithFS(stat func(string) (time.Time, error), read func(string) ([]byte, error), hash func(string) (string, error)) Option {
	return func(d *Dispatcher) { d.stat, d.read, d.hash = stat, read, hash }
}

func WithStager(s PromptStager) Option { return func(d *Dispatcher) { d.stager = s } }

func WithLedgerOpener(open func(path string) (io.WriteCloser, error)) Option {
	return func(d *Dispatcher) { d.openLedger = open }
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(d *Dispatcher) { d.signals = c }
}

func (d *Dispatcher) SignalsWired() bool { return d.center() != nil }

func (d *Dispatcher) center() *signalcenter.Center {
	if d.signals == nil {
		return nil
	}
	return d.signals()
}

func (d *Dispatcher) warn(id identity, cycle int, code signalcenter.Code, reason string, fields map[string]string) {
	f := make(map[string]string, len(fields)+2)
	for k, v := range fields {
		f[k] = v
	}
	f["role"] = id.role
	if id.worker != "" {
		f["worker"] = id.worker
	}
	d.center().Emit(signalcenter.Event{
		Cycle: cycle, RunID: id.runID, Phase: id.agent, Module: signalcenter.ModuleBridge, Origin: "Dispatcher.Dispatch",
		Kind: signalcenter.KindBridgeWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: f,
	})
}
