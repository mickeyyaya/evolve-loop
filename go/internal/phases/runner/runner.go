// Package runner provides BaseRunner, a Template Method implementation
// of the shared phase-dispatch skeleton. Each subagent-dispatching phase
// (intent, scout, triage, tdd, build, audit) supplies a tiny Hooks
// implementation; BaseRunner orchestrates the identical surrounding
// logic — profile lookup, prompt composition, bridge dispatch, artifact
// reading, classification, response packaging.
//
// Pattern: Template Method (GoF). The "template" is BaseRunner.Run; the
// "primitive operations" are the Hooks methods. Phases override the
// variation points without touching the dispatch shape.
//
// Goals:
//
//   - DRY: collapse ~70 LoC of identical boilerplate per phase to ~5
//   - SRP: each Hooks method does one thing (compose, classify, etc.)
//   - Test-stability: the existing per-phase integration tests assert
//     the same external contract (BridgeRequest shape, PhaseResponse
//     fields), so they keep passing across the refactor — the
//     behavior-preservation harness called out in the plan.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// Hooks captures the per-phase variation points BaseRunner delegates
// to. Implementations are typically small value types embedded in each
// phase package's Phase struct.
type Hooks interface {
	// PhaseName returns the canonical phase identifier ("build",
	// "scout", "audit"). Used for phaseflags lookup, profile-file name,
	// BridgeRequest.Agent, and model env-var key.
	PhaseName() string

	// AgentPromptName returns the agent doc to load via
	// prompts.Loader.Agent (e.g., "evolve-builder" for the build
	// phase). Differs from PhaseName because agent docs historically
	// carry the "evolve-" prefix.
	AgentPromptName() string

	// ArtifactFilename returns the artifact the agent is contracted to
	// produce, joined with req.Workspace. Takes req so phases can vary
	// the filename per-request (e.g., intent's delta mode chooses
	// "intent-delta.md" instead of "intent.md").
	ArtifactFilename(req core.PhaseRequest) string

	// DefaultModel returns the model identifier to use when
	// EVOLVE_<PHASE>_MODEL is unset. Most phases use "auto"; audit
	// uses "opus" for adversarial cross-family diversity.
	DefaultModel() string

	// ComposePrompt assembles the final prompt sent to the bridge. The
	// agent doc body comes pre-loaded; phases typically append a cycle
	// context block.
	ComposePrompt(agentBody string, req core.PhaseRequest) string

	// Classify inspects the artifact (file contents or stdout) and
	// returns the phase's verdict, any diagnostics, and the next phase
	// name. BaseRunner handles bridge-error and missing-artifact paths
	// before calling Classify; this method only runs on the success
	// branch.
	Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (verdict string, diagnostics []core.Diagnostic, nextPhase string)
}

// Skipper is an optional Hooks extension. When a Hooks implementation
// also satisfies Skipper, BaseRunner consults ShouldSkip before any
// bridge call. If skipped is true, BaseRunner returns a SKIPPED
// PhaseResponse with the supplied verdict and nextPhase and never
// touches the bridge. Used by triage (EVOLVE_TRIAGE_DISABLE), tdd
// (EVOLVE_TEST_PHASE_ENABLED=0), and retro (previous verdict guard).
//
// Why optional? Most phases never skip. Forcing every Hooks impl to
// implement a no-op ShouldSkip violates ISP (interface-segregation).
type Skipper interface {
	ShouldSkip(req core.PhaseRequest) (skipped bool, verdict, nextPhase string, diags []core.Diagnostic)
}

// InlinePromptProvider is an optional Hooks extension. When a Hooks
// implementation also satisfies it AND returns ok=true, BaseRunner composes
// the prompt from the supplied in-band body and never reads
// agents/<AgentPromptName>.md. Returning ("", false) — or not implementing
// this interface at all — preserves the legacy disk-load path byte-for-byte.
//
// Used by minted/spec phases (specrunner) that ship their prompt as data
// (no file on disk). Optional for the same ISP reason as Skipper: built-in
// phases load their agent docs from disk and must not be forced to implement
// a no-op.
// SecondaryArtifactsProvider is an OPTIONAL hook: phases whose contract
// requires deliverables beyond the primary artifact (audit on continuation
// cycles: defect-dispositions.json) return their absolute paths; the bridge
// completion detector holds phase-complete until each exists (Phase B).
type SecondaryArtifactsProvider interface {
	SecondaryArtifacts(req core.PhaseRequest) []string
}

