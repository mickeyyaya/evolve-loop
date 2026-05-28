package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// CycleRequest is the operator-facing input to RunCycle.
type CycleRequest struct {
	ProjectRoot string
	GoalHash    string
	Budget      BudgetEnvelope
	// Env is propagated to every PhaseRequest.Env that runs in this
	// cycle. Phases consult it for CLI/model selection
	// (EVOLVE_CLI, EVOLVE_<PHASE>_MODEL, …). The orchestrator copies the
	// map so post-RunCycle operator mutation does not affect in-flight
	// or completed runs.
	Env map[string]string
	// Context seeds the PhaseRequest.Context every phase receives. Ship
	// requires Context["commit_message"]; Scout reads
	// Context["strategy"]. Copied like Env.
	Context map[string]string
}

// CycleResult summarises what RunCycle did.
type CycleResult struct {
	Cycle        int
	FinalVerdict string
	PhasesRun    []Phase
	// RetroDecision is the failure-adapter's verdict on the retro branch,
	// populated only when retro ran. Format: "<action>: <reason>".
	RetroDecision string
}

// Orchestrator drives one cycle through the state machine, calling a
// PhaseRunner per phase and appending ledger entries. It is pure: all
// I/O is delegated to the injected Storage and Ledger ports.
//
// This is the Phase 1 skeleton — guards, observer, budget enforcement
// land in Phase 2.
type Orchestrator struct {
	storage Storage
	ledger  Ledger
	runners map[Phase]PhaseRunner
	sm      *StateMachine
	now     func() time.Time
	// gitHEAD returns the current git HEAD SHA. Called once at cycle
	// start and once before finalizing the verdict so the orchestrator
	// can detect whether anything got committed during the cycle (e.g.
	// when the build phase invokes `evolve ship --class manual` inline).
	// Errors are swallowed and treated as "no movement detected" — the
	// outcome calculator falls back to SKIPPED_UNKNOWN.
	gitHEAD func() (string, error)

	// gitDirtyPaths returns the set of modified tracked paths in the main
	// repo's working directory (`git diff --name-only HEAD` in repoRoot).
	// Workstream B's tree-diff guard snapshots this before each source-
	// writing phase and compares after — any newly-dirty MAIN-tree path is a
	// leak that escaped the sandbox (each git worktree is a separate working
	// dir, so its writes don't show up here). Injected for tests.
	gitDirtyPaths func(ctx context.Context, repoRoot string) ([]string, error)

	// worktree provisions/cleans the per-cycle source worktree (ADR-0027).
	// Default gitWorktree (real git); injected in tests via
	// WithWorktreeProvisioner so RunCycle runs without touching real git.
	worktree WorktreeProvisioner

	// cfg + strategy drive dynamic phase routing ("model proposes, kernel
	// disposes"). The zero value (Stage:Off, StaticPreset) reproduces the
	// legacy static-state-machine behavior byte-for-byte: routing is
	// computed only when the composition root opts in via WithRouting with
	// a non-Off stage. The orchestrator never reads a routing flag itself —
	// config.Load (the composition root) is the sole env/file reader.
	cfg      config.RoutingConfig
	strategy router.RoutingStrategy

	// planner produces the upfront whole-cycle plan (ADR-0024 §2). Optional:
	// nil ⇒ no advisor plan ⇒ the kernel floor falls back to the configurable
	// never-skip spine (fail-safe to static). Consulted once at cycle start,
	// only at Stage>=Advisory; its output is clamped to the integrity floor
	// before being threaded into every routing decision.
	planner router.Planner

	// catalog is the merged phase catalog (built-in + user overlays). It lets
	// the orchestrator accept and run user-defined phases on the dynamic-routing
	// path WITHOUT hardcoding them in the Phase enum / state machine. Empty (the
	// default) ⇒ only built-in phases exist ⇒ byte-identical legacy behavior.
	catalog phasespec.Catalog

	// reviewer adjudicates a finished phase's deliverable before the cycle
	// advances (Workstream E2). Nil ⇒ noopReviewer default ⇒ every non-error,
	// non-SKIPPED verdict is recorded as a success (pre-E2 behavior). Set via
	// WithReviewer; the deterministic default + future LLM reviewer
	// implementations share the DeliverableReviewer interface.
	reviewer DeliverableReviewer

	// observer is the per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
	// Start is called once before each runner.Run; the returned cancel runs
	// once after. Nil ⇒ noopObserver default ⇒ byte-identical to the pre-
	// ADR-0030 cycle. Set via WithObserver; cmd_cycle.go wires the real
	// implementation when EVOLVE_OBSERVER_AUTOSPAWN != "0" (default 1).
	observer Observer
}

