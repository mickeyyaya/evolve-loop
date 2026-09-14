// Package subagentrun is unit 16 of the component breakdown (ADR-0103): the
// `evolve subagent run` execution path. One Dispatcher owns request admission,
// cli/tier/capability resolution, artifact placement, the challenge token and
// git provenance, prompt composition and staging, the bridge exec port,
// artifact verification (the one verdict ladder every dispatch path shares)
// and the agent_subprocess ledger record. Its collaborators are explicit at
// construction: the ten host-backed ports in Deps (the profile grammar, the
// LLM router, the capability inspector, the tier resolver, the role
// allow-list, the recursion cap, the driver check, the bridge adapter, git and
// the run-id resolver) and six stdlib-backed options (clock, entropy, the
// filesystem trio, the prompt stager, the ledger opener, the Signal Center
// accessor). The leaf never writes stderr, never reads the environment, never
// shells git or tmux, and reports its eleven failure modes as bridge.warning
// under module bridge with the BRIDGE_SUBAGENT_ sub-prefix. Design:
// docs/architecture/decomposition/16-subagentrun.md.
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

// The unit's codes — the eleven WARN conditions of one dispatch, registered
// with their reasons under the bridge module (BRIDGE_SUBAGENT_ fences them
// from the engine's BRIDGE_TOKEN_* / BRIDGE_TELEMETRY_* vocabulary).
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

// The dispatch vocabulary, moved verbatim from the host (the host projects each
// by name so its callers keep their spelling).
const (
	// VerdictPASS, VerdictFAIL and VerdictIntegrityFail are the three verdicts a
	// dispatch ends in.
	VerdictPASS          = "PASS"
	VerdictFAIL          = "FAIL"
	VerdictIntegrityFail = "INTEGRITY_FAIL"
	// ArtifactMaxAge is the freshness window: the artifact must have been
	// written within the last 5 minutes to be considered fresh.
	ArtifactMaxAge = 5 * time.Minute
	// ChallengeTokenBytes is the size of the random source used for the
	// 16-hex token (8 bytes → 16 hex chars).
	ChallengeTokenBytes = 8
)

// ErrInProcessDispatchBanned is returned when a caller requests the retired
// in-process dispatch path (LEGACY_AGENT_DISPATCH=1). The agent-bridge
// (`evolve subagent run`) is the ONE and ONLY supported dispatch path; the
// historical escape hatch is gone — setting the flag fails loudly rather than
// silently routing in-process.
var ErrInProcessDispatchBanned = errors.New(
	"subagent/run: in-process dispatch (LEGACY_AGENT_DISPATCH) is retired — all agent dispatch must go through the bridge (`evolve subagent run`); unset LEGACY_AGENT_DISPATCH",
)

// Deps are the host-backed collaborators of one Dispatcher — every one
// required; a nil field is a programming error and panics at first use (no
// guard). The host seam projects its own shapes onto these once.
type Deps struct {
	// Profile reads and projects the agent profile at path.
	Profile func(path string) (Profile, error)
	// ResolveLLM asks the LLM router for the role's cli and tier.
	ResolveLLM func(role string) (LLM, error)
	// Inspect reads the cli's capability manifest under capDir.
	Inspect func(capDir, cli string) (Capability, error)
	// ResolveTier is the adaptive model-tier resolver, consulted only when the
	// router returned no tier.
	ResolveTier func(TierRequest) (string, error)
	// KnownRole is the agent-role allow-list.
	KnownRole func(role string) bool
	// GuardDepth rejects a dispatch running deeper than the recursion cap and
	// returns the host's sentinel.
	GuardDepth func(depth int) error
	// AdapterExists reports whether the resolved cli has a registered driver.
	// It receives the cli itself — the vestigial <AdaptersDir>/<cli>.sh path
	// is only the rejection's error text and its signal's `path`.
	AdapterExists func(cli string) bool
	// Adapter is the bridge exec port.
	Adapter Adapter
	// GitState captures HEAD and the tree-diff sha of projectRoot.
	GitState func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	// RunID resolves the run identity from the run workspace ("" when none).
	RunID func(workspace string) string
}

// Dispatcher owns one `evolve subagent run` execution path. Immutable after
// New; a host builds one per Run call.
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

// Option configures a Dispatcher at construction (functional options).
type Option func(*Dispatcher)

// New builds the dispatcher over its required ports with the production
// defaults for every stdlib-backed collaborator: the wall clock, crypto/rand,
// os.Stat / os.ReadFile / a streaming sha256, an os.CreateTemp prompt stager,
// an O_APPEND ledger opener and no Signal Center (the Null Object).
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

// WithClock replaces the wall clock (four reads per run: exec start, exec
// end, the verification instant, the ledger timestamp).
func WithClock(now func() time.Time) Option { return func(d *Dispatcher) { d.now = now } }

// WithRand replaces the entropy source the challenge token is minted from.
func WithRand(rng func([]byte) (int, error)) Option { return func(d *Dispatcher) { d.rand = rng } }

// WithFS replaces the artifact-side filesystem trio: the modtime stat and
// the body read the verification ladder gathers with, and the artifact hash.
// os.Stat on the workspace and os.MkdirAll on the artifact directory stay
// direct (a tmp dir and a file in the way provoke both).
func WithFS(stat func(string) (time.Time, error), read func(string) ([]byte, error), hash func(string) (string, error)) Option {
	return func(d *Dispatcher) { d.stat, d.read, d.hash = stat, read, hash }
}

// WithStager replaces the prompt stager (Strategy): how the composed prompt
// is materialised for the adapter.
func WithStager(s PromptStager) Option { return func(d *Dispatcher) { d.stager = s } }

// WithLedgerOpener replaces how the ledger file is opened for append — the
// seam that reaches the write and close branches (signalcenter's ndjson-sink
// idiom).
func WithLedgerOpener(open func(path string) (io.WriteCloser, error)) Option {
	return func(d *Dispatcher) { d.openLedger = open }
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use. A nil accessor, or one returning nil, is the
// Null Object; SignalsWired proves a root wired one.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(d *Dispatcher) { d.signals = c }
}

// SignalsWired reports whether the dispatcher currently reaches a Center.
func (d *Dispatcher) SignalsWired() bool { return d.center() != nil }

func (d *Dispatcher) center() *signalcenter.Center {
	if d.signals == nil {
		return nil
	}
	return d.signals()
}

// warn is the unit's one producer: a bridge.warning WARN under module bridge,
// stamped with the cycle, the FULL dispatched agent name as the phase (the
// same value the ledger's role and the engine's dispatch identity carry) and
// the run id resolved after admission. Every event carries the parsed role,
// the worker subtask when there is one, and the caller's step. A nil Center
// is the Null Object (Emit on nil is a no-op).
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