// secondaryArtifacts resolves the optional hook; nil for phases without it.
func secondaryArtifacts(h Hooks, req core.PhaseRequest) []string {
	if sp, ok := h.(SecondaryArtifactsProvider); ok {
		return sp.SecondaryArtifacts(req)
	}
	return nil
}

type InlinePromptProvider interface {
	InlinePromptBody() (string, bool)
}

// Options is the BaseRunner constructor envelope. Bridge and Prompts
// are required; NowFn defaults to time.Now.
type Options struct {
	Hooks   Hooks
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// ResolveLLM is the seam for resolving the "auto" model sentinel.
	// When nil, defaults to resolvellm.Resolve.
	ResolveLLM func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	// StdoutFilter is the seam for the post-phase .clean.txt writer.
	// When nil, defaults to logfilter.Process. Per-instance field (not a
	// package global) keeps t.Parallel() tests race-free.
	StdoutFilter func(workspace, phase string) error
	// EventsProducer is the seam for the post-phase <phase>-events.ndjson
	// writer (ADR-0020). When nil, defaults to phasestream.Produce. Unlike
	// StdoutFilter this is load-bearing: cyclecost + cycleclassify read the
	// events stream, so it is always-on (no disable flag). prompt is the
	// composed phase prompt, threaded to the Classifier's echo-veto
	// (ProduceConfig.InjectedPrompt, cycle-672) so agent-quoted prompt text
	// never classifies infra_failure.
	EventsProducer func(workspace, phase, cli string, cycle int, prompt string) error
	// Optional marks this phase as non-essential to the cycle. When true, a
	// bridge ErrArtifactTimeout degrades to a WARN that lets the cycle
	// advance (the state machine's successor is verdict-unconditional for
	// optional phases like build-planner) instead of aborting. Set by the
	// owning phase (e.g. buildplanner.New). Default false = hard-fail, the
	// historical behavior for mandatory phases. See Workstream D / cycle-120.
	Optional bool
	// VerifyFn is the seam for the deliverable well-formedness check used by
	// reconcile-on-timeout: when the bridge reports ErrArtifactTimeout but the
	// agent's contracted deliverable is on disk and well-formed, the runner
	// trusts the deliverable's verdict instead of synthesizing FAIL. When nil,
	// defaults to deliverable.Verify. Per-instance (not a package global) so
	// t.Parallel() tests stay race-free, mirroring StdoutFilter.
	VerifyFn func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	// SleepFn is the seam for the delay between verifyReconcileDeliverable's
	// bounded settle-retry attempts (see its doc for the cycles 824/825
	// rationale). When nil, defaults to time.Sleep. Per-instance so
	// t.Parallel() tests can inject a no-op for determinism, mirroring NowFn.
	SleepFn func(time.Duration)
	// PhaseIO is the EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.10). When VerifyFn
	// is nil, it is threaded into the catalog-aware reconcile default so the
	// reconcile-on-timeout rung honors the same stage-gated failure-context
	// requirement as the host gate. Zero value (StageOff) keeps every existing
	// Options{} literal byte-identical — only build/scout/triage set it (the
	// phases with a RequireFailureContextPhaseIO contract).
	PhaseIO config.Stage
	// CompactPrompts, when true, strips on-demand reference sections from
	// disk-loaded agent docs before ComposePrompt. Replaces the former
	// EVOLVE_COMPACT_PROMPTS env read. Inline bodies (minted/spec phases) are
	// never stripped regardless of this setting (R7).
	CompactPrompts bool
	// DisableStdoutFilter, when true, skips the post-phase .clean.txt writer.
	// Replaces the former EVOLVE_STDOUT_FILTER=off env check. Default false =
	// filter enabled, matching the historical "on" default.
	DisableStdoutFilter bool
	// UniversalFallback (workflow.universal_fallback, default true) enables the
	// last-resort dispatch tier: when a phase's whole configured CLI chain has no
	// binary on this host, DiscoverCLIsFn's installed+authed CLIs (family-filtered
	// by the profile allowlist) are appended so the loop routes to whatever LLM is
	// present instead of halting. No-op unless DiscoverCLIsFn is also wired.
	UniversalFallback bool
	// DiscoverCLIsFn is the memoized system-CLI discovery seam (composition root
	// closes it over bridge.Doctor: installed + authed + non-blocked driver names,
	// e.g. "agy-tmux"). nil ⇒ universal fallback is inert (byte-identical to the
	// pre-feature dispatch). Kept a seam so the runner package never imports bridge.
	DiscoverCLIsFn func() []string
	// Diag is the injectable diagnostics logger (T3, cycle-463): the MR4c
	// advisor-overlay observability lines route through it so a test can
	// capture them instead of the global log.Diag() stderr sink. Zero value
	// (both sinks nil) defaults to log.Diag() — production behavior is
	// unchanged.
	Diag log.Console
}