// Option customizes an Orchestrator at construction (functional-options DI).
// Absent any option, the orchestrator runs in legacy Stage:Off mode.
type Option func(*Orchestrator)

// WithRouting injects the loaded routing config + the strategy selected once
// at the composition root. A nil strategy is ignored so the StaticPreset
// default stands; the orchestrator depends only on the RoutingStrategy
// interface, never on a mode conditional.
func WithRouting(cfg config.RoutingConfig, strategy router.RoutingStrategy) Option {
	return func(o *Orchestrator) {
		o.cfg = cfg
		if strategy != nil {
			o.strategy = strategy
		}
	}
}

// WithPlanner injects the whole-cycle phase planner (ADR-0024 §2 hybrid
// cadence). A nil planner is ignored so the no-plan default stands; the
// orchestrator consults it only at Stage>=Advisory and always clamps its
// output to the integrity floor — "model proposes, kernel disposes".
func WithPlanner(p router.Planner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.planner = p
		}
	}
}

// WithCatalog injects the merged phase catalog so the orchestrator can accept
// and run user-defined (non-built-in) phases on the dynamic-routing path. The
// empty default keeps behavior byte-identical to the built-in-only pipeline.
func WithCatalog(cat phasespec.Catalog) Option {
	return func(o *Orchestrator) { o.catalog = cat }
}

// WithWorktreeProvisioner injects a worktree provisioner. Tests pass a fake to
// avoid real git; nil is ignored so the gitWorktree default stands.
func WithWorktreeProvisioner(p WorktreeProvisioner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.worktree = p
		}
	}
}

// WithObserver injects a per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
// The orchestrator calls observer.Start(...) before each phase's runner.Run
// and the returned cancel after — running a background watcher that emits
// stall_no_output events to the workspace when the subagent's stdout-log
// stops growing. A nil observer (default) keeps the noopObserver default,
// which is byte-identical to the pre-ADR-0030 cycle.
//
// cmd_cycle.go wires the real implementation via
// observer.NewCoreAdapter when EVOLVE_OBSERVER_AUTOSPAWN != "0" (default 1).
func WithObserver(o Observer) Option {
	return func(orch *Orchestrator) {
		if o != nil {
			orch.observer = o
		}
	}
}

// WithReviewer injects a per-phase deliverable reviewer (Workstream E2). The
// orchestrator calls reviewer.Review(...) after each phase's runner.Run returns
// a non-error, non-SKIPPED verdict, BEFORE the ledger append or
// CompletedPhases++. Approve=false aborts the cycle with the reviewer's Reason
// (no retry budget yet — that's a follow-up; see the WS-E plan). A nil reviewer
// keeps the noopReviewer default, which is byte-identical to the pre-E2 cycle.
func WithReviewer(r DeliverableReviewer) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.reviewer = r
		}
	}
}

