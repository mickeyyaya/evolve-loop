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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
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
// The fallback covers a phase whose dispatched artifact filename differs from its
// contract's, where Verify judged a DIFFERENT file and so holds no bytes for this
// one (nor any verified-vs-classified claim to make). The known case is intent in
// DELTA mode — it dispatches intent-delta.md while the intent contract names
// intent.md — reachable only with EVOLVE_INTENT_DELTA set, which no in-repo caller
// does, so this is an operator-mode path rather than the default one. It is a
// PRE-EXISTING skew this fix deliberately does not paper over (the contract gate
// verifies a file that phase was never asked to write). Every other BaseRunner phase
// derives its filename from the same registry the contract does, so the paths agree
// and the fast path applies. Also lands here: an errored/path-less Result, and a
// NoArtifact contract (ArtifactPath "") were one ever routed through BaseRunner —
// ship's is not.
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
func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := b.nowFn()
	phase := b.hooks.PhaseName()

	if b.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("%s: bridge required", phase)
	}
	if b.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("%s: prompts loader required", phase)
	}

	// Optional pre-bridge skip predicate. ISP: only phases that opt
	// into Skipper get consulted; the rest skip this branch entirely.
	if skipper, ok := b.hooks.(Skipper); ok {
		if skipped, verdict, nextPhase, diags := skipper.ShouldSkip(req); skipped {
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      verdict,
				ArtifactsDir: req.Workspace,
				NextPhase:    nextPhase,
				DurationMS:   b.nowFn().Sub(start).Milliseconds(),
				Diagnostics:  diags,
			}, nil
		}
	}

	// Inline body wins over disk-load, keyed on the provider's ok flag (not
	// body emptiness). Only phases that opt into InlinePromptProvider are
	// consulted; see its godoc for the ISP rationale.
	body, inline := "", false
	if ip, ok := b.hooks.(InlinePromptProvider); ok {
		body, inline = ip.InlinePromptBody()
	}
	if !inline {
		agent, err := b.prompts.Agent(b.hooks.AgentPromptName())
		if err != nil {
			return core.PhaseResponse{}, fmt.Errorf("%s: load agent: %w", phase, err)
		}
		body = agent.Body
		if b.compactPrompts {
			body = prompts.StripOnDemandSections(body)
		}
	}

	prompt := b.hooks.ComposePrompt(body, req)
	artifactPath := filepath.Join(req.Workspace, b.hooks.ArtifactFilename(req))
	profileDir := filepath.Join(req.ProjectRoot, ".evolve", "profiles")
	// Profile JSON files use the AGENT name (e.g., tdd-engineer.json,
	// builder.json, auditor.json, retrospective.json) — NOT the phase
	// name (tdd, build, audit, retro). The convention is "strip the
	// 'evolve-' prefix from AgentPromptName". Source: cycle 106
	// (2026-05-25) integration smoke where phase=tdd looked for
	// `.evolve/profiles/tdd.json` (which doesn't exist) instead of
	// `.evolve/profiles/tdd-engineer.json` (which does). Also matches
	// CLAUDE.md's `EVOLVE_<AGENT>_<KEY>` env-var convention so the
	// phaseflags env-key generation aligns with the documented
	// `EVOLVE_TDD_ENGINEER_PERMISSION_MODE` (not `EVOLVE_TDD_*`).
	profileName := strings.TrimPrefix(b.hooks.AgentPromptName(), "evolve-")
	profilePath := filepath.Join(profileDir, profileName+".json")

	// CLI resolution chain: EVOLVE_CLI env var > profile.cli > default
	// "claude-p". Before this fix the runner only consulted EVOLVE_CLI
	// and defaulted to claude-p regardless of profile.cli, which meant
	// operators editing .evolve/profiles/<agent>.json:cli to switch a
	// phase to codex or agy had no effect — the runner silently
	// dispatched to claude-p anyway.
	// Source: cycle 107 (2026-05-25) attempted-codex smoke that
	// produced claude-sonnet-4-6 output despite cli=codex in every
	// profile. Operator misread "delegation" because the resolved CLI
	// wasn't logged.
	var prof *profiles.Profile
	if loader := profiles.NewFromDir(profileDir); loader != nil {
		if p, err := loader.Get(profileName); err == nil {
			prof = &p
		} else if st, statErr := os.Stat(profileDir); statErr == nil && st.IsDir() {
			msg := fmt.Sprintf("profile not found: %s", profilePath)
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      core.VerdictFAIL,
				ArtifactsDir: req.Workspace,
				Diagnostics:  []core.Diagnostic{{Severity: "error", Message: msg}},
			}, fmt.Errorf("%s: %s: %w", phase, msg, err)
		}
	}

	// Advisory turn-budget hint: when a profile declares turn_budget_hint, append
	// a non-binding budget note so the agent self-limits (prioritize breadth,
	// finalize once completion gates are met). Purely advisory — the hard stops
	// remain max_turns + the artifact timeout. Activates the otherwise-dormant
	// profiles.Profile.TurnBudgetHint field (declared in ~8 profiles but never
	// consumed before this).
	if prof != nil && prof.TurnBudgetHint > 0 {
		prompt += fmt.Sprintf("\n\n## Budget\nAdvisory turn budget for this phase: ~%d turns. Prioritize breadth over depth; write your report as soon as the completion gates are satisfied.\n", prof.TurnBudgetHint)
	}

	// Challenge-token injection (cycle-269): the bash→Go migration dropped
	// the prompt-side half of the proof-of-read protocol — builders were
	// never TOLD to echo the minted token, so compliance depended on an
	// agent spontaneously reading scout-report line 2 (the claude fallback
	// didn't; a perfect build FAILed at audit). Contract-driven (the same
	// SSOT the deliverable gate checks), deterministic, and absent-token ⇒
	// byte-identical prompt.
	if c, ok := phasecontract.For(phase); ok && c.RequireChallengeToken {
		if tok, terr := os.ReadFile(filepath.Join(req.Workspace, "challenge-token.txt")); terr == nil {
			if t := strings.TrimSpace(string(tok)); t != "" {
				prompt += fmt.Sprintf("\n\n## Challenge Token (proof-of-read — MANDATORY)\nCopy this token verbatim into your report as an HTML comment near the top: <!-- challenge-token: %s -->\nA report without it is rejected and re-dispatched.\n", t)
			}
		}
	}

	// User-controlled policy pin (absolute): a pinned CLI/model for this phase
	// overrides env/profile/default resolution. Keyed by phase name. Validated
	// against the profile guardrails (allowed_clis + model_tier_envelope) — an
	// out-of-bounds pin hard-fails the phase loudly rather than silently
	// breaching the trust-kernel constraints. Escape hatch: --bypass-policy flag.
	var pin *policy.Pin
	// overlayPolicy carries the skill-overlay rules down to the dispatch closure
	// (which skill each phase-agent dispatch preloads — config, not code). Zero
	// value ⇒ compiled-default overlays (ResolveOverlays); under --bypass-policy
	// the compiled default still applies so the operating discipline is not
	// dropped by a pin-bypass.
	var overlayPolicy policy.Policy
	if !req.BypassPolicy {
		pol, perr := policy.Load(filepath.Join(req.ProjectRoot, ".evolve", "policy.json"))
		if perr != nil {
			// Malformed policy must fail loudly, not silently ignore user rules.
			return core.PhaseResponse{
				Phase: phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
				Diagnostics: []core.Diagnostic{{Severity: "error", Message: perr.Error()}},
			}, fmt.Errorf("%s: %w", phase, perr)
		}
		overlayPolicy = pol
		if p, ok := pol.PinFor(phase); ok {
			if verr := policy.ValidatePin(phase, p, prof); verr != nil {
				return core.PhaseResponse{
					Phase: phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
					Diagnostics: []core.Diagnostic{{Severity: "error", Message: verr.Error()}},
				}, fmt.Errorf("%s: %w", phase, verr)
			}
			pin = &p
			log.Diag().Infof("[runner] phase=%s policy pin: cli=%q model=%q\n", phase, p.CLI, p.Model)
		}
	}

	// Single dispatch resolver (llmroute): one Plan carries the CLI fallback
	// chain AND the resolved model, so there is exactly one place that decides
	// "which CLI + model runs this phase". Precedence (preserved verbatim):
	//   CLI:   EVOLVE_<AGENT>_CLI > EVOLVE_CLI > profile.cli > "claude-tmux",
	//          then + profile.cli_fallback (deduped); triggers default
	//          {80,81,124,127}. A single-element chain is byte-identical to
	//          pre-fallback behavior.
	//   model: EVOLVE_<AGENT>_MODEL > profile.model_tier_default >
	//          Hooks.DefaultModel(), then "auto" → autoExpand.
	// The model env key is AGENT-keyed (EVOLVE_<PROFILE_NAME>_MODEL), matching
	// cmd_loop's `--model <agent>=X` writer + the PERMISSION_MODE resolver below.
	//
	// autoExpand bridges the resolvellm seam so "auto" expansion stays
	// byte-identical (keyed by `phase`, NOT profileName — preserved from the
	// pre-Step-9 behavior when the now-removed llm_config layer was phase-keyed).
	// claude -p rejects a literal "auto" (HTTP 404), so this MUST resolve before
	// dispatch. The CLI the seam computes is intentionally NOT used for dispatch —
	// the chain above is authoritative.
	autoExpand := func(role string) (string, bool) {
		res, err := b.resolveLLM(role, resolvellm.Options{})
		if err != nil {
			return "", false
		}
		if res.ModelTier != "" {
			return res.ModelTier, true
		}
		return "", false
	}
	plan := llmroute.Resolve(profileName, phase, b.hooks.DefaultModel(), req.Env, prof, autoExpand, pin)
	// Soft dispatch overlay (cycle-440 MR4c): an advisor-proposed {cli,tier}
	// threaded via PhaseRequest.ModelRoutingCLI/Tier (already clamped upstream
	// by router.ClampPlanModelRouting under model_routing=auto) promotes to
	// chain PRIMARY without discarding the profile's fallback chain — unlike an
	// absolute policy.Pin, a benched overlay CLI still falls back via the
	// capability-probe + cli-health bench passes below. A policy pin always
	// wins (soft overlay never applies alongside one); zero overlay fields are
	// a byte-identical noop.
	overlayProposed := req.ModelRoutingCLI != "" || req.ModelRoutingTier != ""
	modelSource := "profile"
	switch {
	case pin != nil:
		modelSource = "pin"
	case overlayProposed:
		modelSource = "advisor"
	}
	if pin == nil && overlayProposed {
		plan = llmroute.ApplySoftOverlay(plan, llmroute.Overlay{CLI: req.ModelRoutingCLI, Tier: req.ModelRoutingTier}, prof)
		b.diag.Infof("[runner] phase=%s advisor overlay cli=%s tier=%s\n", phase, req.ModelRoutingCLI, req.ModelRoutingTier)
	} else if pin == nil {
		b.diag.Infof("[runner] phase=%s no advisor overlay (profile default)\n", phase)
	}
	// Capability probe: demote (don't delete) candidates whose binary isn't on
	// PATH so a missing CLI doesn't burn a 60s boot timeout before the chain
	// advances. Log the reorder inline with the dispatch log.
	//
	// SKIPPED when a CLI is policy-pinned: the probe reorders by binary
	// availability, which would silently demote a pinned-but-missing CLI out of
	// the primary slot — violating the "policy pin is absolute" contract. A
	// pinned CLI is attempted as-is; if its binary is absent the dispatch
	// surfaces a real ExitMissingBinary (127), which the profile fallback chain
	// can still recover from via the normal trigger path.
	if pin == nil || pin.CLI == "" {
		preCandidates := plan.Candidates
		plan = llmroute.Probe(plan, nil)
		if !sameCandidates(preCandidates, plan.Candidates) {
			log.Diag().Infof("[runner] phase=%s capability probe reordered chain: %v -> %v\n",
				phase, preCandidates, plan.Candidates)
		}
	}
	// CLI-health bench: demote families with an ACTIVE bench (classified
	// transient wall, e.g. rate_limit) so the chain starts at a healthy CLI
	// instead of re-burning the walled primary's boot window (cycle-283).
	// Same pin bypass as the capability probe; lazy expiry inside.
	plan = b.applyBenchToPlan(req.ProjectRoot, phase, plan, pin != nil && pin.CLI != "", req.Env)
	// Universal fallback (last resort): if the whole configured chain's binaries
	// are absent on this host (e.g. an isolated agy-only box whose profiles still
	// name claude/codex), append the DISCOVERED installed+authed CLIs the phase
	// allowlist permits, so the loop routes to a present LLM instead of halting.
	// Pin-bypassed (a policy pin is absolute) and gated on workflow.universal_fallback.
	if (pin == nil || pin.CLI == "") && b.universalFallback && b.discoverCLIsFn != nil {
		discovered := allowedDiscovered(b.discoverCLIsFn(), prof)
		pre := plan.Candidates
		plan = llmroute.ApplyUniversalFallback(plan, discovered, nil)
		if !sameCandidates(pre, plan.Candidates) {
			log.Diag().Infof("[runner] phase=%s UNIVERSAL-FALLBACK: configured chain %v all absent on this host — discovered+allowed CLIs appended -> %v\n",
				phase, pre, plan.Candidates)
		}
	}
	cli := plan.Candidates[0]
	// Disambiguating dispatch log: tells observers which CLI is actually being
	// invoked and why (an output stream saying `model: claude-sonnet-4-6` could
	// otherwise be misread as "codex delegating to claude").
	if len(plan.Candidates) > 1 {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s fallback=%v triggers=%v\n",
			phase, profileName, cli, plan.PrimarySource, profilePath, plan.Candidates[1:], plan.Triggers)
	} else {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s\n",
			phase, profileName, cli, plan.PrimarySource, profilePath)
	}
	model := plan.Model
	// Per-phase permission mode comes from the request snapshot first, then
	// the typed agent profile. The process environment is intentionally not
	// consulted: profiles are the persistent SSOT and req.Env is the explicit
	// per-dispatch override surface. Passed as typed config — the bridge
	// realizes it per-CLI via the LaunchIntent (no raw flag leak).
	permissionMode := req.Env[envchain.PhaseEnvKey(profileName, "PERMISSION_MODE")]
	if permissionMode == "" && prof != nil {
		permissionMode = prof.PermissionMode
	}
	// Interactive policy follows the profile-SSOT model: the bridge resolves
	// the explicit policy.json override surface, and the runner passes the
	// typed profile default here.
	var interactivePolicy string
	if prof != nil {
		interactivePolicy = prof.InteractivePolicy
	}
	// Facet B: resolve the per-agent launch-time system prompt / rules
	// (profileName keys both the profile lookup and the EVOLVE_<AGENT>_* env).
	sysPrompt := systemprompt.Resolve(profileName, profileDir, req.Env)

	// WS-G1: dispatch through the chain via llmroute.Dispatch — the SAME
	// chain-walk implementation the advisor uses (cycle-435,
	// [[never_duplicate_centralize_via_design_patterns]]), rather than a
	// hand-rolled copy of it. Each attempt: build BridgeRequest for the
	// candidate CLI, Launch, normalize events. On a trigger exit (default
	// {80, 81, 124, 127} per cli_chain.go:defaultFallbackOnExit —
	// REPL-boot-timeout / artifact-timeout / coreutils-timeout /
	// missing-binary) Dispatch advances to the next candidate. Any other exit
	// (or success) stops the walk — a legitimate FAIL verdict from a model
	// never silently routes to a different CLI. Final attempt's (bres,
	// bridgeErr) is what the rest of the function consumes; events file
	// reflects the final CLI's stdout so cycleclassify sees what actually
	// happened last.
	var bres core.BridgeResponse
	var bridgeErr error
	var attemptLog []string
	// WS-876: dispatch through the TIER fallback chain. DispatchTiered walks
	// plan.Tiers outer × plan.Candidates inner: within a tier it behaves exactly
	// like Dispatch (trigger exit advances the CLI, a real FAIL stops), and it
	// steps DOWN to the next tier ONLY when every CLI at the current tier exited
	// 85 (quota) — the fable/opus→sonnet step-down the operator needs so a
	// fully-quota-walled top tier fails over to a lower-cost live tier instead of
	// aborting the phase. The tier string flows straight into BridgeRequest.Model:
	// the bridge realizer maps it per-CLI via the manifest's model_tier_map
	// (opus→opus/gpt-5.5, balanced→sonnet/gpt-5.4, …), so no runner-side model
	// resolution is needed — passing the tier is as literal as passing plan.Model.
	tieredRes := llmroute.DispatchTiered(plan, func(candidateCLI, tier string) (int, error) {
		i := len(attemptLog)
		if i > 0 {
			// Read the previous attempt from attemptLog, NOT plan.Candidates[i-1]:
			// under tiering i grows past len(Candidates) (candidates × tiers), so
			// indexing Candidates would panic on the first real step-down.
			log.Diag().Infof(
				"[runner] phase=%s fallback %d: trying cli=%s tier=%s (previous=%s exit=%d)\n",
				phase, i+1, candidateCLI, tier, attemptLog[i-1], bres.ExitCode)
		}
		// Skill overlays are resolved PER ATTEMPT: the fallback tier steps down
		// across attempts, and overlay rules key on tier (e.g. deep/top→fable), so
		// the configured skill set is recomputed for each (cli, tier) actually
		// dispatched. Pure policy lookup; the adapter materializes the SKILL.md.
		overlaySkills := overlayPolicy.ResolveOverlays(policy.DispatchFromPhaseRequest(phase, candidateCLI, tier, tier))
		// Observability: announce the resolved overlay set for THIS (cli, tier)
		// attempt so operators/graders see the persona fired without diffing the
		// prompt file. Rendered even for the empty set (skill-overlays=[]).
		log.Diag().Infof("%s\n", FormatSkillOverlayLog(phase, overlaySkills, tier))
		bres, bridgeErr = b.bridge.Launch(ctx, core.BridgeRequest{
			CLI:                 candidateCLI,
			Profile:             profilePath,
			Model:               tier,
			Prompt:              prompt,
			Workspace:           req.Workspace,
			Worktree:            req.Worktree,
			RunID:               req.RunID,
			ProjectRoot:         req.ProjectRoot,
			ArtifactPath:        artifactPath,
			SecondaryArtifacts:  secondaryArtifacts(b.hooks, req),
			Agent:               phase,
			Cycle:               req.Cycle,
			BudgetScale:         req.BudgetScale,
			Env:                 req.Env,
			PermissionMode:      permissionMode,
			InteractivePolicy:   interactivePolicy,
			SystemPrompt:        sysPrompt,
			Skills:              overlaySkills,
			CorrectionDirective: req.CorrectionDirective,
			OperatorDirectives:  req.OperatorDirectives,
		})
		// Normalize per attempt so the final events file reflects the
		// final CLI's stdout — cycleclassify reads <phase>-events.ndjson
		// and we want it to describe what actually happened last.
		if err := b.eventsProducer(req.Workspace, phase, candidateCLI, req.Cycle, prompt); err != nil {
			log.Diag().Warnf("[runner] WARN events producer phase=%s cli=%s: %v (cost/classification degraded)\n", phase, candidateCLI, err)
		}
		attemptLog = append(attemptLog, fmt.Sprintf("%s@%s=%d", candidateCLI, tier, bres.ExitCode))
		// CLI-health bench: an exit-85 with a fresh benchable escalation
		// report (rate_limit class) is remembered ACROSS dispatches — run on
		// every candidate including the last, so the wall is recorded even
		// when no fallback remains (cycle-283). Staleness is judged against
		// the RUN start: the guard exists to exclude cross-PHASE leftovers in
		// the shared workspace, not earlier attempts of this same run.
		if bridgeErr != nil && bres.ExitCode == 85 {
			b.maybeBenchOnEscalation(req.ProjectRoot, req.Workspace, candidateCLI, start, req.Env)
		}
		return bres.ExitCode, bridgeErr
	}, func(from, to string) {
		log.Diag().Infof("[runner] phase=%s tier step-down: %s → %s (CLI chain exhausted at quota)\n", phase, from, to)
	})
	// ResolvedModel reports the tier the terminal attempt actually ran at (a
	// step-down means the phase ran below its resolved tier); fall back to the
	// resolved model for the empty-candidates edge case DispatchTiered guards.
	resolvedModel := tieredRes.Tier
	if resolvedModel == "" {
		resolvedModel = model
	}
	if len(attemptLog) > 1 {
		log.Diag().Infof("[runner] phase=%s dispatch chain: %s\n", phase, joinAttempts(attemptLog))
	}
	durationMS := b.nowFn().Sub(start).Milliseconds()

	// reconciled is set when a bridge INFRA teardown (timeout OR transient) is
	// overridden by a well-formed deliverable on disk: control then FALLS THROUGH
	// to the same artifact-read + Classify path the happy case uses (so audit's
	// EGPS gate still applies — reconciliation can never ship a green-looking
	// report whose predicates are red). See the reconcile block below.
	reconciled := false
	acsFloorRescued := false
	acsFloorOverriddenCodes := "" // teardown Verify codes the ACS floor overrode (surfaced on the response)
	// reconciledRes is the reconcile probe's Verify result, carried out of this
	// block so the classify step below consumes the bytes that Verify READ
	// (Result.Content) instead of re-reading the path — see the verdict-source
	// comment on the single-read invariant.
	var reconciledRes deliverable.Result
	if bridgeErr != nil {
		// A bridge INFRA teardown — an artifact-wait timeout (exit 81) OR a
		// transient failure (exit 80/85/86: quota, liveness-exhaustion) — ends the
		// SESSION, but is not a verdict: the agent may have written its contracted
		// deliverable before the teardown (cycle-254/255 timeout false-FAIL;
		// cycle-835 quota false-FAIL — a complete PASS audit discarded because the
		// deep-tier auditor hit exit=85 at the tail while agy+codex were walled).
		// Reconcile against the deliverable: if it is on disk and well-formed, trust
		// its verdict (via Classify) instead of synthesizing FAIL. Reconciliation
		// can only UPGRADE toward the agent's real verdict, never downgrade a real one.
		if core.IsInfraTeardownError(bridgeErr) {
			roots := phasecontract.Roots{Workspace: req.Workspace, Worktree: req.Worktree}
			if req.ProjectRoot != "" {
				// EvolveDir completes the roots (orchestrator-target
				// deliverables) AND locates the merged catalog for the
				// catalog-aware default.
				roots.EvolveDir = filepath.Join(req.ProjectRoot, ".evolve")
			}
			// CANCELLATION-IMMUNE ladder (WithoutCancel, deliberate): on THIS path a
			// cancelled ctx is frequently the CAUSE of the teardown, not a reason to stop
			// waiting — the tmux driver, on ctx.Err(), takes one final completion poll and
			// otherwise exits ExitArtifactTimeout, "laundering a finished session into a
			// timeout … the runner's settle-retry was the only thing standing between that
			// mislabel and a false FAIL" (driver_tmux_repl.go). Honoring cancellation here
			// would re-open the cycles-824/825 false-FAIL class the ladder exists for, and
			// it would buy nothing in the standing fleet mode (a lane's `evolve cycle run`
			// passes context.Background()). The ctx-aware bail lives on the clean-exit path
			// below, where the agent exited 0 and nothing more is coming.
			res, verr := b.verifyReconcileDeliverable(context.WithoutCancel(ctx), phase, roots)
			switch {
			case verr == nil && res.OK:
				// Deliverable survived the timeout — fall through to Classify.
				reconciled = true
				reconciledRes = res
			case b.optional:
				// Optional-phase soft-fail (Workstream D / cycle-120): no
				// trustworthy deliverable, but an optional phase's successor is
				// verdict-unconditional, so degrade to WARN and let the cycle
				// advance instead of aborting.
				msg := fmt.Sprintf("optional phase %q degraded: no trustworthy deliverable after a bridge infra teardown (%v); cycle continues", phase, bridgeErr)
				if verr != nil {
					msg = fmt.Sprintf("%s [deliverable unverifiable: %v]", msg, verr)
				}
				return core.PhaseResponse{
					Phase:        phase,
					Verdict:      core.VerdictWARN,
					ArtifactsDir: req.Workspace,
					CostUSD:      bres.CostUSD,
					Tokens:       bres.Tokens,
					DurationMS:   durationMS,
					BootMS:       bres.BootMS,
					Diagnostics: []core.Diagnostic{{
						Severity: "warning",
						Message:  msg,
					}},
				}, nil
			default:
				// ACS DETERMINISTIC FLOOR (verdict-incoherence family: cycles
				// 603/921/924/931/3). The teardown-time deliverable.Verify can return
				// not-OK on a report that standalone-verifies OK — a session-lifecycle
				// artifact (the agent wrote a valid report, then idled without the
				// `evolve phase verify` handshake until ctx-cancel), NOT a defect. Before
				// discarding a possibly-shippable cycle as FAIL, consult the NON-LLM ground
				// truth a stall cannot corrupt: the acssuite verdict. When it is PASS AND
				// the report carries THIS cycle's challenge token with a PASS sentinel
				// (anti-gaming: a forged/stale report fails the token), the phase genuinely
				// passed — reconcile to the agent's own report via the same Classify path
				// the clean exit uses. This is exactly the (audit==PASS && acs==PASS)
				// condition the ADR-0072 coherence floor flags as incoherent, prevented at
				// the source instead of halting after the fact.
				// The verified snapshot is BOTH the rescue's evidence and (below) the
				// classified content. When the probe produced no bytes — an infra read
				// fault returns an empty Result, and an artifact that appeared after the
				// last probe verifies absent — fall back to ONE disk read and adopt those
				// bytes into the snapshot, so the rescue keeps the liveness it had before
				// the single-read change AND the rescue and the classification still judge
				// the same bytes (a rescue on content Classify never sees would FAIL anyway).
				// Setting ArtifactPath alongside Content keeps the snapshot
				// self-consistent (these bytes, and where they came from) so the
				// classify step below reuses them instead of reading a third time.
				// It does widen the field from "the path Verify judged" to "the
				// path these bytes came from" — safe here because audit has no
				// dispatch/contract path skew, and scoped to this rescue branch.
				if res.Content == "" {
					if data, readErr := os.ReadFile(artifactPath); readErr == nil {
						res.ArtifactPath, res.Content = artifactPath, string(data)
					}
				}
				if acsFloorRescues(phase, req.Workspace, res.Content) {
					reconciled = true
					reconciledRes = res
					acsFloorRescued = true
					acsFloorOverriddenCodes = forensicCodes(res.Violations)
					log.Diag().Infof("[runner] ACS-FLOOR phase=%s: teardown Verify not-OK (codes=[%s]) but the acssuite verdict is ship-eligible and the report carries this cycle's challenge token with a PASS sentinel — reconciled to the deterministic verdict\n", phase, acsFloorOverriddenCodes)
					break
				}
				// Mandatory phase, no trustworthy deliverable (absent/malformed/
				// unverifiable): hard-fail as before, enriched with the
				// well-formedness violation when we have one.
				msg := bridgeErr.Error()
				if verr == nil && len(res.Violations) > 0 {
					msg = fmt.Sprintf("%s; deliverable not trustworthy: %s", msg, res.Violations[0].Message)
				}
				// [VERDICT-FORENSIC] The retro's #1 ask (cycle-3): a teardown default-FAIL
				// today records NO reasoning, so a false-FAIL is a 30-minute forensic dig.
				// Log the roots passed to verifyFn, the violation codes, verr, and the
				// on-disk deterministic state so the next recurrence is one grep.
				log.Diag().Infof("[VERDICT-FORENSIC] teardown-FAIL phase=%s roots{ws=%s wt=%s evolve=%s} verr=%v codes=[%s] report{%s} acs{%s}\n",
					phase, roots.Workspace, roots.Worktree, roots.EvolveDir, verr, forensicCodes(res.Violations),
					forensicSnapshot(artifactPath, 160), forensicSnapshot(filepath.Join(req.Workspace, "acs-verdict.json"), 200))
				return core.PhaseResponse{
					Phase:        phase,
					Verdict:      core.VerdictFAIL,
					ArtifactsDir: req.Workspace,
					CostUSD:      bres.CostUSD,
					Tokens:       bres.Tokens,
					DurationMS:   durationMS,
					BootMS:       bres.BootMS,
					Diagnostics:  []core.Diagnostic{{Severity: "error", Message: msg}},
				}, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
			}
		} else {
			// A substantive bridge error (launch/boot/safety/cost — NEITHER infra
			// sentinel) means the process failed in a way that makes any on-disk
			// output untrustworthy: no shippable work was produced — hard-fail,
			// optional or not, without consulting the deliverable.
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      core.VerdictFAIL,
				ArtifactsDir: req.Workspace,
				CostUSD:      bres.CostUSD,
				Tokens:       bres.Tokens,
				DurationMS:   durationMS,
				BootMS:       bres.BootMS,
				Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
			}, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
		}
	}

	artifact := bres.Stdout
	// VERDICT SOURCE (ADR-0072 coherence). For a phase that HAS a deliverable contract,
	// the on-disk report is the SOLE verdict source — the terminal pane (bres.Stdout) is
	// never classified. The pane is bridge scrollback: it can lose the real verdict
	// sentinel to a TUI `Write` collapse AND carry the Deliverable Contract's own
	// prompt-echoed EXAMPLE sentinels, so classifying it fabricates a verdict the agent
	// never emitted — a real PASS recorded as FAIL (cycle-603, then recurring 877→921
	// ≥10× on one goal_hash). The earlier "prefer the file when it verifies, else fall
	// back to the pane" design left that fabricated verdict reachable under any flush-
	// timing pressure; widening the settle window only lowered the odds. This removes the
	// pane from the contracted-verdict path entirely, so timing can no longer cause
	// incoherence — only latency.
	//
	// SINGLE READ (deliverable-verified-bytes-single-read): the classified bytes ARE the
	// verified bytes. verifyFn returns the content it judged (deliverable.Result.Content)
	// and this block consumes THAT — it never re-reads artifactPath. The earlier
	// verify-then-re-read pair left a window in which a process racing the just-finished
	// launch could swap the file, so the recorded verdict could belong to content no gate
	// ever checked; the invariant is now literal ("the file", not "the file as of the
	// Verify read"). classifiedArtifact owns the one-line decision — including the two
	// cases where the verified bytes are not this artifact's (a NoArtifact contract, and a
	// phase whose dispatched filename differs from its contract's); see its doc.
	//
	//   - verr != nil  → no contract for this phase (or an IO fault): well-formedness is
	//     undeterminable, so the pane/Classify remains the legitimate source. UNCHANGED.
	//   - res.OK       → contracted + well-formed: classify the VERIFIED bytes. Anti-gaming
	//     intact — those bytes passed Verify's challenge-token + section + ADR-0039 checks.
	//   - !res.OK      → contracted + malformed/absent after the settle WAIT: a COHERENT
	//     deliverable-production FAIL. The verified bytes still reach Classify (a phase may
	//     derive a legitimate NON-SHIP verdict from partial content — intent delta's
	//     "[intent-unchanged]" → SKIPPED); an ABSENT deliverable verifies as empty content,
	//     so Classify sees no sentinel → FAIL. The ship-guard below then stops a
	//     verification-FAILED deliverable from laundering a ship-eligible verdict, and the
	//     contract Codes are surfaced as diagnostics.
	//
	// The reconcile-on-teardown path already verified the deliverable above, so it reuses
	// that probe's bytes (reconciledRes) instead of verifying — or reading — again.
	var deliverableViolations []deliverable.Violation
	deliverableUnverified := false
	if reconciled {
		artifact = classifiedArtifact(reconciledRes, artifactPath, artifact)
	} else {
		roots := phasecontract.Roots{Workspace: req.Workspace, Worktree: req.Worktree}
		if req.ProjectRoot != "" {
			roots.EvolveDir = filepath.Join(req.ProjectRoot, ".evolve")
		}
		res, verr := b.verifyReconcileDeliverable(ctx, phase, roots)
		switch {
		case verr != nil:
			// Uncontracted phase (or IO fault): keep the pane as the verdict source.
		default:
			// Contracted phase → classify the verified bytes, never the pane.
			artifact = classifiedArtifact(res, artifactPath, artifact)
			if !res.OK {
				deliverableViolations = res.Violations
				deliverableUnverified = true
			}
		}
	}

	// Best-effort: write the <phase>-stdout.clean.txt companion next to
	// the raw log. Default-on; set Options.DisableStdoutFilter=true to skip.
	// Filter failures NEVER block the phase — they WARN and continue,
	// because the raw log remains the forensic source of truth and
	// cyclecost / phaseobserver still read it directly.
	if !b.disableStdoutFilter {
		if err := b.stdoutFilter(req.Workspace, phase); err != nil {
			log.Diag().Warnf("[runner] WARN stdout filter phase=%s: %v\n", phase, err)
		}
	}

	verdict, diags, nextPhase := b.hooks.Classify(artifact, req, bres)
	if deliverableUnverified {
		// SHIP-GUARD (anti-gaming). A deliverable that FAILED its well-formedness/anti-
		// gaming contract must NEVER launder a CLEAN-ship verdict past the failed contract —
		// the CodeMissingChallengeToken case (a PASS sentinel in a file that never echoed the
		// per-cycle challenge token). PASS is the only clean-ship claim; downgrade it (and any
		// non-canonical verdict) to a coherent FAIL. FAIL/SKIPPED/WARN pass through: FAIL and
		// SKIPPED are non-ship, and WARN is NOT a clean ship — it already flags issues, and
		// whether it ships is an orchestrator policy call (workflow.strict_audit promotes
		// WARN→FAIL there), not the runner's to preempt. Downgrading WARN here would break
		// fluent-mode WARN-ships (TestE2EPipeline_AuditWarn_FluentShips) and regress origin/main,
		// which also ships a failed-verify WARN via the pane. Routing is verdict-driven (cyclerun
		// maps resp.Verdict → FinalVerdict/lastVerdict), so downgrading re-routes without
		// touching nextPhase.
		switch verdict {
		case core.VerdictFAIL, core.VerdictWARN, core.VerdictSKIPPED:
		default:
			verdict = core.VerdictFAIL
		}
	}
	// Surface the contract Codes behind a coherent deliverable-production FAIL, so the
	// retro/operator sees WHY the deliverable was rejected — not a bare verdict with no
	// trail. Only populated when a CONTRACTED deliverable failed verification (the
	// !res.OK branch above); empty otherwise, so this is a no-op on the happy path.
	for _, v := range deliverableViolations {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("deliverable contract violation [%s]: %s", v.Code, v.Message),
		})
	}

	resp := core.PhaseResponse{
		Phase:         phase,
		Verdict:       verdict,
		ArtifactsDir:  req.Workspace,
		NextPhase:     nextPhase,
		CostUSD:       bres.CostUSD,
		Tokens:        bres.Tokens,
		DurationMS:    durationMS,
		BootMS:        bres.BootMS,
		Diagnostics:   diags,
		ModelSource:   modelSource,
		ResolvedModel: resolvedModel,
	}
	if reconciled {
		// A well-formed deliverable on a bridge infra teardown (timeout OR
		// transient) means the phase actually COMPLETED — the teardown was a red
		// herring (the bridge gave up on the wait window, or hit a transient
		// exit, just as, or after, the agent finished writing). So we treat it
		// exactly like a normal completed phase: nil error, the agent's own
		// Classify verdict authoritative. A reconciled FAIL therefore routes as a
		// real code-audit-fail (→ retro), NOT an infra-teardown retry — which is
		// both correct classification and avoids re-running a finished phase.
		// Reconciliation only ever upgrades a synthesized FAIL toward the agent's
		// real verdict; it never invents a PASS (Classify, incl. audit's EGPS
		// red_count gate, still decides).
		resp.Reconciled = true
		reconcileMsg := fmt.Sprintf("bridge infra teardown (%v) but deliverable %s is well-formed; reconciled to %s from the agent's own report", bridgeErr, artifactPath, verdict)
		if acsFloorRescued {
			// The deliverable did NOT pass teardown-time Verify — it was rescued by the
			// deterministic ACS floor. Record that accurately so the trail isn't misleading.
			reconcileMsg = fmt.Sprintf("bridge infra teardown (%v): teardown-time deliverable.Verify returned not-OK, but the acssuite verdict is ship-eligible and %s carries this cycle's challenge token with a PASS sentinel — reconciled to %s via the ACS deterministic floor (verdict-incoherence family)", bridgeErr, artifactPath, verdict)
		}
		resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{
			Severity: "warning",
			Message:  reconcileMsg,
		})
		if acsFloorRescued && acsFloorOverriddenCodes != "" {
			// Surface the OVERRIDDEN well-formedness violations on the RESPONSE (not just the
			// log), so the ACS-floor rescue is never a silent bypass: a hygiene flag such as
			// stray_in_worktree that shipped on the deterministic verdict's authority stays
			// visible to the operator/retro for investigation. Not a downgrade — routing keys
			// on resp.Verdict (PASS); this is an informational trail only.
			resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("ACS floor overrode teardown deliverable.Verify violation(s) [%s] — the deterministic acssuite verdict took precedence; investigate if any is a genuine hygiene regression (e.g. a stray worktree artifact)", acsFloorOverriddenCodes),
			})
		}
		log.Diag().Infof("[runner] RECONCILED phase=%s (%v) verdict=%s deliverable=%s acsFloor=%v\n", phase, bridgeErr, verdict, artifactPath, acsFloorRescued)
	}
	return resp, nil
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
	return b.String()
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

// ComposePrompt exposes the phase's prompt assembly (Hooks.ComposePrompt) as a
// public seam on BaseRunner so a caller can compose a prompt without launching
// the bridge — used by the cache-stable-prefix audit to inspect the static
// prefix for every BaseRunner-based phase. Run uses the same hook internally
// (see below); this method adds reach, not behavior.
func (b *BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string {
	return b.hooks.ComposePrompt(body, req)
}