// BaseRunner is the Template Method implementation. Construct one per
// phase via New(); use it as a core.PhaseRunner.
type BaseRunner struct {
	hooks               Hooks
	bridge              core.Bridge
	prompts             *prompts.Loader
	nowFn               func() time.Time
	resolveLLM          func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	stdoutFilter        func(workspace, phase string) error
	eventsProducer      func(workspace, phase, cli string, cycle int, prompt string) error
	optional            bool
	verifyFn            func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	sleepFn             func(time.Duration)
	compactPrompts      bool
	disableStdoutFilter bool
	universalFallback   bool
	discoverCLIsFn      func() []string
	diag                log.Console
}

// New constructs a BaseRunner. Panics if Hooks is nil — that's a
// programmer error caught at startup, not a runtime condition.
func New(opts Options) *BaseRunner {
	if opts.Hooks == nil {
		panic("phases/runner: Hooks required")
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	resolveLLM := opts.ResolveLLM
	if resolveLLM == nil {
		resolveLLM = resolvellm.Resolve
	}
	stdoutFilter := opts.StdoutFilter
	if stdoutFilter == nil {
		stdoutFilter = logfilter.Process
	}
	eventsProducer := opts.EventsProducer
	if eventsProducer == nil {
		eventsProducer = func(workspace, phase, cli string, cycle int, prompt string) error {
			return phasestream.Produce(phasestream.ProduceConfig{
				Workspace: workspace, Phase: phase, CLI: cli, Cycle: cycle,
				InjectedPrompt: prompt,
			})
		}
	}
	verifyFn := opts.VerifyFn
	if verifyFn == nil {
		// Catalog-aware so the reconcile check resolves user/minted phases
		// under the SAME policy as the host gate and the agent self-check —
		// a builtin-only default left an inserted phase's surviving artifact
		// unresolvable on timeout, synthesizing FAIL. Stage-threaded (3.10
		// Slice 1) so the rung also reaches the host gate's verdict at enforce;
		// opts.PhaseIO's zero value (StageOff) is byte-identical to the prior
		// VerifyCatalogAware default.
		stage := opts.PhaseIO
		verifyFn = func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
			return deliverable.VerifyCatalogAwareStage(phase, roots, stage)
		}
	}
	sleepFn := opts.SleepFn
	if sleepFn == nil {
		sleepFn = settleSleep
	}
	diag := opts.Diag
	if diag.Out == nil && diag.Err == nil {
		diag = log.Diag()
	}
	// Universal-fallback defaults: per-instance Options win (test injection);
	// otherwise fall back to the composition-root package seams (set once in
	// cmd_cycle.go from workflow.universal_fallback + a memoized bridge.Doctor
	// closure — same set-once pattern as PhaseBoundaryCheckpointer, so the ~10
	// per-phase constructors need not each thread the discovery closure). Default
	// zero values ⇒ inert, byte-identical to the pre-feature dispatch.
	universalFallback := opts.UniversalFallback || DefaultUniversalFallback
	discoverCLIsFn := opts.DiscoverCLIsFn
	if discoverCLIsFn == nil {
		discoverCLIsFn = DefaultDiscoverCLIsFn
	}
	return &BaseRunner{
		hooks:               opts.Hooks,
		bridge:              opts.Bridge,
		prompts:             opts.Prompts,
		nowFn:               nowFn,
		resolveLLM:          resolveLLM,
		stdoutFilter:        stdoutFilter,
		eventsProducer:      eventsProducer,
		optional:            opts.Optional,
		verifyFn:            verifyFn,
		sleepFn:             sleepFn,
		compactPrompts:      opts.CompactPrompts,
		disableStdoutFilter: opts.DisableStdoutFilter,
		universalFallback:   universalFallback,
		discoverCLIsFn:      discoverCLIsFn,
		diag:                diag,
	}
}