// NewOrchestrator wires the orchestrator with its dependencies. Routing stays
// off unless a WithRouting option supplies an enabled-stage config.
func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		storage:       storage,
		ledger:        ledger,
		runners:       runners,
		sm:            NewStateMachine(),
		now:           time.Now,
		gitHEAD:       defaultGitHEAD,
		gitDirtyPaths: defaultGitDirtyPaths,
		worktree:      gitWorktree{},
		strategy:      router.StaticPreset{},
		reviewer:      noopReviewer{}, // WS-E2: byte-identical default until WithReviewer is used
		observer:      noopObserver{}, // cycle-122 Fix 3 / ADR-0030: byte-identical default until WithObserver is used
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// archivePollutedWorkspace renames <workspace>/ to
// <workspace>.polluted-<UTCnano>/ when it exists and is non-empty.
// Returns nil for the empty-or-missing case (the cycle just runs in a
// fresh directory). Returns the underlying error only when stat/rename
// actually fails. Tests inject a deterministic clock via now.
func archivePollutedWorkspace(workspace string, now func() time.Time) error {
	info, err := os.Stat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return fmt.Errorf("readdir workspace: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	stamp := now().UTC().Format("20060102T150405.000000000")
	archived := workspace + ".polluted-" + stamp
	if err := os.Rename(workspace, archived); err != nil {
		return fmt.Errorf("rename to %s: %w", archived, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] archived polluted workspace: %s -> %s (%d files)\n",
		workspace, archived, len(entries))
	return nil
}

// defaultGitHEAD runs `git rev-parse HEAD` in cwd.
// Returns empty string on error AND emits a one-line WARN to stderr so
// operators see the degraded-mode signal that yields SKIPPED_UNKNOWN.
// finalizeOutcome treats equal strings as no movement.
func defaultGitHEAD() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN git HEAD probe failed (cycle outcome labels degraded): %v\n", err)
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// defaultGitDirtyPaths runs `git diff --name-only HEAD` in repoRoot and
// returns the list of dirty tracked paths (one per line). Workstream B's
// tree-diff guard uses this as a before/after snapshot — any path that
// becomes dirty during a source-writing phase is a leak that escaped the
// sandbox (each worktree is a separate working dir, so worktree writes don't
// appear here). Errors propagate so the guard can degrade to "snapshot
// missed" rather than misreport leaks.
func defaultGitDirtyPaths(ctx context.Context, repoRoot string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", "--name-only", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only HEAD: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// finalizeOutcome translates SKIPPED into a more specific CycleOutcome label
// using HEAD movement and retro text as signals. PASS/FAIL/WARN pass through.
func (o *Orchestrator) finalizeOutcome(lastPhaseVerdict, retroDecision, preHEAD, postHEAD string) string {
	if lastPhaseVerdict != VerdictSKIPPED {
		return lastPhaseVerdict
	}
	// HEAD moved → something shipped inline (build calling `evolve ship --class manual`).
	if preHEAD != "" && postHEAD != "" && preHEAD != postHEAD {
		return CycleOutcomeShippedViaBuild
	}
	if strings.Contains(retroDecision, "would-have-blocked") {
		return CycleOutcomeSkippedAuditAdvisory
	}
	return CycleOutcomeSkippedUnknown
}

// RunCycle drives one cycle from PhaseStart to PhaseEnd, returning a
// summary of what ran. The lock is acquired up front and released on
// every exit path. State is updated incrementally so a crash leaves an
// inspectable trail in .evolve/.
func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (CycleResult, error) {
	release, err := o.storage.AcquireLock(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = release() }()

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read state: %w", err)
	}
	cycle := state.LastCycleNumber + 1

	startedAt := o.now().UTC().Format(time.RFC3339)
	// IntentRequired is the gate for the start→intent vs start→scout
	// edge. Source priority: explicit Context["intent_required"]=="true"
	// from the caller > env EVOLVE_REQUIRE_INTENT=="1" > false. This
	// mirrors the bash dispatcher's check at run-cycle.sh:build_context.
	intentRequired := req.Context["intent_required"] == "true" ||
		req.Env["EVOLVE_REQUIRE_INTENT"] == "1"
	cs := CycleState{
		CycleID:        cycle,
		Phase:          string(PhaseStart),
		StartedAt:      startedAt,
		PhaseStartedAt: startedAt,
		WorkspacePath:  fmt.Sprintf("%s/.evolve/runs/cycle-%d", req.ProjectRoot, cycle),
		IntentRequired: intentRequired,
	}
	// Guard against workspace pollution from a prior killed attempt at
	// the same cycle number. If `<workspace>/` exists and has files,
	// rename to `<workspace>.polluted-<UTCnano>/` BEFORE any phase runs.
	// Without this, leftover scout-report.md / build-report.md from the
	// killed attempt cause Scout to short-circuit (read pre-existing
	// artifacts in seconds instead of redoing discovery) and steer
	// downstream phases via the OLD task selection.
	// Source incident: cycle-108 meta-loop attempts 1-4 (2026-05-26).
	// Opt-out via EVOLVE_DISABLE_WORKSPACE_GUARD=1 — used by tests that
	// pre-seed workspace files to simulate phase state.
	if req.Env["EVOLVE_DISABLE_WORKSPACE_GUARD"] != "1" && os.Getenv("EVOLVE_DISABLE_WORKSPACE_GUARD") != "1" {
		if err := archivePollutedWorkspace(cs.WorkspacePath, o.now); err != nil {
			// Best-effort: WARN but don't block the cycle; the failure
			// mode it prevents is bad-data steering, not safety.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN workspace archive failed: %v\n", err)
		}
	}
	// Provision the per-cycle source worktree (ADR-0027): tdd/build write code
	// here, isolated from the live tree. cs.ActiveWorktree gates source writes
	// in the role-gate and drives worktree-aware ship. Best-effort — on failure
	// the source phases are denied by the role-gate (loud, not silent). Cleaned
	// up on cycle exit (after ship has merged the worktree→main).
	if wtPath, werr := o.worktree.Create(req.ProjectRoot, cycle); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree provisioning failed (source phases will be blocked): %v\n", werr)
	} else {
		cs.ActiveWorktree = wtPath
		defer func() { _ = o.worktree.Cleanup(req.ProjectRoot, wtPath) }()
	}
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		return CycleResult{}, fmt.Errorf("init cycle-state: %w", err)
	}

	// One snapshot per cycle — operator mutation post-call must not
	// retroactively change what phases saw.
	envSnap := make(map[string]string, len(req.Env))
	for k, v := range req.Env {
		envSnap[k] = v
	}
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	// PR 6 (cycle-135 followup): mint the cycle's challenge token here —
	// ONCE per cycle, at orchestrator start, BEFORE any phase runs. Surface
	// it to every phase via Context["challengeToken"] (scout's ComposePrompt
	// reads it at scout.go:64) AND persist it to <workspace>/challenge-
	// token.txt so the agent-templates.md PR 5 fallback source is populated.
	// Pre-PR-6, no Go code injected the token; scout invented its own
	// (cycle 134 audit C1: "no-token-manual-run-cycle-134"; cycle 135 audit
	// C1: scout minted `59576594e2e8d5c3` instead of using `5b96ecb69a0c848f`
	// from challenge-token.txt). The mint is the same 8-byte-hex shape as
	// bridge.defaultChallengeToken so post-cycle ledger entries are
	// indistinguishable from the bridge-minted ones used pre-cycle-135.
	if _, alreadySet := ctxSnap["challengeToken"]; !alreadySet {
		var tokBytes [8]byte
		if _, err := rand.Read(tokBytes[:]); err == nil {
			tok := hex.EncodeToString(tokBytes[:])
			ctxSnap["challengeToken"] = tok
			// Best-effort workspace write — phase agents per agent-templates.md
			// PR 5 read this as fallback source #2 when inputs.challengeToken
			// is empty. Failure is logged but not fatal (the Context path is
			// the primary route; phases that can't read the file just rely on
			// Context).
			_ = os.MkdirAll(cs.WorkspacePath, 0o755)
			if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN challenge-token.txt write failed: %v (Context route still works)\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN challenge token mint failed: %v (phase agents will fall back to their own protocol)\n", err)
		}
	}

	// Capture HEAD before any phase so finalizeOutcome can detect mid-cycle commits.
	preCycleHEAD, _ := o.gitHEAD()

	// Upfront whole-cycle plan (ADR-0024 §2). At Stage>=Advisory with a planner,
	// ask the advisor once which phases to run, CLAMP the answer to the integrity
	// floor (ship⇒build∧audit∧tdd), persist it, and thread the clamped plan into
	// every routing decision below. The clamp is the non-bypassable kernel floor:
	// it can only COMPLETE the ship-chain, never weaken it, so a hallucinated or
	// adversarial plan cannot reach ship without a real build+audit. Any
	// failure leaves clampedPlan nil ⇒ routing falls back to the configurable
	// never-skip spine (fail-safe to static). Below Advisory, no plan is computed.
	// This is the SINGLE gate for the upfront plan: Stage>=Advisory (the advisor
	// drives) AND Mode==DynamicLLM (static mode makes no LLM calls) AND a planner
	// is wired. The composition root passes WithPlanner unconditionally; the
	// Mode check lives here so the invariant ("LLM plan iff DynamicLLM+Advisory")
	// has one source of truth rather than two gates that could drift.
	var clampedPlan *router.PhasePlan
	if o.cfg.Stage >= config.StageAdvisory && o.cfg.Mode == config.ModeDynamicLLM && o.planner != nil {
		planIn := router.RouteInput{
			Current:         string(PhaseStart),
			Signals:         router.RoutingSignals{}, // no handoffs exist yet at cycle start
			Cfg:             o.cfg,
			BudgetRemaining: req.Budget.MaxUSD,
			Now:             o.now(),
			Workspace:       cs.WorkspacePath,
			ProjectRoot:     req.ProjectRoot,
			Cycle:           cycle,
			Env:             envSnap,
		}
		// ClampPlanToFloor's tddPinned reads planIn.Signals, empty here (no
		// handoffs yet) — cycle_size!="trivial" evaluates true, so tdd is pinned on
		// the conservative (more-mandatory) side at plan time.
		if raw, perr := o.planner.Plan(planIn); perr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase advisor Plan failed (degrading to static spine): %v\n", perr)
		} else if raw != nil {
			var clamps []router.Clamp
			clampedPlan, clamps = router.ClampPlanToFloor(planIn, raw)
			o.recordPhasePlan(ctx, cycle, cs, clampedPlan, clamps)
		}
	}

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}
	current := PhaseStart
	lastVerdict := VerdictPASS
	// scheduledNext, when non-empty, overrides the state machine for
	// the next iteration. Set by the retro branch to inject the
	// failure-adapter's decision.
	var scheduledNext Phase
	// routingSeq names the per-cycle routing-decision artifacts
	// (routing-decision-<seq>.json). Incremented only when routing runs.
	routingSeq := 0

	// Bounded loop guards against any transition-table cycle bug.
	for safety := 0; safety < 32; safety++ {
		var next Phase
		switch {
		case scheduledNext != "":
			next = scheduledNext
			scheduledNext = ""
		case current == PhaseStart:
			// First edge is gated by intent_required, not by verdict.
			next = o.sm.NextFromStart(cs.IntentRequired)
		case !current.IsValid():
			// current is a user-defined phase (only reachable when dynamic
			// routing selected it). The static successor is simply the next
			// entry in the configured order; the routing block below refines it.
			next = o.nextInOrder(current)
		default:
			n, err := o.sm.Next(current, lastVerdict)
			if err != nil {
				return result, fmt.Errorf("transition from %s: %w", current, err)
			}
			next = n
		}

		// Dynamic routing (shadow → advisory → enforce). Stage:Off — the
		// default — leaves the static state machine fully in control: no
		// digest, no ledger entry, byte-identical to legacy. When enabled,
		// digest the completed handoffs, ask the Strategy for a decision,
		// record it forensically, and — at Advisory and above — let the clamped
		// decision override the static successor, re-validated against the
		// legality oracle (CanTransition) AND the artifact-backed spine gate
		// (SpineSatisfiedUpTo). Retro keeps its failure-adapter shim
		// (decideAfterRetro) while routing is bedded in, so routing never
		// overrides the retro branch. The configurable mandatory set
		// (cfg.Mandatory) decides which phases are never-skip; the integrity
		// floor (ClampPlanToFloor, applied to the upfront plan) decides what ship
		// still requires regardless of how small the operator makes that set.
		if o.cfg.Stage != config.StageOff {
			routingSeq++
			signals, _ := router.Digest(cs.WorkspacePath, cs.CompletedPhases)
			dec := o.strategy.Decide(router.RouteInput{
				Current:         string(current),
				Verdict:         lastVerdict,
				Signals:         signals,
				History:         entriesFromRecords(state.FailedAt),
				Cfg:             o.cfg,
				BudgetRemaining: req.Budget.MaxUSD,
				Completed:       cs.CompletedPhases,
				Strict:          envSnap["EVOLVE_STRICT_AUDIT"] == "1",
				Now:             o.now(),
				// Proposer context (DynamicLLM only; ignored by pure Route).
				Workspace:   cs.WorkspacePath,
				ProjectRoot: req.ProjectRoot,
				Cycle:       cycle,
				Env:         envSnap,
				// Clamped whole-cycle plan (Stage>=Advisory). nil below Advisory
				// or on planner failure ⇒ shouldRun runs the legacy trigger path.
				Plan: clampedPlan,
			})
			if o.cfg.Stage >= config.StageAdvisory && current != PhaseRetro {
				if forced, ok := o.enforceNext(current, next, signals, dec); ok {
					next = forced
				}
				// Full spine-integrity check on the SELECTED next (static OR
				// override). Fail-open: a missing mandatory-predecessor handoff
				// is a loud WARN recorded in the decision, never a block —
				// Digest is fail-open, so an absent artifact may be a read miss
				// rather than a real gap, and false-blocking a real cycle is
				// worse than surfacing the signal. The override path already
				// declines (blocks) divergent edges that fail this check; here
				// we additionally surface it for the trusted static edge.
				if next != PhaseEnd && !o.sm.SpineSatisfiedUpTo(next, signals, o.cfg) {
					dec.Clamps = append(dec.Clamps, router.Clamp{
						Rule:     "spine-unsatisfied-warn",
						Proposed: string(next),
						Forced:   string(next),
					})
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN spine not satisfied for next=%s (a mandatory predecessor's handoff artifact is missing); proceeding fail-open\n", next)
				}
			}
			o.recordRoutingDecision(ctx, cycle, cs, routingSeq, dec)
		}

		if next == PhaseEnd {
			break
		}

		runner, ok := o.runners[next]
		if !ok {
			return result, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}

		phaseStarted := o.now().UTC()
		cs.Phase = string(next)
		cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		// Source-writing phases (tdd/build) run with cwd=worktree so their
		// code writes land in the isolated worktree the role-gate permits;
		// every other phase writes only its artifact to the absolute workspace.
		phaseWorktree := ""
		if o.worktreePhase(next) {
			phaseWorktree = cs.ActiveWorktree
		}
		// Workstream B: snapshot the main-tree dirty set BEFORE a source-
		// writing phase runs. After it runs we re-snapshot and compare —
		// any newly-dirty MAIN-tree path is a leak that escaped the bridge
		// sandbox (each git worktree is a separate working dir, so its
		// writes don't show up here). The treediff package owns the
		// snapshot/check + SnapshotMissed semantics; the orchestrator just
		// threads it through. Skipped entirely for non-worktree phases.
		var (
			treeGuard      *treediff.Guard
			beforeDirty    []string
			snapshotFailed bool
		)
		if phaseWorktree != "" && o.gitDirtyPaths != nil {
			treeGuard = treediff.New(o.gitDirtyPaths)
			snap, err := treeGuard.Snapshot(ctx, req.ProjectRoot)
			if err != nil {
				snapshotFailed = true
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff pre-phase snapshot failed for %s: %v (sandbox guard degraded; post-phase leak check skipped)\n", next, err)
			} else {
				beforeDirty = snap
			}
		}
		phaseReq := PhaseRequest{
			Cycle:         cycle,
			ProjectRoot:   req.ProjectRoot,
			Workspace:     cs.WorkspacePath,
			Worktree:      phaseWorktree,
			GoalHash:      req.GoalHash,
			Budget:        req.Budget,
			PreviousPhase: string(current),
			Env:           envSnap,
			Context:       ctxSnap,
		}
		// Cycle-122 Fix 3 / ADR-0030: attach the per-phase observer
		// goroutine BEFORE runner.Run and cancel it AFTER. noopObserver
		// (default when WithObserver wasn't used) is byte-identical to
		// the pre-fix cycle. Real implementations spawn a stall detector
		// that watches <workspace>/<agent>-stdout.log and emits stall
		// events to <workspace>/<agent>-observer-events.ndjson.
		obsCancel := o.observer.Start(ctx, string(next), phaseReq)
		resp, err := runner.Run(ctx, phaseReq)
		if obsCancel != nil {
			obsCancel()
		}
		if err != nil {
			return result, fmt.Errorf("phase %s: %w", next, err)
		}
		if !IsVerdict(resp.Verdict) {
			return result, fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
		}

		// Workstream E2: per-phase deliverable review gate. Runs ONLY for
		// non-SKIPPED verdicts (a SKIPPED phase produced no deliverable to
		// review) and BEFORE the tree-diff guard + ledger append, so a reject
		// aborts the cycle without recording the phase as a success. The
		// default reviewer is noopReviewer (every phase approved) so opt-out
		// is byte-identical to pre-E2. Retry/N is a follow-up — today reject
		// = abort.
		if o.reviewer != nil && resp.Verdict != VerdictSKIPPED {
			rin := ReviewInput{
				Phase:       string(next),
				Response:    resp,
				Workspace:   cs.WorkspacePath,
				Worktree:    phaseWorktree,
				ProjectRoot: req.ProjectRoot,
			}
			rr := o.reviewer.Review(ctx, rin)
			if !rr.Approve {
				return result, fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
			}
		}

		// Workstream B: post-phase tree-diff check. Runs BEFORE the ledger
		// append so a leak aborts the cycle without recording the phase as a
		// success. Snapshot failures (pre OR post) degrade silently — the
		// guard is belt-and-suspenders to the OS sandbox, so a transient git
		// read error must never cause a false abort.
		if treeGuard != nil && !snapshotFailed {
			res := treeGuard.Check(ctx, req.ProjectRoot, beforeDirty)
			if res.SnapshotMissed {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff post-phase snapshot failed for %s (sandbox guard degraded; not aborting)\n", next)
			} else if !res.OK() {
				return result, res.Error(string(next), phaseWorktree)
			}
		}

		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:       o.now().UTC().Format(time.RFC3339),
			Cycle:    cycle,
			Role:     string(next),
			Kind:     "phase",
			ExitCode: 0,
		}); err != nil {
			return result, fmt.Errorf("ledger append for %s: %w", next, err)
		}

		cs.CompletedPhases = append(cs.CompletedPhases, string(next))
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state post-%s: %w", next, err)
		}

		result.PhasesRun = append(result.PhasesRun, next)
		result.FinalVerdict = resp.Verdict
		current = next
		lastVerdict = resp.Verdict

		// Retro is the one phase whose successor isn't verdict-driven;
		// the failure-adapter consults cycle history (state.FailedAt) and
		// the retro verdict to pick {ship | tdd | end}. Set scheduledNext
		// so the next loop iteration runs the chosen phase.
		if current == PhaseRetro {
			branch, extraEnv, reason := o.decideAfterRetro(resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			result.RetroDecision = reason
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseRetro, branch) {
				return result, fmt.Errorf("retro→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}
	}

	postCycleHEAD, _ := o.gitHEAD()
	result.FinalVerdict = o.finalizeOutcome(result.FinalVerdict, result.RetroDecision, preCycleHEAD, postCycleHEAD)

	state.LastCycleNumber = cycle
	if err := o.storage.WriteState(ctx, state); err != nil {
		return result, fmt.Errorf("write state: %w", err)
	}
	return result, nil
}

// decideAfterRetro consults the failure-adapter over cycle history
// (state.failedApproaches) to pick the post-retro branch.
//
// Mapping (retro verdict × failureadapter action → next phase):
//   - retro PASS               → ship   (retrospective recovered the cycle)
//   - retro FAIL/WARN + BLOCK-* → end    (cycle history forbids further work)
//   - retro FAIL/WARN + RETRY  → tdd    (retry from earlier phase w/ fallback env)
//   - retro FAIL/WARN + PROCEED → end   (no recovery, no block — exit cleanly)
//
// Returned reason is "<action>: <failureadapter reason>" for the
// CycleResult.RetroDecision audit field.
func (o *Orchestrator) decideAfterRetro(retroVerdict string, history []FailedRecord) (next Phase, extraEnv map[string]string, reason string) {
	// retro PASS → ship; no failureadapter consultation.
	if retroVerdict == VerdictPASS {
		return PhaseShip, nil, "retro-recovered: ship"
	}
	entries := entriesFromRecords(history)
	dec := failureadapter.Decide(entries, failureadapter.Options{Now: o.now()})
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		return PhaseTDD, dec.SetEnv, "retry-with-fallback: " + dec.Reason
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		return PhaseEnd, nil, string(dec.Action) + ": " + dec.Reason
	default: // ActionProceed
		return PhaseEnd, dec.SetEnv, "proceed: " + dec.Reason
	}
}