// reconcileSettleRetries / reconcileSettleInterval bound the settle-WAIT for a
// contracted deliverable that has not finished flushing to disk yet. A clean-exit
// agent can return control before its `Write <phase>-report.md` lands, so the first
// verify probe can miss a report that is moments from valid. This wait is a pure
// LIVENESS ceiling, NOT the verdict-correctness mechanism: correctness comes from the
// file-authoritative rule in Run (the lossy terminal pane is never a verdict source
// for a contracted phase), so the window width can no longer flip a valid PASS into a
// scrollback FAIL — at worst an extreme over-run degrades to a COHERENT "deliverable
// not produced" FAIL, never a fabricated contradicting verdict. ~3s comfortably covers
// observed clean-exit flush latency under CPU/disk contention (the prior ~600ms did
// not — cycle-921, the ADR-0072 verdict-incoherence that motivated the file-authoritative
// rule).
const (
	reconcileSettleRetries  = 15
	reconcileSettleInterval = 200 * time.Millisecond
)

// settleSleep is the process clock for verifyReconcileDeliverable's inter-attempt
// wait — time.Sleep in production; a test init flips it to a no-op so the settle
// window costs zero wall-clock in the package's test suite (the retry now sits on
// the common clean-exit path, so real sleeps would balloon package test time).
var settleSleep = time.Sleep

// verifyReconcileDeliverable waits (bounded) for a contracted deliverable to become
// well-formed, re-probing verifyFn up to reconcileSettleRetries times with
// reconcileSettleInterval between attempts. It serves BOTH the reconcile-on-timeout
// path (cycles 824/825: a next-phase context-cancel laundered into ErrArtifactTimeout
// fires while the deliverable is still being written) AND the clean-exit artifact-read
// path (cycle-603/899/921: a cleanly-exited agent idles while its `Write` flush lands).
//
// It retries ONLY while the report is contracted-but-not-yet-well-formed
// (verr == nil && !res.OK) — the state a late flush passes through (a missing file is
// CodeMissingArtifact, verr==nil). An ERROR (verr != nil) means "no contract for this
// phase" or an IO fault; neither resolves by waiting, so those return immediately —
// uncontracted phases pay ZERO retries (no wasted settle window). Waiting can only
// UPGRADE toward the agent's real on-disk verdict; a never-settling deliverable still
// returns not-OK.
//
// CANCELLATION (verifyreconcile-ctx-cancel-unconditional-sleep): the wait observes
// ctx, so a cancelled phase stops waiting instead of sleeping out the ladder —
// reconcileSettleRetries intervals PLUS a verify probe per rung, each probe nesting
// deliverable's 500ms write-in-flight grace window. The first probe always runs (a
// deliverable already on disk is still caught); cancellation then stops both the
// sleeping and the re-probing, and the last result stands. The bounded retry count
// remains the ceiling while ctx is live, and the OK/not-OK decision and fail-closed
// fallback are untouched — only the waiting is dropped.
//
// SCOPE — this is the CLEAN-EXIT path's guard, and the reconcile-on-teardown call
// site deliberately passes context.WithoutCancel (see there): on that path a
// cancelled ctx is frequently the CAUSE of the teardown (the tmux driver launders a
// ctx-cancel into ExitArtifactTimeout after one final poll —
// driver_tmux_repl.go's "the runner's settle-retry was the only thing standing
// between that mislabel and a false FAIL"), so honoring cancellation there would
// re-open the cycles-824/825 class. On the clean-exit path the agent already exited 0,
// so cancellation genuinely means nothing more is coming.
//
// REACHABILITY — the honored-cancel case is OPERATOR-INTERRUPT only. cmd_cycle.go
// hands RunCycle a context.Background(), so a fleet lane never observes cancel and
// this branch is inert in the standing mode; the one cancellable root is cmd_loop.go's
// signal.NotifyContext. The residual it accepts: a SIGINT landing between the driver's
// clean return and this call, on a phase whose deliverable is still flushing, bails
// after one probe and FAILs where the full settle window would have PASSed. Bounded by
// deliverable's 500ms grace nested in that first probe, and the loop is terminating
// anyway — a wrong verdict on a cycle nobody will ship.
//
// The ctx check straddles the sleep (before and after) rather than racing a timer
// against it: sleepFn is an injected, uninterruptible seam, so selecting on ctx.Done()
// would mean calling the seam from a spawned goroutine and racing tests that count its
// invocations. Post-cancel cost is therefore bounded at ONE interval with no further
// probe — the amplification this fix targets.
func (b *BaseRunner) verifyReconcileDeliverable(ctx context.Context, phase string, roots phasecontract.Roots) (deliverable.Result, error) {
	res, verr := b.verifyFn(phase, roots)
	for attempt := 0; attempt < reconcileSettleRetries && verr == nil && !res.OK; attempt++ {
		if ctx.Err() != nil {
			return res, verr
		}
		b.sleepFn(reconcileSettleInterval)
		if ctx.Err() != nil {
			return res, verr
		}
		res, verr = b.verifyFn(phase, roots)
	}
	return res, verr
}

// forensicSnapshot / forensicCodes render a file's + a violation set's state as a single
// log-safe token for the teardown-reconcile decision log (the retro's cycle-3 ask: today a
// teardown false-FAIL records no reasoning, so a recurrence is a 30-minute forensic dig).
// forensicSnapshot reports a file's existence, byte size, and last tailN bytes (where the
// audit-report.md verdict sentinel lives).
func forensicSnapshot(path string, tailN int) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "absent"
	}
	data, _ := os.ReadFile(path)
	tail := string(data)
	if len(tail) > tailN {
		tail = tail[len(tail)-tailN:]
	}
	return fmt.Sprintf("size=%d tail=%q", fi.Size(), tail)
}

func forensicCodes(vs []deliverable.Violation) string {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		parts = append(parts, string(v.Code))
	}
	return strings.Join(parts, ",")
}

// acsFloorRescues reports whether a teardown-time deliverable.Verify not-OK should
// be OVERRIDDEN by the deterministic ACS ground truth (verdict-incoherence family:
// cycles 603/921/924/931/3). report is the VERIFIED deliverable content (the bytes
// Verify read — Result.Content), never a fresh read: the rescue decision and the
// subsequent Classify must judge the SAME snapshot, or a rescue could fire on bytes
// the classify step never sees. True iff ALL hold:
//   - the phase is audit (the acs-verdict.json + coherence floor are audit-scoped);
//   - the acssuite verdict is PASS — a NON-LLM signal a session stall cannot corrupt;
//   - the report declares a PASS-class verdict sentinel (via the canonical
//     ParseVerdictSentinel, with its placeholder-echo guard — read by ReadCycleVerdicts);
//   - the report echoes THIS cycle's minted challenge token (anti-gaming: a stale,
//     forged, or cross-cycle report cannot be laundered to PASS by the ACS verdict alone).
//
// This is precisely the (audit==PASS && acs==PASS) condition the ADR-0072 coherence
// floor flags as incoherent — reusing coherence.ReadCycleVerdicts keeps a single
// definition of "both verdicts agree on PASS", so the teardown floor rescues exactly
// what the post-hoc floor would otherwise HALT on. It never manufactures a PASS: a
// malformed/verdict-less/token-missing report, or a non-ship-eligible suite, declines.
func acsFloorRescues(phase, workspace, report string) bool {
	if phase != string(core.PhaseAudit) {
		return false
	}
	audit, acs, auditRan := coherence.ReadCycleVerdicts(workspace)
	if !auditRan || audit != "PASS" || acs != "PASS" {
		return false
	}
	tokRaw, err := os.ReadFile(filepath.Join(workspace, "challenge-token.txt"))
	if err != nil {
		return false
	}
	tok := strings.TrimSpace(string(tokRaw))
	if tok == "" {
		return false
	}
	return strings.Contains(report, tok)
}