// recordRoutingDecision marshals the RouterDecision to
// <workspace>/routing-decision-<seq>.json and appends a hash-bound
// routing_decision ledger entry, plus one phase_skipped entry per declined
// optional phase (preserving the PSMAS resume/audit-binding contract).
//
// Best-effort: a marshal/write/append failure WARNs and is swallowed —
// routing forensics must never abort a cycle. Called only when Stage != Off,
// so the legacy path appends nothing new.
func (o *Orchestrator) recordRoutingDecision(ctx context.Context, cycle int, cs CycleState, seq int, dec router.RouterDecision) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, fmt.Sprintf("routing-decision-%d.json", seq))
	sha := ""
	if buf, err := json.MarshalIndent(dec, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}

	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "orchestrator", Kind: "routing_decision",
		ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing_decision ledger append: %v\n", err)
	}
	for _, sp := range dec.SkipPhases {
		if err := o.ledger.Append(ctx, LedgerEntry{
			TS: ts, Cycle: cycle, Role: sp, Kind: "phase_skipped", ExitCode: 0,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_skipped ledger append: %v\n", err)
		}
	}
}

// recordPhasePlan persists the advisor's CLAMPED whole-cycle plan to
// <workspace>/phase-plan.json (a bare PhasePlanEntry array, symmetric with the
// advisor's wire format) and appends a hash-bound phase_plan ledger entry. Any
// integrity-floor clamps that fired are logged for operator visibility (rich
// per-clamp forensics land in a later slice). Best-effort: a marshal/write/
// append failure WARNs and is swallowed — plan forensics must never abort a
// cycle. Called once per cycle, only at Stage>=Advisory with a non-nil plan.
func (o *Orchestrator) recordPhasePlan(ctx context.Context, cycle int, cs CycleState, plan *router.PhasePlan, clamps []router.Clamp) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, "phase-plan.json")
	sha := ""
	if buf, err := json.MarshalIndent(plan.Entries, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-plan marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-plan mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-plan write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}
	for _, c := range clamps {
		fmt.Fprintf(os.Stderr, "[orchestrator] integrity-floor clamp: %s (%s → %s)\n", c.Rule, c.Proposed, c.Forced)
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "orchestrator", Kind: "phase_plan",
		ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_plan ledger append: %v\n", err)
	}
}