// classifiedArtifact returns the content Classify must judge for a CONTRACTED
// phase, given the deliverable's Verify result, the artifact path THIS run
// dispatched, and the terminal pane as the last resort.
//
// SINGLE READ (deliverable-verified-bytes-single-read): when the Verify result
// describes the same file the bridge was told to write, its Content IS the
// classified content — verdict and content come from ONE read, so a writer racing
// the just-finished launch cannot slip bytes past the gate that judged them.
//
// The path-skew half of the fallback is now STRUCTURALLY CLOSED at THIS seam
// (intent-delta-contract-path-skew): the runner threads the dispatched path
// through Roots.DispatchedArtifact, so Verify always judges the file this run
// dispatched — intent in DELTA mode included — and res.ArtifactPath ==
// artifactPath for every contracted phase driven by the REAL verify. The
// no-override seams (host contract gate's rootsFor, `evolve phase verify`)
// still judge the contract path — correct for their callers, but in delta
// mode the gate seam remains open (queued: gate-seam dispatched-path
// threading). The fallback remains for what it still covers: an
// errored/path-less Result, a NoArtifact contract (ArtifactPath "") were one
// ever routed through BaseRunner (ship's is not), and test fakes that verify
// other paths.
//
// The fallback reads the dispatched artifact exactly as the pre-single-read code
// did, including the "absent + !OK ⇒ empty artifact" rule that makes an unwritten
// deliverable a coherent FAIL instead of a pane-scraped one. NOTE that rule now also
// applies on the reconciled call site, which previously could only keep the pane;
// unreachable in practice (it needs a path skew AND a read failure AND !res.OK, and
// the pane is empty on any teardown anyway), but it is a real unification of two
// call sites that had slightly different rules.
// The snapshot is authoritative only when it is BOTH of the same file AND
// non-empty. Empty is not evidence of absence: an infra read fault returns an
// empty Result, and a deliverable that materialises after the ladder's last
// probe verifies absent — in either case the snapshot would classify "" for a
// file that is on disk right now. Falling through costs one syscall on a path
// that was already headed for FAIL, and it rescues the case named above (an
// intent-delta phase deriving [intent-unchanged] → SKIPPED from a late file).
// This is also what makes the two call sites symmetric: the ACS-rescue branch
// does its own late-bytes read for the rescue's own evidence, and without this
// clause the clean-exit site would be the only one without that liveness.
func classifiedArtifact(res deliverable.Result, artifactPath, pane string) string {
	if res.ArtifactPath == artifactPath && res.Content != "" {
		return res.Content
	}
	if data, err := os.ReadFile(artifactPath); err == nil {
		return string(data)
	}
	if !res.OK {
		return "" // contracted file genuinely absent → Classify sees no sentinel → FAIL
	}
	return pane
}

// Name implements core.PhaseRunner.
func (b *BaseRunner) Name() string { return b.hooks.PhaseName() }