// enforceNext maps the router's proposed NextPhase back to a core.Phase and
// returns it ONLY if it differs from the static successor AND survives both
// kernel gates: a legal edge (CanTransition) and the artifact-backed spine
// gate (SpineSatisfiedUpTo). Otherwise the static successor stands. This is
// the non-bypassable "kernel disposes" floor for Enforce mode — neither
// Strategy can reach Ship without a real PASS/WARN audit artifact.
func (o *Orchestrator) enforceNext(current, staticNext Phase, sig router.RoutingSignals, dec router.RouterDecision) (Phase, bool) {
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == staticNext {
		return staticNext, false
	}
	if !o.transitionLegal(current, cand) {
		return staticNext, false
	}
	if !o.sm.SpineSatisfiedUpTo(cand, sig, o.cfg) {
		return staticNext, false
	}
	return cand, true
}

// candidatePhase resolves a router-proposed phase string to a runnable Phase:
// a built-in (via phaseFromRouter) OR a user phase present in the catalog. An
// unknown string yields "" so enforceNext declines it.
func (o *Orchestrator) candidatePhase(s string) Phase {
	if p := phaseFromRouter(s); p != "" {
		return p
	}
	if _, ok := o.catalog.Get(s); ok {
		return Phase(s)
	}
	return ""
}