// Run implements core.PhaseRunner. The template:
//
//  1. validate deps (bridge, prompts)
//  2. load agent prompt body
//  3. compose final prompt via hook
//  4. resolve cli / model / extraFlags from env-chain + profile
//  5. dispatch bridge.Launch
//  6. read artifact (stdout, then file fallback)
//  7. classify via hook
//  8. package PhaseResponse
//
// Bridge errors and missing-prompts errors short-circuit to a FAIL
// response with the error attached as a diagnostic.
// artifactSnapshot is the (size, mtime) identity of the canonical artifact at
// one instant — the same key the bridge's stability window and baseline use,
// so the runner's and the bridge's notion of "unchanged" cannot drift.
type artifactSnapshot struct {
	size    int64
	modTime time.Time
}

// statArtifactSnapshot snapshots path if it is a non-empty regular file.
func statArtifactSnapshot(path string) (artifactSnapshot, bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		return artifactSnapshot{}, false
	}
	return artifactSnapshot{size: fi.Size(), modTime: fi.ModTime()}, true
}

// artifactUnchangedSince reports whether path is still byte-identical (by the
// size+mtime key) to the given pre-dispatch snapshot. Any error reads as
// changed — fail-open toward the pre-existing reconcile behavior.
func artifactUnchangedSince(path string, snap artifactSnapshot) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	return fi.Size() == snap.size && fi.ModTime().Equal(snap.modTime)
}

func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	req.WorktreeVerified = false
	prep, early, err := b.preparePhaseExecution(req)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	if early != nil {
		return *early, nil
	}

	dispatchPlan, early, err := b.resolveDispatchPlan(req, prep)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	dispatchResult := b.dispatchPhaseAttempts(ctx, req, prep, dispatchPlan)
	req.WorktreeVerified = dispatchResult.worktreeVerified

	reconciliation, early, err := b.reconcileDeliverable(ctx, req, prep, dispatchResult)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	if early != nil {
		return *early, nil
	}

	return b.classifyPhaseOutcome(ctx, req, prep, dispatchPlan, dispatchResult, reconciliation)
}

func (b *BaseRunner) withExplanationContract(body, phase string) (string, error) {
	var agentName, heading string
	switch phase {
	case string(core.PhaseBuild):
		agentName = "evolve-builder-reference"
		heading = "## Section: explanation-documentation-contract"
	case string(core.PhaseAudit):
		agentName = "evolve-auditor-reference"
		heading = "## Section: explanation-documentation-review"
	default:
		return body, nil
	}
	if strings.Contains(body, heading) {
		return body, nil
	}
	agent, err := b.prompts.Agent(agentName)
	if err != nil {
		return "", err
	}
	start := strings.Index(agent.Body, heading)
	if start < 0 {
		return "", fmt.Errorf("%s is missing %q", agentName, heading)
	}
	section := agent.Body[start:]
	if next := strings.Index(section[len(heading):], "\n## Section:"); next >= 0 {
		section = section[:len(heading)+next]
	}
	return strings.TrimRight(body, "\n") + "\n\n" + strings.TrimSpace(section) + "\n", nil
}

func requiresExplanationSandbox(phase string, req core.PhaseRequest) bool {
	return phase == string(core.PhaseBuild) && req.ExplanationDocumentationVersion != 0
}

// cycleContextBoundary is the single canonical marker that separates a phase
// prompt's cache-stable static prefix (persona/rules/agent-doc body) from its
// per-cycle dynamic tail. BaseCycleContext writes it and StaticPrefix splits on
// it — one literal, so the two can never drift apart and silently bust the
// provider prompt-cache.
const cycleContextBoundary = "\n\n## Cycle Context\n"

// BaseCycleContext returns the canonical "## Cycle Context" block shared by
// every phase that uses BaseRunner. It writes body, then the four mandatory
// fields (cycle, goal_hash, project_root, workspace). Phase-specific extras
// (worktree, goal, mode, carryover_summary, etc.) are the caller's responsibility
// — they append them after this call so the base block stays the single source.
func BaseCycleContext(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString(cycleContextBoundary)
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if recalled := req.Context[core.CtxKeyRecallMemory]; recalled != "" {
		fmt.Fprintf(&b, "- recalled_lessons_untrusted_json: %q\n", recalled)
	}
	AppendExplanationContext(&b, req)
	return b.String()
}