// transitionLegal is the kernel legality gate for a proposed edge. Built-in
// phases use the hardcoded state-machine graph. A user phase (optional,
// catalog-defined) is legal iff it makes forward progress in the configured
// order (cfg.Order) — the router only proposes the next runnable optional, and
// SpineSatisfiedUpTo independently guards the mandatory anchors, so an optional
// insertion between anchors cannot skip the spine or reach ship illegitimately.
func (o *Orchestrator) transitionLegal(from, cand Phase) bool {
	if from.IsValid() && cand.IsValid() {
		return o.sm.CanTransition(from, cand) // both built-in: hardcoded graph
	}
	// At least one endpoint is NOT a built-in phase — validate via order
	// forward-progress (both-built-in edges took the sm.CanTransition branch above).
	// A user-phase candidate must be optional (the floor). Leapfrogging a
	// mandatory anchor is independently blocked by SpineSatisfiedUpTo in the caller.
	if !cand.IsValid() {
		spec, ok := o.catalog.Get(string(cand))
		if !ok || !spec.Optional {
			return false
		}
	}
	ci, fi := orderIndex(o.cfg.Order, string(cand)), orderIndex(o.cfg.Order, string(from))
	return ci >= 0 && fi >= 0 && ci > fi
}

// nextInOrder returns the phase immediately following p in the configured
// order, or PhaseEnd when p is last/absent. Used to resume the normal sequence
// after a user phase runs. Assumes cfg.Order is the complete registry order
// (applyRegistry appends every registry phase), so a built-in successor is
// always present when a registry is loaded.
func (o *Orchestrator) nextInOrder(p Phase) Phase {
	i := orderIndex(o.cfg.Order, string(p))
	if i < 0 || i+1 >= len(o.cfg.Order) {
		return PhaseEnd
	}
	return Phase(o.cfg.Order[i+1])
}

// orderIndex returns the position of phase in order, or -1 if absent.
func orderIndex(order []string, phase string) int {
	for i, p := range order {
		if p == phase {
			return i
		}
	}
	return -1
}

// worktreePhase reports whether next writes source and so must run with
// cwd=worktree. Built-in tdd/build always do; a user phase does iff its spec
// sets writes_source. Method form (vs the free WorktreePhase) so it consults
// the injected catalog.
func (o *Orchestrator) worktreePhase(p Phase) bool {
	if WorktreePhase(p) {
		return true
	}
	if spec, ok := o.catalog.Get(string(p)); ok {
		return spec.WritesSource
	}
	return false
}

// phaseFromRouter denormalizes a router phase string back to a core.Phase.
// The router speaks canonical "retrospective"/"end"; core uses "retro"/
// PhaseEnd. An unknown string yields "" so enforceNext declines it.
func phaseFromRouter(s string) Phase {
	switch s {
	case "retrospective":
		return PhaseRetro
	case router.PhaseEnd: // "end" — same string as core.PhaseEnd
		return PhaseEnd
	}
	p := Phase(s)
	if !p.IsValid() {
		return ""
	}
	return p
}

// entriesFromRecords converts FailedRecord values into failureadapter.Entry.
// Inlined here (rather than exposed from failureadapter) to avoid a
// circular import between core and failureadapter.
func entriesFromRecords(records []FailedRecord) []failureadapter.Entry {
	out := make([]failureadapter.Entry, len(records))
	for i, r := range records {
		out[i] = failureadapter.Entry{
			TS:                r.TS,
			Cycle:             r.Cycle,
			Verdict:           r.Verdict,
			Classification:    failureadapter.Classification(r.Classification),
			RecordedAt:        r.RecordedAt,
			ExpiresAt:         r.ExpiresAt,
			AuditReportPath:   r.AuditReportPath,
			AuditReportSHA256: r.AuditReportSHA256,
			GitHead:           r.GitHead,
			TreeStateSHA:      r.TreeStateSHA,
			Defects:           r.Defects,
			Retrospected:      r.Retrospected,
			Summary:           r.Summary,
		}
	}
	return out
}