// AppendExplanationContext writes the complete verified Build-explanation
// handoff as a single-line JSON data field plus stable convenience fields.
// Audit and Retro share this serializer so their prompts cannot drift from the
// evidence their deterministic report validators require.
func AppendExplanationContext(b *strings.Builder, req core.PhaseRequest) {
	if req.ExplanationDocumentationVersion != 0 {
		fmt.Fprintf(b, "- explanation_documentation_version: %d\n", req.ExplanationDocumentationVersion)
	}
	if req.BuildExplanationState != "" {
		fmt.Fprintf(b, "- explanation_handoff_state: %s\n", req.BuildExplanationState)
	}
	if req.BuildExplanationError != "" {
		fmt.Fprintf(b, "- explanation_error_untrusted_json: %q\n", req.BuildExplanationError)
	}
	if explanation := req.BuildExplanation; explanation != nil {
		fmt.Fprintf(b, "- explanation_status: %s\n", explanation.Status)
		fmt.Fprintf(b, "- explanation_contract_version: %d\n", explanation.ContractVersion)
		fmt.Fprintf(b, "- explanation_document: %s (sha256:%s)\n", explanation.DocumentPath, explanation.DocumentSHA256)
		if encoded, err := json.Marshal(explanation); err == nil {
			fmt.Fprintf(b, "- explanation_handoff_untrusted_json: %s\n", encoded)
		}
	}
}

// StaticPrefix returns the cache-stable prefix of a composed phase prompt:
// everything before the canonical "## Cycle Context" boundary that
// BaseCycleContext emits. Provider prompt-caches key on this byte-identical
// prefix, so isolating it lets callers verify (and pin, via the cache-stable
// audit) that no per-cycle dynamic value — cycle number, goal_hash, workspace —
// drifts above the boundary. When the boundary is absent the whole prompt is
// the prefix.
func StaticPrefix(prompt string) string {
	if i := strings.Index(prompt, cycleContextBoundary); i >= 0 {
		return prompt[:i]
	}
	return prompt
}

// FormatDigestShadowLog renders the role projection's dispatch-time evidence.
func FormatDigestShadowLog(phase string, record digest.ShadowRecord) string {
	return fmt.Sprintf("[runner] phase=%s digest-shadow outcome=%s full_bytes=%d digest_bytes=%d parity=%t",
		phase, record.Outcome, record.FullBytes, record.DigestBytes, record.Parity)
}

// ComposePrompt exposes the phase's prompt assembly (Hooks.ComposePrompt) as a
// public seam on BaseRunner so a caller can compose a prompt without launching
// the bridge — used by the cache-stable-prefix audit to inspect the static
// prefix for every BaseRunner-based phase. Run uses the same hook internally
// (see below); this method adds reach, not behavior.
func (b *BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string {
	return b.hooks.ComposePrompt(body, req)
}

// PersonaAvailable implements core.PersonaProber: it resolves the persona doc
// exactly as Run does (same loader, same name, same inline-prompt exemption)
// without dispatching anything, so the planner can exclude a phase whose doc
// does not exist instead of discovering it one dispatch, one skip and one
// retrospective later.
func (b *BaseRunner) PersonaAvailable() error {
	if ip, ok := b.hooks.(InlinePromptProvider); ok {
		if _, inline := ip.InlinePromptBody(); inline {
			return nil
		}
	}
	if b.prompts == nil {
		return fmt.Errorf("%s: prompts loader required", b.hooks.PhaseName())
	}
	if _, err := b.prompts.Agent(b.hooks.AgentPromptName()); err != nil {
		if errors.Is(err, fs.ErrNotExist) && !errors.Is(err, prompts.ErrNoSource) {
			return fmt.Errorf("%s: load agent: %w: %w", b.hooks.PhaseName(), core.ErrAgentDocMissing, err)
		}
		return fmt.Errorf("%s: load agent: %w", b.hooks.PhaseName(), err)
	}
	return nil
}
